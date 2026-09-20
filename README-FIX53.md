# FIX53 — CHẾ ĐỘ SỬ DỤNG ONLINE: cầu nối chuyển tiếp vieneu.io → HF Space

> Phạm vi chốt với chủ app: **giữ nguyên gần như toàn bộ code FIX52**, tích
> hợp thêm tính năng **Sử dụng online** — có nút/tab riêng trong giao diện.
> Ở chế độ online, người dùng nhập nội dung (hoặc nạp file .txt/.srt/.md),
> phần mềm làm **cầu nối chuyển tiếp** nội dung lên `https://www.vieneu.io/`;
> nếu hết lượt thì chuyển `pnnbao-ump/VieNeu-TTS-v3-Turbo`; vẫn hết/lỗi thì
> lần lượt theo đúng thứ tự chủ app liệt kê cho tới khi có audio.

---

## 0. Chuỗi cầu nối — đúng thứ tự chủ app chỉ định

| # | Dịch vụ | Kiểu API | Ghi chú thực tế (khảo sát 2026-09-20) |
|---|---|---|---|
| 1 | `https://www.vieneu.io/` | REST `POST /api/tts/demo {text, voiceId}` → `{audioBase64, mimeType}` | Đã test thật: WAV về ngay · catalog 1.204 giọng (featured 10) |
| 2 | `pnnbao-ump/VieNeu-TTS-v3-Turbo` | Gradio `synthesize` | Space của tác giả; đang lỗi tức thì → chuỗi tự nhảy |
| 3 | `Thomcles/yodalingua-tts-arena` | — | **Tự bị bỏ qua**: trang chỉ phục vụ bỏ phiếu mù (pair có sẵn), KHÔNG có API tạo giọng theo văn bản tùy ý |
| 4 | `trangmin11101996/VieNeu-TTS-v3-Turbo` | Gradio `synthesize` | 23 giọng |
| 5 | `eagle0019/VieNeu-TTS-v3-Turbo` | Gradio `synthesize` | **Đã test end-to-end thật: WAV 300 KB** |
| 6 | `xtieps/VieNeu-TTS-v3-Turbo` | Gradio `synthesize` | 14 giọng |
| 7 | `thienan2146/VieNeu-TTS-v3-Turbo` | Gradio `synthesize` | 20 giọng |
| 8 | `Smrfhdl/tts` | Gradio `synthesize` | 23 giọng; app ưu tiên chế độ **CPU không tốn hạn mức 300 giây/ngày** |
| 9 | `doremon102/VieNeu-TTS-v3-Turbo` | Gradio `synthesize` | 23 giọng |
| 10 | `kabinz/VieNeu-TTS-v3-Turbo` | Gradio `synthesize` | 10 giọng |

Mọi signature ở trên đều được **đọc trực tiếp từ `/config` + `/gradio_api/info`
của từng space** và **test gọi thật** (vieneu.io + eagle0019 trả WAV thật) —
không đoán mò. Giọng mỗi space là VALUE thật gradio chấp nhận (không phải label).

## 1. Luồng "hết lượt → chuyển tiếp" hoạt động thế nào

1. Người dùng bấm **"Chuyển đổi online"** (chỉ bấm được 1 lần, nút tự khoá —
   đúng cơ chế FIX52).
2. App đi qua chuỗi **đúng thứ tự 10 dịch vụ** ở bảng trên:
   - Dịch vụ **hết lượt** (429 / quota / hạn mức / GPU) → đánh dấu *cạn lượt
     tới hết hôm nay*, ghi vào settings → **chuyển ngay dịch vụ kế tiếp**.
   - Dịch vụ **lỗi thường / space đang ngủ** → thử đánh thức (đợi ~25s, thử
     lại 1 lần) → vẫn lỗi thì tạm tránh 5 phút → chuyển tiếp.
   - Space trả **heartbeat / process_starts** trong lúc xếp hàng → app cứ
     nhích thanh tiến độ + đồng hồ chờ — **hết cảnh "bấm xong đứng im"**.
3. Dịch vụ nào thành công trước thì lấy; thành công gần nhất được **ưu tiên
   đứng đầu lần sau**.
4. Audio nhận về được decode thành phiên bình thường → **PHÁT / xuất
   WAV·MP3 / danh sách phát dùng lại nguyên hệ FIX52**.
5. Mọi mốc hiện trong thanh tiến độ: `Đang gửi tới vieneu.io…` → `vieneu.io
   hết lượt — tự động chuyển dịch vụ kế tiếp.` → `Đang tổng hợp trên HF ·
   eagle0019…` → toast tóm tắt `đã thử N dịch vụ`.

## 2. Giao diện — nút/tab "Sử dụng online"

- Đầu thẻ **Văn bản** có cặp tab: **🖥 Offline** (mặc định) / **☁ Sử dụng online**.
- Bật online → hiện hàng tuỳ chọn: **dropdown giọng online** (gộp catalog
  thật của các dịch vụ; chọn "Giọng mặc định từng dịch vụ" nếu muốn) + nút
  **Nạp file** `.txt/.srt/.md` (dialog native, cap 200.000 ký tự).
- Nhãn nút đổi thành **"Chuyển đổi online"**; "Nghe ngay" (streaming cục bộ)
  tự ẩn vì không áp dụng.
- Sidebar có thẻ **"Dịch vụ Online"**: 10 dòng, chấm màu trạng thái (xanh =
  hoạt động, vàng = hết lượt, đỏ = lỗi, xám = bỏ qua, xanh nhạt = chưa dùng),
  số lượt đã dùng hôm nay, giờ thành công gần nhất, nút 🔄 làm mới.
