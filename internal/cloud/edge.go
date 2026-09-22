// Package cloud — PATCH FIX58: MICROSOFT EDGE TTS TRỰC TIẾP (XƯƠNG SỐNG MỚI).
//
// Nghiên cứu 10 dịch vụ TTS bên ngoài (ElevenLabs, MiniMax, TTSMaker,
// Ondoku, Speechify, Vbee, EverAI, Luvvoice, Narakeet, Canva — probe thật
// 2026-09-21, scripts59/) cho kết quả: các trang "miễn phí không giới hạn"
// đều có chốt bảo vệ chống gọi tự động (TTSMaker = GeeTest theo IP,
// Luvvoice = Cloudflare Turnstile bắt buộc, EverAI = yêu cầu đăng nhập)
// nên KHÔNG tích hợp trực tiếp được. Nhưng chính Luvvoice chỉ là lớp vỏ
// cho giọng NEURAL của Microsoft Edge (avast "HoaiMy"/"NamMinh" trên
// CDN của họ khớp 1-1 với vi-VN-HoaiMyNeural / vi-VN-NamMinhNeural) —
// và giao thức Edge TTS có thể gọi TRỰC TIẾP:
//   - miễn phí, KHÔNG tài khoản, KHÔNG key, KHÔNG hạn mức phát hiện được
//     (dùng vô tư nhiều năm bởi hàng trăm dự án edge-tts open-source);
//   - 2 giọng tiếng Việt neural: Hoài My (nữ) + Nam Minh (nam);
//   - trả MP3 24kHz — app đã decode MP3 từ FIX56.
//
// Provider này đứng ĐẦU chuỗi vì ổn định nhất (không phụ thuộc space
// cộng đồng, không GPU queue). Giao thức dựng theo edge-tts
// (constants.py/communicate.py/drm.py, phiên bản 7.x):
//  1. WSS  speech.platform.bing.com/…/edge/v1?TrustedClientToken=…
//     &ConnectionId=…&Sec-MS-GEC=…&Sec-MS-GEC-Version=…
//  2. Gửi  "Path:speech.config" (outputFormat audio-24khz-48kbitrate-mono-mp3)
//  3. Gửi  "Path:ssml" (SSML voice + prosody)
//  4. Nhận binary: 2 byte độ-dài-header (big endian) + header "Path:audio"
//     → ghép payload; text "Path:turn.end" → kết thúc.
//
// Sec-MS-GEC = SHA256(ticks + token) — ticks = Windows FILETIME (100ns,
// epoch 1601) làm tròn XUỐNG mốc 5 phút ⇒ dung sai clock ±5 phút.
package cloud

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// edgeTrustedClientToken token public của client Edge (không phải bí mật).
	edgeTrustedClientToken = "6A5AA1D4EAFF4E9FB37E23D68491D6F4"
	// edgeWSSURL đường gốc websocket tổng hợp.
	edgeWSSURL = "wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1?TrustedClientToken=" + edgeTrustedClientToken
	// edgeGECVersion khớp CHROMIUM_FULL_VERSION của edge-tts 7.x.
	edgeGECVersion = "1-143.0.3650.75"
	// edgeUA đủ để server phân loại client Edge (chỉ cần số major khớp).
	edgeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" +
		" (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36 Edg/143.0.0.0"
	// edgeOutputFormat MP3 24kHz mono 48kbps — app decode được từ FIX56.
	edgeOutputFormat = "audio-24khz-48kbitrate-mono-mp3"
	// edgeWinEpoch số giây từ 1601-01-01 tới 1970-01-01.
	edgeWinEpoch = uint64(11644473600)
	// edgeChunkRunes giới hạn an toàn 1 message SSML (server nhận tới ~8KB
	// byte; 4000 rune ≈ 12KB byte UTF-8 tệ nhất… dùng 2000 rune dư an toàn
	// cho văn bản Việt nhiều dấu — mỗi ký tự có dấu tối đa 3 byte).
	edgeChunkRunes = 2000
)

