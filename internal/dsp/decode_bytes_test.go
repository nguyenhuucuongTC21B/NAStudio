package dsp

import (
	"os"
	"testing"
)

// TestReadAudioBytes (PATCH FIX59): decode trên bộ nhớ phải cho kết quả
// GIỐNG NHAU với đường file (chuỗi online dùng bản bytes để lọc chất lượng
// ngay khi nhận audio). Byte rác phải lỗi rõ ràng, không panic.
func TestReadAudioBytes(t *testing.T) {
	// WAV mono 16-bit 24kHz 0.5s qua WriteWav → so sánh 2 đường decode
	const sr = 24000
	samples := make([]float32, sr/2)
	for i := range samples {
		samples[i] = float32(i%100) / 1000
	}
	path := t.TempDir() + "/t.wav"
	if err := WriteWav(path, samples, sr, 1); err != nil {
		t.Fatalf("WriteWav: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("đọc file test: %v", err)
	}
	got, gotSR, _, err := ReadAudioBytes(raw, ".wav")
	if err != nil {
		t.Fatalf("ReadAudioBytes(WAV) lỗi: %v", err)
	}
	want, wantSR, _, err := ReadAudioFile(path)
	if err != nil {
		t.Fatalf("ReadAudioFile lỗi: %v", err)
	}
	if len(got) != len(want) || gotSR != wantSR {
		t.Fatalf("bytes vs file lệch: %d samples @%d vs %d @%d", len(got), gotSR, len(want), wantSR)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("mẫu %d lệch: %v vs %v", i, got[i], want[i])
		}
	}

	// byte rác → lỗi rõ ràng (không panic)
	if _, _, _, err := ReadAudioBytes([]byte("junk-not-audio"), ""); err == nil {
		t.Fatal("byte rác phải báo lỗi")
	}

	// buffer rỗng → lỗi
	if _, _, _, err := ReadAudioBytes(nil, ""); err == nil {
		t.Fatal("buffer rỗng phải báo lỗi")
	}
}
