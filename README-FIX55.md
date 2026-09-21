# FIX55 — 8 GIỌNG ỔN ĐỊNH (OD) · ICON CÁ NHÂN HOÁ · DÒNG CHỜ GỌN

> Ghi đè lên FIX54. Không đụng engine offline (FIX46–FIX52 giữ nguyên 100%).

## 1. FIX55 giải quyết đúng 3 ý của chủ app

| # | Ý kiến gốc | FIX55 làm gì |
|---|------------|--------------|
| 1 | "Lọc ra 8 giọng ổn định nhất… thêm ký hiệu OD sau tên giọng (Mạnh Dũng - OD)" | Dropdown giọng ONLINE: 8 giọng OD được **ghim lên đầu** với nhãn `Tên - OD · Giới · Vùng · Style`, kèm **ô tick "Chỉ OD"** để hiện đúng 8 giọng này. Giá trị gửi đi vẫn là tên gốc cho dịch vụ |
| 2 | "Phần mềm sau khi GitHub tạo ra không có icon cá nhân (tôi có chuẩn bị vks.ico, vks.png rồi)" | Tìm ra **nguyên nhân gốc**: CI build bằng `go build` thuần (không qua `wails build`) nên không bao giờ nhúng resource. FIX55 nhúng sẵn icon tạm qua `rsrc_windows_amd64.syso` + thêm cơ chế **tự nhận `assets/brand/vks.ico` + `vks.png`** của bạn ở lần CI sau (mục 4) |
| 3 | "Dòng 'Đang chờ HF · eagle0019/… (5/13) tổng hợp…' chỉ nên viết là 'Đang chờ Server tổng hợp'" | Toàn bộ dòng tiến trình không còn đổ tên kỹ thuật: **"Đang gửi yêu cầu tới Server tổng hợp…" → "Đang chờ Server tổng hợp…" → "Server đã nhận — đang tổng hợp…" → "Server đang trả kết quả…"**. Chi tiết từng dịch vụ vẫn xem ở thẻ "Dịch vụ Online" |

## 2. Tiện ích lớn đi kèm: chuỗi online LỌC THEO GIỌNG (hết cảnh bị đổi giọng ngầm)

Probe55 (2026-09-21) chứng minh: các space **TỪ CHỐI** tên giọng ngoài catalog của
chúng (error null). Nhờ đó FIX55 thêm lọc thông minh vào chuỗi cầu nối:

- Chọn 1 giọng cụ thể → app chỉ ghé **những dịch vụ có đúng giọng đó**
  (bỏ qua tức thì, 0 request, không tốn chờ).
- Hết cảnh: yêu cầu giọng A mà nhận về giọng B (đây chính là lý do lần
  trước bạn chọn "Ngọc (nữ miền Bắc)" 425 ký tự mà chờ **5:35 ở 36%** trên
  eagle0019 — chuỗi cũ rơi vào dịch vụ không có giọng đó và xử lý rất chậm).
- Giọng nào không dịch vụ nào có → cảnh báo rõ + đi cả chuỗi như cũ.

## 3. Bộ 8 giọng OD (đối chiếu metadata catalog VieNeu gốc)

| Giọng (OD) | Giới · Vùng · Style | Có trên | Ghi chú kiểm chứng |
|---|---|---|---|
| Mai Anh | Nữ · Bắc · **Tin tức** | 7/13 | Template spaces + pnnbao/kabinz |
| Minh Đức | Nam · Bắc · **Tin tức** | 5/13 | Template spaces |
| Ngọc Trân | Nữ · Trung · Tự nhiên | 7/13 | Giọng nữ Trung duy nhất trong bộ gốc |
| Quang Sơn | Nam · Trung · Tự nhiên | 7/13 | Giọng nam Trung duy nhất trong bộ gốc |
| Thùy Dung | Nữ · Nam · **Tin tức** | 7/13 | Template spaces + pnnbao/kabinz |
| Minh Triết | Nam · Nam · **Tin tức** | 5/13 | Giọng tin tức Nam duy nhất trong bộ gốc |
| Thái Sơn | Nam · Nam · **Kể chuyện** | 7/13 | **Có cả trên 2 dịch vụ CPU** — tổng hợp thật OK 2026-09-21 (Tuananh20015 4,5s · eagle0019 4,4s) |
| Ngọc Linh | Nữ · Bắc · **Kể chuyện** | 7/13 | **Có cả trên 2 dịch vụ CPU** — tổng hợp thật OK 2026-09-21 (Tuananh20015 4,2s · eagle0019 4,0s) |

