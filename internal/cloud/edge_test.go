package cloud

// edge_test.go — PATCH FIX58: test đơn vị cho provider Microsoft Edge TTS
// trực tiếp (edge.go) + E2E mạng thật có guard (F58_E2E=1).
import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestSecMsGecVector: vector cứng tính từ công thức drm.py — unix
// 1735689600 (2025-01-01T00:00:00Z) → ticks 133801632000000000.
func TestSecMsGecVector(t *testing.T) {
	got := buildSecMsGec(time.Unix(1735689600, 0).UTC())
	want := "B0EDD22C7C09868E2F24C10264A8A3EB877773A7B6040B68AFA4FBCBABEA0238"
	if got != want {
		t.Fatalf("Sec-MS-GEC sai vector: got %s want %s", got, want)
	}
	// cùng mốc 5 phút → cùng token
	got2 := buildSecMsGec(time.Unix(1735689629, 0).UTC())
	if got2 != want {
		t.Fatalf("cùng mốc 5 phút phải cùng token, got %s", got2)
	}
}

// TestEdgeVoiceMapCoversProviderVoices: mọi giọng khai báo trong registry
// cho edge-tts phải có ánh xạ ShortName — chống lệch tên khi sửa registry.
func TestEdgeVoiceMapCoversProviderVoices(t *testing.T) {
	for _, d := range Registry() {
		if d.ID != "edge-tts" {
			continue
		}
		for _, v := range d.Voices {
			if edgeVoiceMap[v.Name] == "" {
				t.Fatalf("giọng %q của edge-tts thiếu ánh xạ ShortName", v.Name)
			}
		}
	}
	if edgeVoiceName("Hoài My (Nữ)") != "vi-VN-HoaiMyNeural" ||
		edgeVoiceName("Nam Minh (Nam)") != "vi-VN-NamMinhNeural" {
		t.Fatalf("ánh xạ sai: %s / %s", edgeVoiceName("Hoài My (Nữ)"), edgeVoiceName("Nam Minh (Nam)"))
	}
}

// TestBuildEdgeSSML: cấu trúc SSML khớp mkssml() + escape XML đúng.
func TestBuildEdgeSSML(t *testing.T) {
	ssml := buildEdgeSSML("vi-VN-HoaiMyNeural", "Xin chào & chúc <buổi> sáng 'tốt lành'")
	for _, want := range []string{
		"<speak version='1.0' xmlns='http://www.w3.org/2001/10/synthesis' xml:lang='en-US'>",
		"<voice name='vi-VN-HoaiMyNeural'>",
		"<prosody pitch='+0Hz' rate='+0%' volume='+0%'>",
		"Xin chào &amp; chúc &lt;buổi&gt; sáng &apos;tốt lành&apos;",
		"</prosody></voice></speak>",
	} {
		if !strings.Contains(ssml, want) {
			t.Fatalf("SSML thiếu %q\nSSML: %s", want, ssml)
		}
	}
	// entity có sẵn không bị escape kép
	if got := buildEdgeSSML("v", "A &amp; B"); !strings.Contains(got, "A &amp; B") {
		t.Fatalf("entity bị escape kép: %s", got)
	}
}

// TestSplitRunes: chia tại khoảng trắng, không mất ký tự, đủ đoạn.
func TestSplitRunes(t *testing.T) {
	short := "một hai ba"
	if parts := splitRunes(short, 100); len(parts) != 1 || parts[0] != short {
		t.Fatalf("đoạn ngắn không được chia: %v", parts)
	}
	long := strings.Repeat("từ ", 1500) // 4500 rune
	parts := splitRunes(long, 2000)
	if len(parts) < 2 {
		t.Fatalf("văn bản dài phải chia ≥2 đoạn, got %d", len(parts))
	}
	joined := strings.Join(parts, "")
	if len([]rune(joined)) != len([]rune(long)) {
		t.Fatalf("chia đoạn mất ký tự: %d vs %d", len([]rune(joined)), len([]rune(long)))
	}
	for _, p := range parts {
		if len([]rune(p)) > 2000 {
			t.Fatalf("đoạn vượt giới hạn: %d", len([]rune(p)))
		}
	}
}

