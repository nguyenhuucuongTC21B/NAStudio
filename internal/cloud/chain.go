package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"hcstudio/internal/dsp"
)

// PState trạng thái runtime của 1 dịch vụ trong chuỗi.
type PState struct {
	Status         string `json:"status"` // unknown|ok|err|quota|skip
	Err            string `json:"err,omitempty"`
	ExhaustedUntil string `json:"exhaustedUntil,omitempty"` // "2006-01-02" — cạn tới hết ngày
	CountToday     int    `json:"countToday,omitempty"`
	Day            string `json:"day,omitempty"`
	CooldownUntil  int64  `json:"cooldownUntil,omitempty"` // unix — lỗi thường, tránh 5 phút
	LastOKAt       string `json:"lastOKAt,omitempty"`
}

// Snapshot toàn bộ trạng thái chuỗi — persist vào settings để lần mở app
// sau vẫn nhớ dịch vụ nào cạn lượt hôm nay.
type Snapshot struct {
	Providers map[string]*PState `json:"providers"`
	LastGood  string             `json:"lastGood,omitempty"`
}

// badAudioErr — PATCH FIX59: sentinel audio RÁC từ dịch vụ (nhiễu trắng
// vô nghĩa hoặc byte không decode được). Đây là THẤT BẠI của dịch vụ đó:
// chuỗi phải nhảy dịch vụ kế tiếp thay vì giao tiếng rè cho người dùng.
// Không rơi vào cooldown "giọng chưa có" và KHÔNG được thử lại ở lượt 2
// (model hỏng trên server là lỗi kéo dài, thử lại chỉ tốn thời gian).
type badAudioErr struct{ msg string }

func (e *badAudioErr) Error() string { return e.msg }

// IsBadAudio kiểm tra lỗi có phải dạng "audio rè/rác" không.
func IsBadAudio(err error) bool {
	var b *badAudioErr
	return errors.As(err, &b)
}

// verifyAudio (PATCH FIX59) — decode + lọc chất lượng NGAY trong chuỗi.
// Bằng chứng thủ phạm: probe60 11:02 2026-09-21 — nguyenduc1222 trả WAV
// hợp lệ 24kHz/3.0s nhưng nội dung nhiễu trắng thuần (rms_cv 0.01,
// pause 0%, zcr 0.50; giọng thật: 0.67-0.99 / 20-41% / 0.05-0.11).
// Audio qua vòng lọc được ghi Samples/SR vào Result để tái sử dụng.
func verifyAudio(res *Result) error {
	samples, sr, _, err := dsp.ReadAudioBytes(res.Audio, ".wav")
	if err != nil {
		return &badAudioErr{msg: "trả audio không đọc được (byte rác)"}
	}
	if dsp.LooksLikeNoise(samples, sr) {
		return &badAudioErr{msg: "trả audio rè vô nghĩa (nhiễu trắng — model hỏng trên server)"}
	}
	res.Samples, res.SR = samples, sr
	return nil
}

// Chain chuỗi cầu nối có nhớ trạng thái.
type Chain struct {
	mu    sync.Mutex
	snap  Snapshot
	nowFn func() time.Time // injection cho test
	reg   []Desc           // nil → Registry() (test có thể thay)
}

// NewChain dựng chuỗi từ snapshot đã lưu (JSON rỗng = mới tinh).
func NewChain(savedJSON string) *Chain {
	c := &Chain{nowFn: time.Now}
	c.snap.Providers = map[string]*PState{}
	if savedJSON != "" {
		var s Snapshot
		if err := json.Unmarshal([]byte(savedJSON), &s); err == nil && s.Providers != nil {
			c.snap = s
			if c.snap.Providers == nil {
				c.snap.Providers = map[string]*PState{}
			}
		}
	}
	return c
}

// SnapshotJSON xuất trạng thái để persist.
func (c *Chain) SnapshotJSON() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, _ := json.Marshal(c.snap)
	return string(b)
}

