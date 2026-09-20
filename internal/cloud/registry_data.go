package cloud

// Code SINH TỰ ĐỘNG từ kết quả khảo sát thật ngày 2026-09-20 (FIX53) —
// mỗi giọng là VALUE thật mà gradio Space chấp nhận (không phải label).
// Dùng scripts/f53_probe_deep.py + f53_probe_values.py để tái tạo.

// featuredVoicesVieneuIO = 10 giọng featured từ GET /api/tts/voices/featured?engine=v4
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

// voicesEagle0019 — catalog giọng của Space eagle0019/VieNeu-TTS-v3-Turbo (value gradio thật).
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

// voicesSmrfhdl — catalog giọng của Space Smrfhdl/tts (value gradio thật).
var voicesSmrfhdl = []string{
	"Minh Đức", "Phạm Tuyên", "Thái Sơn", "Xuân Vĩnh",
	"Thanh Bình", "Trúc Ly", "Ngọc Linh", "Đoan Trang",
	"Mai Anh", "Thục Đoan", "Minh Triết", "Thùy Dung",
	"Quang Sơn", "Ngọc Trân", "Mỹ Duyên", "Quỳnh Anh",
	"Đức Trí", "Kim Thanh", "Ngọc Huyền", "Adam",
	"Mạnh Dũng", "Minh Quân", "Anh Khôi",
}

// default hf-smrfhdl: 'Minh Đức'

// voiceTables — bảng tra tên biến → catalog (dùng bởi voicesOf).
var voiceTables = map[string][]string{
	"voicesPnnbaoUmp":        voicesPnnbaoUmp,
	"voicesTrangmin11101996": voicesTrangmin11101996,
	"voicesEagle0019":        voicesEagle0019,
	"voicesXtieps":           voicesXtieps,
	"voicesThienan2146":      voicesThienan2146,
	"voicesDoremon102":       voicesDoremon102,
	"voicesKabinz":           voicesKabinz,
	"voicesSmrfhdl":          voicesSmrfhdl,
}
