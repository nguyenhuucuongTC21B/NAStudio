package cloud

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

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
			_ = json.NewEncoder(w).Encode(map[string]string{
				"audioBase64": base64.StdEncoding.EncodeToString([]byte("RIFF-fake-audio")),
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
		_, _ = w.Write([]byte("RIFF-fake-space-audio"))
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
	if string(res.Audio) != "RIFF-fake-audio" || res.MIME != "audio/wav" || res.VoiceUsed != "Adam Tốp Tốp" {
		t.Fatalf("unexpected result: %+v", res)
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
	if string(res.Audio) != "RIFF-fake-space-audio" {
		t.Fatalf("unexpected audio: %q", res.Audio)
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
		_, _ = w.Write([]byte("RIFF-wake-audio"))
	})

	c := testChain()
	c.reg = []Desc{{ID: "sleepy", Label: "Sleepy", Kind: "gradio", Base: srv.URL, API: "synthesize", DataStyle: "template", DefaultVoice: "V"}}
	res, err := c.Synthesize(context.Background(), newHTTP(), Request{Text: "x"}, nil)
	if err != nil || posts.Load() != 2 {
		t.Fatalf("wake retry should succeed after 1 retry: err=%v posts=%d", err, posts.Load())
	}
	if string(res.Audio) != "RIFF-wake-audio" {
		t.Fatalf("unexpected audio %q", res.Audio)
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
	// PATCH FIX54: 10 dịch vụ gốc + 3 dịch vụ CPU mới = 13
	if len(reg) != 13 {
		t.Fatalf("registry must have exactly 13 providers, got %d", len(reg))
	}
	if reg[10].ID != "hf-tuananh20015" || reg[11].ID != "hf-hongqminh" || reg[12].ID != "hf-devtam05" {
		t.Fatalf("wrong tail order: %s %s %s", reg[10].ID, reg[11].ID, reg[12].ID)
	}
	if reg[0].ID != "vieneu-io" || reg[1].ID != "hf-pnnbao-ump" || reg[2].ID != "arena-thomcles" {
		t.Fatalf("wrong order: %s %s %s", reg[0].ID, reg[1].ID, reg[2].ID)
	}
	if reg[2].SkipReason == "" {
		t.Fatalf("arena must carry SkipReason")
	}
	for _, d := range reg {
		if d.ID == "" || d.Label == "" || d.Base == "" {
			t.Fatalf("incomplete desc: %+v", d)
		}
	}
	d := Registry()[4] // eagle0019
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
	// vieneu.io phải có đúng 10 giọng featured thật
	if len(Registry()[0].Voices) != 10 {
		t.Fatalf("vieneu.io featured must be 10, got %d", len(Registry()[0].Voices))
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

// TestChainVoiceUnknownFallsBack: giọng lạ (không có trong catalog nào) →
// giữ hành vi cũ: đi cả chuỗi theo thứ tự (dịch vụ đầu thành công là dừng).
func TestChainVoiceUnknownFallsBack(t *testing.T) {
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
	var infos []string
	res, err := c.Synthesize(context.Background(), newHTTP(),
		Request{Text: "test", VoiceName: "Giọng Lạ"}, func(e Event) {
			if e.Phase == "info" {
				infos = append(infos, e.Message)
			}
		})
	if err != nil || res.ProviderID != "first" {
		t.Fatalf("giọng lạ phải đi cả chuỗi (first thành công trước), got %s err=%v", res.ProviderID, err)
	}
	if len(infos) != 1 || !strings.Contains(infos[0], "Không dịch vụ nào khai báo") {
		t.Fatalf("thiếu cảnh báo giọng lạ: %v", infos)
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