// BeginUserRun — PATCH FIX57: người dùng vừa CHỦ ĐỘNG bấm "Chuyển đổi"
// hoặc nút "Làm mới" → xóa toàn bộ trạng thái tạm (cooldown 5 phút + cạn
// hạn mức) để chuỗi LUÔN gọi thử thật ít nhất 1 lần mỗi dịch vụ trong run
// này. Nguyên nhân: probe 2026-09-21 05:01 chứng minh các space ZeroGPU
// chập chờn THEO CƠN (pnnbao-ump chết lúc 04:04, sống lại 05:01; cùng 1
// space cùng phút — Mai Anh complete 2,8s trong khi Quang Sơn error) nên
// trạng thái "vừa lỗi" từ lần chạy trước KHÔNG đáng tin để bỏ qua. Không
// xóa LastGood (vẫn ưu tiên dịch vụ thành công gần nhất), không đụng bộ
// đếm ngày. Hết lượt thật (429) sẽ bị đánh dấu lại ngay trong run nếu
// server vẫn từ chối — chỉ tốn đúng 1 request kiểm tra.
func (c *Chain) BeginUserRun() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, st := range c.snap.Providers {
		st.CooldownUntil = 0
		st.ExhaustedUntil = ""
	}
}

func (c *Chain) today() string { return c.nowFn().Format("2006-01-02") }

// state lấy trạng thái của 1 dịch vụ, tự reset bộ đếm sang ngày mới.
func (c *Chain) state(id string) *PState {
	st, ok := c.snap.Providers[id]
	if !ok {
		st = &PState{Status: "unknown"}
		c.snap.Providers[id] = st
	}
	if st.Day != c.today() {
		st.Day = c.today()
		st.CountToday = 0
		st.ExhaustedUntil = ""
	}
	return st
}

func (c *Chain) exhausted(id string) bool {
	st := c.state(id)
	return st.ExhaustedUntil == c.today()
}

func (c *Chain) cooling(id string) bool {
	st := c.state(id)
	return st.CooldownUntil > c.nowFn().Unix()
}

// StatusOf trả trạng thái hiển thị của 1 dịch vụ (đọc từ ngoài).
func (c *Chain) StatusOf(id string) PState {
	c.mu.Lock()
	defer c.mu.Unlock()
	st := c.state(id)
	cp := *st
	return cp
}

// LastGoodID dịch vụ thành công gần nhất (ưu tiên đứng đầu lần sau).
func (c *Chain) LastGoodID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snap.LastGood
}

