package dsp

import (
	"os"
	"testing"
)

// TestReadAudioFileMP3Real (FIX56): file MP3 THẬT từ DevTam05 phải giải mã
// được (trước đây ReadWav chết trên MP3 → job online chết ở bước giải mã).
func TestReadAudioFileMP3Real(t *testing.T) {
	src := "/tmp/devtam05_sample.mp3"
	if _, err := os.Stat(src); err != nil {
		t.Skip("không có file MP3 mẫu — chỉ chạy tay khi đóng gói")
	}
	samples, sr, ch, err := ReadAudioFile(src)
	if err != nil {
		t.Fatalf("ReadAudioFile(MP3 thật) lỗi: %v", err)
	}
	if len(samples) < sr/2 { // ít nhất 0.5 giây
		t.Fatalf("audio quá ngắn: %d samples @%dHz", len(samples), sr)
	}
	if sr < 16000 || sr > 96000 {
		t.Fatalf("sample rate bất thường: %d", sr)
	}
	if ch != 1 {
		t.Fatalf("pipeline app cần mono, got %d", ch)
	}
	t.Logf("MP3 OK: %d samples (%.2fs) @%dHz mono", len(samples), float64(len(samples))/float64(sr), sr)
}