- Di chuột vào từng dòng hiện ghi chú/li do (arena giải thích vì sao bị bỏ qua).

## 3. An toàn & lịch sự với dịch vụ

- Không tự động tạo tài khoản, không lưu cookie/token; chỉ gọi API public
  như một trình duyệt (User-Agent tự nhận danh `HCStudio-v5-FIX53`).
- **1 job online duy nhất tại một thời điểm** — bấm Chuyển đổi/Dừng khi đang
  chạy sẽ huỷ job cũ (giống cơ chế offline).
- Bộ đếm lượt từng dịch vụ lưu trong `settings.json` — ngày mới tự reset.
- Engine offline FIX46–FIX52 KHÔNG bị đụng tới: bật/tắt online thuần UI.

## Bảng thay đổi

| File | Nội dung |
|---|---|
| `internal/cloud/cloud.go` | mới — types + Registry() 10 dịch vụ đúng thứ tự + ResolveVoice + timeout profile |
| `internal/cloud/registry_data.go` | mới — catalog giọng SINH TỰ ĐỘNG từ probe thật (10 featured vieneu.io + 8 space) |
| `internal/cloud/http.go` | mới — client vieneu.io (`/api/tts/demo`) + gradio 5.x (POST call → SSE → tải audio) + phân loại lỗi quota/wake |
| `internal/cloud/chain.go` | mới — Chain: đi chuỗi, bỏ qua cạn lượt/cooldown, đánh thức space, nhớ trạng thái, SnapshotJSON persist |
| `internal/cloud/cloud_test.go` | mới — 11 test với server giả httptest: OK, quota, lỗi, đánh thức, thứ tự fallback, arena bỏ qua, round-trip snapshot |
| `internal/appstate/settings.go` | +`CloudState` (snapshot chuỗi) +`CloudVoice` |
| `app.go` | +`CloudProviders`, +`CloudSynthesize`, +`runCloudSynthesis` (job event hợp nhất `hcstudio:job` → tái dùng toàn bộ timeline/PHÁT/xuất), +`CloudPickTextFile`; startup nạp chain; audio cloud thành Session thường (ring 20) |
| `frontend/dist/index.html` | cặp tab Offline/Online + hàng tuỳ chọn online + thẻ "Dịch vụ Online" |
| `frontend/dist/assets/js/ui.js` | +wireFix53/renderModeSeg/renderCloudPanel/rebuildCloudVoiceOptions; convertRequested có nhánh online |
| `frontend/dist/assets/js/state.js` | +`mode/cloudVoice/cloudProviders` (dọn khối FIX52 bị lặp) |
| `frontend/dist/assets/js/bridge.js` | +3 method (thật + mock); dọn `ListSessions` trùng |
| `frontend/dist/assets/js/icons.js` | +cloud/desktop/filetext/refresh |
| `frontend/dist/assets/css/app.css` | style mode-seg, cloud-row, cloud-card, dot trạng thái |

## Kiểm chứng đã chạy

- `gofmt` sạch toàn module · `go vet ./internal/... .` EXIT=0
- `go test ./internal/...` PASS hết (cloud 11/11, dsp, textnorm giữ nguyên)
- `node --check` 5/5 file JS
- Live probe thật ngày 2026-09-20: vieneu.io demo trả WAV 317 KB;
  eagle0019 trả WAV 300 KB qua SSE heartbeat→complete; 8/8 space gradio
  5.49.1 cùng signature; arena xác nhận không có API tạo giọng.

## Nghiệm thu trên máy thật (sau CI)

1. Mở app → thấy cặp tab **Offline / Sử dụng online** đầu thẻ Văn bản.
2. Bấm **Sử dụng online** → hiện hàng giọng online + nút nạp file; thẻ
   "Dịch vụ Online" xuất hiện ở sidebar với 10 dòng.
3. Nhập đoạn văn ngắn → **Chuyển đổi online** → thanh tiến độ chạy, ghi rõ
   từng dịch vụ đang thử; nút chỉ bấm được 1 lần.
4. Nếu vieneu.io còn lượt → audio về rất nhanh, toast hiện "vieneu.io (chính
   thức) · đã thử 1 dịch vụ"; app tự phát.
5. Nếu vieneu.io hết lượt → dòng vàng "hết lượt — chuyển dịch vụ kế tiếp";
   chuỗi tự đi tiếp các space; toast cuối ghi tên dịch vụ thành công.
6. Sau khi xong: **PHÁT** nghe lại được, **WAV/MP3** xuất đúng thư mục
   đã nhớ, kết quả xuất hiện trong **danh sách phát** (kèm nhãn cloud:…).
7. Dropdown giọng online: chọn một giọng có thật (vd "Trúc Ly") → dòng nào
   không có giọng đó sẽ dùng giọng mặc định và ghi chú trong stage/toast.
8. Nút **Nạp file**: chọn .txt/.srt → nội dung vào ô văn bản, đếm ký tự đúng.
9. Bấm 🔄 trong thẻ Dịch vụ Online → trạng thái làm mới; dịch vụ hết lượt
   hôm nay có chấm vàng + chữ "Hết lượt hôm nay".
10. Ngày hôm sau mở lại app: bộ đếm/cạn lượt tự reset (không phải làm gì).
11. Quay lại tab **Offline** → toàn bộ tính năng FIX52 (streaming, hàng đợi,
    nhân bản giọng) chạy như trước — không có gì thay đổi.

## Rollback

Ghi đè repo bằng zip FIX52 — FIX53 là add-on thuần: package `internal/cloud`
mới + method bridge mới + UI mới, KHÔNG đụng engine offline, KHÔNG đụng C++
core, không cần build lại toolchain.
