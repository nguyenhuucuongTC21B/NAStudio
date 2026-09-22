package cloud

// e2e_fix59_test.go — PATCH FIX59: E2E mạng THẬT với đúng 2 kịch bản user
// báo lỗi 2026-09-21:
//  (1) giọng vùng miền "Ngọc (nữ miền Bắc)" — đường duy nhất nguyenduc1222
//      đang trả audio RÈ VÔ NGHIỆM: chuỗi phải TỪ CHỐI audio rè (bộ lọc
//      chất lượng), không bao giờ thành công với tiếng rè, và báo lỗi
//      trung thực sau khi đã thử cả lượt 1 + lượt thử lại tự động;
//  (2) "Hoài My (Nữ)" — xương sống edge-tts phải thành công nhanh.
// Chạy tay:
//   F59_E2E=1 go test ./internal/cloud/ -run TestManualFIX59E2E -v -timeout 14m
// (guard env — không chạy trong go test thường)
import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestManualFIX59E2E(t *testing.T) {
	if os.Getenv("F59_E2E") == "" {
		t.Skip("chỉ chạy khi F59_E2E=1")
	}

	// Kịch bản 1: đường rè (nguyenduc1222) phải bị chặn — kết quả là LỖI
	// trung thực, KHÔNG phải audio rè.
	t.Run("noise-gate: Ngọc (nữ miền Bắc)", func(t *testing.T) {
		c := NewChain("")
		c.BeginUserRun()
		done := make(chan error, 1)
		go func() {
			_, err := c.Synthesize(context.Background(), nil, Request{
				Text:      "Xin chào, đây là bài kiểm tra bộ lọc chất lượng audio.",
				VoiceName: "Ngọc (nữ miền Bắc)",
			}, func(e Event) {
				t.Logf("event %-8s %-22s %s", e.Phase, e.ProviderID, e.Message)
			})
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil {
				t.Fatalf("BOI LO: chuỗi thành công qua đường nguyenduc1222 — bộ lọc rè KHÔNG chặn được (user sẽ nghe tiếng rè)")
			}
			if !strings.Contains(err.Error(), "đã thử") {
				t.Fatalf("lỗi cuối phải trung thực về số lần thử: %v", err)
			}
			t.Logf("ĐÚNG KỲ VỌNG: audio rè bị chặn — lỗi cuối trung thực: %v", err)
		case <-time.After(6 * time.Minute):
			t.Fatalf("quá 6 phút")
		}
	})

	// Kịch bản 2: xương sống edge-tts — thành công nhanh, audio qua lọc.
	t.Run("edge: Hoài My (Nữ)", func(t *testing.T) {
		c := NewChain("")
		c.BeginUserRun()
		done := make(chan error, 1)
		var prov string
		go func() {
			r, err := c.Synthesize(context.Background(), nil, Request{
				Text:      "Xin chào, chế độ online của phần mềm đã hoạt động trở lại.",
				VoiceName: "Hoài My (Nữ)",
			}, func(e Event) {
				t.Logf("event %-8s %-22s %s", e.Phase, e.ProviderID, e.Message)
			})
			if err == nil {
				prov = r.ProviderID
			}
			done <- err
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("edge-tts phải thành công: %v", err)
			}
			if prov != "edge-tts" {
				t.Fatalf("kỳ vọng edge-tts nhận job, got %s", prov)
			}
			t.Logf("E2E OK: Hoài My qua %s", prov)
		case <-time.After(3 * time.Minute):
			t.Fatalf("quá 3 phút")
		}
	})
}
