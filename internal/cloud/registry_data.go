package cloud

// Code SINH TỰ ĐỘNG từ kết quả khảo sát thật ngày 2026-09-20.
// FIX53: lần đầu sinh từ probe /config + /gradio_api/info.
// PATCH FIX54: kiểm chứng TỪNG GIỌNG bằng tổng hợp thật (probe54.py):
//   - vieneu.io 10/10 OK (~2s/giọng)  — lưu ý HTTP 201 đã được chấp nhận.
//   - eagle0019 (cpu-basic) 10/10 OK (~20s/giọng).
//   - Tuananh20015 (cpu-basic) 10/10 OK (~5s/giọng)   — MỚI.
//   - hongqminh  (cpu-basic) 6/6 OK (~3.4s/giọng)     — MỚI.
//   - DevTam05   (cpu-basic) 2/2 OK (~2.3s/giọng)     — MỚI.
//   - 7 space ZeroGPU (pnnbao-ump, trangmin11101996, xtieps, thienan2146,
//     doremon102, kabinz, Smrfhdl): từ IP datacenter probe trả lỗi tức thì
//     (error:null — ZeroGPU chặn nặc danh); giá trị giọng vẫn ĐÚNG theo
//     /config, dùng được từ IP nhà tới khi cạn hạn mức GPU ngày.
// Dùng scripts/probe54.py + test54_voices.py để tái tạo.
//
// PATCH FIX56 (probe 2026-09-21 — nguyên nhân "tất cả dịch vụ đều fail"):
//   - vieneu.io DỜI API sang subdomain api.vieneu.io (vieneu.io/api trả
//     404) — POST https://api.vieneu.io/api/tts/demo → 201 + WAV thật.
//     Demo giới hạn X-RateLimit 50 lượt/~7 giờ/IP và TẬP GIỌNG NHẬN THAY
//     ĐỔI THEO THỜI ĐIỂM (worker chỉ giữ 1 nhóm giọng nạp sẵn — 400
//     "Voice … is not available" là TẠM THỜI, vài phút sau thử lại được).
//   - GET https://api.vieneu.io/api/tts/voices?engine=v4 → catalog ĐẦY
//     ĐỦ 1204 giọng (bản ghi có gender/region/description). 23/25 giọng
//     catalog của app có mặt → đưa vào catalog của dịch vụ vieneu-io
//     để bộ lọc giọng FIX55 đưa vieneu-io vào chuỗi cho mọi giọng app;
//     nếu demo từ chối thì lỗi trả TỨC THÌ (~1s) và chuỗi nhảy dịch vụ
//     kế tiếp — không treo.
//   - 10 space dùng chung template (pnnbao-ump, trangmin, eagle0019,
//     xtieps, thienan2146, doremon102, kabinz, Tuananh20015, hongqminh,
//     nguyenduc1222): ĐANG LỖI ỨNG DỤNG với MỌI người — POST nhận
//     event_id nhưng SSE trả "error: null" / "404: Not Found" trong ~1s
//     (không phải chặn IP: DevTam05 cùng hạ tầng vẫn OK). Giữ trong chuỗi
//     vì chủ space có thể khắc phục bất cứ lúc nào; lỗi trả nhanh nên
//     không gây treo.
//   - Smrfhdl/tts: giờ YÊU CẦU ĐĂNG NHẬP HF (access_status: "Trang này
//     chỉ dành cho một số người") → BỎ KHỎI CHUỖI.
//   - DevTam05: dịch vụ DUY NHẤT chạy ổn định suốt đợt probe → nâng lên
//     vị trí số 2 trong chuỗi.
// Bằng chứng: scripts/probe56_all.log … probe56j.log (kèm bộ FIX56).

// featuredVoicesVieneuIO = 10 giọng featured từ GET /api/tts/voices/featured?engine=v4
// (10/10 đã tổng hợp thật thành công ngày 2026-09-20).
var featuredVoicesVieneuIO = []Voice{
	{Name: "Adam Tốp Tốp", Gender: "male"},
	{Name: "Duyên Hà My", Gender: "female"},
	{Name: "Đăng Quân", Gender: "male"},
	{Name: "Bình Bon", Gender: "male"},
	{Name: "My Méo", Gender: "female"},
	{Name: "Nam Nũng Nịu", Gender: "male"},
	{Name: "Nhỏ Ngọt Ngào", Gender: "female"},
	{Name: "Tưởng Vy", Gender: "female"},
	{Name: "Mai Bé Phương", Gender: "female"},
	{Name: "Anh Khôi", Gender: "male"},
}

