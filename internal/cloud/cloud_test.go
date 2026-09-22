package cloud

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ---------- PATCH FIX59: WAV giả cho test (giọng mô phỏng / nhiễu trắng) ----------

// wavBytes đóng gói PCM int16 mono thành WAV 16-bit chuẩn.
func wavBytes(pcm []int16, sr int) []byte {
	b := make([]byte, 44+len(pcm)*2)
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:8], uint32(36+len(pcm)*2))
	copy(b[8:12], "WAVE")
	copy(b[12:16], "fmt ")
	binary.LittleEndian.PutUint32(b[16:20], 16)
	binary.LittleEndian.PutUint16(b[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(b[22:24], 1) // mono
	binary.LittleEndian.PutUint32(b[24:28], uint32(sr))
	binary.LittleEndian.PutUint32(b[28:32], uint32(sr*2))
	binary.LittleEndian.PutUint16(b[32:34], 2)
	binary.LittleEndian.PutUint16(b[34:36], 16)
	copy(b[36:40], "data")
	binary.LittleEndian.PutUint32(b[40:44], uint32(len(pcm)*2))
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(b[44+i*2:], uint16(v))
	}
	return b
}

// fakeSpeechWav — 1.6s giọng mô phỏng: 4 cụm âm tiết 180ms cách nhau 70ms
// lặng, sóng hài 180Hz + 900Hz. Đảm bảo QUA bộ lọc LooksLikeNoise (bộ lọc
// trong chuỗi FIX59 coi đây là audio hợp lệ).
func fakeSpeechWav() []byte {
	const sr = 24000
	n := sr * 16 / 10
	pcm := make([]int16, n)
	for i := 0; i < n; i++ {
		t := float64(i) / sr
		amp := 0.0
		if int(t*1000)%250 < 180 {
			amp = 0.6
		}
		v := amp * (0.7*math.Sin(2*math.Pi*180*t) + 0.3*math.Sin(2*math.Pi*900*t))
		pcm[i] = int16(v * 32767)
	}
	return wavBytes(pcm, sr)
}

// fakeNoiseWav — 1.5s nhiễu trắng đều [-0.5, 0.5], giống HỆT output probe60
// bắt được từ nguyenduc1222 (thủ phạm "giọng rè rè vô nghĩa"). Bộ lọc
// LooksLikeNoise phải bắt được.
func fakeNoiseWav() []byte {
	const sr = 24000
	n := sr * 3 / 2
	rng := rand.New(rand.NewSource(59))
	pcm := make([]int16, n)
	for i := range pcm {
		pcm[i] = int16((rng.Float64()*2 - 1) * 16384)
	}
	return wavBytes(pcm, sr)
}

// ---------- fake vieneu.io ----------

func fakeVieneu(t *testing.T, mode string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tts/demo", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Text    string `json:"text"`
			VoiceID string `json:"voiceId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		switch mode {
		case "ok":
			// PATCH FIX59: fake giờ trả WAV giọng mô phỏng HỢP LỆ —
			// chuỗi kiểm chất lượng thật nên byte rác sẽ bị coi là thất bại.
			_ = json.NewEncoder(w).Encode(map[string]string{
				"audioBase64": base64.StdEncoding.EncodeToString(fakeSpeechWav()),
				"mimeType":    "audio/wav",
			})
		case "quota":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"Bạn đã hết lượt miễn phí hôm nay"}`))
		case "down":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
		}
	})
	return httptest.NewServer(mux)
}

// ---------- fake gradio ----------

func fakeGradioFull(t *testing.T, mode string, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	audio := "AUDIO_URL" // điền sau khi biết srv.URL — dùng path tương đối qua handler riêng
	srv := httptest.NewServer(mux)
	audio = srv.URL + "/gradio_api/file=/tmp/gradio/x/audio.wav"
	// Đăng ký CẢ pattern chính xác (POST /call/synthesize) lẫn subtree
	// (GET /call/synthesize/{event_id}) — ServeMux không tự khớp chéo.
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			calls.Add(1)
			switch mode {
			case "ok":
				_ = json.NewEncoder(w).Encode(map[string]string{"event_id": "ev-1"})
			case "quota":
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte("GPU quota exceeded"))
			case "sleeping":
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte("space is starting"))
			case "error":
				_ = json.NewEncoder(w).Encode(map[string]string{"event_id": "ev-err"})
			}
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if mode == "error" {
			_, _ = w.Write([]byte("event: error\ndata: null\n\n"))
			return
		}
		_, _ = w.Write([]byte("event: heartbeat\ndata: null\n\nevent: process_starts\ndata: null\n\nevent: complete\ndata: " +
			`[{"path":"/tmp/gradio/x/audio.wav","url":"` + audio + `","meta":{"_type":"gradio.FileData"}},"⏱ 12s · RTF 1.2"]` + "\n\n"))
	}
	mux.HandleFunc("/gradio_api/call/synthesize", h)
	mux.HandleFunc("/gradio_api/call/synthesize/", h)
	mux.HandleFunc("/gradio_api/file=/tmp/gradio/x/audio.wav", func(w http.ResponseWriter, r *http.Request) {
		// PATCH FIX59: serve WAV hợp lệ — chuỗi giờ kiểm chất lượng thật.
		_, _ = w.Write(fakeSpeechWav())
	})
	return srv
}