// Synthesize chạy cầu nối: đi qua registry đúng thứ tự, bỏ qua dịch vụ
// cạn lượt/lỗi cooldown/không hỗ trợ, gọi từng cái cho tới khi có audio.
//
// PATCH FIX55 — LỌC THEO GIỌNG: khi người dùng chọn một giọng cụ thể và
// giọng đó có trong catalog của ít nhất một dịch vụ, chuỗi CHỈ ghé các
// dịch vụ có đúng giọng đó. Lý do: probe55 (2026-09-21) chứng minh các
// space TỪ CHỐI tên giọng ngoài catalog của chúng (error null) — nếu
// gửi nhầm thì hoặc lỗi hoặc bị thay bằng giọng mặc định mà người dùng
// không hay biết. Lọc trước vừa ĐÚNG GIỌNG vừa nhanh (bỏ qua tức thì
// những dịch vụ không có giọng, không tốn một request nào).
func (c *Chain) Synthesize(ctx context.Context, hc *http.Client, r Request, onEv func(Event)) (Result, error) {
	if hc == nil {
		hc = newHTTP()
	}
	reg := c.reg
	if reg == nil {
		reg = Registry()
	}
	totalAll := len(reg)
	filtered := false
	// PATCH FIX55: lọc dịch vụ theo giọng yêu cầu (giữ nguyên thứ tự gốc).
	if r.VoiceName != "" {
		var with []Desc
		for _, d := range reg {
			if d.HasVoice(r.VoiceName) {
				with = append(with, d)
			}
		}
		if len(with) > 0 {
			reg = with
			filtered = true
			emitEv(onEv, Event{Phase: "info", Label: r.VoiceName,
				Message: fmt.Sprintf("Giọng %q có trên %d/%d dịch vụ — chỉ gửi tới các dịch vụ có đúng giọng này.",
					r.VoiceName, len(with), totalAll),
				Pct: 1, ElapsedSec: 0})
		} else {
			// PATCH FIX56: KHÔNG chạy chuỗi với "giọng gần đúng" nữa —
			// người dùng sẽ nghe GIỌNG KHÁC mà không hay biết (đúng
			// tên giọng là nguyên tắc của bộ lọc FIX55). Báo lỗi rõ
			// ngay lập tức, không tốn một request nào.
			msg := fmt.Sprintf("Giọng %q không có trên bất kỳ dịch vụ online nào của chuỗi — hãy chọn giọng khác (8 giọng OD là ổn định nhất) hoặc dùng chế độ Offline.", r.VoiceName)
			emitEv(onEv, Event{Phase: "fail", Label: r.VoiceName,
				Message: msg, Pct: 0, ElapsedSec: 0})
			return Result{}, fmt.Errorf("%s", msg)
		}
	}
	total := len(reg)
	started := time.Now()
	tried := 0
	var lastErr error
	// PATCH FIX59: các dịch vụ vừa CHẬP CHỜN (lỗi thoáng qua) của lượt 1 —
	// ứng viên cho lượt thử lại tự động khi cả lượt 1 thất bại.
	var transient []Desc

	for i, d := range reg {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		basePct := float64(i) / float64(total) * 90

		if d.SkipReason != "" {
			c.mu.Lock()
			c.state(d.ID).Status = "skip"
			c.mu.Unlock()
			emitEv(onEv, Event{Phase: "skip", ProviderID: d.ID, Label: d.Label,
				Message: "Bỏ qua " + d.Label + " — " + d.SkipReason, Pct: basePct,
				ElapsedSec: time.Since(started).Seconds()})
			continue
		}
		if c.exhausted(d.ID) {
			emitEv(onEv, Event{Phase: "quota", ProviderID: d.ID, Label: d.Label,
				Message: d.Label + " đã hết lượt hôm nay — chuyển dịch vụ kế tiếp.", Pct: basePct,
				ElapsedSec: time.Since(started).Seconds()})
			continue
		}
		if c.cooling(d.ID) && c.snap.LastGood != d.ID {
			emitEv(onEv, Event{Phase: "skip", ProviderID: d.ID, Label: d.Label,
				Message: d.Label + " vừa lỗi gần đây — tạm thử dịch vụ khác trước.", Pct: basePct,
				ElapsedSec: time.Since(started).Seconds()})
			continue
		}

		tried++
		// PATCH FIX55: thanh tiến trình KHÔNG còn hiển thị tên kỹ thuật
		// HF/URL — chi tiết từng dịch vụ xem ở thẻ “Dịch vụ Online”.
		emitEv(onEv, Event{Phase: "trying", ProviderID: d.ID, Label: d.Label,
			Message: "Đang gửi yêu cầu tới Server tổng hợp…", Pct: basePct + 2,
			ElapsedSec: time.Since(started).Seconds()})

		res, err := callProvider(ctx, hc, d, r, func(ev gradioEvent) {
			// heartbeat/process_starts của gradio → nhích % để người dùng
			// thấy app còn sống (chống cảm giác "đứng im").
			if onEv != nil && (ev.name == "heartbeat" || ev.name == "process_starts" || ev.name == "process_generating") {
				// PATCH FIX55: thông điệp gọn theo yêu cầu UI —
				// "Đang chờ Server tổng hợp" thay cho tên kỹ thuật.
				msg := "Đang chờ Server tổng hợp…"
				if ev.name == "process_starts" {
					msg = "Server đã nhận — đang tổng hợp…"
				} else if ev.name == "process_generating" {
					msg = "Server đang trả kết quả…"
				}
				creep := basePct + 2 + minF(6, time.Since(started).Seconds()/30)
				emitEv(onEv, Event{Phase: "waiting", ProviderID: d.ID, Label: d.Label,
					Message: msg, Pct: creep, ElapsedSec: time.Since(started).Seconds()})
			}
		})
		if err == nil {
			// PATCH FIX59: kiểm chất lượng audio NGAY TẠI CHUỖI — audio
			// rè/rác = thất bại của dịch vụ này, tự nhảy dịch vụ kế.
			err = verifyAudio(&res)
		}

		if err == nil {
			c.mu.Lock()
			st := c.state(d.ID)
			st.Status = "ok"
			st.Err = ""
			st.CooldownUntil = 0
			st.CountToday++
			st.LastOKAt = c.nowFn().Format("15:04:05")
			c.snap.LastGood = d.ID
			c.mu.Unlock()
			res.Tried = tried
			res.DurationSec = 0
			emitEv(onEv, Event{Phase: "done", ProviderID: d.ID, Label: d.Label,
				Message: "Thành công qua " + d.Label, Pct: 92,
				ElapsedSec: time.Since(started).Seconds()})
			return res, nil
		}

		lastErr = err
		// PATCH FIX57: lỗi "giọng tạm chưa có" (vieneu.io 400) KHÔNG đặt
		// cooldown — dịch vụ còn sống, giọng khác của lần chạy sau vẫn dùng
		// được ngay; nếu cooldown thì lần thử giọng khác lại bị bỏ qua oan.
		if IsVoiceNotAvailable(err) {
			c.mu.Lock()
			st := c.state(d.ID)
			st.Status, st.Err = "err", err.Error()
			c.mu.Unlock()
			emitEv(onEv, Event{Phase: "fail", ProviderID: d.ID, Label: d.Label,
				Message: d.Label + " lỗi: " + err.Error(),
				Pct:     basePct + 4, ElapsedSec: time.Since(started).Seconds()})
			continue
		}
		if IsQuota(err) {
			c.mu.Lock()
			st := c.state(d.ID)
			st.Status = "quota"
			st.Err = err.Error()
			st.ExhaustedUntil = c.today()
			c.mu.Unlock()
			emitEv(onEv, Event{Phase: "quota", ProviderID: d.ID, Label: d.Label,
				Message: d.Label + " hết lượt — tự động chuyển dịch vụ kế tiếp.", Pct: basePct + 4,
				ElapsedSec: time.Since(started).Seconds()})
			continue
		}
		if IsWake(err) && ctx.Err() == nil {
			// space đang ngủ — đợi rồi thử ĐÚNG dịch vụ đó lại 1 lần
			emitEv(onEv, Event{Phase: "waiting", ProviderID: d.ID, Label: d.Label,
				Message: d.Label + " đang khởi động (cold start) — đợi ~25s rồi thử lại…",
				Pct:     basePct + 3, ElapsedSec: time.Since(started).Seconds()})
			select {
			case <-time.After(WakeRetryDelay):
			case <-ctx.Done():
				return Result{}, ctx.Err()
			}
			res, err = callProvider(ctx, hc, d, r, nil)
			if err == nil {
				// PATCH FIX59: cả kết quả đánh thức cũng phải qua lọc chất lượng.
				err = verifyAudio(&res)
			}
			if err == nil {
				c.mu.Lock()
				st := c.state(d.ID)
				st.Status, st.Err, st.CooldownUntil, st.CountToday = "ok", "", 0, st.CountToday+1
				c.snap.LastGood = d.ID
				c.mu.Unlock()
				res.Tried = tried
				emitEv(onEv, Event{Phase: "done", ProviderID: d.ID, Label: d.Label,
					Message: "Thành công qua " + d.Label + " (sau khi đánh thức)", Pct: 92,
					ElapsedSec: time.Since(started).Seconds()})
				return res, nil
			}
		}

		// lỗi thường → cooldown ngắn, chuyển tiếp
		until := c.nowFn().Add(ErrCooldown).Unix()
		c.mu.Lock()
		st := c.state(d.ID)
		st.Status, st.Err, st.CooldownUntil = "err", err.Error(), until
		c.mu.Unlock()
		emitEv(onEv, Event{Phase: "fail", ProviderID: d.ID, Label: d.Label,
			Message: d.Label + " lỗi: " + err.Error() + " — chuyển dịch vụ kế tiếp.",
			Pct:     basePct + 4, ElapsedSec: time.Since(started).Seconds()})
		// PATCH FIX59: lỗi thoáng qua (không phải audio-rác) → ứng viên
		// thử lại ở lượt 2 (space ZeroGPU chập chờn THEO TỪNG REQUEST —
		// probe58b: cùng space cùng phút, Mai Anh OK 2,8s trong khi
		// Quang Sơn + Ngọc Trân error 0,7s).
		if !IsBadAudio(err) {
			transient = append(transient, d)
		}
	}

	// ── PATCH FIX59: LƯỢT THỬ LẠI TỰ ĐỘNG ────────────────────────────
	// Người dùng báo "tất cả giọng OD không dùng được" xảy ra khi CẢ nhóm
	// space chập chờn rơi vào cửa sổ lỗi đúng lúc chạy chuỗi — nhưng cùng
	// những server đó vài phút trước/vài request sau lại OK (probe58b,
	// probe60: eagle0019 25s OK lúc 11:02 sau khi error cả sáng). Vậy
	// trước khi trả lỗi cho người dùng, tự thử lại MỘT lượt các server
	// vừa lỗi thoáng qua (kèm trễ ngắn giữa lần gọi — không dồn dập).
	// Audio-rác KHÔNG được thử lại (lỗi kéo dài, cố tình bỏ).
	if len(transient) > 0 && ctx.Err() == nil {
		if len(transient) > maxRetryPass {
			transient = transient[:maxRetryPass] // giữ thứ tự ưu tiên đầu chuỗi
		}
		emitEv(onEv, Event{Phase: "info", Label: r.VoiceName,
			Message: fmt.Sprintf("Không dịch vụ nào nhận được ở lượt 1 — tự động thử lại %d server vừa chập chờn…", len(transient)),
			Pct:     45, ElapsedSec: time.Since(started).Seconds()})
		for _, d := range transient {
			if ctx.Err() != nil {
				return Result{}, ctx.Err()
			}
			select {
			case <-time.After(RetryPassDelay):
			case <-ctx.Done():
				return Result{}, ctx.Err()
			}
			tried++
			emitEv(onEv, Event{Phase: "trying", ProviderID: d.ID, Label: d.Label,
				Message: "Đang thử lại " + d.Label + "…",
				Pct:     46, ElapsedSec: time.Since(started).Seconds()})
			res, err := callProvider(ctx, hc, d, r, nil)
			if err == nil {
				err = verifyAudio(&res)
			}
			if err == nil {
				c.mu.Lock()
				st := c.state(d.ID)
				st.Status, st.Err, st.CooldownUntil = "ok", "", 0
				st.CountToday++
				st.LastOKAt = c.nowFn().Format("15:04:05")
				c.snap.LastGood = d.ID
				c.mu.Unlock()
				res.Tried = tried
				res.DurationSec = 0
				emitEv(onEv, Event{Phase: "done", ProviderID: d.ID, Label: d.Label,
					Message: "Thành công qua " + d.Label + " (ở lượt thử lại)", Pct: 92,
					ElapsedSec: time.Since(started).Seconds()})
				return res, nil
			}
			lastErr = err
			emitEv(onEv, Event{Phase: "fail", ProviderID: d.ID, Label: d.Label,
				Message: "Thử lại " + d.Label + " vẫn lỗi: " + err.Error(),
				Pct:     47, ElapsedSec: time.Since(started).Seconds()})
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("không còn dịch vụ online nào khả dụng trong chuỗi")
	}
	// PATCH FIX56: khi chuỗi đã lọc theo giọng và VẪN hỏng hết → gợi ý
	// người dùng rõ nguyên nhân + lối thoát (giọng khác / chờ chủ space).
	if filtered && r.VoiceName != "" {
		// PATCH FIX57: thông điệp sát thật hơn — các space ZeroGPU chập chờn
		// theo cơn (probe58b: cùng space cùng phút, 1 giọng OK giọng lỗi) nên
		// khuyên thử lại NGAY thay vì gợi ý giọng khác (riêng giọng OD thì
		// gợi ý "giọng OD khác" là vô nghĩa).
		return Result{}, fmt.Errorf("tất cả dịch vụ có giọng %q đều không thành công (đã thử %d). Các Server này đang chập chờn theo cơn — bấm Chuyển đổi thử lại NGAY (lần sau thường được), hoặc chọn giọng khác, hoặc dùng chế độ Offline. Lỗi cuối: %w", r.VoiceName, tried, lastErr)
	}
	return Result{}, fmt.Errorf("tất cả dịch vụ online đều không thành công (đã thử %d). Lỗi cuối: %w", tried, lastErr)
}

func callProvider(ctx context.Context, hc *http.Client, d Desc, r Request, onEv func(gradioEvent)) (Result, error) {
	switch d.Kind {
	case "edge":
		// PATCH FIX58: Microsoft Edge TTS trực tiếp (websocket).
		return synthEdge(ctx, hc, d, r)
	case "vieneuio":
		return synthVieneuIO(ctx, hc, d, r)
	case "gradio":
		return synthGradio(ctx, hc, d, r, onEv)
	default:
		return Result{}, fmt.Errorf("loại dịch vụ chưa hỗ trợ gọi trực tiếp: %s", d.Kind)
	}
}

func emitEv(onEv func(Event), ev Event) {
	if onEv != nil {
		onEv(ev)
	}
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