// PATCH FIX56: 23/25 giọng catalog app được xác nhận NẰM TRONG catalog
// đầy đủ 1204 giọng của vieneu.io (GET /api/tts/voices?engine=v4, probe
// 2026-09-21). Demo chỉ nhận giọng đang được worker nạp sẵn nên mỗi tên
// ở đây có thể 400 "is not available" TẠM THỜI — chuỗi hiểu và nhảy
// dịch vụ kế tiếp tức thì. "Ngọc Huyền" và "Adam bựa" không có trong
// catalog vieneu.io nên không đưa vào.
var vieneuIOAppVoices = []string{
	"Adam",
	"Phạm Tuyên",
	"Minh Đức",
	"Thanh Bình",
	"Trúc Ly",
	"Đoan Trang",
	"Ngọc Linh",
	"Mai Anh",
	"Quỳnh Anh",
	"Quang Sơn",
	"Ngọc Trân",
	"Xuân Vĩnh",
	"Thái Sơn",
	"Minh Triết",
	"Đức Trí",
	"Thục Đoan",
	"Thùy Dung",
	"Mỹ Duyên",
	"Kim Thanh",
	"Anh Khôi",
	"Minh Quân Pro",
	"Thiền Tâm Đức",
	"Mạnh Dũng",
}

// voicesPnnbaoUmp — catalog giọng của Space pnnbao-ump/VieNeu-TTS-v3-Turbo (value gradio thật).
var voicesPnnbaoUmp = []string{
	"Adam bựa", "Trúc Ly", "Anh Khôi", "Mai Anh",
	"Minh Quân Pro", "Thùy Dung", "Thiền Tâm Đức", "Ngọc Huyền",
	"Quang Sơn", "Ngọc Trân",
}

// default hf-pnnbao-ump: 'Minh Quân Pro'

// voicesTrangmin11101996 — catalog giọng của Space trangmin11101996/VieNeu-TTS-v3-Turbo (value gradio thật).
var voicesTrangmin11101996 = []string{
	"Minh Đức", "Phạm Tuyên", "Thái Sơn", "Xuân Vĩnh",
	"Thanh Bình", "Trúc Ly", "Ngọc Linh", "Đoan Trang",
	"Mai Anh", "Thục Đoan", "Minh Triết", "Thùy Dung",
	"Quang Sơn", "Ngọc Trân", "Mỹ Duyên", "Quỳnh Anh",
	"Đức Trí", "Kim Thanh", "Ngọc Huyền", "Adam",
	"Mạnh Dũng", "Minh Quân", "Anh Khôi",
}

// default hf-trangmin11101996: 'Minh Quân'

// voicesEagle0019 — catalog giọng của Space eagle0019/VieNeu-TTS-v3-Turbo
// (value gradio thật; FIX54: 10/10 giọng đã tổng hợp thật thành công).
var voicesEagle0019 = []string{
	"Ngọc Lan", "Gia Bảo", "Thái Sơn", "Đức Trí",
	"Mỹ Duyên", "Trúc Ly", "Xuân Vĩnh", "Trọng Hữu",
	"Bình An", "Ngọc Linh",
}

// default hf-eagle0019: 'Ngọc Linh'

// voicesXtieps — catalog giọng của Space xtieps/VieNeu-TTS-v3-Turbo (value gradio thật).
var voicesXtieps = []string{
	"Minh Đức", "Phạm Tuyên", "Thái Sơn", "Xuân Vĩnh",
	"Thanh Bình", "Trúc Ly", "Ngọc Linh", "Đoan Trang",
	"Mai Anh", "Thục Đoan", "Minh Triết", "Thùy Dung",
	"Quang Sơn", "Ngọc Trân",
}

// default hf-xtieps: 'Phạm Tuyên'

// voicesThienan2146 — catalog giọng của Space thienan2146/VieNeu-TTS-v3-Turbo (value gradio thật).
var voicesThienan2146 = []string{
	"Minh Đức", "Phạm Tuyên", "Thái Sơn", "Xuân Vĩnh",
	"Thanh Bình", "Trúc Ly", "Ngọc Linh", "Đoan Trang",
	"Mai Anh", "Thục Đoan", "Minh Triết", "Thùy Dung",
	"Quang Sơn", "Ngọc Trân", "Mỹ Duyên", "Quỳnh Anh",
	"Đức Trí", "Kim Thanh", "Ngọc Huyền", "Adam",
}

// default hf-thienan2146: 'Adam'

