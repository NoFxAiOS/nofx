# 📘 NOFX 使用指南

**语言:** [English](README.md) | [中文](README.zh-CN.md)

帮助您有效使用 NOFX 的综合指南。

---

## 📚 可用指南

### 🔧 基础使用

| 指南 | 描述 | 状态 |
|------|------|------|
| [FAQ (中文)](faq.zh-CN.md) | 常见问题解答 | ✅ 可用 |
| [FAQ (English)](faq.en.md) | Frequently asked questions | ✅ 可用 |
| 配置指南 | 高级设置和选项 | 🚧 即将推出 |
| 交易策略 | AI 交易策略示例 | 🚧 即将推出 |

---

## 🐛 故障排除

### 常见问题

**问题：找不到 TA-Lib**
```bash
# macOS
brew install ta-lib

# Ubuntu/Debian
sudo apt-get install libta-lib0-dev
```

**问题：精度错误**
- 系统自动处理交易所的 LOT_SIZE
- 检查网络连接
- 验证交易所 API 可访问

**问题：AI API 超时**
- 检查 API 密钥有效性
- 验证网络连接
- 检查 API 余额/额度
- 超时设置为 120 秒

**问题：前端无法连接**
- 确保后端正在运行 (http://localhost:8080)
- 检查端口 8080 是否可用
- 检查浏览器控制台错误

---

## 📖 使用技巧

### 最佳实践

**1. 风险管理**
- 从小金额开始（100-500 USDT）
- 使用子账户增加安全性
- 设置合理的杠杆限制
- 监控每日亏损限制

**2. 性能监控**
- 定期检查决策日志
- 分析胜率和盈利因子
- 审查 AI 推理（思维链）
- 跟踪权益曲线趋势

**3. 配置**
- 先在测试网测试
- 逐步增加交易金额
- 调整扫描间隔（推荐 3-5 分钟）
- 初学者使用默认币种列表

---

## 🎯 进阶主题

### 多交易员竞赛
同时运行多个 AI 模型：
- Qwen vs DeepSeek 对决
- 实时比较性能
- 识别表现最佳的策略

### 自定义币种池
- 使用外部 API 进行币种选择
- 结合 AI500 + OI Top 数据
- 按流动性和交易量过滤

### 交易所集成
- Binance Futures（中心化交易所）
- Hyperliquid（去中心化交易所）
- Aster DEX（兼容 Binance）

---

## 📊 理解指标

### 关键性能指标

**胜率（Win Rate）**
- 盈利交易的百分比
- 目标：>50% 以获得稳定盈利

**盈利因子（Profit Factor）**
- 总盈利与总亏损的比率
- 目标：>1.5（1.5:1 或更好）

**夏普比率（Sharpe Ratio）**
- 风险调整后的收益衡量
- 越高越好（>1.0 为良好）

**最大回撤（Maximum Drawdown）**
- 从峰值到谷值的最大跌幅
- 为安全起见保持在 20% 以下

---

## 🔗 相关文档

- [快速开始](../getting-started/README.zh-CN.md) - 初始设置
- [社区](../community/README.md) - 贡献和悬赏
- [FAQ 中文](faq.zh-CN.md) - 常见问题
- [FAQ English](faq.en.md) - Common questions

---

## 🆘 需要帮助？

**找不到您需要的内容？**

- 🐛 [GitHub Issues](https://github.com/NoFxAiOS/nofx/issues)
- 🐦 [Twitter @nofx_official](https://x.com/nofx_official)

---

[← 返回文档首页](../README.md)