// fakeGradioAudio (PATCH FIX59) — fake gradio trả ĐÚNG audio bytes chỉ định
// (dùng cho test bộ lọc chất lượng: noise / junk / speech).
func fakeGradioAudio(t *testing.T, calls *atomic.Int32, audio func() []byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			calls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]string{"event_id": "ev-1"})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: complete\ndata: " +
			`[{"url":"` + srv.URL + `/audio.wav","meta":{"_type":"gradio.FileData"}}]` + "\n\n"))
	}
	mux.HandleFunc("/gradio_api/call/synthesize", h)
	mux.HandleFunc("/gradio_api/call/synthesize/", h)
	mux.HandleFunc("/audio.wav", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(audio())
	})
	return srv
}

func testChain() *Chain {
	c := NewChain("")
	c.nowFn = func() time.Time { return time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC) }
	return c
}

func TestVieneuIODemoOK(t *testing.T) {
	srv := fakeVieneu(t, "ok")
	defer srv.Close()
	d := Desc{ID: "vieneu-io", Label: "vieneu.io", Kind: "vieneuio", Base: srv.URL,
		API: "/api/tts/demo", DefaultVoice: "Adam Tốp Tốp",
		Voices: []Voice{{Name: "Adam Tốp Tốp"}}}
	res, err := synthVieneuIO(context.Background(), newHTTP(), d, Request{Text: "xin chào", VoiceName: "Adam Tốp Tốp"})
	if err != nil {
		t.Fatalf("expected OK, got %v", err)
	}
	if len(res.Audio) < 4 || string(res.Audio[:4]) != "RIFF" || res.MIME != "audio/wav" || res.VoiceUsed != "Adam Tốp Tốp" {
		t.Fatalf("unexpected result: audio=%dB mime=%s voice=%s", len(res.Audio), res.MIME, res.VoiceUsed)
	}
}

func TestVieneuIOQuota(t *testing.T) {
	srv := fakeVieneu(t, "quota")
	defer srv.Close()
	d := Desc{ID: "vieneu-io", Base: srv.URL, API: "/api/tts/demo", DefaultVoice: "A"}
	_, err := synthVieneuIO(context.Background(), newHTTP(), d, Request{Text: "x"})
	if !IsQuota(err) {
		t.Fatalf("expected quota error, got %v", err)
	}
}

func TestGradioTemplateOK(t *testing.T) {
	calls := &atomic.Int32{}
	srv := fakeGradioFull(t, "ok", calls)
	defer srv.Close()
	d := Desc{ID: "hf-x", Label: "space-x", Kind: "gradio", Base: srv.URL, API: "synthesize",
		DataStyle: "template", DefaultVoice: "Trúc Ly",
		Voices: []Voice{{Name: "Trúc Ly"}}}
	res, err := synthGradio(context.Background(), newHTTP(), d, Request{Text: "xin chào", VoiceName: "Trúc Ly"}, nil)
	if err != nil {
		t.Fatalf("expected OK, got %v", err)
	}
	if len(res.Audio) < 4 || string(res.Audio[:4]) != "RIFF" {
		t.Fatalf("unexpected audio: %dB", len(res.Audio))
	}
	if !strings.Contains(res.Info, "RTF") {
		t.Fatalf("expected info from markdown output, got %q", res.Info)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 POST, got %d", calls.Load())
	}
}

func TestGradioDataShapes(t *testing.T) {
	d := Desc{DataStyle: "template"}
	got := fmt.Sprint(gradioData(d, "T", "V"))
	want := fmt.Sprint([]any{"T", "V", nil, 0.8, 25, 0.95, 1.2, 300, 256})
	if got != want {
		t.Fatalf("template data mismatch:\n got %s\nwant %s", got, want)
	}
	d2 := Desc{DataStyle: "smrfhdl"}
	got2 := fmt.Sprint(gradioData(d2, "T", "V"))
	want2 := fmt.Sprint([]any{"T", "V", "cpu", nil})
	if got2 != want2 {
		t.Fatalf("smrfhdl data mismatch: %s", got2)
	}
}

func TestGradioErrorEventNotQuota(t *testing.T) {
	calls := &atomic.Int32{}
	srv := fakeGradioFull(t, "error", calls)
	defer srv.Close()
	d := Desc{ID: "hf-y", Base: srv.URL, API: "synthesize", DataStyle: "template", DefaultVoice: "V"}
	_, err := synthGradio(context.Background(), newHTTP(), d, Request{Text: "x"}, nil)
	if err == nil || IsQuota(err) {
		t.Fatalf("expected non-quota error, got %v", err)
	}
}

