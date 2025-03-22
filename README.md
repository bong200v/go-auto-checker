# Go Auto Checker - Công cụ tự động đăng nhập đa luồng

Ứng dụng viết bằng Go hỗ trợ tự động đăng nhập hàng loạt tài khoản từ file Excel với tốc độ cao, mô phỏng thiết bị thật và giải captcha tự động.

## Chức năng chính

- **Đọc tài khoản từ file Excel**: Hỗ trợ đọc danh sách tài khoản/mật khẩu từ file Excel
- **Xử lý song song**: Chạy nhiều worker cùng lúc, mỗi worker đăng nhập một tài khoản
- **Mô phỏng thiết bị**: Mỗi worker sử dụng thiết bị ảo riêng với UserAgent, FingerIDX và độ phân giải màn hình khác nhau
- **Giải captcha tự động**: Tự động xác minh slider captcha với tỷ lệ thành công cao
- **Lưu kết quả**: Lưu kết quả đăng nhập vào file JSON và cập nhật lại file Excel ban đầu
- **Hiệu suất cao**: Đăng nhập hàng chục tài khoản chỉ trong vài giây

## Cài đặt

Yêu cầu Go phiên bản 1.16 trở lên.

```bash
# Tải về mã nguồn
git clone https://github.com/username/Go_auto_checker.git
cd Go_auto_checker

# Cài đặt các dependency
go mod download
```

## Cách sử dụng

### Tạo file Excel mẫu

```bash
go run cmd/create_sample_excel/main.go
```

Lệnh này sẽ tạo file Excel mẫu trong thư mục `sample_data/accounts.xlsx` với một số tài khoản mẫu.

### Chạy chương trình với file Excel

```bash
go run cmd/excel_workers/main.go -excel sample_data/accounts.xlsx -max 10 -save
```

Tham số:
- `-excel`: Đường dẫn đến file Excel chứa danh sách tài khoản
- `-max`: Số lượng worker chạy đồng thời tối đa (mặc định: 10)
- `-save`: Lưu kết quả vào file
- `-v`: Hiển thị thông tin chi tiết (verbose)
- `-start`: Dòng bắt đầu đọc trong file Excel (mặc định: 2, bỏ qua tiêu đề)

### Định dạng file Excel

- **Cột A**: STT (không bắt buộc)
- **Cột B**: Tài khoản
- **Cột C**: Mật khẩu
- **Cột D-G**: Kết quả (sẽ được cập nhật sau khi chạy với flag `-save`)

## Cấu trúc dự án

```
.
├── cmd/
│   ├── create_sample_excel/  # Tạo file Excel mẫu
│   └── excel_workers/        # Chương trình chính xử lý Excel
├── internal/
│   ├── models/               # Các model dữ liệu
│   ├── session/              # Xử lý phiên đăng nhập
│   └── workers/              # Xử lý đa luồng
├── results/                  # Thư mục chứa kết quả
├── sample_data/              # Dữ liệu mẫu
├── go.mod
├── go.sum
└── README.md
```

## Hiệu suất

- Với 10 tài khoản, chạy song song 10 worker: ~1.8 giây
- Mỗi worker mất trung bình 1.7 giây để đăng nhập thành công
- Thời gian tổng thể chỉ phụ thuộc vào worker lâu nhất

## Giải pháp kỹ thuật

### Mô phỏng thiết bị thực

Mỗi worker mô phỏng một thiết bị khác nhau với:
- UserAgent ngẫu nhiên: Windows, MacOS, Linux
- Độ phân giải màn hình ngẫu nhiên: Desktop hoặc Mobile
- FingerIDX duy nhất cho mỗi phiên

### Xử lý song song hiệu quả

- Sử dụng goroutines để chạy các worker song song
- Xử lý theo batch khi số lượng tài khoản lớn hơn max worker
- Mỗi worker hoàn toàn độc lập với session riêng

## Một số lưu ý

- Chương trình sẽ bỏ qua các dòng trong Excel không có đủ thông tin tài khoản/mật khẩu
- Kết quả đăng nhập được lưu trong thư mục `results/`
- File Excel kết quả có định dạng `accounts_result.xlsx` 