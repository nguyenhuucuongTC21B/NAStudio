package cloud

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PATCH FIX57: voiceNotAvailErr đánh dấu lỗi TẠM THỜI "giọng chưa được
// worker nạp" (vieneu.io demo 400 + "is not available"). Dịch vụ CÒN SỐNG
// — chỉ giọng này chưa có lúc đó (tập giọng demo xoay vòng) — nên chuỗi
// KHÔNG được đặt cooldown cho dịch vụ (lỗi khác ví 429 vẫn cooldown như cũ).
type voiceNotAvailErr struct{ err error }

func (e voiceNotAvailErr) Error() string { return e.err.Error() }
func (e voiceNotAvailErr) Unwrap() error { return e.err }

// IsVoiceNotAvailable kiểm tra lỗi có phải dạng "giọng tạm chưa có" không.
func IsVoiceNotAvailable(err error) bool {
	var v voiceNotAvailErr
	return errors.As(err, &v)
}

// httpx client dùng chung — User-Agent tử tế, không cache.
func newHTTP() *http.Client {
	return &http.Client{Timeout: 0} // timeout do context quản lý từng bước
}

func doGet(ctx context.Context, hc *http.Client, url string, timeout time.Duration) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := hc.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxAudioBytes))
	return b, resp.StatusCode, err
}

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) HCStudio-v5-FIX53/1.0"
const maxAudioBytes = 256 << 20 // 256 MB — chặn tải bất thường

// ---------- vieneu.io ----------

