# assets/brand — Icon cá nhân hoá (VKS)

Thư mục này chứa bộ brand của bạn. **FIX55 đã nhúng sẵn icon tạm** (icon
"VKS · HCStudio" vẽ sẵn) nên exe build từ GitHub đã có icon ngay lập tức.
Khi bạn muốn dùng icon cá nhân của mình, chỉ cần chép đúng tên file vào đây:

```
assets/brand/
  vks.ico   ← icon chính cho file HCStudio.exe (Windows) + installer
  vks.png   ← bản PNG vuông (khuyên 1024×1024) làm appicon của Wails
```

## Cách hoạt động (tự động 100%, không cần sửa code)

Lần CI sau khi bạn push, `scripts/build.ps1` sẽ:

1. Thấy `assets/brand/vks.png` → chép đè lên `build/appicon.png`.
2. Thấy `assets/brand/vks.ico` → chép đè `build/windows/icon.ico` và gọi
   `go-winres` (tự `go install` nếu máy build chưa có) để sinh lại
   `rsrc_windows_amd64.syso` — file resource mà `go build` TỰ ĐỘNG nhúng
   vào exe. Kết quả: `HCStudio.exe` hiện icon của bạn trên Explorer,
   thanh title bar, taskbar và trong Properties (tab Details).
3. Nếu không có `vks.*` → dùng lại `rsrc_windows_amd64.syso` có sẵn trong
   repo (icon tạm) — build không bao giờ fail vì thiếu icon.

## Lưu ý kỹ thuật

- Nguyên nhân gốc để FIX55 sửa: CI build bằng `go build` thuần (không qua
  `wails build`) nên không bao giờ có bước nhúng resource → exe trơn nhẵn
  không icon. File `.syso` đặt cạnh `main.go` là cơ chế chuẩn của Go tool
  để nhúng icon/version mà không cần đổi dòng lệnh build.
- `vks.ico` nên chứa nhiều cỡ (16/24/32/48/64/128/256) để sắc nét ở mọi
  chỗ hiển thị. Icon tạm trong repo đã làm đúng cách này.
- `make-setup.ps1` (Inno Setup) cũng tự dùng `build/windows/icon.ico` làm
  SetupIconFile — file Setup.exe sẽ đồng bộ icon với exe.