// TestManualFIX58E2E: E2E mạng THẬT qua cả 2 giọng Edge — chạy tay:
//
//	F58_E2E=1 go test ./internal/cloud/ -run TestManualFIX58E2E -v -timeout 8m
func TestManualFIX58E2E(t *testing.T) {
	if os.Getenv("F58_E2E") == "" {
		t.Skip("chỉ chạy khi F58_E2E=1")
	}
	cases := []struct {
		voice string
		text  string
	}{
		{"Hoài My (Nữ)", "Xin chào, đây là giọng Hoài My đọc thử nghiệm trực tiếp từ dịch vụ Microsoft Edge của phần mềm HCStudio."},
		{"Nam Minh (Nam)", "Giọng Nam Minh miền Nam kiểm tra kết nối thành công, không cần tài khoản và không giới hạn số lần sử dụng."},
	}
	for _, tc := range cases {
		t.Run(tc.voice, func(t *testing.T) {
			started := time.Now()
			res, err := synthEdge(context.Background(), nil, Registry()[0], Request{
				Text: tc.text, VoiceName: tc.voice,
			})
			if err != nil {
				t.Fatalf("edge synth %q thất bại: %v", tc.voice, err)
			}
			if len(res.Audio) < 20000 {
				t.Fatalf("audio quá nhỏ: %d bytes", len(res.Audio))
			}
			if res.MIME != "audio/mpeg" {
				t.Fatalf("MIME sai: %s", res.MIME)
			}
			t.Logf("E2E OK: %d bytes trong %.1fs (voiceUsed=%s)",
				len(res.Audio), time.Since(started).Seconds(), res.VoiceUsed)
		})
	}
}

// TestEdgeAudioPayload (PATCH FIX59): khối binary Edge = [2 byte BE = N]
// [N byte header][body]. Bản FIX58 cắt data[hl:] → payload thừa "\r\n"
// cuối header → go-mp3 từ chối MP3 Edge ⇒ Edge TTS chưa bao giờ phát được
// trong app (user: "audio không đọc được"). Test ghim phép cắt đúng 2+N.
func TestEdgeAudioPayload(t *testing.T) {
	hdr := "X-RequestId:AB\r\nContent-Type:audio/mpeg\r\nPath:audio\r\n"
	frame := append([]byte{byte(len(hdr) >> 8), byte(len(hdr))}, append([]byte(hdr), 0xFF, 0xF3, 0x01, 0x02)...)
	pl, ok := edgeAudioPayload(frame)
	if !ok {
		t.Fatal("frame audio hợp lệ phải cắt được payload")
	}
	if len(pl) != 4 || pl[0] != 0xFF || pl[1] != 0xF3 {
		t.Fatalf("payload phải bắt đầu bằng sync MP3 (bỏ qua \\r\\n header), got %x", pl)
	}

	// frame Path:audio.metadata → không phải audio
	hdrMeta := "X-RequestId:AB\r\nPath:audio.metadata\r\n"
	frameMeta := append([]byte{0, byte(len(hdrMeta))}, append([]byte(hdrMeta), "json"...)...)
	if _, ok := edgeAudioPayload(frameMeta); ok {
		t.Fatal("audio.metadata không được nhận là audio")
	}

	// frame quá ngắn / header lệch → an toàn
	if _, ok := edgeAudioPayload([]byte{0x00}); ok {
		t.Fatal("frame quá ngắn phải từ chối")
	}
	bad := []byte{0x00, 0x20, 'P'} // hl=32 nhưng chỉ có 1 byte còn lại
	if _, ok := edgeAudioPayload(bad); ok {
		t.Fatal("header dài hơn frame phải từ chối")
	}
}
