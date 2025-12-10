# NOFX - Hệ Thống Giao Dịch AI

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![React](https://img.shields.io/badge/React-18+-61DAFB?style=flat&logo=react)](https://reactjs.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.0+-3178C6?style=flat&logo=typescript)](https://www.typescriptlang.org/)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)

**Ngôn ngữ:** [English](../../../README.md) | [中文](../zh-CN/README.md) | [Tiếng Việt](README.md)

---

## Nền Tảng Giao Dịch Crypto Sử Dụng AI

**NOFX** là hệ thống giao dịch AI mã nguồn mở cho phép bạn chạy nhiều mô hình AI để tự động giao dịch hợp đồng tương lai crypto. Cấu hình chiến lược qua giao diện web, theo dõi hiệu suất theo thời gian thực, và để các AI agent cạnh tranh tìm ra phương pháp giao dịch tốt nhất.

### Tính Năng Chính

- **Hỗ trợ Đa AI**: Chạy DeepSeek, Qwen, GPT, Claude, Gemini, Grok, Kimi - chuyển đổi mô hình bất cứ lúc nào
- **Đa Sàn Giao Dịch**: Giao dịch trên Binance, Bybit, OKX, Hyperliquid, Aster DEX, Lighter từ một nền tảng
- **Strategy Studio**: Trình tạo chiến lược trực quan với nguồn coin, chỉ báo và kiểm soát rủi ro
- **Chế Độ Thi Đấu AI**: Nhiều AI trader cạnh tranh theo thời gian thực, theo dõi hiệu suất song song
- **Cấu Hình Web**: Không cần chỉnh sửa JSON - cấu hình mọi thứ qua giao diện web
- **Dashboard Thời Gian Thực**: Vị thế trực tiếp, theo dõi P/L, nhật ký quyết định AI với chuỗi suy luận

### Được hỗ trợ bởi [Amber.ac](https://amber.ac)

> **Cảnh Báo Rủi Ro**: Hệ thống này mang tính thử nghiệm. Giao dịch tự động AI có rủi ro đáng kể. Chỉ nên sử dụng cho mục đích học tập/nghiên cứu hoặc kiểm tra với số tiền nhỏ!

## Cộng Đồng Nhà Phát Triển

Tham gia cộng đồng Telegram: **[NOFX Developer Community](https://t.me/nofx_dev_community)**

---

## Bắt Đầu Nhanh

### Tùy chọn 1: Triển khai Docker (Khuyến nghị)

```bash
git clone https://github.com/NoFxAiOS/nofx.git
cd nofx
chmod +x ./start.sh
./start.sh start --build
```

Truy cập giao diện Web: **http://localhost:3000**

### Tùy chọn 2: Cài đặt Thủ công

```bash
# Yêu cầu: Go 1.21+, Node.js 18+, TA-Lib

# Cài đặt TA-Lib (macOS)
brew install ta-lib

# Clone và thiết lập
git clone https://github.com/NoFxAiOS/nofx.git
cd nofx
go mod download
cd web && npm install && cd ..

# Khởi động backend
go build -o nofx && ./nofx

# Khởi động frontend (terminal mới)
cd web && npm run dev
```

---

## Thiết Lập Ban Đầu

1. **Cấu hình Mô hình AI** — Thêm API key AI
2. **Cấu hình Sàn giao dịch** — Thiết lập thông tin API sàn
3. **Tạo Chiến lược** — Cấu hình chiến lược giao dịch trong Strategy Studio
4. **Tạo Trader** — Kết hợp Mô hình AI + Sàn + Chiến lược
5. **Bắt đầu Giao dịch** — Khởi động các trader đã cấu hình

---

## Triển Khai Máy Chủ

### Triển Khai Nhanh (HTTP qua IP)

Mặc định, mã hóa truyền tải bị **tắt**, cho phép bạn truy cập NOFX qua địa chỉ IP không cần HTTPS:

```bash
# Triển khai lên máy chủ
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

Truy cập qua `http://YOUR_SERVER_IP:3000` - hoạt động ngay lập tức.

### Bảo Mật Nâng Cao (HTTPS)

Để tăng cường bảo mật, bật mã hóa truyền tải trong `.env`:

```bash
TRANSPORT_ENCRYPTION=true
```

Khi được bật, trình duyệt sử dụng Web Crypto API để mã hóa API key trước khi truyền. Điều này yêu cầu:
- `https://` - Bất kỳ domain nào có SSL
- `http://localhost` - Phát triển local

### Thiết Lập HTTPS Nhanh với Cloudflare

1. **Thêm domain vào Cloudflare** (gói miễn phí hoạt động)
   - Truy cập [dash.cloudflare.com](https://dash.cloudflare.com)
   - Thêm domain và cập nhật nameserver

2. **Tạo DNS record**
   - Loại: `A`
   - Tên: `nofx` (hoặc subdomain của bạn)
   - Nội dung: IP máy chủ của bạn
   - Trạng thái proxy: **Proxied** (đám mây màu cam)

3. **Cấu hình SSL/TLS**
   - Vào cài đặt SSL/TLS
   - Đặt chế độ mã hóa thành **Flexible**

   ```
   User ──[HTTPS]──→ Cloudflare ──[HTTP]──→ Your Server:3000
   ```

4. **Bật mã hóa truyền tải**
   ```bash
   # Chỉnh sửa .env và đặt
   TRANSPORT_ENCRYPTION=true
   ```

5. **Hoàn tất!** Truy cập qua `https://nofx.yourdomain.com`

---

## Thiết Lập Ban Đầu (Giao Diện Web)

Sau khi khởi động hệ thống, cấu hình qua giao diện web:

1. **Cấu hình Mô hình AI** - Thêm API key AI của bạn (DeepSeek, OpenAI, v.v.)
2. **Cấu hình Sàn giao dịch** - Thiết lập thông tin xác thực API sàn
3. **Tạo Chiến lược** - Cấu hình chiến lược giao dịch trong Strategy Studio
4. **Tạo Trader** - Kết hợp Mô hình AI + Sàn + Chiến lược
5. **Bắt đầu Giao dịch** - Khởi động các trader đã cấu hình

Tất cả cấu hình được thực hiện qua giao diện web - không cần chỉnh sửa file JSON.

---

## Tính Năng Giao Diện Web

### Trang Thi Đấu
- Bảng xếp hạng ROI theo thời gian thực
- Biểu đồ so sánh hiệu suất đa AI
- Theo dõi P/L trực tiếp và xếp hạng

### Dashboard
- Biểu đồ nến kiểu TradingView
- Quản lý vị thế theo thời gian thực
- Nhật ký quyết định AI với lý luận Chuỗi Suy Nghĩ
- Theo dõi đường cong vốn

### Strategy Studio
- Cấu hình nguồn coin (Danh sách tĩnh, nhóm AI500, OI Top)
- Chỉ báo kỹ thuật (EMA, MACD, RSI, ATR, Khối lượng, OI, Tỷ lệ Funding)
- Cài đặt kiểm soát rủi ro (đòn bẩy, giới hạn vị thế, sử dụng ký quỹ)
- Kiểm tra AI với xem trước prompt theo thời gian thực

---

## Vấn Đề Thường Gặp

### Không tìm thấy TA-Lib
```bash
# macOS
brew install ta-lib

# Ubuntu
sudo apt-get install libta-lib0-dev
```

### AI API timeout
- Kiểm tra API key có đúng không
- Kiểm tra kết nối mạng
- Thời gian chờ hệ thống là 120 giây

### Frontend không kết nối được backend
- Đảm bảo backend đang chạy trên http://localhost:8080
- Kiểm tra cổng có bị chiếm không

---

## Cảnh Báo Rủi Ro

1. Thị trường crypto biến động cực kỳ mạnh - Quyết định AI không đảm bảo lợi nhuận
2. Giao dịch hợp đồng tương lai sử dụng đòn bẩy - Thua lỗ có thể vượt quá vốn
3. Điều kiện thị trường cực đoan có thể dẫn đến thanh lý



## Giấy Phép

Dự án này được cấp phép theo **GNU Affero General Public License v3.0 (AGPL-3.0)** - Xem file [LICENSE](LICENSE).

---

## Đóng Góp

Chúng tôi hoan nghênh các đóng góp! Xem:
- **[Hướng Dẫn Đóng Góp](CONTRIBUTING.md)** - Quy trình làm việc và PR
- **[Quy Tắc Ứng Xử](CODE_OF_CONDUCT.md)** - Hướng dẫn cộng đồng  
- **[Chính Sách Bảo Mật](SECURITY.md)** - Báo cáo lỗ hổng

---

## Chương Trình Airdrop Cho Người Đóng Góp

Tất cả đóng góp được theo dõi trên GitHub. Khi NOFX tạo ra doanh thu, người đóng góp sẽ nhận được airdrop dựa trên mức đóng góp của họ.

**PR giải quyết [Issue Được Ghim](https://github.com/NoFxAiOS/nofx/issues) nhận phần thưởng CAO NHẤT!**

| Loại Đóng Góp | Trọng Số |
|------------------|:------:|
| **PR Issue Được Ghim** | ⭐⭐⭐⭐⭐⭐ |
| **Commit Code** (PR đã merge) | ⭐⭐⭐⭐⭐ |
| **Sửa Lỗi** | ⭐⭐⭐⭐ |
| **Đề Xuất Tính Năng** | ⭐⭐⭐ |
| **Báo Lỗi** | ⭐⭐ |
| **Tài Liệu** | ⭐⭐ |

---

## Liên Hệ

- **GitHub Issues**: [Gửi Issue](https://github.com/NoFxAiOS/nofx/issues)
- **Cộng đồng Nhà phát triển**: [Nhóm Telegram](https://t.me/nofx_dev_community)

---

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.com/#NoFxAiOS/nofx&Date)
