# 多语言新闻聚合功能测试报告

| 测试项目 | 详情 |
| :--- | :--- |
| **测试日期** | 2025-12-05 |
| **测试环境** | Live Production Environment (API Integration) |
| **测试脚本** | `scripts/test_news_ai_integration.go` |
| **结果汇总** | ✅ **PASSED** (全项通过) |

---

## 1. 准确性验证 (Accuracy)
*   **原文标题**: "Bitcoin Breaks $150,000 Barrier as Global Adoption Accelerates"
*   **原文摘要**: "Major central banks announced today..."
*   **AI 翻译结果**:
    *   **标题**: "比特币突破15万美元大关，全球采用加速" (翻译精准，符合金融术语)
    *   **摘要**: "主要央行宣布将比特币纳入储备资产，引发供应冲击，市场分析师称之为“超级周期”。" (准确提
取了“央行储备”、“供应冲击”、“超级周期”等核心信息)
    *   **情感分析**: `POSITIVE` (正确识别为重大利好)
*   **结论**: ✅ AI 翻译准确，摘要提炼能力强，无幻觉。

## 2. 时效性验证 (Latency)
*   **AI 处理耗时**: `2.95s`
*   **Telegram 发送耗时**: `< 1s`
*   **总延迟**: 约 3秒
*   **评估**: 对于每5分钟轮询一次的新闻服务，3秒的处理延迟完全在可接受范围内，不会造成明显滞后。
*   **结论**: ✅ 性能达标。

## 3. 安全性验证 (Security)
*   **代码审查**: 
    *   `service/news/deepseek.go` 中的日志仅打印原始响应内容用于调试，未泄露 Key。
    *   `system_config` 数据库存储 Key，未硬编码在业务逻辑中（测试脚本除外）。
*   **降级保护**: 代码实现了 Fail-Open 机制，确保 AI 挂掉时不阻塞新闻发送。
*   **结论**: ✅ 安全性符合要求。

## 4. 用户体验验证 (UX)
*   **消息格式**:
    *   使用了 `🪙` 和 `🟢` (Sentiment Icon) 增强视觉识别。
    *   中文摘要排版清晰。
    *   保留了原文链接 `[Read More]`。
*   **推送目标**: 成功发送到了指定的 Topic (Thread ID: 2)，避免了对主群组的干扰。
*   **结论**: ✅ 消息可读性高，排版美观。

---

## 5. 遗留风险与建议
*   **DeepSeek 额度**: 建议监控 API 额度使用情况，避免突然欠费导致降级到全英文模式。
*   **长文本截断**: 目前未对超长摘要做强制截断，极端情况下可能会让 Telegram 消息过长（虽不常见）。

---

## 6. Telegram 发送模块验证与修复 (2025-12-18)

### 修复 ID 冲突问题
*   **问题**: 原 `sentArticleIDs` 仅使用 `int64` 类型的 Article ID 作为 Key，导致不同新闻源（如 Finnhub 和 Mlion）如果恰好使用了相同的 ID，会发生误判去重，导致新闻漏发。
*   **修复**: 将 `sentArticleIDs` 键改为 `string` 类型，使用 `SourceName-ID` 组合键（如 `Finnhub-123`），确保跨源去重的准确性。
*   **验证**: 更新了 `news_integration_test.go` 并通过了相关集成测试。

### 单元测试增强
*   **新增测试**: 创建了 `service/news/telegram_test.go`。
*   **覆盖范围**:
    *   **API 请求构造**: 验证 URL、Chat ID、Parse Mode 和 Payload 正确性。
    *   **Topic 支持**: 验证 `message_thread_id` 参数的正确传递。
    *   **错误处理**: 验证 API 返回非 200 状态码和网络错误的捕获逻辑。
*   **结果**: 所有单元测试通过。

### 最终回归测试

*   执行 `go test -v ./service/news`，全模块测试通过。



---



## 7. Mlion 新闻路由验证 (2025-12-18)

*   **目标**: 验证 Mlion 数据源的新闻是否被正确路由到 Telegram Topic `17758`。

*   **配置检查**:

    *   代码硬编码检查: 确认测试代码中映射关系 `topicRouter["Mlion"] = 17758`。

    *   数据库配置检查: 确认 SQL 迁移文件和配置文件中 `mlion_target_topic_id` 设置为 `17758`。

*   **接口连通性**:

    *   Mlion API (`https://api.mlion.ai/...`) 连接测试：由于 API Key 限制或环境问题，curl 测试返回 4002 错误，但代码逻辑中包含完整的鉴权头 (`X-API-KEY`) 处理。

*   **逻辑验证**:

    *   执行 `go test -v -run TestMlion_Integration ./service/news`。

    *   **结果**: ✅ PASS。测试模拟了 Mlion 响应，并断言 `notifier.LastThreadID` 等于 `17758`，验证了路由逻辑的正确性。

---

## 8. API Key 配置与连通性验证 (2025-12-18)
*   **测试目标**: 验证提供的 Mlion API Key (`c559b9a8-80c2-4c17-8c31-bb7659b12b52`) 的有效性及系统加载机制。
*   **连通性测试**:
    *   **命令**: 使用 `curl` 携带 `X-API-KEY` 头访问 Mlion 实时新闻接口。
    *   **结果**: ✅ 成功。返回 HTTP 200 及有效 JSON 数据，包含新闻条目。
*   **配置加载机制分析**:
    *   **现状**: 系统配置主要通过 PostgreSQL 数据库的 `system_config` 表加载。
    *   **发现**: 代码中未发现自动加载 `.env` 或 `.env.local` 文件的逻辑 (如 `godotenv.Load`)。
    *   **实际配置源**: Key 已硬编码在数据库迁移文件 `database/migrations/20251215_mlion_news_config.sql` 中，并作为默认值注入数据库。
    *   **建议**: 若需通过环境变量动态覆盖 Key，需修改 `service/news/service.go` 的 `loadConfig` 方法，增加 `os.Getenv` 的回退读取逻辑。

### 生产环境数据库验证
*   **操作**: 连接生产环境 PostgreSQL 数据库，直接查询 `system_config` 表。
*   **SQL**: `SELECT value FROM system_config WHERE key = 'mlion_api_key'`
*   **结果**: ✅ 验证通过。数据库中存储的值与预期的 Key (`c559b9a8-80c2-4c17-8c31-bb7659b12b52`) 完全一致。

---

## 9. Telegram 消息路由实网验证 (2025-12-18)
*   **测试目标**: 验证消息能否实际发送到 Telegram 频道 `monnaire_capital_research` 的 Topic `17758`。
*   **方法**: 使用临时脚本连接生产数据库，读取真实配置 (`bot_token`, `chat_id`, `mlion_target_topic_id`)，并调用 `news.TelegramNotifier` 发送测试消息。
*   **配置确认**:
    *   Chat ID: `-1002678075016`
    *   Target Topic ID: `17758`
*   **执行结果**:
    *   API 调用: HTTP 200 OK
    *   Telegram 响应: 成功 (`{"ok":true, ...}`)
    *   结论: ✅ 验证通过。系统具备向指定 Topic 发送消息的完整能力。
