// Package cloud — PATCH FIX53: CHẾ ĐỘ SỬ DỤNG ONLINE.
//
// App đóng vai CẦU CHUYỂN TIẾP: văn bản (hoặc file .txt/.srt) của người
// dùng được gửi lần lượt tới các dịch vụ TTS online theo đúng thứ tự chủ
// app chỉ định. Dịch vụ nào hết lượt / lỗi / đang ngủ thì tự động chuyển
// sang dịch vụ kế tiếp, nhớ trạng thái để lần sau ưu tiên cái còn dùng được.
//
// Nguyên tắc:
//   - KHÔNG đụng engine offline (FIX46–FIX52 giữ nguyên 100%).
//   - KHÔNG lưu gì dữ liệu người dùng ở đâu khác ngoài phiên làm việc.
//   - Mọi provider là dịch vụ CÔNG KHAI (trang demo chính thức + HF Space
//     công khai); app chỉ gọi API public như một trình duyệt bình thường,
//     có giới hạn 1 job đồng thời và tôn trọng lỗi hạn mức (quota).
package cloud

import "time"

// Voice một giọng của một dịch vụ online.
type Voice struct {
	Name   string `json:"name"`
	Gender string `json:"gender,omitempty"`
}

// Desc mô tả tĩnh một dịch vụ trong chuỗi cầu nối.
type Desc struct {
	ID           string  `json:"id"`    // "vieneu-io", "hf-eagle0019", …
	Label        string  `json:"label"` // tên hiển thị UI
	Kind         string  `json:"kind"`  // vieneuio | gradio | arena
	Base         string  `json:"base"`  // URL gốc dịch vụ
	API          string  `json:"api"`   // tên endpoint gradio (synthesize)
	DataStyle    string  `json:"-"`     // template | smrfhdl — cách dựng mảng data
	DefaultVoice string  `json:"defaultVoice"`
	Voices       []Voice `json:"voices,omitempty"`
	SkipReason   string  `json:"skipReason,omitempty"` // khác rỗng → bị bỏ qua (arena)
	Note         string  `json:"note,omitempty"`
}

// HasVoice kiểm tra tên giọng có nằm trong catalog của dịch vụ.
func (d Desc) HasVoice(name string) bool {
	if name == "" {
		return false
	}
	for _, v := range d.Voices {
		if v.Name == name {
			return true
		}
	}
	return false
}

// ResolveVoice chọn giọng thực tế sẽ gửi: đúng tên → dùng; không có →
// default của dịch vụ. Trả về (giọng gửi, có đổi so với yêu cầu không).
func (d Desc) ResolveVoice(name string) (string, bool) {
	if name == "" {
		return d.DefaultVoice, false
	}
	if d.HasVoice(name) {
		return name, false
	}
	// so khớp không phân biệt hoa thường (giữ nguyên dấu tiếng Việt)
	for _, v := range d.Voices {
		if equalFold(v.Name, name) {
			return v.Name, false
		}
	}
	return d.DefaultVoice, true
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := lowerASCII(a[i]), lowerASCII(b[i])
		if ca != cb {
			return false
		}
	}
	return true
}

func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

// Request yêu cầu tổng hợp online.
type Request struct {
	Text      string `json:"text"`
	VoiceName string `json:"voiceName"` // "" = giọng mặc định của dịch vụ
}

// Result kết quả tổng hợp online thành công.
type Result struct {
	Audio         []byte  `json:"-"`
	MIME          string  `json:"mime"`
	ProviderID    string  `json:"providerId"`
	ProviderLabel string  `json:"providerLabel"`
	VoiceUsed     string  `json:"voiceUsed"`
	Info          string  `json:"info"`  // thống kê đi kèm (thời gian sinh, RTF…)
	Tried         int     `json:"tried"` // số dịch vụ đã thử
	DurationSec   float64 `json:"-"`     // người gọi điền sau khi decode WAV
}

// Event tiến trình phát lên UI trong lúc chạy chuỗi cầu nối.
type Event struct {
	Phase      string  `json:"phase"` // trying|waiting|skip|quota|fail|done
	ProviderID string  `json:"providerId,omitempty"`
	Label      string  `json:"label,omitempty"`
	Message    string  `json:"message"`
	Pct        float64 `json:"pct"`
	ElapsedSec float64 `json:"elapsedSec,omitempty"`
}