func TestChainFallbackOrderRealPath(t *testing.T) {
	srvV := fakeVieneu(t, "quota")
	defer srvV.Close()
	calls := &atomic.Int32{}
	srvE := fakeGradioFull(t, "error", calls)
	defer srvE.Close()
	calls2 := &atomic.Int32{}
	srvOK := fakeGradioFull(t, "ok", calls2)
	defer srvOK.Close()

	c := testChain()
	c.reg = []Desc{
		{ID: "p1", Label: "P1", Kind: "vieneuio", Base: srvV.URL, API: "/api/tts/demo", DefaultVoice: "A"},
		{ID: "p2", Label: "P2", Kind: "gradio", Base: srvE.URL, API: "synthesize", DataStyle: "template", DefaultVoice: "V"},
		{ID: "p3", Label: "P3", Kind: "gradio", Base: srvOK.URL, API: "synthesize", DataStyle: "template", DefaultVoice: "V"},
	}
	var phases []string
	res, err := c.Synthesize(context.Background(), newHTTP(), Request{Text: "test"}, func(e Event) {
		phases = append(phases, e.Phase+"@"+e.ProviderID)
	})
	if err != nil {
		t.Fatalf("expected success via p3, got %v (phases=%v)", err, phases)
	}
	if res.ProviderID != "p3" || res.Tried != 3 {
		t.Fatalf("unexpected result provider=%s tried=%d", res.ProviderID, res.Tried)
	}
	if c.snap.Providers["p1"].Status != "quota" || c.snap.Providers["p1"].ExhaustedUntil != c.today() {
		t.Fatalf("p1 should be exhausted today: %+v", c.snap.Providers["p1"])
	}
	if c.snap.LastGood != "p3" {
		t.Fatalf("lastGood should be p3")
	}
	joined := strings.Join(phases, ",")
	if !strings.Contains(joined, "quota@p1") || !strings.Contains(joined, "fail@p2") || !strings.Contains(joined, "done@p3") {
		t.Fatalf("missing phase events: %v", phases)
	}

	// lần 2: p1 cạn lượt (skip hết ngày), p2 cooldown 5 phút → nhảy thẳng p3
	res2, err := c.Synthesize(context.Background(), newHTTP(), Request{Text: "test"}, nil)
	if err != nil || res2.ProviderID != "p3" || res2.Tried != 1 {
		t.Fatalf("second run should hit p3 directly, got provider=%s tried=%d err=%v", res2.ProviderID, res2.Tried, err)
	}
}

func TestChainWakeRetry(t *testing.T) {
	oldDelay := WakeRetryDelay
	WakeRetryDelay = 30 * time.Millisecond
	defer func() { WakeRetryDelay = oldDelay }()

	var posts atomic.Int32
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()
	wh := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if posts.Add(1) == 1 {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte("starting"))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"event_id": "ev-ok"})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: complete\ndata: " +
			`[{"url":"` + srv.URL + `/file.wav","meta":{"_type":"gradio.FileData"}}]` + "\n\n"))
	}
	mux.HandleFunc("/gradio_api/call/synthesize", wh)
	mux.HandleFunc("/gradio_api/call/synthesize/", wh)
	mux.HandleFunc("/file.wav", func(w http.ResponseWriter, r *http.Request) {
		// PATCH FIX59: serve WAV hợp lệ (chuỗi giờ lọc chất lượng thật)
		_, _ = w.Write(fakeSpeechWav())
	})

	c := testChain()
	c.reg = []Desc{{ID: "sleepy", Label: "Sleepy", Kind: "gradio", Base: srv.URL, API: "synthesize", DataStyle: "template", DefaultVoice: "V"}}
	res, err := c.Synthesize(context.Background(), newHTTP(), Request{Text: "x"}, nil)
	if err != nil || posts.Load() != 2 {
		t.Fatalf("wake retry should succeed after 1 retry: err=%v posts=%d", err, posts.Load())
	}
	if len(res.Audio) < 4 || string(res.Audio[:4]) != "RIFF" {
		t.Fatalf("unexpected audio %dB", len(res.Audio))
	}
}

