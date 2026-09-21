package dsp

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	mp3dec "github.com/hajimehoshi/go-mp3"
)

// PATCH FIX56 — ReadAudioFile: decoder nhận biết ĐỊNH DẠNG cho audio nhận
// về từ dịch vụ online. Trước đây app chỉ gọi ReadWav, trong khi
// DevTam05/vieneu-tts (dịch vụ ổn định nhất đợt probe 2026-09-21) trả
// FILE MP3 → job chết ở bước "Giải mã audio" dù tổng hợp thành công.
//
// Phân loại theo magic bytes:
//   - "RIFF"                      → WAV (parser đầy đủ FIX46)
//   - "ID3" hoặc sync 0xFFEx      → MP3 (go-mp3, pure Go — không cgo)
//
// go-mp3 v0.3.4 luôn xuất PCM16 stereo interleaved kể cả nguồn mono →
// trộn (L+R)/2 về mono float32 để khớp pipeline của app.
func ReadAudioFile(path string) ([]float32, int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer f.Close()
	head := make([]byte, 4)
	n, _ := io.ReadFull(f, head)
	if n >= 4 && string(head[:4]) == "RIFF" {
		_ = f.Close()
		return ReadWav(path)
	}
	_ = f.Close()
	if n >= 3 && string(head[:3]) == "ID3" {
		return readMP3(path)
	}
	if n >= 2 && head[0] == 0xFF && (head[1]&0xE0) == 0xE0 {
		return readMP3(path)
	}
	// không nhận dạng được — thử WAV cho khớp hành vi cũ (đủ tên .wav)
	if strings.EqualFold(filepath.Ext(path), ".wav") {
		return ReadWav(path)
	}
	return nil, 0, 0, fmt.Errorf("định dạng audio không nhận dạng được (head=% x)", head[:n])
}

// readMP3 giải mã MP3 → mono float32 + sampleRate + channels(=1).
func readMP3(path string) ([]float32, int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer f.Close()
	dec, err := mp3dec.NewDecoder(f)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("mở MP3 lỗi: %w", err)
	}
	pcm, err := io.ReadAll(dec)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("giải mã MP3 lỗi: %w", err)
	}
	ns := len(pcm) / 4 // số frame stereo (2 sample × 2 byte)
	out := make([]float32, ns)
	for i := 0; i < ns; i++ {
		l := int16(binary.LittleEndian.Uint16(pcm[i*4:]))
		r := int16(binary.LittleEndian.Uint16(pcm[i*4+2:]))
		out[i] = (float32(l) + float32(r)) / 2 / 32768
	}
	sr := dec.SampleRate()
	if sr <= 0 {
		sr = 44100
	}
	return out, sr, 1, nil
}
