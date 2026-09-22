# HCStudio-v5 — FIX59: Bộ lọc chất lượng audio + Lượt thử lại tự động

**Ngày:** 2026-09-21 · **Cơ sở:** Báo lỗi trên FIX58: *"kiểm tra code thật kỹ, tại sao tất cả các giọng OD đều không sử dụng được, ngoài ra có một số giọng tạo thành giọng rè rè vô nghĩa"*

---

## 1. Kết quả điều tra — 3 nguyên nhân gốc, có bằng chứng probe thật

### Nguyên nhân A — "Giọng rè rè vô nghĩa": 2 space trả audio RÁC mà app không kiểm nội dung

Đây là lần đầu tiên hệ điều tra **phân tích NỘI DUNG audio** (13 file giọng thật vs 2 file rè) thay vì chỉ kiểm tra "có file không" như các FIX trước. Kết quả probe60 (11:02 2026-09-21, `scripts60/`):

| Dịch vụ | Audio | Phân tích (rms_cv / pause / zcr) | Kết luận |
|---|---|---|---|
| **hf-nguyenduc1222** (giọng Ngọc, Dung) | WAV hợp lệ 24kHz/3.0s | **0.01 / 0.0% / 0.50** — nhiễu trắng thuần, 2 giọng khác nhau ra cùng độ dài 144.044 byte | **RÈ — model hỏng trên server** |
| **hf-hongqminh** (E2E qua code Go thật) | WAV hợp lệ | — | **RÈ — cùng họ cùng bệnh** (bị bắt trong E2E FIX59) |
| vieneu.io, eagle0019, Tuananh20015, DevTam05, Edge (13 file) | WAV/MP3 | 0.67–0.99 / 20–41% / 0.05–0.11 | Giọng nói thật, sạch |

Space nguyenduc1222 được thêm vào chuỗi từ FIX57 chỉ kiểm tra "complete 1,6s" — KHÔNG ai nghe thử. Model trên space này vỡ → vocoder xuất buffer rác (mẫu phân phối đều [-0.5, 0.5]) → app nhận trọn vì WAV header hoàn toàn hợp lệ. hongqminh cũng rơi vào trạng thái tương tự. **Đây chính là "một số giọng tạo thành giọng rè rè vô nghĩa".**

### Nguyên nhân B — FIX58 có bug cắt frame: Edge TTS chưa bao giờ phát được trong app

Cấu trúc binary frame của Edge TTS: `[2 byte độ-dài-header N][N byte header][body]`. Code FIX58 cắt `data[hl:]` thay vì `data[2+hl:]` → **payload MP3 thừa đúng 2 byte `"\r\n"` cuối header**. ffmpeg dung sai nên probe Python không phát hiện, nhưng go-mp3 từ chối → mọi audio Edge (Hoài My/Nam Minh) chết ở bước giải mã với thông báo "Audio nhận về không đọc được". Bằng chứng: file probe `scripts60/audio/edge-hoaimy.mp3` bắt đầu `0d 0a ff f3` (CRLF + sync MP3).

### Nguyên nhân C — "Tất cả giọng OD không dùng được": dịch vụ có giọng OD đang chập chờn, KHÔNG phải lỗi code mới

Probe60 11:02 chụp đúng thời điểm:
- **vieneu.io**: xoay tập giọng theo worker — 4/8 OD bị từ chối 400 ("Mai Anh", "Ngọc Trân", "Quang Sơn", "Minh Triết"), 2 OD nhận được ("Minh Đức", "Thùy Dung"), còn "Thái Sơn", "Ngọc Linh" chuyển sang 2 space CPU.
- **6 space ZeroGPU template** (pnnbao-ump, trangmin, xtieps, thienan2146, doremon102, kabinz): toàn bộ error "null" tức thì — group này chập chờn **theo từng request** (probe58b: cùng space cùng phút, Mai Anh OK 2,8s trong khi Quang Sơn lỗi).
- **2 space CPU xương sống** eagle0019/Tuananh20015 (chỉ có Thái Sơn + Ngọc Linh): hồi phục, OK 3,1–3,6s.

→ Khi user bấm thử đúng lúc cả nhóm space rơi vào cửa sổ lỗi + vieneu chưa nạp giọng, **toàn bộ OD thất bại thật** — và chuỗi cũ chỉ thử 1 lượt rồi trả lỗi.

### Dò thêm (probe61): không có space mới đáng thêm

22 space VieNeu ngoài chuỗi: chỉ 3 RUNNING — dongnguyen95 (không có API công khai), hungthai84 (platform hội thoại, không gọi trực tiếp được), Arrcttacsrks-CPU2 (**"Model chưa tải"** — hỏng giống nguyenduc1222). Không thêm dịch vụ nào.