func TestChainArenaSkipped(t *testing.T) {
	srv := fakeVieneu(t, "ok")
	defer srv.Close()
	c := testChain()
	c.reg = []Desc{
		{ID: "arena-x", Label: "arena", Kind: "arena", Base: srv.URL, SkipReason: "chỉ bỏ phiếu"},
		{ID: "p1", Label: "P1", Kind: "vieneuio", Base: srv.URL, API: "/api/tts/demo", DefaultVoice: "A"},
	}
	res, err := c.Synthesize(context.Background(), newHTTP(), Request{Text: "x"}, nil)
	if err != nil || res.ProviderID != "p1" {
		t.Fatalf("arena should be skipped, got %v %v", res.ProviderID, err)
	}
	if c.snap.Providers["arena-x"].Status != "skip" {
		t.Fatalf("arena status should be skip")
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	c := testChain()
	c.snap.Providers["x"] = &PState{Status: "quota", ExhaustedUntil: c.today(), CountToday: 3, Day: c.today()}
	j := c.SnapshotJSON()
	c2 := NewChain(j)
	c2.nowFn = c.nowFn // PATCH FIX55: ghim cả đồng hồ của bản khôi phục —
	// nếu không, chạy test sang ngày khác sẽ đụng bộ reset theo ngày
	// (đúng hành vi production: hạn mức theo ngày phải reset) và test vỡ.
	if !c2.exhausted("x") {
		t.Fatalf("exhaustion should survive round-trip: %s", j)
	}
	st := c2.StatusOf("x")
	if st.CountToday != 3 || st.Status != "quota" {
		t.Fatalf("state mismatch: %+v", st)
	}
}

func TestRegistryShapeAndVoiceResolve(t *testing.T) {
	reg := Registry()
	// PATCH FIX56: 12 dịch vụ — Smrfhdl bị gỡ (cần đăng nhập HF);
	// DevTam05 + hongqminh được NÂNG LÊN ĐẦU chuỗi (probe 2026-09-21).
	// PATCH FIX57: 13 dịch vụ — thêm hf-nguyenduc1222 (dự phòng cùng họ
	// hongqminh, cpu-basic; probe 05:04 complete 1,6s).
	// PATCH FIX58: 14 dịch vụ — edge-tts (Microsoft Edge TTS trực tiếp,
	// miễn phí không giới hạn) đứng ĐẦU chuỗi làm xương sống.
	// PATCH FIX59: vẫn 14 dịch vụ nhưng hf-nguyenduc1222 DỜI XUỐNG CUỐI
	// (đang trả audio rè vô nghĩa — probe60 11:02 2026-09-21).
	if len(reg) != 14 {
		t.Fatalf("registry must have exactly 14 providers, got %d", len(reg))
	}
	if reg[0].ID != "edge-tts" || reg[0].Kind != "edge" {
		t.Fatalf("first provider must be edge-tts (kind=edge), got %s/%s", reg[0].ID, reg[0].Kind)
	}
	if reg[1].ID != "vieneu-io" || reg[2].ID != "hf-devtam05" || reg[3].ID != "hf-hongqminh" {
		t.Fatalf("wrong head order: %s %s %s", reg[1].ID, reg[2].ID, reg[3].ID)
	}
	if reg[4].ID != "hf-pnnbao-ump" || reg[5].ID != "arena-thomcles" {
		t.Fatalf("wrong order: %s %s", reg[4].ID, reg[5].ID)
	}
	if reg[12].ID != "hf-tuananh20015" || reg[13].ID != "hf-nguyenduc1222" {
		t.Fatalf("tail order wrong (FIX59: nguyenduc1222 cuối chuỗi): %s %s", reg[12].ID, reg[13].ID)
	}
	for _, d := range reg {
		if d.ID == "hf-smrfhdl" {
			t.Fatalf("Smrfhdl phải bị gỡ khỏi chuỗi (cần đăng nhập HF)")
		}
	}
	if reg[5].SkipReason == "" {
		t.Fatalf("arena must carry SkipReason")
	}
	// PATCH FIX58: edge-tts khai báo ĐÚNG 2 giọng Việt neural của Edge.
	eg := reg[0]
	if len(eg.Voices) != 2 || !eg.HasVoice("Hoài My (Nữ)") || !eg.HasVoice("Nam Minh (Nam)") {
		t.Fatalf("edge-tts voices wrong: %+v", eg.Voices)
	}
	for _, d := range reg {
		if d.ID == "" || d.Label == "" || d.Base == "" {
			t.Fatalf("incomplete desc: %+v", d)
		}
	}
	d := Registry()[7] // eagle0019 (FIX58: lùi 1 vì thêm edge-tts; FIX59 giữ nguyên vị trí 7)
	if len(d.Voices) == 0 || d.Voices[0].Name == "" {
		t.Fatalf("eagle0019 must have probed voices")
	}
	v, changed := d.ResolveVoice("trúc ly")
	if changed {
		t.Fatalf("case-insensitive match should hit, got %q changed=%v", v, changed)
	}
	v, _ = d.ResolveVoice("giọng lạ hoàn toàn")
	if v != d.DefaultVoice {
		t.Fatalf("unknown voice should fallback to default, got %q", v)
	}
	// PATCH FIX56: vieneu.io = 10 featured + 23 giọng app − 1 trùng
	// ("Anh Khôi") = 32; đồng thời Base phải là api.vieneu.io (host mới)
	vi := Registry()[1]
	if len(vi.Voices) != 32 {
		t.Fatalf("vieneu.io catalog must be 32 (10 featured + 23 app − 1 dup), got %d", len(vi.Voices))
	}
	if vi.Base != "https://api.vieneu.io" {
		t.Fatalf("vieneu.io Base must be api.vieneu.io (API dời subdomain), got %s", vi.Base)
	}
	seen := map[string]bool{}
	for _, v := range vi.Voices {
		if seen[v.Name] {
			t.Fatalf("vieneu.io catalog trùng tên: %s", v.Name)
		}
		seen[v.Name] = true
	}
	// 8 giọng OD phải nằm trong catalog vieneu.io (đưa dịch vụ #1 vào
	// chuỗi cho mọi giọng OD)
	for _, od := range ODVoices() {
		if !vi.HasVoice(od.Name) {
			t.Fatalf("giọng OD %q thiếu trong catalog vieneu.io", od.Name)
		}
	}
}

func TestParseSSE(t *testing.T) {
	var names []string
	parseSSE(strings.NewReader("event: heartbeat\ndata: null\n\nevent: complete\ndata: [1,2]\n\n"), func(e gradioEvent) {
		names = append(names, e.name+":"+e.data)
	})
	if len(names) != 2 || names[0] != "heartbeat:null" || names[1] != "complete:[1,2]" {
		t.Fatalf("parseSSE broken: %v", names)
	}
}

// ─── PATCH FIX55: lọc chuỗi theo giọng + bất biến bộ 8 giọng OD ─────────

// TestChainVoiceFilter: chọn giọng mà CHỈ 1 dịch vụ khai báo → chuỗi phải
// bỏ qua hoàn toàn các dịch vụ khác (0 request) và ghé đúng dịch vụ đó.
func TestChainVoiceFilter(t *testing.T) {
	callsYes := &atomic.Int32{}
	srvYes := fakeGradioFull(t, "ok", callsYes)
	defer srvYes.Close()
	callsNo := &atomic.Int32{}
	srvNo := fakeGradioFull(t, "ok", callsNo)
	defer srvNo.Close()

	c := testChain()
	c.reg = []Desc{
		{ID: "no1", Label: "No1", Kind: "gradio", Base: srvNo.URL, API: "synthesize",
			DataStyle: "template", DefaultVoice: "V"},
		{ID: "yes", Label: "Yes", Kind: "gradio", Base: srvYes.URL, API: "synthesize",
			DataStyle: "template", DefaultVoice: "V",
			Voices: []Voice{{Name: "Ngọc Linh"}}},
	}
	var infos []string
	res, err := c.Synthesize(context.Background(), newHTTP(),
		Request{Text: "test", VoiceName: "Ngọc Linh"}, func(e Event) {
			if e.Phase == "info" {
				infos = append(infos, e.Message)
			}
		})
	if err != nil || res.ProviderID != "yes" {
		t.Fatalf("expected via 'yes', got provider=%s err=%v", res.ProviderID, err)
	}
	if callsYes.Load() != 1 || callsNo.Load() != 0 {
		t.Fatalf("voice filter broken: yes=%d no=%d (phải là 1/0)", callsYes.Load(), callsNo.Load())
	}
	if len(infos) != 1 || !strings.Contains(infos[0], "1/2") {
		t.Fatalf("thiếu event info lọc giọng: %v", infos)
	}
}

// TestChainVoiceUnknownFailsFast (PATCH FIX56 — đổi hành vi từ FIX55):
// giọng lạ (không nằm trong catalog của dịch vụ nào) → BÁO LỖI NGAY,
// không gọi request nào. Lý do: chạy chuỗi với "giọng gần đúng" (mặc định
// của từng dịch vụ) khiến người dùng nghe SAI giọng mà không hay biết.
func TestChainVoiceUnknownFailsFast(t *testing.T) {
	calls := &atomic.Int32{}
	srv := fakeGradioFull(t, "ok", calls)
	defer srv.Close()

	c := testChain()
	c.reg = []Desc{
		{ID: "first", Label: "First", Kind: "gradio", Base: srv.URL, API: "synthesize",
			DataStyle: "template", DefaultVoice: "V"},
		{ID: "second", Label: "Second", Kind: "gradio", Base: srv.URL, API: "synthesize",
			DataStyle: "template", DefaultVoice: "V",
			Voices: []Voice{{Name: "Ngọc Linh"}}},
	}
	var fails []string
	_, err := c.Synthesize(context.Background(), newHTTP(),
		Request{Text: "test", VoiceName: "Giọng Lạ"}, func(e Event) {
			if e.Phase == "fail" {
				fails = append(fails, e.Message)
			}
		})
	if err == nil {
		t.Fatalf("giọng lạ phải báo lỗi (không chạy chuỗi sai giọng)")
	}
	if calls.Load() != 0 {
		t.Fatalf("không được gọi request nào với giọng lạ, got %d", calls.Load())
	}
	if len(fails) != 1 || !strings.Contains(fails[0], "không có trên bất kỳ dịch vụ online nào") {
		t.Fatalf("thiếu event fail giải thích: %v", fails)
	}
	if !strings.Contains(err.Error(), "8 giọng OD") {
		t.Fatalf("thông điệp lỗi phải gợi ý giọng OD: %v", err)
	}
}

// TestChainFilteredAllFailHint (PATCH FIX56): giọng CHỈ có trên dịch vụ
// đang lỗi → lỗi cuối phải gợi ý rõ "chọn giọng khác / thử lại sau".
func TestChainFilteredAllFailHint(t *testing.T) {
	calls := &atomic.Int32{}
	srv := fakeGradioFull(t, "error", calls)
	defer srv.Close()

	c := testChain()
	c.reg = []Desc{
		{ID: "dead", Label: "Dead", Kind: "gradio", Base: srv.URL, API: "synthesize",
			DataStyle: "template", DefaultVoice: "V",
			Voices: []Voice{{Name: "Thái Sơn"}}},
		{ID: "novoice", Label: "NoVoice", Kind: "gradio", Base: srv.URL, API: "synthesize",
			DataStyle: "template", DefaultVoice: "V"},
	}
	_, err := c.Synthesize(context.Background(), newHTTP(),
		Request{Text: "test", VoiceName: "Thái Sơn"}, nil)
	if err == nil {
		t.Fatalf("chuỗi phải lỗi khi dịch vụ duy nhất có giọng hỏng")
	}
	if !strings.Contains(err.Error(), "Thái Sơn") || !strings.Contains(err.Error(), "chọn giọng khác") {
		t.Fatalf("lỗi cuối thiếu gợi ý FIX56: %v", err)
	}
	// PATCH FIX59: dịch vụ KHÔNG có giọng không được bị gọi (bộ lọc vẫn
	// hoạt động); tổng request = lượt 1 (1) + lượt thử lại tự động (1) = 2.
	if calls.Load() != 2 {
		t.Fatalf("chỉ được gọi dịch vụ có giọng (1 ở lượt 1 + 1 ở lượt thử lại), got %d", calls.Load())
	}
}

// TestVieneuIOVoiceNotAvailable (PATCH FIX56): 400 + "is not available"
// phải trả thông điệp TẠM THỜI rõ ràng, KHÔNG phải quota.
func TestVieneuIOVoiceNotAvailable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/tts/demo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"statusCode":400,"message":"Voice \"Thái Sơn\" is not available","error":"Bad Request"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	d := Desc{ID: "vieneu-io", Label: "vieneu.io", Base: srv.URL, API: "/api/tts/demo", DefaultVoice: "A"}
	_, err := synthVieneuIO(context.Background(), newHTTP(), d, Request{Text: "x", VoiceName: "Thái Sơn"})
	if err == nil {
		t.Fatalf("phải lỗi khi demo từ chối giọng")
	}
	if IsQuota(err) {
		t.Fatalf("không được xếp là quota: %v", err)
	}
	if !strings.Contains(err.Error(), "chưa có trên demo lúc này") {
		t.Fatalf("thiếu thông điệp tạm thời FIX56: %v", err)
	}
}

