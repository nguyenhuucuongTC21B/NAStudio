package cloud

// e2e_manual_test.go — TEST THẬT mạng (không mock), chạy tay trước khi đóng gói rồi XÓA.
// go test ./internal/cloud/ -run TestManualChainE2E -v -timeout 12m

import (
	"context"
	"os"
	"testing"
)

func TestManualChainE2E(t *testing.T) {
	if os.Getenv("F54_E2E") == "" {
		t.Skip("chỉ chạy khi F54_E2E=1")
	}
	c := NewChain("")
	r, err := c.Synthesize(context.Background(), nil, Request{
		Text:      "Xin chào, đây là kiểm tra end-to-end FIX54.",
		VoiceName: "",
	}, func(e Event) {
		t.Logf("event %-8s %-22s %s pct=%.0f", e.Phase, e.ProviderID, e.Message, e.Pct)
	})
	if err != nil {
		t.Fatalf("E2E thất bại: %v", err)
	}
	if len(r.Audio) < 20000 {
		t.Fatalf("audio quá nhỏ: %d bytes", len(r.Audio))
	}
	t.Logf("E2E OK: provider=%s voice=%s bytes=%d info=%q", r.ProviderID, r.VoiceUsed, len(r.Audio), r.Info)
}