// synthVieneuIO gọi POST /api/tts/demo {text, voiceId} → {audioBase64, mimeType}.
// Đây chính xác cách trang chủ vieneu.io gọi chính nó (đọc từ bundle JS
// index-Bg2KJ1AF.js ngày 2026-09-20).
func synthVieneuIO(ctx context.Context, hc *http.Client, d Desc, r Request) (Result, error) {
	voice, changed := d.ResolveVoice(r.VoiceName)
	note := ""
	if changed && r.VoiceName != "" {
		note = fmt.Sprintf(" · giọng %q không có trên dịch vụ này — dùng mặc định %q", r.VoiceName, voice)
	}
	payload, _ := json.Marshal(map[string]string{"text": r.Text, "voiceId": voice})
	ctx, cancel := context.WithTimeout(ctx, AttemptTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.Base+d.API, strings.NewReader(string(payload)))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := hc.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	// PATCH FIX54: vieneu.io thực tế trả HTTP 201 (Created) kèm audioBase64
	// hợp lệ (probe 2026-09-20 — FIX53 chỉ nhận 200 nên luôn bỏ dịch vụ đầu
	// tiên của chuỗi). Chấp nhận toàn bộ dải 2xx.
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// PATCH FIX56: demo vieneu.io từ chối giọng không nằm trong
		// nhóm đang được worker nạp (400 + "Voice … is not available")
		// — đây là trạng thái TẠM THỜI theo thời điểm (probe 56f/56i:
		// cùng giọng 5 phút trước còn OK). Thông điệp rõ ràng để UI/
		// người dùng hiểu là nhảy dịch vụ, không phải hỏng vĩnh viễn.
		if resp.StatusCode == http.StatusBadRequest && strings.Contains(string(body), "is not available") {
			// PATCH FIX57: bọc sentinel — chuỗi sẽ KHÔNG cooldown dịch vụ này.
			return Result{}, voiceNotAvailErr{fmt.Errorf("%s: giọng %q chưa có trên demo lúc này (tập giọng demo thay đổi theo thời điểm) — tự động chuyển dịch vụ kế tiếp", d.Label, voice)}
		}
		return Result{}, classifyHTTP(resp.StatusCode, string(body), d)
	}
	var out struct {
		AudioBase64 string `json:"audioBase64"`
		MimeType    string `json:"mimeType"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return Result{}, fmt.Errorf("vieneu.io trả dữ liệu lạ: %w", err)
	}
	audio, err := base64.StdEncoding.DecodeString(out.AudioBase64)
	if err != nil || len(audio) == 0 {
		return Result{}, fmt.Errorf("vieneu.io trả audio rỗng")
	}
	return Result{
		Audio: audio, MIME: out.MimeType,
		ProviderID: d.ID, ProviderLabel: d.Label, VoiceUsed: voice,
		Info: fmt.Sprintf("voiceId=%s%s", voice, note),
	}, nil
}

// ---------- gradio ----------

type gradioEvent struct {
	name string
	data string
}

// synthGradio gọi 1 Space gradio 5.x:
//  1. POST {base}/gradio_api/call/{api}  {"data":[…]} → {"event_id"}
//     (fallback /call/{api} cho cấu hình cũ)
//  2. GET  {base}/gradio_api/call/{api}/{event_id} → SSE
//     heartbeat / process_starts / process_generating / complete / error
//  3. complete → out[0].url → GET tải audio.
func synthGradio(ctx context.Context, hc *http.Client, d Desc, r Request, onEv func(gradioEvent)) (Result, error) {
	voice, changed := d.ResolveVoice(r.VoiceName)
	infoNote := ""
	if changed && r.VoiceName != "" {
		infoNote = fmt.Sprintf(" · giọng %q không có trên space này — dùng mặc định %q", r.VoiceName, voice)
	}
	data := gradioData(d, r.Text, voice)

	body, code, err := postJSON(ctx, hc, d.Base+"/gradio_api/call/"+d.API, map[string]any{"data": data})
	if code == 404 || (err != nil && code == 0) {
		// cấu hình gradio cũ hơn không có tiền tố /gradio_api
		body, code, err = postJSON(ctx, hc, d.Base+"/call/"+d.API, map[string]any{"data": data})
	}
	if err != nil {
		return Result{}, err
	}
	if code == http.StatusServiceUnavailable || code == http.StatusBadGateway || code == 504 {
		return Result{}, &wakeErr{code: code}
	}
	if code != http.StatusOK {
		return Result{}, classifyHTTP(code, body, d)
	}
	var eid struct {
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal([]byte(body), &eid); err != nil || eid.EventID == "" {
		return Result{}, fmt.Errorf("space không cấp event_id (HTTP %d): %.120s", code, body)
	}

	base := d.Base + "/gradio_api/call/" + d.API + "/" + eid.EventID
	sseCtx, cancel := context.WithTimeout(ctx, AttemptTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(sseCtx, http.MethodGet, base, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := hc.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	var audioURL, info, errMsg string
	parseSSE(resp.Body, func(ev gradioEvent) {
		if onEv != nil {
			onEv(ev)
		}
		switch ev.name {
		case "error":
			errMsg = strings.TrimSpace(ev.data)
		case "complete":
			arr := parseCompleteData(ev.data)
			if len(arr) > 0 {
				audioURL, info = extractAudioURL(arr[0]), joinInfo(arr, 1)
			}
		}
	})
	if err := sseCtx.Err(); err != nil {
		return Result{}, fmt.Errorf("hết giờ chờ %s (%v)", d.Label, err)
	}
	if audioURL == "" {
		msg := errMsg
		if msg == "" || msg == "null" {
			msg = "space trả lỗi không rõ nội dung"
		}
		return Result{}, classifyMessage(msg, d)
	}
	audio, code, err := doGet(ctx, hc, audioURL, 3*time.Minute)
	if err != nil {
		return Result{}, fmt.Errorf("tải audio từ space lỗi: %w", err)
	}
	if code != http.StatusOK || len(audio) == 0 {
		return Result{}, fmt.Errorf("tải audio HTTP %d", code)
	}
	// PATCH FIX56: MIME theo phần mở rộng thật của URL — DevTam05 trả
	// .mp3 (không phải .wav như các space template); ghi sai MIME làm
	// bước giải mã sau này chọn nhầm đường đọc.
	mime := "audio/wav"
	if strings.Contains(strings.ToLower(audioURL), ".mp3") {
		mime = "audio/mpeg"
	}
	return Result{
		Audio: audio, MIME: mime,
		ProviderID: d.ID, ProviderLabel: d.Label, VoiceUsed: voice,
		Info: strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(info+infoNote), "·")),
	}, nil
}

// gradioData dựng mảng "data" theo kiểu dịch vụ — GIỮ ĐÚNG THỨ TỰ input
// đã khảo sát từ /config của từng space (scripts/f53_probe_deep.py):
//   - template: [text, voice, refAudio(nil), temperature, top_k, top_p,
//     repetition_penalty, max_new_frames, max_chars] — đúng default của UI.
//   - smrfhdl:  [text, voice, mode("cpu"), browserstate(nil)] — mode CPU
//     KHÔNG tốn hạn mức GPU 300 giây/ngày của space này.
func gradioData(d Desc, text, voice string) []any {
	// PATCH FIX54: thêm 2 dạng khảo sát được từ các Space CPU (không GPU
	// quota) — speech2: [text, voice]; speech5: [text, voice, refAudio,
	// customText, thamSốThứ5] (các phần sau nil).
	switch d.DataStyle {
	case "smrfhdl":
		return []any{text, voice, "cpu", nil}
	case "speech2":
		return []any{text, voice}
	case "speech5":
		return []any{text, voice, nil, nil, nil}
	}
	return []any{text, voice, nil, 0.8, 25, 0.95, 1.2, 300, 256}
}

func postJSON(ctx context.Context, hc *http.Client, url string, payload any) (string, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(b)))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := hc.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	return string(body), resp.StatusCode, nil
}

// parseSSE đọc stream SSE thô, gọi callback cho từng event khớp dạng
// "event: <tên>\ndata: <một dòng>".
func parseSSE(r io.Reader, cb func(gradioEvent)) {
	buf := make([]byte, 0, 8192)
	tmp := make([]byte, 4096)
	curName, curData := "", ""
	flush := func() {
		if curName != "" {
			cb(gradioEvent{name: curName, data: curData})
		}
		curName, curData = "", ""
	}
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			for {
				idx := indexByteSlice(buf, '\n')
				if idx < 0 {
					break
				}
				line := string(buf[:idx])
				buf = buf[idx+1:]
				switch {
				case strings.HasPrefix(line, "event:"):
					if curName != "" {
						flush()
					}
					curName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				case strings.HasPrefix(line, "data:"):
					curData = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				case line == "":
					flush()
				}
			}
		}
		if err != nil {
			flush()
			return
		}
	}
}

func indexByteSlice(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}

// parseCompleteData an toàn: complete.data là JSON array (string) hoặc null.
func parseCompleteData(s string) []any {
	if s == "" || s == "null" {
		return nil
	}
	var arr []any
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return nil
	}
	return arr
}

// extractAudioURL lấy url từ object gradio.FileData.
func extractAudioURL(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	if u, ok := m["url"].(string); ok && u != "" {
		return u
	}
	return ""
}

func joinInfo(arr []any, from int) string {
	var parts []string
	for i := from; i < len(arr); i++ {
		if s, ok := arr[i].(string); ok && strings.TrimSpace(s) != "" {
			parts = append(parts, strings.TrimSpace(s))
		}
	}
	return strings.Join(parts, " · ")
}

// ---------- phân loại lỗi ----------

// wakeErr space đang ngủ/cold-start → thử lại sau khi đợi.
type wakeErr struct{ code int }

func (w *wakeErr) Error() string { return fmt.Sprintf("space đang khởi động (HTTP %d)", w.code) }

// QuotaError phân biệt "hết lượt" với lỗi thường: hết lượt thì dịch vụ đó
// bị đánh dấu cạn ĐẾN HẾT NGÀY, lỗi thường chỉ cooldown 5 phút.
type QuotaError struct{ Msg string }

func (q *QuotaError) Error() string { return q.Msg }

func IsQuota(err error) bool {
	_, ok := err.(*QuotaError)
	return ok
}

func IsWake(err error) bool {
	_, ok := err.(*wakeErr)
	return ok
}

// classifyHTTP phân loại theo mã HTTP + nội dung phản hồi.
func classifyHTTP(code int, body string, d Desc) error {
	msg := truncateRunes(strings.TrimSpace(body), 220)
	if msg == "" {
		msg = fmt.Sprintf("HTTP %d", code)
	}
	if code == http.StatusTooManyRequests {
		return &QuotaError{Msg: fmt.Sprintf("%s: hết lượt (HTTP 429)", d.Label)}
	}
	if code == http.StatusPaymentRequired || code == http.StatusUnauthorized || code == http.StatusForbidden {
		return fmt.Errorf("%s: cần đăng nhập/không cho truy cập (HTTP %d)", d.Label, code)
	}
	return classifyMessage(fmt.Sprintf("HTTP %d: %s", code, msg), d)
}

// classifyMessage soi từ khoá hạn mức trong thông điệp lỗi (tiếng Việt +
// tiếng Anh) — đúng cái app cần để biết "hết lượt" mà chuyển tiếp.
func classifyMessage(msg string, d Desc) error {
	low := strings.ToLower(msg)
	for _, kw := range []string{
		"quota", "hạn mức", "han muc", "giới hạn", "gioi han",
		"exceeded", "exhausted", "too many requests", "rate limit", "ratelimit",
		"hết lượt", "het luot", "out of credits", "gpu",
	} {
		if strings.Contains(low, kw) {
			return &QuotaError{Msg: fmt.Sprintf("%s: %s", d.Label, truncateRunes(msg, 180))}
		}
	}
	return fmt.Errorf("%s: %s", d.Label, truncateRunes(msg, 180))
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