---

## 2. FIX59 thay đổi gì

| # | Thay đổi | File |
|---|---|---|
| 1 | **Bộ lọc chất lượng audio** `dsp.LooksLikeNoise` — đo rms_cv + pause_ratio + zcr ngay tại chuỗi; ngưỡng đặt giữa 2 nhóm với dư địa rất rộng (ưu tiên không báo nhầm giọng thật). Audio rè/rác/im-lặng = **THẤT BẠI của dịch vụ đó** → tự nhảy dịch vụ kế, không bao giờ giao tiếng rè cho người dùng. Audio < 1s không đủ dữ kiện → không chặn | `internal/dsp/quality.go` (mới) |
| 2 | **Decode trên bộ nhớ** `dsp.ReadAudioBytes` — chuỗi kiểm chất lượng ngay khi nhận (không chờ tới bước phát); Result mang sẵn `Samples/SR` để app không decode 2 lần | `internal/dsp/decode.go`, `wav.go`, `app.go` |
| 3 | **Sửa lỗi cắt frame Edge** `edgeAudioPayload` — body bắt đầu tại `2+hl` (bản cũ `hl` thừa `"\r\n"` vào đầu MP3) → **Edge TTS phát được thật** (E2E: Hoài My thành công 0,4s) | `internal/cloud/edge.go` |
| 4 | **Lượt thử lại tự động**: hết lượt 1 mà chưa có audio → tự thử lại 1 lượt các server vừa lỗi thoáng qua (trễ 1,2s/lần, tối đa 8 dịch vụ, audio-rác KHÔNG thử lại). Tăng gấp đôi cơ hội trúng cửa sổ hồi phục của space chập chờn theo từng request | `internal/cloud/chain.go` |
| 5 | **nguyenduc1222 dời xuống cuối chuỗi** + note thành thật "ĐANG TRẢ AUDIO RÈ — bị bộ lọc chặn tự động" (giữ lại chờ chủ space sửa là dùng lại ngay, không cần cập nhật app) | `internal/cloud/cloud.go` |
| 6 | Thông điệp fail ghi rõ: *"trả audio rè vô nghĩa (nhiễu trắng — model hỏng trên server)"* — người dùng hiểu nguyên nhân thật | `chain.go` |
| 7 | Test mới: `TestLooksLikeNoiseCalibrated`, `TestReadAudioBytes`, `TestEdgeAudioPayload`, `TestChainNoiseAudioSkipped`, `TestChainJunkAudioSkipped`, `TestChainRetryPassSecondChance`; cập nhật 5 test cũ | `*_test.go` |
| 8 | E2E mạng thật (guard `F59_E2E=1`): đường rè bị chặn đúng + Edge thành công — **PASS 2/2** | `internal/cloud/e2e_fix59_test.go` |

**Thứ tự chuỗi sau FIX59 (14 dịch vụ):** edge-tts → vieneu.io → DevTam05 → hongqminh → pnnbao-ump → (arena bỏ qua) → trangmin → eagle0019 → xtieps → thienan2146 → doremon102 → kabinz → Tuananh20015 → nguyenduc1222 (cuối, đang bị lọc).

## 3. Kỳ vọng thực tế sau FIX59 (theo sức khoẻ server lúc 11:02 2026-09-21)

| Giọng | Đường | Kỳ vọng |
|---|---|---|
| Hoài My (Nữ), Nam Minh (Nam) | **edge-tts** (sửa xong) + DevTam05 | ✓ nhanh 1–3s, ổn định nhất |
| Thái Sơn (OD), Ngọc Linh (OD) | vieneu 400 → **eagle0019/Tuananh20015** | ✓ 3–26s (eagle0019 CPU chậm ~25s là bình thường) |
| Minh Đức (OD), Thùy Dung (OD) | **vieneu.io** (worker nạp đúng lúc) | ✓ tùy cửa sổ xoay vòng — hỏng thì thử lại |
| Ngọc Lan, Gia Bảo, Mỹ Duyên, Trúc Ly, Xuân Vĩnh… | vieneu + 2 space CPU | ✓ |
| Mai Anh, Ngọc Trân, Quang Sơn, Minh Triết (OD) | vieneu (xoay vòng) + 6 space template (đang lỗi) | ✗ tạm thời — giờ có lượt thử lại tự động + thông báo trung thực; tự dùng được khi space/vieneu hồi phục |
| 9 giọng vùng miền (Tuyên/Vĩnh/Bình/Ngọc/Ly…) | hongqminh + nguyenduc1222 — **cả 2 đang trả rè** | ✗ tạm thời — giờ bị chặn sạch, KHÔNG còn nghe rè; tự hết khi chủ space sửa |