// Registry trả danh sách dịch vụ của chuỗi cầu nối.
//
// PATCH FIX56 — SẮP LẠI THỨ TỰ THEO KẾT QUẢ PROBE THẬT 2026-09-21:
//  1. vieneu.io (api.vieneu.io) — nhanh (~2s), catalog 1204 giọng, nhưng
//     demo giới hạn 50 lượt/~7h/IP và tập giọng nhận được xoay vòng.
//  2. DevTam05 — dịch vụ DUY NHẤT còn chạy ổn định suốt đợt probe.
//  3. hongqminh — CPU, chập chờn (1 lần OK / phần còn lại lỗi tức thì).
//  4. … nhóm space template (pnnbao-ump và các bản clone) — đang lỗi
//     ứng dụng với MỌI người (error null / 404: Not Found trả trong ~1s
//     nên không gây treo); giữ nguyên vị trí tương đối để tự phục hồi
//     khi chủ space sửa. Arena Thomcles vẫn được liệt kê nhưng tự bỏ
//     qua như FIX53. Smrfhdl bị GỎ (giờ yêu cầu đăng nhập HF).
func Registry() []Desc {
	return []Desc{
		{
			ID: "vieneu-io", Label: "vieneu.io (chính thức)", Kind: "vieneuio",
			Base: "https://api.vieneu.io", API: "/api/tts/demo",
			DefaultVoice: "Adam Tốp Tốp",
			Voices:       vieneuIOCatalog(),
			Note:         "API demo chính thức (đã dời sang api.vieneu.io, FIX56) — nhanh; giới hạn 50 lượt/~7 giờ; tập giọng demo nhận thay đổi theo thời điểm, bị từ chối sẽ tự nhảy dịch vụ kế tiếp.",
		},
		{
			ID: "hf-devtam05", Label: "HF · DevTam05/vieneu-tts", Kind: "gradio",
			Base: "https://devtam05-vieneu-tts.hf.space", API: "synthesize",
			DataStyle: "speech2", DefaultVoice: "Nam Minh (Nam)", Voices: voicesOf("voicesDevTam05"),
			Note: "CPU thường — dịch vụ ổn định nhất trong đợt probe 2026-09-21 (mọi lần gọi đều OK).",
		},
		{
			ID: "hf-hongqminh", Label: "HF · hongqminh/VieNeu-TTS", Kind: "gradio",
			Base: "https://hongqminh-vieneu-tts.hf.space", API: "synthesize_speech",
			DataStyle: "speech5", DefaultVoice: "Tuyên (nam miền Bắc)", Voices: voicesOf("voicesHongqminh"),
			Note: "CPU thường — chập chờn trong probe 2026-09-21 (1 lần OK, phần còn lại lỗi tức thì).",
		},
		{
			ID: "hf-pnnbao-ump", Label: "HF · pnnbao-ump/VieNeu-TTS-v3-Turbo", Kind: "gradio",
			Base: "https://pnnbao-ump-vieneu-tts-v3-turbo.hf.space", API: "synthesize",
			DataStyle: "template", DefaultVoice: "Minh Quân Pro", Voices: voicesOf("voicesPnnbaoUmp"),
			Note: "Space của tác giả mô hình — đang lỗi ứng dụng (probe 2026-09-21), lỗi trả tức thì nên không treo chuỗi.",
		},
		{
			ID: "arena-thomcles", Label: "HF · Thomcles/yodalingua-tts-arena", Kind: "arena",
			Base:       "https://thomcles-yodalingua-tts-arena.hf.space",
			SkipReason: "Trang CHỈ phục vụ bỏ phiếu mù giữa các mô hình (pair có sẵn) — không có API tạo giọng theo văn bản tùy ý nên bị bỏ qua trong chuỗi.",
			Note:       "Arena & leaderboard.",
		},
		{
			ID: "hf-trangmin11101996", Label: "HF · trangmin11101996/VieNeu-TTS-v3-Turbo", Kind: "gradio",
			Base: "https://trangmin11101996-vieneu-tts-v3-turbo.hf.space", API: "synthesize",
			DataStyle: "template", DefaultVoice: "Minh Quân", Voices: voicesOf("voicesTrangmin11101996"),
			Note: "Đang lỗi ứng dụng (probe 2026-09-21) — giữ trong chuỗi chờ chủ space khắc phục.",
		},
		{
			ID: "hf-eagle0019", Label: "HF · eagle0019/VieNeu-TTS-v3-Turbo", Kind: "gradio",
			Base: "https://eagle0019-vieneu-tts-v3-turbo.hf.space", API: "synthesize",
			DataStyle: "template", DefaultVoice: "Ngọc Linh", Voices: voicesOf("voicesEagle0019"),
			Note: "Kiểm chứng tốt ngày 2026-09-20, đang lỗi ứng dụng ngày 2026-09-21 — lỗi trả tức thì.",
		},
		{
			ID: "hf-xtieps", Label: "HF · xtieps/VieNeu-TTS-v3-Turbo", Kind: "gradio",
			Base: "https://xtieps-vieneu-tts-v3-turbo.hf.space", API: "synthesize",
			DataStyle: "template", DefaultVoice: "Phạm Tuyên", Voices: voicesOf("voicesXtieps"),
			Note: "Đang lỗi ứng dụng (probe 2026-09-21).",
		},
		{
			ID: "hf-thienan2146", Label: "HF · thienan2146/VieNeu-TTS-v3-Turbo", Kind: "gradio",
			Base: "https://thienan2146-vieneu-tts-v3-turbo.hf.space", API: "synthesize",
			DataStyle: "template", DefaultVoice: "Adam", Voices: voicesOf("voicesThienan2146"),
			Note: "Đang lỗi ứng dụng (probe 2026-09-21).",
		},
		{
			ID: "hf-doremon102", Label: "HF · doremon102/VieNeu-TTS-v3-Turbo", Kind: "gradio",
			Base: "https://doremon102-vieneu-tts-v3-turbo.hf.space", API: "synthesize",
			DataStyle: "template", DefaultVoice: "Minh Quân", Voices: voicesOf("voicesDoremon102"),
			Note: "Đang lỗi ứng dụng (probe 2026-09-21).",
		},
		{
			ID: "hf-kabinz", Label: "HF · kabinz/VieNeu-TTS-v3-Turbo", Kind: "gradio",
			Base: "https://kabinz-vieneu-tts-v3-turbo.hf.space", API: "synthesize",
			DataStyle: "template", DefaultVoice: "Minh Quân Pro", Voices: voicesOf("voicesKabinz"),
			Note: "Đang lỗi ứng dụng (probe 2026-09-21).",
		},
		{
			ID: "hf-tuananh20015", Label: "HF · Tuananh20015/VieNeu-TTS-v3-Turbo", Kind: "gradio",
			Base: "https://tuananh20015-vieneu-tts-v3-turbo.hf.space", API: "synthesize",
			DataStyle: "template", DefaultVoice: "Ngọc Lan", Voices: voicesOf("voicesTuananh20015"),
			Note: "CPU thường — kiểm chứng tốt 2026-09-20, đang lỗi ứng dụng 2026-09-21; space còn có chế độ hội thoại nhiều giọng.",
		},
	}
}

