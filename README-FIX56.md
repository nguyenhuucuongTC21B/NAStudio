# HCStudio-v5 — FIX56: SỬA "TẤT CẢ DỊCH VỤ ONLINE ĐỀU KHÔNG THÀNH CÔNG"

**Ngày:** 2026-09-21 · **Cơ sở:** báo cáo người dùng *"BÁO LỖI tất cả các dịch vụ online đều không thành công"* trên bộ FIX55.

---

## 1. Nguyên nhân gốc (đã probe THẬT từng dịch vụ ngày 2026-09-21)

| # | Phát hiện | Bằng chứng |
|---|-----------|------------|
| 1 | **vieneu.io DỜI API sang `api.vieneu.io`** — app gọi `vieneu.io/api/tts/demo` → 404 → dịch vụ #1 (nhanh nhất, unlimited nhất) luôn bị bỏ qua | POST host mới → **HTTP 201 + WAV thật**; header `X-RateLimit-Limit: 50` (50 lượt/~7 giờ/IP) |
| 2 | **10 space dùng chung template (pnnbao-ump + các bản clone: eagle0019, Tuananh20015, trangmin, xtieps, thienan2146, doremon102, kabinz, hongqminh…) đang LỖI ỨNG DỤNG với MỌI NGƯỜI** — POST vẫn cấp event_id nhưng SSE trả `error: null` / `404: Not Found` trong ~1s | Probe 56d/56e: cùng lúc DevTam05 (hạ tầng giống nhau) vẫn OK ⇒ không phải chặn IP. app.py không đổi signature ⇒ lỗi nội bộ bên phía space |
| 3 | **Smrfhdl/tts giờ YÊU CẦU ĐĂNG NHẬP HF** ("Trang này chỉ dành cho một số người… Đăng nhập với HuggingFace") | Endpoint `access_status` của chính space |
| 4 | **DevTam05 trả file MP3** nhưng app chỉ có decoder WAV → dù tổng hợp thành công cũng chết ở bước "Giải mã audio" | File thật `.mp3` tải về; `dsp` chỉ có `ReadWav` + `ExportMP3` |
| 5 | **Bộ lọc giọng FIX55 + các dịch vụ chết ở trên** ⇒ với nhiều giọng, chuỗi còn đúng 1–2 dịch vụ và đều đang lỗi ⇒ trải nghiệm "tất cả đều fail" | Ma trận catalog × trạng thái probe |
| 6 | Demo vieneu.io **chỉ nhận giọng đang được worker nạp sẵn** — tập giọng THAY ĐỔI THEO THỜI ĐIỂM (cùng giọng "Mạnh Dũng": 201 lúc 10:20, 400 lúc 10:40) | Probe 56f→56i liên tiếp |

## 2.FIX56 thay đổi gì

**Chuỗi online 12 dịch vụ (bỏ Smrfhdl) — thứ tự mới theo kết quả probe:**

| Vị trí | Dịch vụ | Trạng thái 2026-09-21 |
|--------|---------|------------------------|
| 1 | **vieneu.io (api.vieneu.io)** | ✅ OK ~2s — host mới, catalog đầy đủ 1204 giọng, 23/25 giọng app có mặt; bị từ chối giọng → lỗi tức thì ~1s rồi tự nhảy |
| 2 | **DevTam05** (CPU) | ✅ ổn định nhất (mọi lần gọi đều OK) — **mới sửa decode MP3** |
| 3 | hongqminh (CPU) | ⚠️ chập chờn (1 lần OK) |
| 4 | pnnbao-ump (space tác giả) | ❌ đang lỗi app — lỗi trả tức thì, không treo |
| 5 | arena Thomcles | tự bỏ qua (chỉ bỏ phiếu — như FIX53) |
| 6–12 | trangmin, eagle0019, xtieps, thienan2146, doremon102, kabinz, Tuananh20015 | ❌ đang lỗi app — giữ trong chuỗi chờ chủ space sửa (lỗi trả ~1s) |

**Điểm mới kỹ thuật:**

1. **`internal/cloud/registry_data.go`** — ghi nhận nguyên nhân FIX56; thêm `vieneuIOAppVoices` (23 giọng app có trên vieneu.io, sinh từ catalog thật 1204 giọng) → dịch vụ #1 nằm trong chuỗi cho hầu hết giọng, kể cả 8 giọng OD.
2. **`internal/cloud/cloud.go`** — Base vieneu.io → `https://api.vieneu.io`; sắp lại thứ tự Registry; **AttemptTimeout 8 phút → 3 phút** (báo cáo "chờ 5:35 đạt 36%" không còn xảy ra — một dịch vụ chậm tối đa 3 phút rồi tự nhảy).
3. **`internal/cloud/http.go`** — 400 `is not available` của demo vieneu.io → thông điệp TẠM THỜI rõ ràng (không phải hỏng vĩnh viễn); MIME theo phần mở rộng URL thật (`.mp3` → audio/mpeg).
4. **`internal/cloud/chain.go`** — giọng không có trên dịch vụ nào → **báo lỗi NGAY, không gọi request nào** (không còn "nghe giọng khác mà không hay biết"); chuỗi lọc theo giọng hỏng hết → lỗi cuối gợi ý *"thử lại sau / chọn giọng khác (vd giọng OD) / dùng Offline"*.
5. **`internal/dsp/decode.go` (mới)** — `ReadAudioFile`: nhận biết WAV/MP3 theo magic bytes, giải mã MP3 bằng `github.com/hajimehoshi/go-mp3` (pure Go, không cgo); trộn stereo → mono float32. **MP3 thật DevTam05 4.68s @24kHz decode PASS.**
6. **`app.go`** — decode audio online qua `ReadAudioFile` (WAV + MP3).
7. **UI** — tooltip dịch vụ đang lỗi ưu tiên HIỂN THỊ NGUYÊN NHÂN LỖI ngay trên tooltip; mock bridge khớp thứ tự + host mới; mọi số "13 dịch vụ" cũ cập nhật "12".