Ghi chú trung thực:

- Bộ giọng VieNeu gốc **chưa có** giọng style "tin tức" cho miền Trung —
  2 giọng Trung duy nhất (Ngọc Trân, Quang Sơn) là style "tự nhiên".
- Probe55 còn chứng minh dịch vụ CPU **không nhận** tên giọng ngoài
  dropdown của nó → không thể "mượn" catalog; nhờ lọc theo giọng nên
  không bao giờ gửi nhầm nữa.
- Hongqminh (CPU, không giới hạn) vẫn là lựa chọn hay cho "Ngọc (nữ miền
  Bắc)", "Tuyên (nam miền Bắc)"… — giờ đã được điều hướng đúng 100%.

## 4. Icon cá nhân hoá — cách dùng bộ vks của bạn

FIX55 đã nhúng icon tạm (VKS · HCStudio) nên **exe build lần này đã có
icon ngay**. Muốn dùng icon riêng của bạn:

1. Chép 2 file vào repo: `assets/brand/vks.ico` + `assets/brand/vks.png`.
2. Push → CI. `scripts/build.ps1` tự:
   - vks.png → `build/appicon.png`;
   - vks.ico → `build/windows/icon.ico` + sinh lại `.syso` bằng go-winres
     (tự `go install` nếu chưa có);
   - `go build` tự nhúng `.syso` → exe hiện icon của bạn trên Explorer,
     title bar, taskbar, Properties.
3. Không có `vks.*` → dùng icon tạm có sẵn; **build không bao giờ fail** vì thiếu icon.
- Kèm theo: `make-setup.ps1` giờ cũng gắn icon vào file `HCStudio-Setup.exe`.

Bằng chứng nhúng: cross-compile windows/amd64 kèm `.syso` = 10.561.536 B,
không kèm = 10.508.800 B (chênh ≈ kích thước resource) — cả 2 đều build PASS.

## 5. Thay đổi kỹ thuật (so với FIX54)

| File | Thay đổi |
|---|---|
| `internal/cloud/chain.go` | +Lọc dịch vụ theo giọng (giữ thứ tự gốc) + event "info"; 4 message tiến trình thành "Server…" chung chung |
| `internal/cloud/od.go` | **MỚI** — bảng 8 ODVoice (name/gender/region/style/note) + `ODVoices()` + `IsODVoice()` |
| `internal/cloud/cloud_test.go` | +3 test (lọc giọng 0-call dư, giọng lạ giữ hành vi cũ, bất biến bộ OD); sửa test snapshot ghim đồng hồ (bug date-sensitive từ FIX54) |
| `app.go` | +`CloudODVoices()` endpoint cho UI |
| `frontend/dist/index.html` | +checkbox "Chỉ OD" trong hàng tuỳ chọn online + CSS `.od-filter`; title ô chọn giọng cập nhật |
| `frontend/dist/assets/js/ui.js` | `rebuildCloudVoiceOptions`: ghim OD đầu danh sách với nhãn `- OD · …`, tooltip số dịch vụ có giọng; lọc Chỉ OD; giữ lựa chọn hợp lệ |
| `frontend/dist/assets/js/bridge.js` | +`CloudODVoices` (real + mock); mock steps đồng bộ thông điệp "Server…" |
| `frontend/dist/assets/js/state.js` | +`cloudOD`, `cloudODOnly` |
| `rsrc_windows_amd64.syso` | **MỚI** (repo root) — resource icon + version 5.0.0.0, `go build` tự nhúng |
| `assets/brand/*` | **MỚI** — icon tạm (png 1024/ico 7 cỡ) + README hướng dẫn vks |
| `build/appicon.png`, `build/windows/icon.ico` | Đổi thành icon tạm FIX55 |
| `scripts/build.ps1` | +bước FIX55: nhận diện `assets/brand/vks.*`, sinh lại .syso (ASCII thuần, không phá patch cũ) |
| `scripts/make-setup.ps1` | +`SetupIconFile` (đường dẫn tuyệt đối, có guard) |

## 6. Bảng 13 dịch vụ (trạng thái ngày probe55 — 2026-09-21)