// TestODVoicesInvariant: đủ 8 giọng, không trùng, mỗi giọng phải tồn tại
// trong catalog của ≥1 dịch vụ thật (nếu không UI sẽ ghim giọng "ma"),
// đủ ma trận vùng (Bắc 3 · Trung 2 · Nam 3) và ≥2 giọng có trên dịch vụ CPU.
func TestODVoicesInvariant(t *testing.T) {
	ods := ODVoices()
	if len(ods) != 8 {
		t.Fatalf("bộ OD phải đúng 8 giọng, got %d", len(ods))
	}
	seen := map[string]bool{}
	cntRegion := map[string]int{}
	for _, od := range ods {
		if seen[od.Name] {
			t.Fatalf("trùng tên OD: %s", od.Name)
		}
		seen[od.Name] = true
		cntRegion[od.Region]++
	}
	if cntRegion["Bắc"] != 3 || cntRegion["Trung"] != 2 || cntRegion["Nam"] != 3 {
		t.Fatalf("sai ma trận vùng: %v", cntRegion)
	}
	reg := Registry()
	for _, od := range ods {
		n := 0
		for _, d := range reg {
			if d.HasVoice(od.Name) {
				n++
			}
		}
		if n == 0 {
			t.Fatalf("giọng OD %q không nằm trên dịch vụ nào trong Registry", od.Name)
		}
	}
	cpu := 0
	for _, od := range ods {
		for _, d := range reg {
			if (d.ID == "hf-eagle0019" || d.ID == "hf-tuananh20015") && d.HasVoice(od.Name) {
				cpu++
				break
			}
		}
	}
	if cpu < 2 {
		t.Fatalf("cần ≥2 giọng OD có trên dịch vụ CPU (eagle0019/Tuananh20015), got %d", cpu)
	}
}