// voicesOf tra bảng giọng sinh tự động (registry_data.go).
func voicesOf(varName string) []Voice {
	names, ok := voiceTables[varName]
	if !ok {
		return nil
	}
	return voicesOfNames(names)
}

// voicesOfNames dựng []Voice từ danh sách tên (PATCH FIX56).
func voicesOfNames(names []string) []Voice {
	out := make([]Voice, 0, len(names))
	for _, n := range names {
		out = append(out, Voice{Name: n})
	}
	return out
}

// vieneuIOCatalog ghép 10 giọng featured + 23 giọng app có trên catalog
// đầy đủ của vieneu.io (PATCH FIX56), bỏ trùng lặp (vd "Anh Khôi" nằm ở
// cả hai nhóm).
func vieneuIOCatalog() []Voice {
	out := make([]Voice, 0, len(featuredVoicesVieneuIO)+len(vieneuIOAppVoices))
	seen := map[string]bool{}
	for _, v := range featuredVoicesVieneuIO {
		if !seen[v.Name] {
			seen[v.Name] = true
			out = append(out, v)
		}
	}
	for _, n := range vieneuIOAppVoices {
		if !seen[n] {
			seen[n] = true
			out = append(out, Voice{Name: n})
		}
	}
	return out
}

// TimeoutProfile các mốc thời gian dùng chung.
const (
	// AttemptTimeout giới hạn cho 1 lần gọi 1 dịch vụ.
	// PATCH FIX56: 8 phút → 3 phút. Báo cáo người dùng (2026-09-21):
	// chờ eagle0019 5:35 mới đạt 36% — quá lâu trước khi bỏ một dịch vụ
	// chậm. Các dịch vụ khỏe trả kết quả trong vài giây tới ~90 giây
	// (CPU, đoạn 425 ký tự); 3 phút là đủ dãi mà không treo chuỗi.
	// Người dùng bấm Dừng vẫn huỷ tức thì qua context.
	AttemptTimeout = 3 * time.Minute
	// ErrCooldown khoảng thời gian 1 dịch vụ vừa lỗi được tạm tránh.
	ErrCooldown = 5 * time.Minute
)

// WakeRetryDelay chờ space "dậy" khi trả 502/503 (cold start HF).
// var (không const) để test rút ngắn.
var WakeRetryDelay = 25 * time.Second
