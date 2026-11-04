<p align="center">
  <img src="../../assets/nofx-banner.svg" alt="nofx — Trí tuệ đi vào giao dịch. Hệ thống giao dịch mở do bạn tự vận hành." width="100%"/>
</p>

<p align="center">
  <a href="#frontier">Frontier</a> ·
  <a href="#run-locally">Bắt đầu</a> ·
  <a href="#documentation">Tài liệu</a> ·
  <a href="#community">Cộng đồng</a>
</p>

<p align="center">
  <a href="https://github.com/NoFxAiOS/nofx/stargazers"><img src="https://img.shields.io/github/stars/NoFxAiOS/nofx?style=flat&labelColor=192434&color=295BE8" alt="GitHub stars"></a>
  <a href="../../../LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-295BE8?style=flat&labelColor=192434" alt="License: AGPL-3.0"></a>
  <a href="https://t.me/nofx_dev_community"><img src="https://img.shields.io/badge/telegram-community-295BE8?style=flat&labelColor=192434&logo=telegram&logoColor=white" alt="Telegram community"></a>
</p>

<p align="center">
  <a href="../../../README.md">English</a> ·
  <a href="../zh-CN/README.md">中文</a> ·
  <a href="../ja/README.md">日本語</a> ·
  <a href="../ko/README.md">한국어</a> ·
  <a href="../ru/README.md">Русский</a> ·
  <a href="../uk/README.md">Українська</a> ·
  <a href="README.md">Tiếng Việt</a>
</p>

NOFX là hệ thống giao dịch mã nguồn mở chạy trên máy tính của bạn. Dùng **Frontier** để trao đổi ý tưởng giao dịch với AI, so sánh cách vào lệnh trên biểu đồ và tự phê duyệt lệnh. Dùng **Autopilot** để vận hành chiến lược liên tục. Cả hai cùng sử dụng các mô hình và tài khoản sàn mà bạn đã cấu hình.