// TestChainBeginUserRunClearsSkip (PATCH FIX57): sau 1 lần hỏng, lần chạy
// kế tiếp bị cooldown bỏ qua (chính là case "(đã thử 0)" user gặp); sau
// BeginUserRun (user chủ động bấm Chuyển đổi/Làm mới lại) chuỗi phải GỌI
// THẬT trở lại vì space ZeroGPU hồi phục theo cơn.
func TestChainBeginUserRunClearsSkip(t *testing.T) {
	calls := &atomic.Int32{}
	srv := fakeGradioFull(t, "error", calls)
	defer srv.Close()

	c := testChain()
	c.reg = []Desc{
		{ID: "dead", Label: "Dead", Kind: "gradio", Base: srv.URL, API: "synthesize",
			DataStyle: "template", DefaultVoice: "V"},
	}
	// Run 1: lỗi thường → cooldown 5 phút. PATCH FIX59: 1 chuỗi hỏng =
	// lượt 1 (1 request) + lượt thử lại tự động (1 request) = 2 request.
	if _, err1 := c.Synthesize(context.Background(), newHTTP(), Request{Text: "x"}, nil); err1 == nil {
		t.Fatalf("run 1 phải lỗi")
	}
	if calls.Load() != 2 {
		t.Fatalf("run 1 = 2 request (lượt 1 + lượt thử lại), got %d", calls.Load())
	}
	// Run 2: đang cooldown → bị bỏ qua, không request mới ("đã thử 0")
	before := calls.Load()
	if _, err2 := c.Synthesize(context.Background(), newHTTP(), Request{Text: "x"}, nil); err2 == nil {
		t.Fatalf("run 2 phải lỗi (bỏ qua do cooldown)")
	}
	if got := calls.Load() - before; got != 0 {
		t.Fatalf("run 2 không được gọi thêm (cooldown): + %d", got)
	}
	// BeginUserRun (PATCH FIX57) → phải gọi thật trở lại (2 request/run)
	c.BeginUserRun()
	if _, err3 := c.Synthesize(context.Background(), newHTTP(), Request{Text: "x"}, nil); err3 == nil {
		t.Fatalf("run 3 vẫn phải lỗi (fake vẫn error) nhưng phải được GỌI")
	}
	if got := calls.Load() - before; got != 2 {
		t.Fatalf("sau BeginUserRun phải gọi thật thêm 2 lần (2 lượt), got +%d", got)
	}
}

