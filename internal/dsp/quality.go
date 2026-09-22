package dsp

import "math"

// PATCH FIX59 — BỘ LỌC CHẤT LƯỢNG AUDIO CHO CHUỖI ONLINE.
//
// Báo lỗi người dùng 2026-09-21: "có một số giọng tạo thành giọng rè rè
// vô nghĩa". Điều tra probe60 (scripts60/, 11:02) tìm ra thủ phạm: space
// nguyenduc1222/VieNeu-TTS trả WAV HỢP LỆ (header chuẩn, 24kHz, 3.0s)
// nhưng nội dung là NHIỄU TRẮNG THUẦN — buffer rác của model hỏng trên
// server (hai giọng khác nhau trả cùng độ dài 144044 byte, mẫu phân phối
// đều [-0.5, 0.5]). App trước đây chỉ kiểm tra "đọc được header không"
// nên nhận trọn tiếng rè.
//
// Thước đo phân loại (đo trên 15 file thật probe60 — 13 giọng thật,
// 2 file rè):
//
//	               speech          noise (nguyenduc1222)
//	rms_cv         0.67 – 0.99     0.01   (nhiễu đều đều, không modulation)
//	pause_ratio    0.20 – 0.41     0.00   (giọng nói luôn có ngắt nghỉ)
//	zcr            0.05 – 0.11     0.50   (nhiễu broadband lật dấu ~1/2)
//
// Ngưỡng chốt đặt ở GIỮA với dư địa rất rộng ưu tiên KHÔNG BÁO NHẦM
// giọng thật: cv < 0.25 VÀ pause < 3% VÀ zcr > 0.35 mới kết luận rè.
// Audio < 1 giây không đủ dữ kiện thống kê → không chặn (tránh oan
// đoạn ngắn). Im lặng tuyệt đối cũng là audio rác → chặn.
//
// Hạn mức được ghi nhận trung thực: bộ lọc NHIỄU BROADBAND + IM LẶNG;
// một dạng rác khác — "hum" đơn tông biên độ đều — thoát zcr (zcr thấp)
// nhưng vẫn bị hai điều kiện cv+pause siết; không thể phân biệt hum với
// giọng ngân nga mà không dùng FFT/khác — chấp nhận để giữ bộ lọc nhẹ,
// không phụ thuộc FFT.
func LooksLikeNoise(pcm []float32, sr int) bool {
	if sr <= 0 || len(pcm) < sr { // < 1s — không đủ dữ kiện, không chặn
		return false
	}
	// ZCR toàn cục: tỉ lệ mẫu lật dấu.
	flips := 0
	for i := 1; i < len(pcm); i++ {
		if (pcm[i] >= 0) != (pcm[i-1] >= 0) {
			flips++
		}
	}
	zcr := float64(flips) / float64(len(pcm)-1)

	// RMS theo khung 50ms.
	hop := sr / 20
	if hop < 1 {
		hop = 1
	}
	n := len(pcm) / hop
	if n < 8 {
		return false
	}
	rms := make([]float64, n)
	for k := 0; k < n; k++ {
		var s2 float64
		for j := 0; j < hop; j++ {
			v := float64(pcm[k*hop+j])
			s2 += v * v
		}
		rms[k] = math.Sqrt(s2 / float64(hop))
	}
	var mean float64
	maxR := 0.0
	for _, v := range rms {
		mean += v
		if v > maxR {
			maxR = v
		}
	}
	mean /= float64(n)
	if mean < 1e-5 { // im lặng gần tuyệt đối — audio rác
		return true
	}
	var vr float64
	for _, v := range rms {
		d := v - mean
		vr += d * d
	}
	cv := math.Sqrt(vr/float64(n)) / mean
	pause := 0
	for _, v := range rms {
		if v < maxR*0.10 {
			pause++
		}
	}
	pauseRatio := float64(pause) / float64(n)

	return cv < 0.25 && pauseRatio < 0.03 && zcr > 0.35
}
