# FIX54 — RÀ SOÁT SÂU CHẾ ĐỘ ONLINE: kiểm chứng từng giọng + 3 dịch vụ CPU "không giới hạn"

> FIX54 giữ nguyên 100% code FIX53 (và toàn bộ FIX46–52), chỉ **tăng cường chế độ online**:
> (1) sửa 1 bug thật khiến vieneu.io bị bỏ qua, (2) kiểm chứng **từng giọng** bằng tổng hợp
> thật, (3) thêm **3 dịch vụ CPU mới** không phụ thuộc hạn mức GPU — xương sống ổn định cho
> mục tiêu *"ổn định và gần như không giới hạn"*.

## 0. Vì sao phải có FIX54 — kết quả rà soát sâu (probe thật 2026-09-20)

Chạy probe54.py + test54_voices.py + test54_missing.py gọi **thật** từng dịch vụ, từng giọng:

| # | Dịch vụ | Phần cứng | Kết quả kiểm chứng | Ghi chú |
|---|---------|-----------|--------------------|---------|
| 1 | vieneu.io (chính thức) | server chính thức | **10/10 giọng OK** (~2s/giọng) | ⚠ Phát hiện BUG: API trả **HTTP 201** + audio hợp lệ — FIX53 chỉ nhận 200 nên **luôn bỏ dịch vụ đầu tiên**. FIX54 đã sửa. |
| 2 | pnnbao-ump/VieNeu-TTS-v3-Turbo | ZeroGPU | 0/1 từ IP datacenter (error:null tức thì) | ZeroGPU chặn nặc danh theo hạn mức GPU/ngày/IP; từ IP nhà dùng được tới khi cạn hạn mức |
| 3 | Thomcles/yodalingua-tts-arena | docker | BỎ QUA (giữ nguyên) | Chỉ bỏ phiếu mù, không có API tạo giọng |
| 4 | trangmin11101996/VieNeu-TTS-v3-Turbo | ZeroGPU | như dòng 2 | catalog 23 giọng giữ nguyên (value đúng theo /config) |
| 5 | eagle0019/VieNeu-TTS-v3-Turbo | **cpu-basic** | **10/10 giọng OK** (~20s/giọng) | Không tốn hạn mức GPU — ổn định, chậm hơn |
| 6 | xtieps/VieNeu-TTS-v3-Turbo | ZeroGPU | như dòng 2 | catalog 14 giọng |
| 7 | thienan2146/VieNeu-TTS-v3-Turbo | ZeroGPU | như dòng 2 | catalog 20 giọng |
| 8 | Smrfhdl/tts | ZeroGPU | như dòng 2 | catalog 23 giọng; ưu tiên mode CPU |
| 9 | doremon102/VieNeu-TTS-v3-Turbo | ZeroGPU | như dòng 2 | catalog 23 giọng |
| 10 | kabinz/VieNeu-TTS-v3-Turbo | ZeroGPU | như dòng 2 | catalog 10 giọng |
| 11 | **Tuananh20015/VieNeu-TTS-v3-Turbo** (MỚI) | **cpu-basic** | **10/10 giọng OK** (~5s/giọng) | Cùng dạng API chuẩn 9 tham số; space còn có **hội thoại nhiều giọng** |
| 12 | **hongqminh/VieNeu-TTS** (MỚI) | **cpu-basic** | **6/6 giọng OK** (~3.4s/giọng) | Dạng 5 tham số [text, voice, nil, nil, nil] |
| 13 | **DevTam05/vieneu-tts** (MỚI) | **cpu-basic** | **2/2 giọng OK** (~2.3s/giọng) | Dạng 2 tham số [text, voice] |

**Tổng: 38 giọng trên 5 dịch vụ đã tổng hợp thật 100% thành công** + 7 dịch vụ ZeroGPU dự
phóng (dùng được từ IP nhà). App tự chuyển qua lại giữa tất cả — đây là lý do chế độ online
sau FIX54 gần như không thể "cạn chung".

### Ý nghĩa của cpu-basic vs ZeroGPU (quan trọng)

- **ZeroGPU** (7 space): mỗi IP nặc danh chỉ có một hạn mức GPU nhỏ mỗi ngày. Cạn → space đó
  trả lỗi cho tới hôm sau. Lợi thế: tổng hợp nhanh.
- **cpu-basic** (eagle0019, Tuananh20015, hongqminh, DevTam05): KHÔNG có hạn mức GPU — chạy
  được liên tục, đổi lại chậm hơn (2–20 giây/câu ngắn). Đây là "xương sống" của chuỗi.
- vieneu.io: API chính thức, tốc độ ~2 giây, hiện là dịch vụ tốt nhất chuỗi sau khi vá bug.

## 1. Thay đổi code

| File | Thay đổi |
|------|----------|
| `internal/cloud/http.go` | **FIX BUG**: `synthVieneuIO` nhận dải **2xx** (trước chỉ 200 — vieneu.io trả 201 nên luôn bị coi là lỗi). + `gradioData` thêm 2 dạng: `speech2` [text, voice], `speech5` [text, voice, nil, nil, nil] |
| `internal/cloud/cloud.go` | `Registry()` 10 → **13 dịch vụ**; chú thích cập nhật |
| `internal/cloud/registry_data.go` | Tái sinh: ghi rõ kết quả kiểm chứng từng giọng; +3 catalog mới `voicesTuananh20015` (10), `voicesHongqminh` (6), `voicesDevTam05` (2) |
| `internal/cloud/cloud_test.go` | Test registry 13 dịch vụ + đúng thứ tự đuôi chuỗi |
| `internal/cloud/e2e_manual_test.go` | **MỚI** — test E2E gọi mạng THẬT (chỉ chạy khi `F54_E2E=1`, CI bỏ qua) |
| `app.go` | Comment `CloudProviders` cập nhật |
| `frontend/dist/index.html` | Ghi chú thẻ "Dịch vụ Online": nhấn mạnh các dịch vụ CPU là xương sống |
| `frontend/dist/assets/js/state.js` | Comment cập nhật |
| `frontend/dist/assets/js/bridge.js` | Mock thêm 3 dịch vụ mới (dùng cho smoke test) |

