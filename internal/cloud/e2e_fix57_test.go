package cloud

// e2e_fix57_test.go — PATCH FIX57: E2E mạng THẬT với đúng kịch bản user
// ("Quang Sơn" từng báo "(đã thử 0)"). Chạy tay:
//   F57_E2E=1 go test ./internal/cloud/ -run TestManualFIX57E2E -v -timeout 12m
// (guard env — không chạy trong go test thường)
import (
	"context"
	"os"
	"testing"
	"time"
)

func TestManualFIX57E2E(t *testing.T) {
	if os.Getenv("F57_E2E") == "" {
		t.Skip("chỉ chạy khi F57_E2E=1")
	}
	cases := []struct {
		voice string
		text  string
	}{
		{"Quang Sơn", "Chuỗi online đã được khắc phục, giọng Quang Sơn hoạt động trở lại."},
		{"Mai Anh", "Giọng Mai Anh miền Bắc đọc tin tức thử nghiệm thành công."},
		{"Ngọc (nữ miền Bắc)", "Xin chào, đây là giọng Ngọc nữ miền Bắc kiểm tra kết nối."},
	}
	for _, tc := range cases {
		t.Run(tc.voice, func(t *testing.T) {
			c := NewChain("")
			c.BeginUserRun() // PATCH FIX57: đúng luồng app.go mới
			done := make(chan error, 1)
			var (
				prov   string
				nbytes int
			)
			go func() {
				r, err := c.Synthesize(context.Background(), nil, Request{
					Text: tc.text, VoiceName: tc.voice,
				}, func(e Event) {
					t.Logf("event %-8s %-22s %s", e.Phase, e.ProviderID, e.Message)
				})
				if err == nil {
					prov, nbytes = r.ProviderID, len(r.Audio)
				}
				done <- err
			}()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("E2E %q thất bại: %v", tc.voice, err)
				}
				if nbytes < 20000 {
					t.Fatalf("audio quá nhỏ: %d bytes", nbytes)
				}
				t.Logf("E2E OK: voice=%s qua %s, %d bytes", tc.voice, prov, nbytes)
			case <-time.After(4 * time.Minute):
				t.Fatalf("E2E %q quá 4 phút", tc.voice)
			}
		})
	}
}