---

## 4. 10 ca nghiệm thu

| # | Ca | Bước làm | Kết quả đúng |
|---|---|---|---|
| 1 | Edge TTS đã phát được thật (bug FIX58) | Chuyển đổi online → chọn giọng **"Hoài My (Nữ)"** → bấm Chuyển đổi | Audio phát NGHE ĐƯỢC trong 1–3s, thẻ "Microsoft Edge TTS (miễn phí)" sáng xanh. (FIX58: báo "audio không đọc được") |
| 2 | Không còn tiếng rè vô nghĩa | Nghe lại toàn bộ kết quả thành công | 100% audio là giọng đọc rõ; không còn file rè chỉ phát "soát soát" |
| 3 | Thẻ dịch vụ rè hiện nguyên nhân | Mở thẻ "Dịch vụ Online" → rê chuột vào **nguyenduc1222/hongqminh** | Trạng thái "Lỗi", tooltip ghi *"trả audio rè vô nghĩa (nhiễu trắng — model hỏng trên server)"* |
| 4 | nguyenduc1222 cuối chuỗi | Xem thẻ Dịch vụ Online | "HF · nguyenduc1222/VieNeu-TTS" đứng dòng cuối, note thành thật về audio rè |
| 5 | Lượt thử lại tự động | Chọn giọng "Thái Sơn - OD" bấm Chuyển đổi lúc server chập chờn; quan sát dòng tiến trình | Thấy thông báo *"Không dịch vụ nào nhận được ở lượt 1 — tự động thử lại N server vừa chập chờn…"* trước khi kết luận thất bại |
| 6 | OD ổn định qua CPU | "Thái Sơn - OD" / "Ngọc Linh - OD" khi vieneu 400 | Vẫn có audio qua eagle0019/Tuananh20015 (~3–26s — eagle0019 CPU chậm là bình thường) |
| 7 | Giọng OD chưa có ở đâu vẫn báo trung thực | Thử "Quang Sơn" lúc space nhóm lỗi | Thông báo liệt kê *"đã thử N"* + gợi ý bấm lại/dùng Offline — không treo, không giả giọng |
| 8 | Edge văn bản dài tự ghép | Dán đoạn 2.000+ ký tự, giọng Nam Minh | Có audio liền mạch (chia đoạn ghép MP3 đúng frame — không rè đầu mỗi đoạn như FIX58) |
| 9 | Xuất file sạch | Kết quả online → Xuất WAV/MP3 → mở bằng trình phát khác | File phát bình thường, đúng độ dài hiển thị |
| 10 | Offline không đổi | Chuyển chế độ Offline → tổng hợp giọng local | Hoạt động nguyên trạng các FIX 46–52, không ảnh hưởng |

## 5. Rollback

- Về **FIX58**: giải nén `HCStudio-v5-FIX58-edge-tts-backbone.zip` đè repo (giữ nguyên `scripts/`), commit + push. Lưu ý: FIX58 còn 2 lỗi đã được FIX59 chữa (Edge phát không được + audio rè xuyên qua).
- Các file FIX59 đụng tới: `app.go`, `internal/cloud/{chain,cloud,edge}.go`, `internal/cloud/*_test.go`, `internal/dsp/{decode,wav,quality}.go`, `frontend/dist/assets/js/{bridge,state}.js`. Rollback thủ công chỉ cần khôi phục các file này từ FIX58.

## 6. Bằng chứng kèm trong gói (`scripts60/`)

- `probe60.py/.log` — trạng thái 14 dịch vụ + 8 giọng OD + tải 15 audio thật
- `analyze60.py` — phân tích nội dung: 13 giọng thật vs 2 file rè (bảng số liệu ở mục 1)
- `probe61.py/.log`, `probe61b.py/.log` — dò 22 space ngoài chuỗi, CPU2 "Model chưa tải"
- `audio/nguyenduc1222-*.audio` — 2 file rè thủ phạm (WAV hợp lệ, nhiễu trắng)
- `audio/edge-hoaimy.mp3` — bằng chứng lỗi 2 byte (`0d 0a ff f3…`) của FIX58
- `audio/DevTam05-NamMinh.audio` — mẫu giọng thật đối chiếu

## 7. Lưu ý khi push

- Thư mục `scripts56/`, `scripts58/`, `scripts59/`, `scripts60/` chỉ là **bằng chứng điều tra** — build không tham chiếu; có thể xóa trước khi push cho gọn repo (giữ nguyên thư mục `scripts/` không số — bắt buộc cho build).
- CI build tự động như các FIX trước; không thêm dependency mới (gorilla/websocket đã có từ FIX58).