// edgeVoiceMap ánh xạ tên giọng trong app → ShortName của Edge.
// Tên app GIỮ NGUYÊN từ catalog DevTam05 để bộ lọc FIX55 tự routing:
// yêu cầu "Hoài My (Nữ)" sẽ ghé CẢ edge-tts lẫn DevTam05.
var edgeVoiceMap = map[string]string{
	"Hoài My (Nữ)":   "vi-VN-HoaiMyNeural",
	"Nam Minh (Nam)": "vi-VN-NamMinhNeural",
}

// edgeDefaultVoiceName tên app dùng làm mặc định của provider này.
const edgeDefaultVoiceName = "Hoài My (Nữ)"

// edgeVoiceName đổi tên app → ShortName Edge; rơi về Hoài My nếu lạ
// (không xảy ra trong chuỗi thật vì bộ lọc giọng chặn trước).
func edgeVoiceName(appName string) string {
	if v, ok := edgeVoiceMap[appName]; ok {
		return v
	}
	return edgeVoiceMap[edgeDefaultVoiceName]
}

// buildSecMsGec sinh token DRM theo drm.py: thời điểm hiện tại (+skew bỏ
// qua — sai số đồng hồ ≤5 phút chấp nhận được) → FILETIME 100ns → làm
// tròn xuống mốc 5 phút → ghép token → SHA256 hex HOA.
func buildSecMsGec(now time.Time) string {
	ticks := uint64(now.Unix()) + edgeWinEpoch
	ticks -= ticks % 300 // làm tròn xuống 5 phút (300 giây)
	ticks *= 10_000_000  // giây → 100ns
	h := sha256.Sum256([]byte(fmt.Sprintf("%d%s", ticks, edgeTrustedClientToken)))
	return strings.ToUpper(hex.EncodeToString(h[:]))
}

