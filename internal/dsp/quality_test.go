package dsp

import (
	"math"
	"math/rand"
	"testing"
)

// TestLooksLikeNoiseCalibrated (PATCH FIX59): bộ lọc chất lượng phải bắt
// được nhiễu trắng thuần (thủ phạm "giọng rè rè vô nghĩa" — nguyenduc1222
// trả WAV hợp lệ nhưng nội dung nhiễu), QUA được giọng mô phỏng có modulation
// + ngắt nghỉ, bắt im lặng tuyệt đối, và KHÔNG chặn audio quá ngắn.
func TestLooksLikeNoiseCalibrated(t *testing.T) {
	const sr = 24000
	n := sr * 3 / 2 // 1.5s

	// 1) nhiễu trắng đều [-0.5, 0.5] — giống hệt output probe60 bắt được
	rng := rand.New(rand.NewSource(59))
	noise := make([]float32, n)
	for i := range noise {
		noise[i] = float32(rng.Float64()*2-1) * 0.5
	}
	if !LooksLikeNoise(noise, sr) {
		t.Fatal("nhiễu trắng thuần phải bị bộ lọc bắt (thủ phạm rè rè vô nghĩa)")
	}

	// 2) giọng mô phỏng: 4 cụm âm tiết 180ms cách nhau 70ms lặng, sóng hài
	// 180Hz + 900Hz — đặc trưng giống speech thật (cv cao, pause rõ, zcr thấp)
	speech := make([]float32, n)
	for i := range speech {
		tt := float64(i) / sr
		amp := 0.0
		if int(tt*1000)%250 < 180 {
			amp = 0.6
		}
		speech[i] = float32(amp * (0.7*math.Sin(2*math.Pi*180*tt) + 0.3*math.Sin(2*math.Pi*900*tt)))
	}
	if LooksLikeNoise(speech, sr) {
		t.Fatal("giọng mô phỏng (có modulation + ngắt nghỉ) KHÔNG được báo nhầm là rè")
	}

	// 3) im lặng tuyệt đối = audio rác
	if !LooksLikeNoise(make([]float32, sr*2), sr) {
		t.Fatal("im lặng tuyệt đối phải bị bắt")
	}

	// 4) audio < 1s: không đủ dữ kiện thống kê → không chặn (tránh oan)
	if LooksLikeNoise(noise[:sr/2], sr) {
		t.Fatal("audio ngắn hơn 1s không được chặn (không đủ dữ kiện)")
	}
}