// voicesDoremon102 — catalog giọng của Space doremon102/VieNeu-TTS-v3-Turbo (value gradio thật).
var voicesDoremon102 = []string{
	"Minh Đức", "Phạm Tuyên", "Thái Sơn", "Xuân Vĩnh",
	"Thanh Bình", "Trúc Ly", "Ngọc Linh", "Đoan Trang",
	"Mai Anh", "Thục Đoan", "Minh Triết", "Thùy Dung",
	"Quang Sơn", "Ngọc Trân", "Mỹ Duyên", "Quỳnh Anh",
	"Đức Trí", "Kim Thanh", "Ngọc Huyền", "Adam",
	"Mạnh Dũng", "Minh Quân", "Anh Khôi",
}

// default hf-doremon102: 'Minh Quân'

// voicesKabinz — catalog giọng của Space kabinz/VieNeu-TTS-v3-Turbo (value gradio thật).
var voicesKabinz = []string{
	"Adam bựa", "Trúc Ly", "Anh Khôi", "Mai Anh",
	"Minh Quân Pro", "Thùy Dung", "Thiền Tâm Đức", "Ngọc Huyền",
	"Quang Sơn", "Ngọc Trân",
}

// default hf-kabinz: 'Minh Quân Pro'

// ─── PATCH FIX54: 3 catalog MỚI (dịch vụ CPU, từng giọng đã kiểm chứng) ───

// voicesTuananh20015 — catalog giọng của Space Tuananh20015/VieNeu-TTS-v3-Turbo
// (value gradio thật; FIX54: 10/10 giọng đã tổng hợp thật thành công, ~5s/giọng).
var voicesTuananh20015 = []string{
	"Ngọc Lan", "Gia Bảo", "Thái Sơn", "Đức Trí",
	"Mỹ Duyên", "Trúc Ly", "Xuân Vĩnh", "Trọng Hữu",
	"Bình An", "Ngọc Linh",
}

// default hf-tuananh20015: 'Ngọc Lan'

// voicesHongqminh — catalog giọng của Space hongqminh/VieNeu-TTS
// (value gradio thật; FIX54: 6/6 giọng đã tổng hợp thật thành công).
var voicesHongqminh = []string{
	"Tuyên (nam miền Bắc)", "Vĩnh (nam miền Nam)", "Bình (nam miền Bắc)",
	"Đoan (nữ miền Nam)", "Ngọc (nữ miền Bắc)", "Ly (nữ miền Bắc)",
}

// default hf-hongqminh: 'Tuyên (nam miền Bắc)'

// PATCH FIX57: voicesNguyenduc1222 — catalog giọng của Space
// nguyenduc1222/VieNeu-TTS (value Literal thật từ /info; probe 2026-09-21
// 05:04: synthesize_speech 5 tham số → complete 1,6s với 'Ngọc (nữ miền
// Bắc)'). Cùng họ hongqminh — dự phòng cho 6 giọng vùng miền khi hongqminh
// chập chờn, cộng thêm 3 giọng miền Nam (Nguyên/Sơn/Dung).
var voicesNguyenduc1222 = []string{
	"Tuyên (nam miền Bắc)", "Vĩnh (nam miền Nam)", "Bình (nam miền Bắc)",
	"Nguyên (nam miền Nam)", "Sơn (nam miền Nam)", "Đoan (nữ miền Nam)",
	"Ngọc (nữ miền Bắc)", "Ly (nữ miền Bắc)", "Dung (nữ miền Nam)",
}

// default hf-nguyenduc1222: 'Tuyên (nam miền Bắc)'

// voicesDevTam05 — catalog giọng của Space DevTam05/vieneu-tts
// (value gradio thật; FIX54: 2/2 giọng đã tổng hợp thật thành công).
var voicesDevTam05 = []string{
	"Hoài My (Nữ)", "Nam Minh (Nam)",
}

// default hf-devtam05: 'Nam Minh (Nam)'

// voiceTables — bảng tra tên biến → catalog (dùng bởi voicesOf).
var voiceTables = map[string][]string{
	"voicesPnnbaoUmp":        voicesPnnbaoUmp,
	"voicesTrangmin11101996": voicesTrangmin11101996,
	"voicesEagle0019":        voicesEagle0019,
	"voicesXtieps":           voicesXtieps,
	"voicesThienan2146":      voicesThienan2146,
	"voicesDoremon102":       voicesDoremon102,
	"voicesKabinz":           voicesKabinz,
	"voicesSmrfhdl":          nil, // FIX56: đã gỡ khỏi chuỗi (cần đăng nhập HF)
	"voicesTuananh20015":     voicesTuananh20015,
	"voicesHongqminh":        voicesHongqminh,
	"voicesNguyenduc1222":    voicesNguyenduc1222,
	"voicesDevTam05":         voicesDevTam05,
}