**KHÔNG đụng:** engine offline, xuất audio, danh sách phát, wizard weights, icon FIX55, 8 giọng OD (tên giữ nguyên — giờ có thêm vieneu.io hỗ trợ).

## 3. Đóng gói & rollback

- **File:** `HCStudio-v5-FIX56-online-revive.zip` (111 file) · giải nén ĐÈ nguyên repo → push → CI.
- **Rollback:** khôi phục cây nguồn từ `HCStudio-v5-FIX55-od-voices-icon.zip` (đè lại). Không có thay đổi data/migration — `settings.json` giữ nguyên.

## 4. 10 ca nghiệm thu

| Ca | Bước thực hiện | Kết quả đúng |
|----|----------------|--------------|
| 1 | Build CI xanh, exe chạy lên, vào tab **Online** | Thẻ Dịch vụ Online có **12 dòng**, thứ tự: vieneu.io → DevTam05 → hongqminh → pnnbao-ump → arena (Bỏ qua) → trangmin → eagle0019 → xtieps → thienan2146 → doremon102 → kabinz → Tuananh20015 |
| 2 | Không chọn giọng (mặc định), bấm **Chuyển đổi online** với 1 đoạn văn ngắn | Thành công **qua vieneu.io trong vài giây** (audio phát tự động); toast "Chế độ online — hoàn tất" |
| 3 | Chọn giọng **Mai Anh - OD** (nữ Bắc tin tức), chuyển đổi | Dòng tiến trình hiện "Giọng \\"Mai Anh\\" có trên N/12 dịch vụ…"; nếu demo vieneu.io từ chối thì hiện thông điệp *"chưa có trên demo lúc này"* rồi tự nhảy dịch vụ kế tiếp — KHÔNG treo quá 3 phút/dịch vụ |
| 4 | Dùng đoạn văn ~400–500 ký tự qua **DevTam05** (khi vieneu.io hết lượt) | Audio **MP3 giải mã và phát/xuất được** — không còn lỗi "Audio nhận về không đọc được" |
| 5 | Chuột lên dịch vụ có chấm **đỏ** trong thẻ Dịch vụ Online | Tooltip hiển thị **nguyên nhân lỗi thật** (vd "app space trả error null…"), không phải ghi chú chung |
| 6 | Chọn giọng **Ngọc Huyền** (chỉ có offline, không trên dịch vụ online nào) | Lỗi NGAY với thông điệp "không có trên bất kỳ dịch vụ online nào — chọn giọng khác (8 giọng OD) hoặc dùng Offline"; KHÔNG phát audio sai giọng |
| 7 | Chờ 1 dịch vụ đang lỗi nhưng có sẵn trong chuỗi (vd eagle0019) | Bị bỏ qua sau **~1 giây** (không chờ 5:35 như FIX55); tổng thời gian chờ tối đa mỗi dịch vụ 3 phút |
| 8 | Chạy khi viẽu.io hết 50 lượt/7 giờ (hiếm) | Thẻ dịch vụ hiện "hết lượt", tự chuyển DevTam05; lần gọi sau vẫn thử lại được (không khoá cả ngày) |
| 9 | Tab Offline: synth, phát, xuất WAV/MP3, danh sách phát | Giữ nguyên 100% hành vi FIX52–FIX55 |
| 10 | EXE build CI vẫn có **icon VKS** (FIX55) | Icon nguyên vẹn — FIX56 không đụng tài nguyên icon |

## 5. Ghi chú trung thực

- Các space template **đang hỏng bên phía chủ space** (mọi client đều lỗi) — FIX56 không thể "sửa hộ" nhưng đã đảm bảo: lỗi bị nhận diện TỨC THÌ, không treo, không đốt thời gian; khi chủ space sửa lại, chuỗi **tự sử dụng lại không cần cập nhật app**.
- Demo vieneu.io có giới hạn 50 lượt/~7 giờ/IP và tập giọng nhận thay đổi theo thời điểm — đây là chính sách phía dịch vụ; app xử lý nhẹ nhàng (nhảy nhanh, thông báo đúng).
- Với giọng OD (Thái Sơn, Ngọc Linh…): ngoài các space template (đang lỗi), giờ còn được vieneu.io tiếp nhận khi worker nạp sẵn giọng đó — khả năng thành công cao hơn FIX55 rõ rệt.

## 6. Bằng chứng probe (kèm trong gói, thư mục `scripts56/`)

`probe56_all.log` (13 dịch vụ) · `probe56b.log` (đào sâu vieneu.io + hongqminh + Smrfhdl access_status) · `probe56c.log` (api.vieneu.io 201 + session_hash) · `probe56d.log` (repo model nguyên vẹn + search 80 spaces) · `probe56e.log` (runtime + probe 33 space) · `probe56f/g/h.log` (catalog 1204 giọng + ranh giới demo + rate limit) · `probe56i.log` (0/25 vì demo xoay giọng) · `probe56j.log` (CPU space error null) · `vieneu_full_voices_v4.json` (catalog đầy đủ) · `vieneu_voice_map.json` (kết quả dò id).
