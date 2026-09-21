# HCStudio-v5 — FIX57: SỬA "(ĐÃ THỬ 0)" + GIỌNG OD KHÔNG DÙNG ĐƯỢC

**Ngày:** 2026-09-21 · **Cơ sở:** báo cáo người dùng trên bộ FIX56 — *"tất cả dịch vụ có giọng "Quang Sơn" đều không thành công (đã thử 0). Giọng này hiện chỉ có trên các dịch vụ đang lỗi… Hiện tại các giọng OD không sử dụng được và nhiều giọng cũng không sử dụng được."*

---

## 1. Nguyên nhân gốc (probe thật 2026-09-21 04:23 → 05:20 — scripts58/)

| # | Phát hiện | Bằng chứng |
|---|-----------|------------|
| 1 | **"(đã thử 0)" = cooldown 5 phút tự khóa toàn bộ chuỗi.** Mỗi dịch vụ lỗi bị đánh dấu `CooldownUntil = now+5ph` và **ghi nhớ vào settings**; lần bấm Chuyển đổi kế tiếp (dưới 5 phút) bị **bỏ qua toàn bộ không gọi request nào** — trong khi đó các space ZeroGPU **hồi phục theo CƠN** nên trạng thái cũ hoàn toàn đánh giá sai | `chain.go` cũ: `ErrCooldown = 5 * time.Minute` + snapshot persist. probe58: pnnbao-ump chết lúc 04:04 → **sống lại 05:01** (complete 5,2s) |
| 2 | **Lỗi 400 "giọng chưa có" của vieneu.io bị đặt cooldown oan** — dịch vụ còn sống, nhưng bị khóa 5 phút kể cả khi lần sau chọn GIỌNG KHÁC vẫn dùng được | probe58: cùng 1 lần chạy, giọng A 400 còn giọng B 201 (tập giọng demo xoay vòng) |
| 3 | **Hạn mức vieneu.io bị khóa "hết ngày" mặc dù server chỉ khóa ~7 giờ/lượt cửa sổ** — app không bao giờ biết hạn mức đã nạp lại | `ExhaustedUntil = today()` + persist |
| 4 | **Space ZeroGPU chập chờn THEO TỪNG REQUEST, không chết hẳn**: cùng 1 space pnnbao-ump, cùng 1 phút — giọng **Mai Anh complete 2,8s** trong khi **Quang Sơn + Ngọc Trân error 0,7s**; 3 phút trước đó space này complete với "Minh Quân Pro" | probe58b 05:04 — log kèm gói |
| 5 | **hongqminh + eagle0019 + Tuananh20015 cũng hồi phục/ổn định trở lại**: hongqminh complete 1,5s ("Ngọc (nữ miền Bắc)"), eagle0019 20,1s, Tuananh20015 3,5s | probe58/58b |
| 6 | **Phát hiện dịch vụ CPU MỚI dùng được**: `nguyenduc1222/VieNeu-TTS` (cpu-basic) — cùng họ hongqminh, 9 giọng vùng miền, **complete 1,6s** qua 5 tham số như hongqminh | probe58b + /info Literal thật |

**Kết luận:** 2 cơ chế cùng gây ra hiện tượng "OD không dùng được": (a) cooldown/khóa-hạn-mức ghi nhớ từ lần hỏng trước khiến chuỗi **không thèm thử** dù server đã hồi phục — đúng lúc các space lại chập chờn theo cơn; (b) 4 giọng OD (Mai Anh, Ngọc Trân, Quang Sơn, Minh Triết) đang phụ thuộc vào nhóm space chập chờn + tập giọng demo vieneu.io xoay vòng. FIX57 không biến các server yếu mạnh lên được — nhưng bảo đảm **mỗi lần bấm là một lượt thử thật sự**, không bỏ sót bất kỳ cửa sổ hồi phục nào.

## 2. FIX57 thay đổi gì

