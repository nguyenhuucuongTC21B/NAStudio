package cloud

// PATCH FIX55 — 8 GIỌNG ỔN ĐỊNH (OD = "ổn định") — bộ lọc nhanh theo yêu
// cầu chủ app: 6 giọng tin tức phủ đủ 3 miền (nữ/nam × Bắc/Trung/Nam) +
// 2 giọng kể chuyện (nữ/nam). Đây là những giọng được kiểm chứng tổng hợp
// thật nhiều lần trên chuỗi dịch vụ online (probe FIX54 + probe55 ngày
// 2026-09-21) và có mặt trên nhiều dịch vụ nhất trong catalog.
//
// Ghi chú trung thực từ dữ liệu probe:
//   - Bộ giọng VieNeu gốc KHÔNG có giọng style "tin tức" cho miền Trung —
//     2 giọng Trung duy nhất (Ngọc Trân, Quang Sơn) là style "tự nhiên".
//   - "Thái Sơn" (nam kể chuyện) và "Ngọc Linh" (nữ kể chuyện) là 2 giọng
//     DUY NHẤT có mặt cả trên 2 dịch vụ CPU xương sống (eagle0019,
//     Tuananh20015) — tức là dùng được cả khi các space ZeroGPU hết hạn
//     mức: đã tổng hợp thật thành công 2026-09-21 trên cả 2 dịch vụ CPU.
//   - "Minh Triết" (nam · Nam · tin tức) là giọng tin tức Nam DUY NHẤT
//     trong bộ gốc; chỉ 4/13 dịch vụ khai báo — chuỗi sẽ tự lọc đúng.
type ODVoice struct {
	Name   string `json:"name"`           // tên giọng ĐÚNG như catalog dịch vụ (giá trị gửi đi)
	Gender string `json:"gender"`         // "Nữ" | "Nam"
	Region string `json:"region"`         // "Bắc" | "Trung" | "Nam"
	Style  string `json:"style"`          // "Tin tức" | "Kể chuyện" | "Tự nhiên"
	Note   string `json:"note,omitempty"` // ghi chú ngắn hiển thị tooltip
}

var odVoices = []ODVoice{
	{Name: "Mai Anh", Gender: "Nữ", Region: "Bắc", Style: "Tin tức",
		Note: "Nữ miền Bắc · tin tức — 7/13 dịch vụ"},
	{Name: "Minh Đức", Gender: "Nam", Region: "Bắc", Style: "Tin tức",
		Note: "Nam miền Bắc · tin tức — 5/13 dịch vụ"},
	{Name: "Ngọc Trân", Gender: "Nữ", Region: "Trung", Style: "Tự nhiên",
		Note: "Nữ miền Trung — giọng Trung duy nhất (bộ gốc chưa có style tin tức cho Trung)"},
	{Name: "Quang Sơn", Gender: "Nam", Region: "Trung", Style: "Tự nhiên",
		Note: "Nam miền Trung — giọng Trung duy nhất (bộ gốc chưa có style tin tức cho Trung)"},
	{Name: "Thùy Dung", Gender: "Nữ", Region: "Nam", Style: "Tin tức",
		Note: "Nữ miền Nam · tin tức — 7/13 dịch vụ"},
	{Name: "Minh Triết", Gender: "Nam", Region: "Nam", Style: "Tin tức",
		Note: "Nam miền Nam · tin tức — giọng tin tức Nam duy nhất trong bộ gốc (5/13 dịch vụ)"},
	{Name: "Thái Sơn", Gender: "Nam", Region: "Nam", Style: "Kể chuyện",
		Note: "Nam kể chuyện — có cả trên 2 dịch vụ CPU không giới hạn, đã kiểm chứng 2026-09-21"},
	{Name: "Ngọc Linh", Gender: "Nữ", Region: "Bắc", Style: "Kể chuyện",
		Note: "Nữ kể chuyện — có cả trên 2 dịch vụ CPU không giới hạn, đã kiểm chứng 2026-09-21"},
}

// ODVoices trả bản copy danh sách 8 giọng ổn định cho UI.
func ODVoices() []ODVoice {
	out := make([]ODVoice, len(odVoices))
	copy(out, odVoices)
	return out
}

// IsODVoice kiểm tra nhanh một tên giọng có thuộc bộ OD không.
func IsODVoice(name string) bool {
	for _, v := range odVoices {
		if v.Name == name {
			return true
		}
	}
	return false
}