| # | Dịch vụ | Hạng mục | Ghi chú |
|---|---|---|---|
| 1 | vieneu.io | chính thức · hạn mức lượt/ngày | 10 giọng featured — OK FIX54 (đã fix HTTP 201) |
| 2 | pnnbao-ump | ZeroGPU | dùng được từ IP nhà tới khi cạn hạn mức |
| 3 | Thomcles arena | — | tự bỏ qua (chỉ bỏ phiếu mù) |
| 4 | trangmin11101996 | ZeroGPU | như trên |
| 5 | eagle0019 | **CPU-basic** | OK 2026-09-21 — chậm (~20s/đoạn ngắn) nhưng không tốn hạn mức GPU |
| 6 | xtieps | ZeroGPU | |
| 7 | thienan2146 | ZeroGPU | |
| 8 | Smrfhdl | ZeroGPU (có cpu) | 23 giọng |
| 9 | doremon102 | ZeroGPU | |
| 10 | kabinz | ZeroGPU | |
| 11 | Tuananh20015 | **CPU-basic** | OK 2026-09-21 — ~4–5s, 10/10 giọng |
| 12 | hongqminh | **CPU-basic** | OK 2026-09-21 — 3,0s, giọng có nhãn miền |
| 13 | DevTam05 | **CPU-basic** | OK FIX54 — 2,3s |

## 7. 10 ca nghiệm thu

1. Mở app → tab **"Sử dụng online"** → hàng tuỳ chọn có ô chọn giọng + ô tick **"Chỉ OD"**.
2. Mở dropdown giọng online → **8 giọng có đuôi `- OD` đứng đầu**, mỗi giọng dạng
   `Tên - OD · Giới Vùng · Style`; rê chuột thấy tooltip "có trên N/13 dịch vụ".
3. Tick **"Chỉ OD"** → dropdown chỉ còn giọng OD (dịch vụ thật: tối đa 8 +
   dòng "Giọng mặc định từng dịch vụ"); bỏ tick → đầy đủ trở lại.
4. Chọn 1 giọng OD (vd **Ngọc Linh - OD**) → gõ/nạp văn bản → **Chuyển đổi online**
   → dòng tiến trình là **"Đang gửi yêu cầu tới Server tổng hợp…"** rồi
   **"Đang chờ Server tổng hợp…"** — KHÔNG còn chữ "HF · tên space".
5. Trong lúc chạy: thẻ **"Dịch vụ Online"** ở sidebar vẫn hiện chi tiết từng
   dịch vụ (chấm màu + số lượt + giờ OK) — thông tin kỹ thuật không mất đi,
   chỉ là không còn dồn lên thanh tiến trình.
6. Sau khi có audio: toast + tên bài trong danh sách phát thể hiện giọng đúng
   như đã chọn (session engine `cloud:…` như FIX53/54); PHÁT/xuất WAV·MP3 hoạt
   động như cũ.
7. Chọn giọng **"Ngọc (nữ miền Bắc)"** (hongqminh) → run → log/telemetry cho
   thấy app BỎ QUA ngay các dịch vụ không có giọng này (không chờ eagle0019
   như lần trước nữa) và thành công qua hongqminh (CPU, nhanh).
8. Icon: tải artifact `HCStudio-exe` (run FIX55) → `HCStudio.exe` có icon
   (icon tạm VKS·HCStudio) trên Explorer + title bar; chuột phải → Properties
   → tab Details thấy Product name/version 5.0.0.0.
9. (Tùy chọn) Chép `vks.ico` + `vks.png` của bạn vào `assets/brand/` → push →
   lần CI sau exe mang icon cá nhân của bạn (README trong `assets/brand/`).
10. Chế độ OFFLINE: chọn giọng, đọc, nghe ngay, hàng đợi, xuất WAV/MP3 —
    hoạt động y nguyên trước FIX55 (không đụng engine).

## 8. Rollback

Giải nén `HCStudio-v5-FIX54-online-deepcheck.zip` đè lên repo (FIX55 chỉ
thêm file mới: `internal/cloud/od.go`, `rsrc_windows_amd64.syso`,
`assets/brand/*` — xóa tay nếu muốn sạch hoàn toàn).

## 9. Bằng chứng kiểm thử (sandbox, 2026-09-21)

- `go test ./internal/cloud/ ./internal/dsp/ ./internal/textnorm/` → **ok** (cloud có 3 test FIX55 mới).
- `go vet ./internal/... .` → sạch. `gofmt` → sạch. `node --check` 5/5 file JS.
- Cross-compile `windows/amd64` Lite kèm `.syso` → PASS (bằng chứng icon nhúng theo chênh lệch size).
- Headless smoke (mock): OD đứng đầu dropdown, lọc Chỉ OD = 3 option (mock có
  2 giọng OD trùng catalog), value gửi đi là tên gốc, MutationObserver bắt đủ
  3 thông điệp "Server…", 0 console error.
