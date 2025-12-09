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

## Cảnh Báo Rủi Ro

1. Thị trường crypto biến động cực kỳ mạnh — Quyết định AI không đảm bảo lợi nhuận
2. Giao dịch hợp đồng tương lai sử dụng đòn bẩy — Thua lỗ có thể vượt quá vốn
3. Điều kiện thị trường cực đoan có thể dẫn đến thanh lý

---

## Giấy Phép

**GNU Affero General Public License v3.0 (AGPL-3.0)**

---

## Liên Hệ

- **GitHub Issues**: [Gửi Issue](https://github.com/NoFxAiOS/nofx/issues)
- **Cộng đồng Nhà phát triển**: [Nhóm Telegram](https://t.me/nofx_dev_community)