1. **`internal/cloud/chain.go` — `BeginUserRun()` (PATCH chính):** mỗi lần user **chủ động** bấm *Chuyển đổi* (hoặc nút *Làm mới*) → xóa toàn bộ cooldown + cạn-hạn-mức lưu sẵn để chuỗi **LUÔN gọi thật ít nhất 1 lần mỗi dịch vụ**. Hết hẳn case "(đã thử 0)". LastGood (ưu tiên dịch vụ thành công gần nhất) giữ nguyên. Hạn mức thật (429) sẽ bị đánh dấu lại ngay trong run nếu server vẫn từ chối — chỉ tốn đúng 1 request kiểm tra.
2. **`internal/cloud/http.go` — sentinel `voiceNotAvailErr`:** lỗi 400 "is not available" của demo vieneu.io **KHÔNG còn đặt cooldown** — dịch vụ còn sống, giọng khác của lần chạy sau dùng được ngay.
3. **`internal/cloud/chain.go` — thông điệp lỗi cuối sát thật:** *"…(đã thử N). Các Server này đang chập chờn theo cơn — bấm Chuyển đổi thử lại NGAY (lần sau thường được)…"* (bỏ gợi ý "giọng OD khác" vô nghĩa khi chính giọng đó là OD).
4. **`internal/cloud/cloud.go` + `registry_data.go` — thêm `hf-nguyenduc1222`** (CPU, không tốn hạn mức GPU) vào vị trí #4, ngay sau hongqminh: dự phòng cho 6 giọng vùng miền (Tuyên/Vĩnh/Bình/Ngọc/Ly/Đoan) khi hongqminh chập chờn + thêm 3 giọng Nam mới (Nguyên/Sơn/Dung). Chuỗi trở lại **13 dịch vụ**.
5. **`app.go`** — `CloudSynthesize()` gọi `BeginUserRun()` trước mỗi run; `CloudProviders()` (nút Làm mới) cũng gọi — thẻ Dịch vụ Online luôn phản ánh sức khỏe HIỆN TẠI khi user chủ động kiểm tra.
6. **Test mới (2):** `TestChainBeginUserRunClearsSkip` (run 2 bị skip → sau BeginUserRun được gọi thật), `TestChainVoiceNotAvailNoCooldown` (400 giọng-tạm-chưa-có không cooldown + lần 2 vẫn gọi thật); cập nhật `TestRegistryShapeAndVoiceResolve` 13 dịch vụ + thứ tự mới.
7. **UI/bridge mock** — thêm dịch vụ #4 nguyenduc1222 khớp backend.
8. **E2E mạng thật qua code Go (guard `F57_E2E=1`):** "Ngọc (nữ miền Bắc)" → **thành công 144 KB qua hongqminh 3,2s**; "Quang Sơn"/"Mai Anh" → chuỗi thử thật đủ **7/7 dịch vụ** (~6s), vienеu 400 bỏ qua tức thì, thông báo trung thực đúng thời điểm space chập chờn (Mai Anh đã complete 2,8s lúc 05:04 — chứng minh chỉ cần gặp đúng cửa sổ hồi phục).

**Chuỗi online 13 dịch vụ sau FIX57:**

| Vị trí | Dịch vụ | Trạng thái lúc probe 05:04–05:20 |
|--------|---------|--------------------------------|
| 1 | vieneu.io (api.vieneu.io) | ✅ 201 ~2s (5/11 giọng app nhận lúc 04:23: Adam Tốp Tốp, Ngọc Lan, Minh Đức, Thùy Dung, Mạnh Dũng; tập giọng xoay vòng) |
| 2 | DevTam05 (CPU) | ✅ ổn định nhất (MP3 — đã decode được từ FIX56) |
| 3 | hongqminh (CPU) | ✅ hồi phục — 1,5s ("Ngọc (nữ miền Bắc)") |
| 4 | **nguyenduc1222 (CPU) — MỚI** | ✅ 1,6s — dự phòng họ hongqminh |
| 5 | pnnbao-ump (space tác giả) | ⚠️ chập chờn theo cơn — 05:01 complete 5,2s; 05:04 Mai Anh OK 2,8s |
| 6 | arena Thomcles | tự bỏ qua (chỉ bỏ phiếu) |
| 7–13 | trangmin, eagle0019 ✅ 20,1s, xtieps, thienan2146, doremon102, kabinz, Tuananh20015 ✅ 3,5s | ⚠️ chập chờn — mỗi lần bấm Chuyển đổi đều được thử thật lại |

**Kỳ vọng cho 8 giọng OD (đại bộ phận phụ thuộc cửa sổ hồi phục của server):**

| Giọng OD | Nguồn khả dụng hôm nay |
|----------|------------------------|
| Minh Đức, Thùy Dung | vieneu.io ✅ + template spaces — dùng được ngay |
| Thái Sơn, Ngọc Linh | eagle0019/Tuananh20015 (CPU) ✅ + vieneu.io xoay vòng — dùng được ngay (~4–20s) |
| Mai Anh | pnnbao/kabinz ⚠️ + vieneu.io xoay vòng — bấm lại khi gặp cửa sổ hồi phục |
| Ngọc Trân, Quang Sơn, Minh Triết | nhóm template ⚠️ + vieneu.io xoay vòng — bấm lại khi gặp cửa sổ hồi phục |