Chain/QuotaError/cooldown/snapshot **giữ nguyên thiết kế FIX53** (đã đúng): hết lượt → đánh
dấu cạn tới hết ngày; lỗi thường → cooldown 5 phút; space ngủ → đợi 25s thử lại; dịch vụ
thành công gần nhất (`lastGood`) được ưu tiên lần sau.

## 2. Kiểm chứng đã chạy (sandbox, 2026-09-20)

- `gofmt` sạch · `go vet ./internal/... .` EXIT=0 · `go build ./...` EXIT=0
- `go test ./internal/cloud/` PASS (13 dịch vụ + tail order + toàn bộ test FIX53 11/11 giữ nguyên)
- `node --check` 5/5 file JS OK
- **E2E mạng thật** (`F54_E2E=1 go test -run TestManualChainE2E`): chuỗi 13 dịch vụ chạy qua
  code Go thật → **vieneu.io (1/13) thành công 3,72s, audio 391.724 B** — bug 201 xác nhận đã hết
- Headless smoke (mock): thẻ "Dịch vụ Online" render 6/6 dòng mock (gồm 3 dịch vụ mới),
  0 console error
- Probe scripts: `scripts54/probe54.py`, `test54_voices.py`, `test54_missing.py`,
  `inspect54.py`, `discover54.py` (đính kèm ngoài zip — xem cách tái tạo bảng trên)

## 3. Nghiệm thu trên máy thật (sau CI)

| # | Ca | Bước | Kết quả mong đợi |
|---|----|------|------------------|
| 1 | Tab online mở | Chuyển tab Online | Thẻ "Dịch vụ Online" liệt kê **13 dịch vụ** đúng thứ tự bảng trên |
| 2 | vientu.io hết bị bỏ qua | Tổng hợp online 1 câu ngắn | **Lần đầu tiên FIX cũng dùng được vieneu.io ngay** (trước đây bị bug bỏ qua) — audio ~2–5s |
| 3 | Giọng vieneu.io | Chọn từng giọng featured (10 giọng) | Tất cả tạo được tiếng (đã kiểm chứng 10/10) |
| 4 | Giọng eagle0019 | Chọn giọng eagle0019 bất kỳ | Tất cả tạo được tiếng (10/10) — chậm ~20s |
| 5 | Giọng Tuananh20015 | Chọn giọng Tuananh20015 bất kỳ | Tất cả tạo được tiếng (10/10) — ~5s |
| 6 | Giọng hongqminh / DevTam05 | Chọn giọng tương ứng | 6/6 và 2/2 tạo được tiếng |
| 7 | Hàng đợi timeline | Đặt 3–4 câu online liên tiếp | % chạy, đồng hồ chờ, PHÁT, xuất WAV/MP3 như FIX52 |
| 8 | Hết lượt dịch vụ đầu | Ép dùng tới khi 1 ZeroGPU cạn (hoặc tường lửa chặn 1 space) | Tự chuyển dịch vụ kế tiếp, toast báo rõ; trạng thái vàng/đỏ trên thẻ |
| 9 | Nhớ trạng thái | Thoát app, mở lại | Số lượt hôm nay + dịch vụ cạn còn nhớ (snapshot settings) |
| 10 | Giọng không có trên dịch vụ | Chọn giọng eagle0019 rồi dịch vụ rơi về Tuananh20015 | Tự dùng catalog đúng (ResolveVoice), thông tin hiện trong Info |
| 11 | Offline vẫn nguyên vẹn | Rút mạng, tổng hợp local | Engine FIX46–52 không bị đụng — hoạt động như cũ |

## 4. Rollback

FIX54 chỉ đụng `internal/cloud/*`, 3 file frontend và comment. Nếu cần lùi: giải nén
**FIX53** đè lên repo (không mất gì FIX46–52). Nếu muốn giữ 3 dịch vụ mới nhưng bỏ vá
2xx: không khuyến nghị — bug 201 khiến vieneu.io vô dụng.

## 5. Ghi chú kỹ thuật

- ZeroGPU spaces có thể "hỏng" từ IP văn phòng/datacenter (nặc danh bị chặn) nhưng vẫn chạy
  từ IP nhà. App không thể phân biệt 2 trường hợp qua thông điệp `error:null` — nên trạng
  thái hiển thị là "lỗi" + cooldown 5 phút, và THỬ LẠI sau; ngày hôm sau hạn mức tự reset.
- Thomcles/yodalingua-tts-arena vẫn nằm ở vị trí #3 như chủ app chỉ định nhưng tự bị bỏ qua
  (không có API tạo giọng) — đã ghi rõ trong UI.
- Các dịch vụ CPU mới được đặt CUỐI chuỗi theo thứ tự: Tuananh20015 → hongqminh → DevTam05,
  tôn trọng thứ tự 10 dịch vụ gốc chủ app đã chỉ định.
