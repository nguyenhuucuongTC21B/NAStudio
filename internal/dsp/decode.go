package dsp

import (
	"bytes"
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
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, 0, err
	}
	return ReadAudioBytes(raw, filepath.Ext(path))
}

// PATCH FIX59 — ReadAudioBytes: như ReadAudioFile nhưng làm việc trên
// BỘ NHỚ. Chuỗi online dùng bản này để kiểm tra chất lượng audio NGAY khi
// nhận về (audio rè/rác phải bị coi là thất bại của dịch vụ đó và tự nhảy
// dịch vụ kế tiếp — không chờ tới bước phát mới phát hiện). ext chỉ dùng
// làm gợi ý dự phòng khi không nhận dạng được magic bytes (".wav").
func ReadAudioBytes(b []byte, ext string) ([]float32, int, int, error) {
	if len(b) >= 4 && string(b[:4]) == "RIFF" {
		return readWavBytes(b)
	}
	if len(b) >= 3 && string(b[:3]) == "ID3" {
		return readMP3From(bytes.NewReader(b))
	}
	if len(b) >= 2 && b[0] == 0xFF && (b[1]&0xE0) == 0xE0 {
		return readMP3From(bytes.NewReader(b))
	}
	// không nhận dạng được — thử WAV cho khớp hành vi cũ (đủ tên .wav)
	if strings.EqualFold(ext, ".wav") {
		return readWavBytes(b)
	}
	head := b
	if len(head) > 4 {
		head = head[:4]
	}
	return nil, 0, 0, fmt.Errorf("định dạng audio không nhận dạng được (head=% x)", head)
}

// readMP3From giải mã MP3 từ stream → mono float32 + sampleRate.
// (FIX59: tách từ readMP3 để decode được cả từ bộ nhớ — bytes.Reader
// cũng là io.Seeker nên go-mp3 hoạt động như với file.)
func readMP3From(r io.Reader) ([]float32, int, int, error) {
	dec, err := mp3dec.NewDecoder(r)
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

// readMP3 (giữ cho tương thích nội bộ) — giải mã MP3 từ file.
func readMP3(path string) ([]float32, int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer f.Close()
	return readMP3From(f)
}