**KHÔNG đụng:** engine offline, xuất audio, danh sách phát, wizard weights, icon FIX55, bộ lọc giọng FIX55, timeout 3 phút FIX56.

## 3. Đóng gói & rollback

- **File:** `HCStudio-v5-FIX57-online-revive2.zip` (giải nén ĐÈ nguyên repo → push → CI).
- **Rollback:** khôi phục cây nguồn từ `HCStudio-v5-FIX56-online-revive.zip` (đè lại). Không thay đổi data/migration — `settings.json` giữ nguyên (các trạng thái cooldown cũ trong settings sẽ tự được xóa ở lần Chuyển đổi đầu tiên).

## 4. 10 ca nghiệm thu

| Ca | Bước thực hiện | Kết quả đúng |
|----|----------------|--------------|
| 1 | Chuyển đổi online, giọng **"Ngọc (nữ miền Bắc)"** (hoặc Tuyên/Vĩnh/Bình/Đoan/Ly), câu ngắn | Có audio trong vài giây (qua hongqminh hoặc nguyenduc1222) — đúng giọng nữ miền Bắc |
| 2 | Chuyển đổi online, giọng **"Quang Sơn"** — đúng kịch bản từng báo lỗi | Nếu gặp cửa sổ hồi phục: có audio đúng giọng nam Trung. Nếu tất cả đang chập chờn: thông báo **"(đã thử 7)"** + khuyên bấm lại NGAY — **không bao giờ còn "(đã thử 0)"** |
| 3 | Bấm Chuyển đổi **ngay lần thứ 2** sau 1 lần hỏng (dưới 5 phút) | Chuỗi **vẫn gọi thật từng dịch vụ** (không bị khóa 5 phút như FIX56) |
| 4 | Chọn giọng **"Minh Đức" hoặc "Thùy Dung" hoặc "Mạnh Dũng"** | Audio nhanh ~2–4s qua vieneu.io (giọng đang được demo nạp) |
| 5 | Chọn giọng **"Thái Sơn" hoặc "Ngọc Linh"** (OD kể chuyện) | Audio ~4–20s qua Tuananh20015/eagle0019 (CPU), đúng giọng |
| 6 | Chọn giọng **"Hoài My (Nữ)" / "Nam Minh (Nam)"** | Audio ~1–2s qua DevTam05 (MP3 decode sẵn) |
| 7 | Thẻ **Dịch vụ Online** → bấm **Làm mới** | Danh sách 13 dịch vụ (có thêm nguyenduc1222 #4); trạng thái lỗi cũ được xóa sạch — phản ánh sức khỏe hiện tại |
| 8 | Chọn giọng lạ (không ở đâu có) | Báo lỗi NGAY không tốn request, gợi ý 8 giọng OD/Offline (giữ nguyên FIX56) |
| 9 | Khi vienеu.io từ chối giọng rồi chọn giọng KHÁC mà vienеu.io có | vienеu.io **không bị khóa oan** — vẫn được thử đầu chuỗi ngay lần kế |
| 10 | Chế độ Offline + toàn bộ xuất/phát/playlist | Không đổi so với FIX52–56 — engine offline hoạt động bình thường |

## 5. Bằng chứng kèm gói

- `scripts58/probe57.py + probe57.log` — 04:23: www.vieneu.io còn sống (201), api.vieneu.io 5/11 giọng, hongqminh lỗi, eagle0019/Tuananh20015 hồi phục, DevTam05 MP3.
- `scripts58/probe58.py + probe58.log` — 05:01: pnnbao-ump hồi phục (5,2s); phát hiện nguyenduc1222.
- `scripts58/probe58b.py + probe58b.log` — 05:04: cùng space cùng phút, Mai Anh OK 2,8s / Quang Sơn error → chập chờn theo request; nguyenduc1222 5p complete 1,6s; hongqminh 1,5s; eagle0019 20,1s.
- E2E qua code Go (guard `F57_E2E`): "Ngọc (nữ miền Bắc)" 144 KB ✅; "Quang Sơn"/"Mai Anh" đã thử 7/7 dịch vụ thật.