Được xây dựng cùng [Vergex](https://vergex.trade). Truy cập AI và dữ liệu qua [Claw402](https://claw402.ai).

<a id="run-locally"></a>

## Chạy trên máy của bạn

**Cài đặt bằng một lệnh — Linux / macOS**

Cài đặt và khởi động Docker có Compose, rồi chạy:

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

Mở **http://127.0.0.1:3000** trong trình duyệt trên máy tính.

Trình cài đặt tạo các khóa cấu hình còn thiếu và khởi động NOFX bằng Docker image đã phát hành. Không cần cài Go hay Node.js. Tiếp tục với [thiết lập lần đầu](#your-first-session), hoặc xem [biên dịch từ mã nguồn và các lựa chọn khác](#source-builds-and-other-options).

## Frontier

Một biểu đồ, một cuộc trao đổi và một kế hoạch giao dịch có thể kiểm tra trước khi đặt lệnh.

1. **Chọn thị trường.** Duyệt các mã có khối lượng giao dịch cao, tìm theo mã gốc của sàn hoặc dùng danh sách Binance AI500. Các thị trường liên kết với cổ phiếu sẽ xuất hiện nếu sàn hỗ trợ.
2. **So sánh cách vào lệnh.** Yêu cầu kế hoạch dùng lệnh thị trường, lệnh giới hạn, lệnh bậc thang theo tỷ trọng và TWAP. Xem cùng lúc giá vào, cắt lỗ, mục tiêu, đòn bẩy, giá trị vị thế và ký quỹ ước tính.
3. **Kiểm tra và giao dịch.** Chọn một kế hoạch để tập trung biểu đồ vào kế hoạch đó. Kiểm tra tài khoản hiện tại và chi tiết lệnh, rồi chủ động phê duyệt thực thi.
4. **Quản lý vị thế.** Xem vị thế thực tế và lệnh trên sàn ở dưới biểu đồ; có thể mở lệnh thủ công hoặc đóng vị thế tại đây.

Frontier hỗ trợ **hợp đồng vĩnh cửu USDT của Binance, Hyperliquid, OKX và Bitget**. Bạn có thể xem dữ liệu thị trường công khai trước khi kết nối tài khoản giao dịch; muốn đặt lệnh cần có tài khoản đúng sàn. Tạo kế hoạch không đồng nghĩa với gửi lệnh.

| Cách vào lệnh | Cách hoạt động |
| :--- | :--- |
| **Thị trường** | Vào theo giá thị trường có thể khớp sau khi phê duyệt; giá hiển thị là ước tính. |
| **Giới hạn** | Gửi lệnh vào tại mức giá đã chọn. |
| **Bậc thang** | Chia vị thế theo tỷ trọng giữa nhiều mức giá. |
| **TWAP** | Gửi một số lượng lệnh hữu hạn theo thời gian, trong giới hạn giá đã phê duyệt. |

Lệnh giới hạn và các phần lệnh theo lịch có thể không khớp. Xem [chi tiết thực thi và khôi phục](../../features/trade-planning.md).

## Autopilot

Để chạy chiến lược liên tục, hãy cấu hình một trader với mô hình, tài khoản sàn và chiến lược. Autopilot đọc thị trường, đưa ra quyết định và gửi lệnh qua hệ thống thực thi của NOFX. Dashboard hiển thị vị thế, hiệu suất và lịch sử quyết định.

**Frontier yêu cầu phê duyệt từng giao dịch. Autopilot có thể tự đặt lệnh khi đang chạy.** Khởi động và dừng Autopilot là các thao tác riêng.

| Khu vực | Công dụng |
| :--- | :--- |
| **Config** | Kết nối mô hình, nạp tiền vào ví AI và quản lý tài khoản giao dịch. |
| **Frontier** | Khám phá thị trường, so sánh kế hoạch vào lệnh và đặt lệnh đã kiểm tra. |
| **Dashboard** | Theo dõi trader đang chạy, vị thế và lịch sử quyết định. |
| **Strategy** | Cấu hình lựa chọn thị trường, chỉ báo, quy mô vị thế và chỉ dẫn giao dịch. |
| **Leaderboard** | So sánh hiệu suất trader được công khai. |

<a id="your-first-session"></a>

## Thiết lập lần đầu

1. **Tạo tài khoản cục bộ.** Đăng nhập vào NOFX vừa khởi động.
2. **Thiết lập AI trong Config.** Chọn mô hình. Với Claw402, tạo hoặc nhập ví thanh toán phí AI, lưu khóa riêng và chuyển **USDC trên mạng Base** vào địa chỉ ví. Số dư này dùng để trả phí yêu cầu mô hình và dữ liệu trả phí.
3. **Thêm tài khoản giao dịch.** Kết nối sàn bạn muốn dùng, kiểm tra tổng giá trị tài khoản và số dư khả dụng. Tiền giao dịch tách biệt với ví phí AI.
4. **Mở Frontier.** Chọn sàn, tài khoản và mã giao dịch. Tạo kế hoạch, so sánh trên biểu đồ và kiểm tra lệnh khi đã sẵn sàng.

Để giao dịch liên tục, hãy cấu hình và khởi động Autopilot riêng.

### Cập nhật bản cài bằng trình cài đặt

Dừng các trader Autopilot đang hoạt động và kiểm tra các giao dịch Frontier đang thực thi, rồi chạy trong thư mục cài đặt (mặc định là `~/nofx`):

```bash
docker compose pull
docker compose up -d
```

Giữ lại `.env` và thư mục `data/` khi cập nhật. Chúng chứa cấu hình, dữ liệu cần cho mã hóa và trạng thái ứng dụng đã lưu.

Sau khi khởi động lại, kiểm tra vị thế và lệnh trên sàn, đồng thời đối soát mọi lượt thực thi được đánh dấu cần kiểm tra trước khi đặt giao dịch tiếp theo. Dừng tiến trình không đóng vị thế.

<a id="source-builds-and-other-options"></a>

## Biên dịch từ mã nguồn và các lựa chọn khác

<details>
<summary>Dành cho nhà phát triển: biên dịch và cập nhật từ mã nguồn</summary>

Dùng cách này khi phát triển NOFX hoặc thử thay đổi mã nguồn trước khi image được cập nhật. Cài Git và Docker có Compose, rồi khởi động Docker:

```bash
git clone --recurse-submodules https://github.com/NoFxAiOS/nofx.git
cd nofx
./start.sh start --build
```

Script khởi động tạo `.env` khi cần và sinh các khóa mã hóa còn thiếu. Nếu clone mà chưa tải submodule, hãy chạy `git submodule update --init web/vendor/charting` trước khi biên dịch. Tài nguyên biểu đồ TradingView tuân theo giấy phép tương ứng; xem [ghi chú tích hợp biểu đồ](../../features/trade-planning.md#harness-chart-and-plugins).

Để cập nhật bản cài từ mã nguồn, dừng trader Autopilot đang hoạt động, kiểm tra giao dịch Frontier đang thực thi, rồi chạy:

```bash
git pull --ff-only
git submodule update --init web/vendor/charting
./start.sh start --build
```

</details>

<details>
<summary>Windows, Railway và triển khai từ xa</summary>

**Windows**

Dùng Docker Desktop và môi trường Bash như WSL2, sau đó chạy lệnh cài đặt một dòng ở trên.

**Railway**

[![Triển khai lên Railway](https://railway.com/button.svg)](https://railway.com/deploy/nofx?referralCode=nofx)

**Triển khai từ xa**

Trình cài đặt dùng cấu hình Compose cho môi trường production, mặc định công khai cổng trên mọi giao diện mạng của máy chủ. Cấu hình Compose trong mã nguồn mặc định gắn frontend và API với `127.0.0.1`. Trước khi cho phép truy cập từ xa, hãy cấu hình địa chỉ lắng nghe và proxy HTTPS có xác thực.

</details>

<details>
<summary>Phát triển không dùng Docker</summary>

Dùng phiên bản Go quy định trong [go.mod](../../../go.mod), Node.js 24 như trong bản dựng Docker của frontend, và TA-Lib. Khởi tạo submodule biểu đồ trước khi chạy frontend. Cấu hình môi trường theo [.env.example](../../../.env.example).

```bash
# Backend
go build -o nofx .
./nofx

# Frontend, chạy trong terminal thứ hai
cd web
npm ci
npm run dev
```

</details>

## Mô hình và dữ liệu

Frontier và Autopilot dùng chung các mô hình đã cấu hình trong NOFX. [Claw402](https://claw402.ai) cung cấp nhiều mô hình, thanh toán theo mức sử dụng bằng USDC trên Base. Chọn mô hình trong Config và chuyển đổi trong không gian làm việc.

Giá và nến đến từ sàn đã chọn. AI500 và các phân tích người nắm giữ được hỗ trợ đi qua tuyến dữ liệu trả phí của Claw402. Dữ liệu có sẵn tùy thị trường và nhà cung cấp: dữ liệu người nắm giữ trên Hyperliquid không thay thế dữ liệu Binance, OKX hay Bitget. Luồng hoạt động hiển thị quá trình đọc dữ liệu và tiến độ lập kế hoạch.

Phí AI, phí dữ liệu và tiền giao dịch trên sàn được quản lý riêng. Yêu cầu mô hình có thể phát sinh phí ngay cả khi không có giao dịch nào được thực hiện.

## Sàn giao dịch

Frontier hiện cho phép chọn Binance, Hyperliquid, OKX và Bitget. Hệ thống thực thi chung của NOFX còn có Bybit, Gate, KuCoin, Aster và Lighter. Sản phẩm, loại lệnh và quyền tài khoản khả dụng tùy thuộc từng sàn.

Các liên kết dưới đây là liên kết đăng ký đối tác nhằm hỗ trợ phát triển. Ưu đãi phí do từng sàn quy định.

| Sàn giao dịch | Hỗ trợ | Liên kết đối tác |
| :--- | :---: | :--- |
| <img src="../../../web/public/exchange-icons/binance.jpg" width="20" height="20" style="vertical-align: middle;"/> **Binance**       |   ✅   | [Đăng ký](https://www.binance.com/join?ref=NOFXENG)                                |
| <img src="../../../web/public/exchange-icons/bybit.png" width="20" height="20" style="vertical-align: middle;"/> **Bybit**           |   ✅   | [Đăng ký](https://partner.bybit.com/b/83856)                                       |
| <img src="../../../web/public/exchange-icons/okx.svg" width="20" height="20" style="vertical-align: middle;"/> **OKX**               |   ✅   | [Đăng ký](https://www.okx.com/join/1865360)                                        |
| <img src="../../../web/public/exchange-icons/hyperliquid.png" width="20" height="20" style="vertical-align: middle;"/> **Hyperliquid** |   ✅   | [Đăng ký](https://app.hyperliquid.xyz/join/AITRADING)                              |
| <img src="../../../web/public/exchange-icons/bitget.svg" width="20" height="20" style="vertical-align: middle;"/> **Bitget**         |   ✅   | [Đăng ký](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |
| <img src="../../../web/public/exchange-icons/kucoin.svg" width="20" height="20" style="vertical-align: middle;"/> **KuCoin**         |   ✅   | [Đăng ký](https://www.kucoin.com/r/broker/CXEV7XKK)                                |
| <img src="../../../web/public/exchange-icons/gate.svg" width="20" height="20" style="vertical-align: middle;"/> **Gate**             |   ✅   | [Đăng ký](https://www.gatenode.xyz/share/VQBGUAxY)                                 |
| <img src="../../../web/public/exchange-icons/aster.svg" width="20" height="20" style="vertical-align: middle;"/> **Aster**           |   ✅   | [Đăng ký](https://www.asterdex.com/en/referral/fdfc0e)                             |
| <img src="../../../web/public/exchange-icons/lighter.png" width="20" height="20" style="vertical-align: middle;"/> **Lighter**       |   ✅   | [Đăng ký](https://app.lighter.xyz/?referral=68151432)                              |

## Cách thực thi lệnh

Kế hoạch và quyết định chiến lược sử dụng các bộ kết nối sàn hiện có của NOFX. Prompt gửi đến mô hình không chứa khóa riêng của ví hay thông tin xác thực sàn.

- **Kiểm tra trước khi phê duyệt.** Frontier hiển thị tài khoản, quy mô và chi tiết vào/thoát lệnh. Autopilot tuân theo chiến lược đã cấu hình và giới hạn của hệ thống thực thi.
- **Quy mô theo tài khoản.** Frontier dùng đòn bẩy đã chọn và ký quỹ khả dụng hiện tại, đồng thời dành phần chi phí thực thi. Giới hạn số lượng, bước giá và quy định của sàn vẫn áp dụng.
- **Lệnh bảo vệ trên sàn.** Khi ghi nhận lệnh khớp, hệ thống đặt các lệnh cắt lỗ và chốt lời được hỗ trợ. Vào lệnh và đặt bảo vệ không phải một thao tác nguyên tử; hãy kiểm tra tại sàn nếu đặt bảo vệ thất bại.
- **Đối soát lệnh.** Khi kết quả gửi lệnh chưa rõ, hệ thống kiểm tra với sàn thay vì gửi lại một cách mù quáng. Trạng thái thực thi chưa được giải quyết có thể tạm dừng các lệnh vào tiếp theo.
- **Cấu hình cục bộ.** Thông tin xác thực sàn được máy chủ NOFX lưu dưới dạng mã hóa. Bản cài đặt của bạn cũng giữ dữ liệu dùng để ký, vì vậy hãy bảo vệ tệp và bản sao lưu.

## Kiến trúc

```text
Trình duyệt trên máy tính
  Frontier · Config · Dashboard · Strategy
    → NOFX API
      → Frontier: lập kế hoạch, kiểm tra và phê duyệt
      → Autopilot: vòng lặp chiến lược
    → Thực thi chung → Bộ kết nối sàn → Tài khoản giao dịch của bạn

Mô hình và dữ liệu trả phí: ứng dụng khách nhà cung cấp NOFX · Claw402
Lưu trữ: cấu hình cục bộ, kế hoạch và bản ghi thực thi
```

<details>
<summary>Video giới thiệu giao diện trước đây</summary>

Video này sử dụng phiên bản giao diện cũ. Điều hướng và thiết kế hiện đã thay đổi.

https://github.com/user-attachments/assets/3310f495-14c5-4586-a1cc-3d32e44aa505

</details>

## Tài nguyên thương hiệu

- [Biểu tượng logo — SVG](../../../web/public/icons/nofx-flow.svg)
- [Ảnh đại diện GitHub — PNG, 512 × 512](../../../web/public/brand/nofx-avatar-512.png)
- [Ảnh đại diện GitHub — tệp nguồn SVG](../../../web/public/brand/nofx-avatar.svg)
- [Banner README — SVG](../../assets/nofx-banner.svg)

<a id="documentation"></a>

## Tài liệu

| Hướng dẫn | Nội dung |
| :--- | :--- |
| [Frontier](../../features/trade-planning.md) | Lập kế hoạch, thực thi giao dịch thật, dữ liệu và khôi phục |
| [Bắt đầu](../../getting-started/README.md) | Triển khai và hướng dẫn API sàn |
| [Kiến trúc](../../architecture/README.md) | Thiết kế hệ thống và danh mục mô-đun |
| [Mô-đun chiến lược](../../architecture/STRATEGY_MODULE.md) | Chọn mã, prompt AI và thực thi |
| [Câu hỏi thường gặp](../../guides/faq.en.md) | Giải đáp các câu hỏi phổ biến |
| [Khắc phục sự cố](../../guides/TROUBLESHOOTING.md) | Chẩn đoán các vấn đề thường gặp |

<a id="community"></a>

## Cộng đồng

[Telegram](https://t.me/nofx_dev_community) · [Twitter/X](https://x.com/vergex_ai) · [Issues](https://github.com/NoFxAiOS/nofx/issues) · [vergex.trade](https://vergex.trade) · [Dashboard trực tiếp](https://vergex.trade/explore)

## Đóng góp

Code, tài liệu, bản dịch và báo cáo lỗi đều được hoan nghênh — xem [Hướng dẫn đóng góp](../../../CONTRIBUTING.md), [Quy tắc ứng xử](../../../CODE_OF_CONDUCT.md) và [Chính sách bảo mật](../../../SECURITY.md).

NOFX theo dõi các đóng góp có ý nghĩa và dự định thưởng cho người đóng góp khi hệ sinh thái phát triển. Issue ưu tiên có trọng số cao hơn.

| Đóng góp      | Trọng số |
| :---------------- | :----: |
| PR cho issue được ghim  | ★★★★★★ |
| Code (PR đã merge) | ★★★★★  |
| Sửa lỗi         |  ★★★★  |
| Ý tưởng tính năng     |  ★★★   |
| Báo cáo lỗi       |   ★★   |
| Tài liệu     |   ★★   |

<a href="https://github.com/NoFxAiOS/nofx/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=NoFxAiOS/nofx" alt="Contributors"/>
</a>

## Nhà tài trợ

<a href="https://github.com/pjl914335852-ux"><img src="https://github.com/pjl914335852-ux.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/cat9999aaa"><img src="https://github.com/cat9999aaa.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/1733055465"><img src="https://github.com/1733055465.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/kolal2020"><img src="https://github.com/kolal2020.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/CyberFFarm"><img src="https://github.com/CyberFFarm.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/vip3001003"><img src="https://github.com/vip3001003.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/mrtluh"><img src="https://github.com/mrtluh.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/cpcp1117-source"><img src="https://github.com/cpcp1117-source.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/match-007"><img src="https://github.com/match-007.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/leiwuhen1715"><img src="https://github.com/leiwuhen1715.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/SHAOXIA1991"><img src="https://github.com/SHAOXIA1991.png" width="50" height="50" style="border-radius:50%"/></a>

[Trở thành nhà tài trợ](https://github.com/sponsors/NoFxAiOS)

<br/>

Nếu NOFX hữu ích với bạn, một ngôi sao sẽ giúp các trader khác tìm thấy nó.

[![Star History Chart](https://star-history.dera.page/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.dera.page/#NoFxAiOS/nofx&Date)

## Giấy phép

[AGPL-3.0](../../../LICENSE)

<sub>Giao dịch tự động tiềm ẩn rủi ro đáng kể. Các chiến lược do AI điều khiển mang tính thử nghiệm và có thể thua lỗ. Hãy đặt kích thước vị thế hợp lý, hiểu rõ từng sàn giao dịch, và đừng bao giờ giao dịch bằng số tiền mà bạn không thể để mất. Xem đầy đủ [tuyên bố miễn trừ trách nhiệm](../../../DISCLAIMER.md).</sub>
