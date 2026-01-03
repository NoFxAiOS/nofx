## High priority issues listed in Issue Tab
- [ ] [Issue 1](https://github.com/NoFxAiOS/nofx/issues/1263): 
    ### Feature Request: EMA, MACD, RSI, ATR parameters in strategy studio
    ```markdown
        - Reuqest:  策略工作室中的EMA 、macd、rsi、atr均线参数均为硬编码，自定义无效，因为交易信号的生成可通过调整均线值快速识别趋势，请不要硬编码
    ```
- [ ] [Issue 2](https://github.com/NoFxAiOS/nofx/issues/1273):
    ### Bug Report: 回测模式与策略模式 K 线数量不一致
    ```markdown
        - 问题描述 : 回测模式和策略模式给 AI 的 K 线数量不一致。前端配置的 K 线数量（如 30）在回测模式下被忽略，固定使用 10 根。
        - 调用链对比策略模式（实时）
            decision.GetFullDecisionWithStrategy()
            → fetchMarketDataWithStrategy()
                → market.GetWithTimeframes(symbol, timeframes, primaryTimeframe, klineCount)  // klineCount = 30
                → calculateTimeframeSeries(klines, tf, count)  // count = 30
                    → data.TimeframeData[tf] = seriesData

        - 结果: AI 看到 30 根 K 线（通过 TimeframeData）

        - 回测模式
            backtest.Runner.runDecision()
            → decision.GetFullDecisionWithStrategy()
                → engine.BuildUserPrompt(ctx)
                → e.formatMarketData(marketData)
                    → data.IntradaySeries  // 来自 BuildDataFromKlines

            backtest.DataFeed.BuildMarketData()
            → market.BuildDataFromKlines(symbol, series, longer)
                → calculateIntradaySeries(primary)  // 硬编码 10 根
                → start := len(klines) - 10

        - 结果: AI 看到 10 根 K 线（通过 IntradaySeries，硬编码）

        - 关键代码位置
            market/data.go:1051 - calculateIntradaySeries 硬编码 10 根
            market/data.go:661 - calculateTimeframeSeries 使用可配置的 count
            backtest/datafeed.go:207 - 回测调用 BuildDataFromKlines
            decision/engine.go:1101-1110 - 格式化时优先使用 TimeframeData，fallback 到 IntradaySeries
    
        - 影响
            模式	K线数量	时间跨度(3m)	数据来源
            策略模式	30 根	90 分钟	TimeframeData
            回测模式	10 根	30 分钟	IntradaySeries
            
            - 回测结果可能与实盘表现不一致
            - AI 在回测中看到的历史数据更少
            - 累积指标的回看时间也应该与 K 线数量对齐

        - 建议修复方案
            修改 BuildDataFromKlines 或 BuildDataFromKlinesWithMakerStrengthFull，使其也填充 TimeframeData 并使用可配置的 K 线数量。
    ```

- [ ] [Issue 3](https://github.com/NoFxAiOS/nofx/issues/1282)
    ### max position逻辑有问题 平仓信号没从服务器返回 调仓显示仓满
    ```markdown
        - vibe coding修了一下
        - 🛠️ 解决方案：引入“预期净持仓”逻辑
            为了彻底解决这个问题，我已经在 trader/auto_trader.go 中重构了风控检查逻辑。

        - 修复核心逻辑：
            在循环内追踪成功指令：在每个交易周期（Cycle）内，增加一个 successfulClosesInCycle 计数器。
            逻辑预减免：当系统执行“先平后开”时，如果平仓指令发送成功，计数器加 1。
            计算净持仓（Net Position）：后续执行开仓风控检查时，不再死扣 GetPositions() 返回的陈旧数据，而是使用：
            净持仓数 = 当前实际持仓数 - 本周期内已成功发送平仓指令的数量
            容错处理：如果由于 API 延迟 GetPositions() 还没更新，预减逻辑会自动抵消掉这部分滞后，确保开仓指令能顺利发给交易所。

        - 💻 代码变更点
        - enforceMaxPositions：现在接受一个 successfulClosesInCycle 参数，用于计算 netPositionCount。
        - runCycle：在循环执行决策时，实时更新该计数器并传递给执行函数。
        - executeOpenLong/ShortWithRecord：更新了函数签名以支持该逻辑。
    ```

- [ ] [Issue 4](https://github.com/NoFxAiOS/nofx/issues/1262)
    ### tradingview feature enhancement request
    ```markdown
        - Reuqest:  tradingview的功能增强请求
        - 具体需求：通过接收tradingview 的webhook 消息内容，作为指标入参. 目前的指标太过于固化，而且参数不够优化
        - Proposed Solution: 通过接收webhook，然后接收并处理
    ```

- [ ] [Issue 5](https://github.com/NoFxAiOS/nofx/issues/1257)
    ### Optimizing tool selection
    ```markdown
    Issue Summary: 4H Candle Update Failure
    Problem: 4-hour candles stop updating while shorter timeframes (15m, 1h) continue working normally.

    Root Cause: 
    - NOFX subscribes to WebSocket streams for all trading pairs (~534 pairs) across multiple timeframes (3m + 4h)
    - This creates ~1,068 concurrent streams, exceeding Binance's 1,024 stream limit
    - Binance closes the connection with "1008 policy violation: Invalid request"

    What Happens:
    1. WebSocket connection gets terminated due to too many streams
    2. System reconnects but only restores dynamic subscriptions (15m, 1h)
    3. 4H bulk streams are NOT re-subscribed, leaving 4H data stale/frozen
    4. 4H candles remain stuck at the last cached value

    Impact:
    - Strategies using 4H timeframes get outdated data
    - Can lead to incorrect trading decisions
    - Only affects 4H data; shorter timeframes work fine

    Proposed Solutions:
    1. Limit subscriptions to only symbols actually used by active strategies (not all 534 pairs)
    2. Split streams across multiple WebSocket connections to stay under limits
    3. Fix reconnect logic to properly restore all subscription types
    4. Add fallback to REST API for stale 4H data detection/refresh

    Severity: High - affects trading accuracy for 4H-based strategies.

- [ ] [Issue 6](https://github.com/NoFxAiOS/nofx/issues/1251)
    ### 入场价显示不一致
    ```markdown
        - 问题描述：在交易界面中，入场价显示不一致，导致用户混淆。
        - 复现步骤：
            1. 在交易界面打开某个币种的交易对。
            2. 查看当前持仓的入场价显示。
            3. 切换到另一个界面或刷新页面，观察入场价显示是否一致。
        - 预期结果：入场价应在所有界面和刷新后保持一致。
        - 实际结果：入场价在不同界面或刷新后显示不一致。
        - 影响范围：所有用户在使用交易界面时可能遇到此问题，影响用户体验和交易决策。
        - 建议修复方案：
            1. 检查前端代码中获取和显示入场价的逻辑，确保数据源一致。
            2. 确保在不同组件或页面中使用相同的状态管理方法来存储和访问入场价数据。
            3. 添加单元测试以验证入场价在各种情况下的一致性。
    ```

- [ ] [Issue 7](https://github.com/NoFxAiOS/nofx/issues/1245)
    ### Issue Summary: Binance Spot Trading Feature Request
    **Request**: Add Binance spot trading functionality to NOFX

    **Key Points**:
    - User wants **Binance spot trading** option because it's:
    - Simple, transparent, and secure
    - Allows trading **real crypto assets without leverage**
    - More robust and safer than futures trading
    - Offers low fees, fast execution, and user-friendly interface

    **What They Want**:
    1. **Integration** of Binance spot trading into NOFX's system architecture
    2. **Implementation** of spot trading support alongside existing futures trading
    3. **Enhanced system completeness** and practical utility

    **User's Reasoning**:
    - Spot trading is suitable for both **beginners and experienced traders**
    - **No leverage risk** - you own actual cryptocurrency
    - Binance's **reliable infrastructure** and competitive fees
    - **Safer trading approach** compared to futures/derivatives

    **Request Type**: Feature enhancement to support both spot and futures trading

    **Priority**: User specifically mentions "strongly suggest and request" and asks for "developer help"

    **Current Status**: NOFX appears to focus primarily on futures trading; this would add spot trading as an alternative trading mode.

- [ ] [Issue 8](https://github.com/NoFxAiOS/nofx/issues/1241)
    ### Issue Summary: Real-Time Drawdown Monitoring Feature Request

    **Requester**: VioletEvergar-den (3 weeks ago)

    **Request**: Add real-time profit drawdown monitoring with automatic position closure

    **Feature Description**:
    - **Real-time drawdown tracking** on profitable positions
    - **Automated stop-loss** when profit drops by X% from peak
    - **Immediate code-based position closure** (not relying on AI decisions)

    **Problem Being Solved**:
    - **AI scanning delays/lag** causing missed opportunities
    - **Profit erosion** - positions that were profitable turn into losses
    - Need for **faster risk management** than AI decision-making speed

    **Example Scenario**:
    1. Position becomes profitable
    2. Profit peaks, then starts declining
    3. When drawdown reaches X% from peak profit → **automatic closure**
    4. Protects against AI being "too slow" to react

    **Response from Developer** (h72by2sz8y-prog):
    - Suggested user can **modify it with AI assistance**
    - Mentioned project **"now has local logic"** 
    - Implied this feature could be implemented by users themselves

    **Issue Type**: Enhancement/New feature request

    **Priority**: User seems frustrated with profit losses due to AI reaction delays

    **Current Status**: Developer suggested self-implementation rather than built-in feature

    This is essentially a **trailing stop-loss** feature for protecting profits from drawdowns when AI trading decisions are too slow.
    ### Solution Summary: Real-Time Drawdown Monitoring Implementation

    **Feature Implemented**: `checkPositionDrawdown` function for automated profit protection

    **Key Implementation Details**:

    **Trigger Conditions**:
    - **Current profit margin > 5.0%** (position must be profitable first)
    - **Drawdown from peak ≥ 40.0%** (closes when profit drops 40% from highest point)

    **Execution Logic**:
    - **Monitoring**: `checkPositionDrawdownMonitor` function runs periodic checks
    - **Emergency Close**: Uses `emergencyClosePosition` function for immediate closure
    - **Platform Integration**: Works across trading platforms after configuration

    **Code Locations**:
    - **Condition Check**: `trader/auto_trader.go:1550`
    - **Execution Logic**: `trader/auto_trader.go:1555` 
    - **Monitoring Loop**: `trader/auto_trader.go:1560`

    **Monitoring Frequency**:
    - **Periodic checks** every cycle for profitable positions

    **Behavior**:
    - **Activation**: Only when position is profitable (>5% profit)
    - **Trigger**: When profit drops 40% from peak (e.g., from 10% profit to 6% profit)
    - **Action**: Immediately closes position to preserve remaining profit

    **Design Philosophy**: 
    - **Conservative approach** - waits for meaningful profit (5%+) before monitoring
    - **Substantial drawdown threshold** (40%) to avoid premature closes  
    - **Profit preservation** rather than loss prevention focus

    This directly addresses VioletEvergar-den's concern about AI reaction delays by implementing automated profit protection independent of AI decision-making speed.

- [ ]. [Issue 9](https://github.com/NoFxAiOS/nofx/issues/1239)
    ### Issue #9: Current Price Data Not Updating - Large Price Deviation

    **🔍 Bug Category**: Trading execution / Backend/API

    **📋 Problem Description**:
    Current price (`current_price`) remains stuck at outdated values, creating significant deviation from actual market price during trading operations.

    **Evidence from Logs**:
    - **Logged current_price**: `2950.1000` (ETHUSDT)
    - **Actual trading current price**: `2925.4800` (ETHUSDT) 
    - **Deviation**: ~$25 difference (~0.85% deviation)

    **📸 Key Details**:
    ```
    Time: 2025-12-17 05:44:54 UTC | Period: #1 | Runtime: 0 minutes
    ETHUSDT SHORT | Current 2925.4800 | Position Value 614.35 USDT
    current_price = 2950.1000  ← Stuck/stale price
    ```

    **📊 Additional Context**:
    - **API Endpoint Change**: Log mentions `/fapi/v1/ticker/price` upgraded to `/fapi/v2/ticker/price`
    - **Impact**: Price deviation affects trading calculations and position management
    - **Frequency**: Appears to be persistent (similar to the 4H candle stale data issue)

    **💡 Suspected Cause**:
    1. **API endpoint deprecation** - System still using old v1 endpoint
    2. **Price feed not updating** - Similar to WebSocket stream reconnection issues
    3. **Stale cache** - Current price not refreshing from live data

    **🔧 Possible Solution**:
    1. **Update API endpoint** from `/fapi/v1/ticker/price` to `/fapi/v2/ticker/price`
    2. **Add price staleness detection** and fallback refresh mechanism
    3. **Verify WebSocket price stream** is properly updating current price cache

    **⚠️ Impact**: High - Incorrect pricing affects trading accuracy and position calculations

- [ ] [Issue 10](https://github.com/NoFxAiOS/nofx/issues/1153)
    ### Issue: Enhanced Market Microstructure Data for AI Decision Making

    **🔍 Bug Category**: Enhancement / New feature request
    **📋 Current Limitation**:
    AI trading decisions are limited by insufficient market data, currently only providing:
    - **K-line data** (OHLCV candles)
    - **Technical indicators** 
    - **Open Interest (OI)**
    - **Trading volume**

    **🎯 Requested Additional Data**:
    1. **Order book depth** (盘口深度) - Bid/ask levels and quantities
    2. **Order cancellation rates** (取消挂单率) - Market maker behavior analysis
    3. **Large order cluster analysis** (大单簇分析) - Institutional activity detection
    4. **VWAP deviation** (VWAP差值) - Price vs volume-weighted average price
    5. **Real-time order book** (实时成交簿) - Live market depth updates

    **💡 Business Justification**:
    - **Current problem**: Lack of microstructure data makes **modeB decision scoring** unreliable
    - **Risk concern**: Opening positions with incomplete data violates core principles:
    - **"Quality over quantity"** (质量优于数量)
    - **"Capital preservation first"** (资金保全第一)
    - **Goal**: Enable more sophisticated AI market analysis and better trading decisions

    **📊 Impact**:
    - **Current**: AI decisions based on limited technical data
    - **Proposed**: AI can analyze market microstructure for higher-quality entries
    - **Benefit**: Improved risk management and trade quality

    **🔧 Implementation Requirements**:
    - **Data sources**: Real-time order book feeds from exchanges
    - **Processing**: Market microstructure analysis algorithms
    - **Integration**: Feed additional data into AI decision-making prompts

    **Priority**: Enhancement - would significantly improve AI trading quality and risk management capabilities.

- [ ] [Issue 11](https://github.com/NoFxAiOS/nofx/issues/1142)
    ### Issue #11: Paper Trading / Simulation Mode Feature Request

    **🔍 Bug Category**: Enhancement / New feature request

    **📋 Feature Description**:
    Add **paper trading (simulation mode)** option when creating AI traders, using dedicated simulation endpoints (e.g., Binance testnet)

    **🎯 Problem to Solve**:
    - **Risk aversion**: Some users don't dare trade with real money initially
    - **Testing needs**: Users want to evaluate AI trader performance before committing real capital
    - **Learning curve**: Safe environment to understand system behavior

    **💡 Proposed Solution**:
    - **UI Enhancement**: Add **checkbox/toggle** in trader creation interface
    - **Backend routing**: When enabled, use simulation API endpoints instead of live trading
    - **Seamless switching**: Same interface, different execution environment

    **🔧 Technical Implementation**:
    ```
    ✅ Trader Creation UI:
    [ ] Enable Paper Trading Mode

    Backend API routing:
    - Live: api.binance.com
    - Sim:  testnet.binancefuture.com
    ```

    **✅ Acceptance Criteria**:
    - AI traders can **access simulation market data**
    - **Normal trading operations** work in simulation mode
    - **Performance tracking** and analytics remain functional
    - **Clear indication** when trader is in simulation vs live mode

    **📚 Benefits**:
    - **Risk-free testing** for new users
    - **Strategy validation** before live deployment  
    - **Educational tool** for learning AI trading behavior
    - **Development testing** for new features

    **Priority**: High user demand - reduces barrier to entry and improves user confidence

    **💻 Technical Scope**: "Should just need a few lines of code" according to requester - mainly API endpoint routing logic.

- [ ][Issu3 12](https://github.com/NoFxAiOS/nofx/issues/1126)
    ### Issue #12: Real-Time News Integration for AI Trading Decisions

    **🔍 Bug Category**: Enhancement / New feature request

    **📋 Feature Description**:
    Add **real-time news analysis** capability to AI trading decisions, combining news sentiment with technical indicators and trading conditions

    **🎯 Current Limitation**:
    AI trading decisions currently rely only on:
    - **Technical indicators** (MACD, RSI, etc.)
    - **Trading data** (volume, price action, OI)
    - **Market patterns** from historical data

    **💡 Proposed Enhancement**:
    Integrate **fundamental analysis** through:
    - **Real-time news feeds** for relevant cryptocurrencies
    - **News sentiment analysis** 
    - **Combined decision-making**: News + Technical + Trading conditions

    **📊 Use Cases**:
    - **Major announcements** (regulatory news, partnerships, etc.)
    - **Market sentiment shifts** from breaking news
    - **Event-driven trading** (Fed meetings, earnings, etc.)
    - **FUD/FOMO detection** and appropriate response

    **🔧 Technical Implementation Needs**:
    - **News API integration** (CoinDesk, CoinTelegraph, etc.)
    - **NLP sentiment analysis** for crypto-related news
    - **News filtering** by relevance to trading pairs
    - **Decision prompt enhancement** to include news context
    - **Real-time processing** to keep news current

    **✅ Expected Outcome**:
    AI traders make more **informed decisions** by considering:
    1. **Technical signals** (current capability)
    2. **Market conditions** (current capability)  
    3. **Fundamental news events** (new capability)

    **📈 Benefits**:
    - **More comprehensive analysis** beyond pure technical trading
    - **Better risk management** during news-driven volatility
    - **Improved timing** for entries/exits around events
    - **Competitive advantage** over purely technical strategies

    **Priority**: Enhancement - would significantly improve AI decision quality by adding fundamental analysis layer.

- [ ] [Issue 13](https://github.com/NoFxAiOS/nofx/issues/1097)
    ### Issue #13: Dynamic Stop Loss/Take Profit Calculation Bug

    **🔍 Bug Category**: Trading execution

    **🐛 Problem Description**:
    When AI dynamically adjusts stop loss/take profit levels during a trade, the system records incorrect P&L after position closure on exchange (Binance)

    **📋 Detailed Issue**:
    1. **AI opens position** with initial stop loss/take profit levels
    2. **AI dynamically adjusts** stop loss/take profit during trade
    3. **Exchange triggers closure** based on updated levels
    4. **System incorrectly calculates P&L** using **original stop loss/take profit** instead of **actual execution price**

    **❌ Current Behavior**:
    - P&L calculation uses **stale/original** stop loss/take profit values
    - **Inaccurate trading records** and performance metrics
    - **Disconnect** between exchange execution and internal tracking

    **✅ Expected Behavior**:
    - P&L should reflect **actual execution price** from exchange
    - Trading records should be **accurate and up-to-date**

    **💡 Proposed Solutions**:

    **Option 1 - Exchange Sync Approach**:
    - **Periodically fetch trading records** from exchange APIs
    - **Don't maintain internal P&L calculations**
    - Use exchange as **source of truth** for trade outcomes

    **Option 2 - Internal Update Approach**:
    - When AI **updates stop loss/take profit**, **overwrite original values**
    - Ensure internal tracking **reflects current settings**
    - Calculate P&L using **updated stop loss/take profit levels**

    **🔧 Technical Impact**:
    - **Accuracy**: Trading performance metrics become unreliable
    - **Analytics**: Historical analysis based on incorrect data
    - **Trust**: Users can't rely on system-reported P&L

    **📊 Recommended Fix**:
    **Hybrid approach**: Update internal records when AI adjusts levels + periodic exchange sync for validation

    **Priority**: High - affects core trading functionality and user trust in P&L accuracy.

- [ ] [Issue 14](https://github.com/NoFxAiOS/nofx/issues/1053)
    ### Feature: reqeust contract features

- [ ] [Issue 15](https://github.com/NoFxAiOS/nofx/issues/977)
    ### KLine type enhancement 
    - 现在是3min k和4h k，希望能够选择5min 或者30min，1h这种

- [ ] [Issue 16](https://github.com/NoFxAiOS/nofx/issues/1237)
    ### Issue #16: Adaptive AI Trigger Strategy vs Fixed Time Cycles

    **🔍 Bug Category**: Enhancement / New feature request

    **📋 Current System Limitation**:
    AI analysis runs on **fixed time cycles** regardless of market conditions, which is inefficient for different volatility environments

    **🎯 Problem Identified**:
    - **Low volatility periods**: Fixed cycles waste AI calls on minimal market changes
    - **High volatility periods**: Fixed cycles may miss rapid market movements
    - **Inefficient resource usage**: AI analysis triggered unnecessarily during quiet markets

    **💡 Proposed Enhancement**:
    **Pre-strategy trigger mechanism** instead of fixed time loops

    **🔧 Technical Implementation**:

    **Real-time monitoring layer**:
    - **TICK data stream analysis** 
    - **Market momentum detection**
    - **Order book imbalance monitoring**
    - **Energy/volatility thresholds**

    **Trigger conditions**:
    - Significant price movement
    - Volume spike detection
    - Order book disruption
    - Momentum shift indicators

    **Benefits**:
    - **Reduced AI calls** during low-activity periods
    - **Faster response** during high-volatility events
    - **More comprehensive data** can be provided to AI when triggered
    - **Resource optimization** - only analyze when meaningful

    **📊 Expected Outcome**:
    - **Smart triggering**: AI analysis only when market conditions warrant it
    - **Enhanced data quality**: More detailed indicators when analysis is triggered
    - **Improved efficiency**: Reduced computational overhead
    - **Better timing**: AI decisions aligned with actual market dynamics

    **🎯 Use Cases**:
    - **Scalping strategies**: React immediately to order flow changes
    - **Trend following**: Trigger on momentum breakouts
    - **Mean reversion**: Activate on volatility spikes
    - **News events**: Respond to sudden market movements

    **Priority**: Enhancement - would significantly improve system efficiency and responsiveness.

- [ ] [Issue 17](https://github.com/NoFxAiOS/nofx/issues/1227)
    ### 输入数据中的历史持仓不对