var edgeRandHex = func(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// edgeDateStr kiểu "Mon Jan 02 2006 15:04:05 GMT+0000 (Coordinated
// Universal Time)" — đúng định dạng date_to_string() của edge-tts.
func edgeDateStr(t time.Time) string {
	return t.UTC().Format("Mon Jan 02 2006 15:04:05 GMT+0000 (Coordinated Universal Time)")
}

// buildEdgeSSML dựng khối SSML như mkssml() (pitch +0Hz, rate/volume +0%).
func buildEdgeSSML(edgeVoice, text string) string {
	var b strings.Builder
	b.WriteString("<speak version='1.0' xmlns='http://www.w3.org/2001/10/synthesis' xml:lang='en-US'>")
	b.WriteString("<voice name='" + edgeVoice + "'>")
	b.WriteString("<prosody pitch='+0Hz' rate='+0%' volume='+0%'>")
	b.WriteString(xmlEscapeText(text))
	b.WriteString("</prosody></voice></speak>")
	return b.String()
}

var xmlEscaper = strings.NewReplacer(
	"&", "&amp;", "<", "&lt;", ">", "&gt;", "'", "&apos;", "\"", "&quot;",
)

var xmlEntitiesRe = regexp.MustCompile(`&[a-z#0-9]+;`)

// xmlEscapeText escape XML cho text SSML; giữ nguyên entity hợp lệ
// (người dùng gõ &amp; thì không escape hai lần).
func xmlEscapeText(s string) string {
	if xmlEntitiesRe.MatchString(s) {
		// có entity: escape từng phần ngoài entity để không kép đôi.
		var b strings.Builder
		i := 0
		for _, loc := range xmlEntitiesRe.FindAllStringIndex(s, -1) {
			b.WriteString(xmlEscaper.Replace(s[i:loc[0]]))
			b.WriteString(s[loc[0]:loc[1]])
			i = loc[1]
		}
		b.WriteString(xmlEscaper.Replace(s[i:]))
		return b.String()
	}
	return xmlEscaper.Replace(s)
}

// splitRunes chia text thành các đoạn ≤ n rune (cắt tại khoảng trắng gần
// nhất để không đứt giữa từ).
func splitRunes(s string, n int) []string {
	r := []rune(s)
	if len(r) <= n {
		return []string{s}
	}
	var out []string
	start := 0
	for start < len(r) {
		end := start + n
		if end >= len(r) {
			out = append(out, string(r[start:]))
			break
		}
		// tìm khoảng trắng lùi lại để cắt tại biên từ
		cut := end
		for i := end; i > start+n/2; i-- {
			if r[i-1] == ' ' || r[i-1] == '\n' {
				cut = i
				break
			}
		}
		out = append(out, string(r[start:cut]))
		start = cut
	}
	return out
}

// synthEdge tổng hợp qua websocket Edge TTS (mô tả đầy đủ ở đầu file).
// Text dài được chia nhiều SSML trên CÙNG một kết nối (turn.end đánh dấu
// từng đoạn), audio MP3 ghép lại — MP3 frame độc lập nên nối trực tiếp.
// PATCH FIX58 (bổ sung sau probe): Microsoft thỉnh thoảng RST kết nối /
// bỏ không trả audio khi bị gọi dồn dập (probe 59: python 1/4 lần
// NoAudioReceived; Go đợt đầu RST liên tiếp rồi tự hết) ⇒ thử lại 1 lần
// sau 1,2s với kết nối + token MỚI trước khi trả lỗi cho chuỗi.
func synthEdge(ctx context.Context, hc *http.Client, d Desc, r Request) (Result, error) {
	res, err := synthEdgeOnce(ctx, hc, d, r)
	if err == nil {
		return res, nil
	}
	if ctx.Err() != nil {
		return Result{}, err
	}
	select {
	case <-time.After(1200 * time.Millisecond):
	case <-ctx.Done():
		return Result{}, err
	}
	return synthEdgeOnce(ctx, hc, d, r)
}

func synthEdgeOnce(ctx context.Context, hc *http.Client, d Desc, r Request) (Result, error) {
	voice, changed := d.ResolveVoice(r.VoiceName)
	note := ""
	if changed && r.VoiceName != "" {
		note = fmt.Sprintf(" · giọng %q không có trên dịch vụ này — dùng mặc định %q", r.VoiceName, voice)
	}
	dialer := &websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	wssURL := fmt.Sprintf("%s&ConnectionId=%s&Sec-MS-GEC=%s&Sec-MS-GEC-Version=%s",
		edgeWSSURL, edgeRandHex(16), buildSecMsGec(time.Now()), edgeGECVersion)
	hdr := http.Header{}
	hdr.Set("Pragma", "no-cache")
	hdr.Set("Cache-Control", "no-cache")
	hdr.Set("Origin", "chrome-extension://jdiccldimpdaibmpdkjnbmckianbfold")
	hdr.Set("User-Agent", edgeUA)
	hdr.Set("Accept-Encoding", "gzip, deflate, br")
	hdr.Set("Accept-Language", "en-US,en;q=0.9")
	hdr.Set("Cookie", "muid="+edgeRandHex(16)+";")

	wsCtx, cancel := context.WithTimeout(ctx, AttemptTimeout)
	defer cancel()
	ws, resp, err := dialer.DialContext(wsCtx, wssURL, hdr)
	if err != nil {
		if resp != nil {
			return Result{}, fmt.Errorf("%s: không kết nối được (HTTP %d)", d.Label, resp.StatusCode)
		}
		return Result{}, fmt.Errorf("%s: không kết nối được: %w", d.Label, err)
	}
	defer ws.Close()
	ws.SetReadLimit(1 << 24) // 16MB đủ cho 1 frame audio lớn

	// speech.config — định dạng "sentence boundary" như mặc định edge-tts.
	cfg := "X-Timestamp:" + edgeDateStr(time.Now()) + "\r\n" +
		"Content-Type:application/json; charset=utf-8\r\n" +
		"Path:speech.config\r\n\r\n" +
		`{"context":{"synthesis":{"audio":{"metadataoptions":{` +
		`"sentenceBoundaryEnabled":"true","wordBoundaryEnabled":"false"` +
		`},"outputFormat":"` + edgeOutputFormat + `"}}}}` + "\r\n"
	if err := ws.WriteMessage(websocket.TextMessage, []byte(cfg)); err != nil {
		return Result{}, fmt.Errorf("%s: gửi cấu hình lỗi: %w", d.Label, err)
	}

	chunks := splitRunes(r.Text, edgeChunkRunes)
	var audio []byte
	for _, chunk := range chunks {
		if wsCtx.Err() != nil {
			return Result{}, fmt.Errorf("%s: hết giờ (%w)", d.Label, wsCtx.Err())
		}
		reqID := strings.ToUpper(edgeRandHex(16))
		msg := "X-RequestId:" + reqID + "\r\n" +
			"Content-Type:application/ssml+xml\r\n" +
			"X-Timestamp:" + edgeDateStr(time.Now()) + "Z\r\n" + // đúng "bug" Edge: thêm Z
			"Path:ssml\r\n\r\n" +
			buildEdgeSSML(edgeVoiceName(voice), chunk)
		if err := ws.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			return Result{}, fmt.Errorf("%s: gửi SSML lỗi: %w", d.Label, err)
		}

		gotAudio := false
		for {
			mt, data, err := ws.ReadMessage()
			if err != nil {
				return Result{}, fmt.Errorf("%s: đọc kết quả lỗi: %w", d.Label, err)
			}
			if mt == websocket.TextMessage {
				path := wsPathOf(data)
				if path == "turn.end" {
					break
				}
				// audio.metadata / turn.start / response — bỏ qua.
				continue
			}
			// binary: [2 byte header len][headers][payload]
			// PATCH FIX59: edgeAudioPayload — cắt đúng 2+hl, hết lỗi
			// thừa "\r\n" vào đầu MP3 của FIX58.
			if pl, ok := edgeAudioPayload(data); ok {
				audio = append(audio, pl...)
				gotAudio = true
			}
		}
		if !gotAudio {
			// đúng hành vi edge-tts NoAudioReceived — lỗi tạm thời từng
			// request (probe59: 1/4 lần gọi bị vậy), chuỗi tự nhảy dịch vụ.
			return Result{}, fmt.Errorf("%s: server không trả audio cho giọng %q (tạm thời — thử lại được)", d.Label, voice)
		}
	}
	if len(audio) == 0 {
		return Result{}, fmt.Errorf("%s: trả audio rỗng", d.Label)
	}
	return Result{
		Audio: audio, MIME: "audio/mpeg",
		ProviderID: d.ID, ProviderLabel: d.Label, VoiceUsed: voice,
		Info: fmt.Sprintf("edgeVoice=%s · %d đoạn%s", edgeVoiceName(voice), len(chunks), note),
	}, nil
}

// wsPathOf lấy giá trị "Path:…" trong khối header kiểu "\r\n"-phân tách.
func wsPathOf(hdr []byte) string {
	for _, line := range strings.Split(string(hdr), "\r\n") {
		if strings.HasPrefix(line, "Path:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Path:"))
		}
	}
	return ""
}

// edgeAudioPayload — PATCH FIX59: cắt payload audio từ 1 binary frame Edge.
// Cấu trúc: [2 byte BE = N độ dài header][N byte header][body]. Body bắt
// đầu tại 2+N (bản FIX58 cắt tại N — thừa "\r\n" cuối header vào đầu MP3,
// go-mp3 từ chối → Edge TTS chưa bao giờ phát được trong app).
func edgeAudioPayload(data []byte) (payload []byte, ok bool) {
	if len(data) < 2 {
		return nil, false
	}
	hl := int(data[0])<<8 | int(data[1])
	if hl < 2 || 2+hl > len(data) {
		return nil, false
	}
	if wsPathOf(data[2:2+hl]) != "audio" {
		return nil, false
	}
	return data[2+hl:], true
}