// TestChainVoiceNotAvailNoCooldown (PATCH FIX57): vieneu.io 400
// "is not available" → trạng thái err nhưng KHÔNG cooldown — dịch vụ còn
// sống, lần chạy sau (giọng khác) vẫn phải dùng được ngay.
func TestChainVoiceNotAvailNoCooldown(t *testing.T) {
	calls := &atomic.Int32{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/tts/demo", func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"statusCode":400,"message":"Voice \"Thái Sơn\" is not available","error":"Bad Request"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := testChain()
	c.reg = []Desc{
		{ID: "vieneu-io", Label: "vieneu.io", Kind: "vieneuio", Base: srv.URL,
			API: "/api/tts/demo", DefaultVoice: "A",
			// có giọng trong catalog để đi qua bộ lọc giọng của chuỗi
			Voices: []Voice{{Name: "Thái Sơn"}}},
	}
	_, err := c.Synthesize(context.Background(), newHTTP(), Request{Text: "x", VoiceName: "Thái Sơn"}, nil)
	if err == nil {
		t.Fatalf("phải lỗi khi demo từ chối giọng")
	}
	if !IsVoiceNotAvailable(err) {
		t.Fatalf("lỗi phải mang sentinel voiceNotAvailErr: %v", err)
	}
	if st := c.StatusOf("vieneu-io"); st.CooldownUntil != 0 {
		t.Fatalf("giọng-tạm-chưa-có KHÔNG được đặt cooldown (PATCH FIX57), got %d", st.CooldownUntil)
	}
	// lần 2 vẫn được gọi thật (không bị skip oan)
	_, _ = c.Synthesize(context.Background(), newHTTP(), Request{Text: "x", VoiceName: "Thái Sơn"}, nil)
	if calls.Load() != 2 {
		t.Fatalf("lần 2 phải được gọi thật (2 request), got %d", calls.Load())
	}
}

// ─── PATCH FIX59: bộ lọc chất lượng audio + lượt thử lại tự động ─────────

// TestChainNoiseAudioSkipped: dịch vụ trả WAV hợp lệ nhưng nội dung NHIỄU
// TRẮNG (đúng kiểu nguyenduc1222 — thủ phạm "giọng rè rè vô nghĩa") phải
// bị coi là THẤT BẠI → chuỗi tự nhảy dịch vụ kế, không giao tiếng rè cho
// người dùng. Audio-rác KHÔNG được thử lại ở lượt 2.
func TestChainNoiseAudioSkipped(t *testing.T) {
	oldDelay := RetryPassDelay
	RetryPassDelay = time.Millisecond
	defer func() { RetryPassDelay = oldDelay }()

	callsBad := &atomic.Int32{}
	srvBad := fakeGradioAudio(t, callsBad, fakeNoiseWav)
	defer srvBad.Close()
	callsGood := &atomic.Int32{}
	srvGood := fakeGradioAudio(t, callsGood, fakeSpeechWav)
	defer srvGood.Close()

	c := testChain()
	c.reg = []Desc{
		{ID: "noisy", Label: "Noisy", Kind: "gradio", Base: srvBad.URL, API: "synthesize",
			DataStyle: "template", DefaultVoice: "V"},
		{ID: "clean", Label: "Clean", Kind: "gradio", Base: srvGood.URL, API: "synthesize",
			DataStyle: "template", DefaultVoice: "V"},
	}
	var fails []string
	res, err := c.Synthesize(context.Background(), newHTTP(), Request{Text: "test"}, func(e Event) {
		if e.Phase == "fail" {
			fails = append(fails, e.ProviderID+": "+e.Message)
		}
	})
	if err != nil || res.ProviderID != "clean" {
		t.Fatalf("chuỗi phải bỏ qua dịch vụ rè và thành công qua clean: err=%v provider=%s", err, res.ProviderID)
	}
	// PCM đã decode phải được đưa vào Result (không decode lại lần 2)
	if len(res.Samples) == 0 || res.SR <= 0 {
		t.Fatalf("Result.Samples/SR phải được điền sau lọc chất lượng (SR=%d)", res.SR)
	}
	// fail event phải nói rõ nguyên nhân "rè vô nghĩa"
	joined := strings.Join(fails, "|")
	if !strings.Contains(joined, "noisy") || !strings.Contains(joined, "rè vô nghĩa") {
		t.Fatalf("fail event phải ghi rõ audio rè vô nghĩa: %v", fails)
	}
	// trạng thái: noisy = err, clean = ok
	if st := c.StatusOf("noisy"); st.Status != "err" {
		t.Fatalf("noisy phải ở trạng thái err: %+v", st)
	}
	if c.snap.LastGood != "clean" {
		t.Fatalf("lastGood phải là clean, got %s", c.snap.LastGood)
	}
	// audio-rác KHÔNG vào lượt thử lại: noisy bị gọi đúng 1 lần
	if callsBad.Load() != 1 {
		t.Fatalf("dịch vụ trả rè chỉ được gọi 1 lần (không thử lại), got %d", callsBad.Load())
	}
	if callsGood.Load() != 1 {
		t.Fatalf("clean chỉ được gọi 1 lần (thành công lượt 1), got %d", callsGood.Load())
	}
}

// TestChainJunkAudioSkipped: byte rác (không phải WAV/MP3) cũng phải bị
// coi là thất bại của dịch vụ → chuỗi nhảy tiếp, không chết ở bước decode.
func TestChainJunkAudioSkipped(t *testing.T) {
	oldDelay := RetryPassDelay
	RetryPassDelay = time.Millisecond
	defer func() { RetryPassDelay = oldDelay }()

	junk := []byte("junk-not-audio-at-all")
	callsBad := &atomic.Int32{}
	srvBad := fakeGradioAudio(t, callsBad, func() []byte { return junk })
	defer srvBad.Close()
	callsGood := &atomic.Int32{}
	srvGood := fakeGradioAudio(t, callsGood, fakeSpeechWav)
	defer srvGood.Close()

	c := testChain()
	c.reg = []Desc{
		{ID: "junk", Label: "Junk", Kind: "gradio", Base: srvBad.URL, API: "synthesize",
			DataStyle: "template", DefaultVoice: "V"},
		{ID: "clean", Label: "Clean", Kind: "gradio", Base: srvGood.URL, API: "synthesize",
			DataStyle: "template", DefaultVoice: "V"},
	}
	res, err := c.Synthesize(context.Background(), newHTTP(), Request{Text: "test"}, nil)
	if err != nil || res.ProviderID != "clean" {
		t.Fatalf("byte rác phải bị bỏ qua, thành công qua clean: err=%v provider=%s", err, res.ProviderID)
	}
	if callsBad.Load() != 1 {
		t.Fatalf("junk chỉ được gọi 1 lần (không thử lại), got %d", callsBad.Load())
	}
}

// TestChainRetryPassSecondChance (PATCH FIX59): cả lượt 1 thất bại vì lỗi
// thoáng qua → chuỗi TỰ thử lại 1 lượt; server flaky nhận ở lượt 2.
// Bằng chứng thực tế (probe58b): cùng space cùng phút, 1 request OK request
// sau lỗi — 1 lượt thử lại tăng gấp đôi cơ hội trúng cửa sổ hồi phục.
func TestChainRetryPassSecondChance(t *testing.T) {
	oldDelay := RetryPassDelay
	RetryPassDelay = time.Millisecond
	defer func() { RetryPassDelay = oldDelay }()

	var posts atomic.Int32
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()
	wh := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			n := posts.Add(1)
			if n <= 2 { // lượt 1: p1 và p2 đều lỗi
				_ = json.NewEncoder(w).Encode(map[string]string{"event_id": "ev-err"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"event_id": "ev-ok"})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if strings.HasSuffix(r.URL.Path, "ev-err") {
			_, _ = w.Write([]byte("event: error\ndata: null\n\n"))
			return
		}
		_, _ = w.Write([]byte("event: complete\ndata: " +
			`[{"url":"` + srv.URL + `/audio.wav","meta":{"_type":"gradio.FileData"}}]` + "\n\n"))
	}
	mux.HandleFunc("/gradio_api/call/synthesize", wh)
	mux.HandleFunc("/gradio_api/call/synthesize/", wh)
	mux.HandleFunc("/audio.wav", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fakeSpeechWav())
	})

	c := testChain()
	c.reg = []Desc{
		{ID: "p1", Label: "P1", Kind: "gradio", Base: srv.URL, API: "synthesize",
			DataStyle: "template", DefaultVoice: "V"},
		{ID: "p2", Label: "P2", Kind: "gradio", Base: srv.URL, API: "synthesize",
			DataStyle: "template", DefaultVoice: "V"},
	}
	var infos []string
	res, err := c.Synthesize(context.Background(), newHTTP(), Request{Text: "test"}, func(e Event) {
		if e.Phase == "info" {
			infos = append(infos, e.Message)
		}
	})
	if err != nil || res.ProviderID != "p1" {
		t.Fatalf("lượt 2 phải cứu được chuỗi qua p1: err=%v provider=%s", err, res.ProviderID)
	}
	if res.Tried != 3 { // 2 lượt 1 + 1 lượt 2 (p1 nhận đầu tiên)
		t.Fatalf("tried phải là 3 (2 lượt 1 + p1 lượt 2), got %d", res.Tried)
	}
	if posts.Load() != 3 {
		t.Fatalf("tổng POST phải là 3, got %d", posts.Load())
	}
	found := false
	for _, m := range infos {
		if strings.Contains(m, "thử lại") {
			found = true
		}
	}
	if !found {
		t.Fatalf("thiếu event info lượt thử lại: %v", infos)
	}
}
