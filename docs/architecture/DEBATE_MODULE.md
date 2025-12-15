# Debate Arena Module - Technical Documentation

**Language:** [English](DEBATE_MODULE.md) | [中文](DEBATE_MODULE.zh-CN.md)

## Overview

The Debate Arena is a collaborative AI decision-making system where multiple AI models with different personalities debate market conditions and reach consensus on trading decisions. The system supports multi-round debates, real-time streaming, voting mechanisms, and automatic trade execution.

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Backend Components](#2-backend-components)
3. [Debate Execution Flow](#3-debate-execution-flow)
4. [Personality System](#4-personality-system)
5. [Consensus Algorithm](#5-consensus-algorithm)
6. [Auto-Execution](#6-auto-execution)
7. [API Reference](#7-api-reference)
8. [Real-Time Updates (SSE)](#8-real-time-updates-sse)
9. [Database Schema](#9-database-schema)
10. [Frontend Components](#10-frontend-components)
11. [Integration Points](#11-integration-points)
12. [Error Handling](#12-error-handling)

---

## 1. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            Debate Arena System                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐  │
│  │   Bull AI   │    │   Bear AI   │    │ Analyst AI  │    │ Risk Mgr AI │  │
│  │     🐂      │    │     🐻      │    │     📊      │    │     🛡️      │  │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘    └──────┬──────┘  │
│         │                  │                  │                  │          │
│         └──────────────────┴──────────────────┴──────────────────┘          │
│                                    │                                        │
│                          ┌─────────▼─────────┐                              │
│                          │   Debate Engine   │                              │
│                          │  (debate/engine)  │                              │
│                          └─────────┬─────────┘                              │
│                                    │                                        │
│         ┌──────────────────────────┼──────────────────────────┐            │
│         │                          │                          │            │
│  ┌──────▼──────┐         ┌─────────▼─────────┐      ┌────────▼────────┐   │
│  │ Market Data │         │  Voting System    │      │  Auto-Executor  │   │
│  │  Assembly   │         │  & Consensus      │      │   (optional)    │   │
│  └─────────────┘         └───────────────────┘      └─────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### File Structure

```
├── debate/
│   └── engine.go          # Core debate engine logic
├── api/
│   └── debate.go          # HTTP handlers and SSE streaming
├── store/
│   └── debate.go          # Database operations and schema
└── web/src/pages/
    └── DebateArenaPage.tsx # Frontend UI
```

---

## 2. Backend Components

### 2.1 Core Files

| File | Purpose | Key Functions |
|------|---------|---------------|
| `debate/engine.go` | Core debate logic | `StartDebate()`, `runDebate()`, `collectVotes()`, `determineConsensus()` |
| `api/debate.go` | HTTP handlers | `HandleCreateDebate()`, `HandleStartDebate()`, `HandleDebateStream()` |
| `store/debate.go` | Database ops | `CreateSession()`, `AddMessage()`, `AddVote()`, `GetSessionWithDetails()` |

### 2.2 Debate Engine Structure

```go
// debate/engine.go

type DebateEngine struct {
    store           *store.DebateStore
    aiClients       map[string]ai.Client
    strategyEngine  *strategy.Engine
    subscribers     map[string]map[chan []byte]bool
}

// Event callbacks for real-time updates
var OnRoundStart    func(sessionID string, round int)
var OnMessage       func(sessionID string, msg *DebateMessage)
var OnVote          func(sessionID string, vote *DebateVote)
var OnConsensus     func(sessionID string, decision *DebateDecision)
var OnError         func(sessionID string, err error)
```

---

## 3. Debate Execution Flow

### 3.1 Session Creation

```
POST /api/debates
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. Validate user authentication                             │
│ 2. Parse CreateDebateRequest:                               │
│    - name, strategy_id, symbol, max_rounds, participants    │
│    - interval_minutes, prompt_variant, auto_execute         │
│ 3. Validate strategy ownership                              │
│ 4. Auto-select symbol if not provided:                      │
│    - Static coins → Use first coin from strategy            │
│    - CoinPool → Fetch from AI500 API                        │
│    - OI Top → Fetch from OI ranking API                     │
│    - Mixed → Try pool first, fallback to OI                 │
│ 5. Set defaults:                                            │
│    - max_rounds: 3 (range 2-5)                              │
│    - interval_minutes: 5                                    │
│    - prompt_variant: "balanced"                             │
│ 6. Create DebateSession in database                         │
│ 7. Add participants with AI models and personalities        │
│ 8. Return full session with participants                    │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 Debate Start

**Location:** `debate/engine.go:StartDebate()` (Lines 114-154)

```
POST /api/debates/:id/start
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. Validate session status (must be pending)                │
│ 2. Validate participants (minimum 2)                        │
│ 3. Initialize AI clients for all participants               │
│ 4. Get strategy configuration                               │
│ 5. Update status to "running"                               │
│ 6. Launch goroutine: runDebate()                            │
└─────────────────────────────────────────────────────────────┘
```

### 3.3 Market Context Building

**Location:** `debate/engine.go:buildMarketContext()` (Lines 292-362)

```
┌─────────────────────────────────────────────────────────────┐
│ buildMarketContext()                                        │
├─────────────────────────────────────────────────────────────┤
│ 1. Get candidate coins from strategy engine                 │
│ 2. Fetch market data for each candidate:                    │
│    - Multiple timeframes (15m, 1h, 4h)                      │
│    - K-line count from strategy config                      │
│    - OHLCV data, indicators                                 │
│ 3. Fetch quantitative data batch:                           │
│    - Capital flow                                           │
│    - Position changes                                       │
│ 4. Fetch OI ranking data (market-wide)                      │
│ 5. Build Context object with:                               │
│    - Account info (simulated: $1000 equity)                 │
│    - Candidate coins                                        │
│    - Market data map                                        │
│    - Quant data map                                         │
│    - OI ranking data                                        │
└─────────────────────────────────────────────────────────────┘
```

### 3.4 Debate Rounds

**Location:** `debate/engine.go:runDebate()` (Lines 157-289)

```
┌─────────────────────────────────────────────────────────────┐
│ For each round (1 to max_rounds):                           │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ 1. Broadcast "round_start" event                        │ │
│ │ 2. For each participant (in speak_order):               │ │
│ │    ┌─────────────────────────────────────────────────┐  │ │
│ │    │ a. Build personality-enhanced system prompt     │  │ │
│ │    │ b. Build user prompt with:                      │  │ │
│ │    │    - Market data (from strategy engine)         │  │ │
│ │    │    - Previous debate messages (if round > 1)    │  │ │
│ │    │ c. Call AI model with 60s timeout               │  │ │
│ │    │ d. Parse multi-coin decisions from response     │  │ │
│ │    │ e. Save message to database                     │  │ │
│ │    │ f. Broadcast "message" event                    │  │ │
│ │    └─────────────────────────────────────────────────┘  │ │
│ │ 3. Broadcast "round_end" event                          │ │
│ └─────────────────────────────────────────────────────────┘ │
│                                                             │
│ After all rounds:                                           │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ 1. Enter voting phase (status = "voting")               │ │
│ │ 2. Collect final votes from all participants            │ │
│ │ 3. Determine multi-coin consensus                       │ │
│ │ 4. Store final decisions                                │ │
│ │ 5. Update status to "completed"                         │ │
│ │ 6. Broadcast "consensus" event                          │ │
│ └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

---

## 4. Personality System

### 4.1 Available Personalities

| Personality | Emoji | Name | Description | Trading Bias |
|------------|-------|------|-------------|--------------|
| Bull | 🐂 | Aggressive Bull | Looks for long opportunities | Optimistic, trend-following |
| Bear | 🐻 | Cautious Bear | Skeptical, focuses on risks | Pessimistic, short bias |
| Analyst | 📊 | Data Analyst | Neutral, purely data-driven | No bias, objective analysis |
| Contrarian | 🔄 | Contrarian | Challenges majority view | Alternative perspectives |
| Risk Manager | 🛡️ | Risk Manager | Focus on risk control | Position sizing, stop loss |

### 4.2 Personality Prompt Enhancement

**Location:** `debate/engine.go:buildDebateSystemPrompt()` (Lines 365-426)

```
## DEBATE MODE - ROUND {round}/{max_rounds}

You are participating as {emoji} {personality}.

### Your Debate Role:
{personality_description}

### Debate Rules:
1. Analyze ALL candidate coins
2. Support arguments with specific data
3. Respond to other participants (round > 1)
4. Be persuasive but data-driven
5. Can recommend multiple coins with different actions

### Output Format (STRICT JSON):
<reasoning>
  - Market analysis with data references
  - Main trading thesis
  - Response to others (if round > 1)
</reasoning>

<decision>
[
  {"symbol": "BTCUSDT", "action": "open_long", "confidence": 75, ...},
  {"symbol": "ETHUSDT", "action": "open_short", "confidence": 80, ...}
]
</decision>
```

### 4.3 Personality-Specific Prompts

**Bull (🐂):**
```
As a bull, you are optimistic about market trends.
Look for long opportunities, identify bullish patterns,
and support your thesis with technical and fundamental data.
Focus on: breakout patterns, momentum, support levels.
```

**Bear (🐻):**
```
As a bear, you are cautious and skeptical.
Look for short opportunities, identify bearish patterns,
and highlight risks and potential downside.
Focus on: resistance levels, divergences, overbought conditions.
```

**Analyst (📊):**
```
As a data analyst, you are completely neutral.
Provide objective analysis based purely on data.
No emotional bias - let the numbers speak.
Focus on: key metrics, statistical patterns, historical comparisons.
```

**Contrarian (🔄):**
```
As a contrarian, challenge the majority view.
Look for overlooked opportunities and hidden risks.
Play devil's advocate to strengthen the debate.
Focus on: crowd positioning, sentiment extremes, neglected signals.
```

**Risk Manager (🛡️):**
```
As a risk manager, focus on capital preservation.
Evaluate position sizing, stop loss levels, and risk/reward ratios.
Ensure all decisions have appropriate risk controls.
Focus on: max drawdown, position limits, volatility-adjusted sizing.
```

---

## 5. Consensus Algorithm

### 5.1 Vote Collection

**Location:** `debate/engine.go:collectVotes()` (Lines 542-567)

```
For each participant:
┌─────────────────────────────────────────────────────────────┐
│ 1. Build voting system prompt                               │
│ 2. Build voting user prompt with debate summary             │
│ 3. Call AI model for final vote                             │
│ 4. Parse multi-coin decisions                               │
│ 5. Validate/fix symbols against session.Symbol             │
│ 6. Save vote to database                                    │
│ 7. Broadcast "vote" event                                   │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 Multi-Coin Consensus Determination

**Location:** `debate/engine.go:determineMultiCoinConsensus()` (Lines 752-924)

**Algorithm:**

```
1. Collect all coin decisions from all votes
2. Group by: symbol → action → aggregated data

3. For each vote decision:
   weight = confidence / 100.0
   Accumulate:
   ┌─────────────────────────────────────────────────────────┐
   │ score += weight                                         │
   │ total_confidence += confidence                          │
   │ total_leverage += leverage                              │
   │ total_position_pct += position_pct                      │
   │ total_stop_loss += stop_loss                            │
   │ total_take_profit += take_profit                        │
   │ count++                                                 │
   └─────────────────────────────────────────────────────────┘

4. For each symbol:
   Find winning action (max score)
   Calculate averages:
   ┌─────────────────────────────────────────────────────────┐
   │ avg_confidence = total_confidence / count               │
   │ avg_leverage = clamp(total_leverage / count, 1, 20)     │
   │ avg_position_pct = clamp(total_pct / count, 0.1, 1.0)   │
   │ avg_stop_loss = default 3% if not set                   │
   │ avg_take_profit = default 6% if not set                 │
   └─────────────────────────────────────────────────────────┘

5. Return array of consensus decisions
```

### 5.3 Consensus Example

**Input Votes:**
```
AI1 (Bull):     BTC open_long  (conf=80, lev=10, pos=0.3)
AI2 (Bear):     BTC open_short (conf=60, lev=5, pos=0.2)
AI3 (Analyst):  BTC open_long  (conf=70, lev=8, pos=0.25)
```

**Calculation:**
```
open_long:
  score = 0.80 + 0.70 = 1.50
  avg_conf = (80 + 70) / 2 = 75
  avg_lev = (10 + 8) / 2 = 9
  avg_pos = (0.3 + 0.25) / 2 = 0.275

open_short:
  score = 0.60
  avg_conf = 60
  avg_lev = 5
  avg_pos = 0.2

Winner: open_long (score 1.50 > 0.60)
```

**Output:**
```json
{
  "symbol": "BTCUSDT",
  "action": "open_long",
  "confidence": 75,
  "leverage": 9,
  "position_pct": 0.275,
  "stop_loss": 0.03,
  "take_profit": 0.06
}
```

---

## 6. Auto-Execution

### 6.1 Execution Flow

**Location:** `debate/engine.go:ExecuteConsensus()` (Lines 932-1052)

```
POST /api/debates/:id/execute
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. Validate session status = completed                      │
│ 2. Validate final_decision exists and not executed          │
│ 3. Validate action is open_long or open_short               │
│ 4. Get current market price                                 │
│ 5. Get account balance:                                     │
│    - Try available_balance                                  │
│    - Fallback to total_equity or wallet_balance             │
│ 6. Calculate position size:                                 │
│    position_size_usd = available_balance × position_pct     │
│    (minimum $12 to meet exchange requirements)              │
│ 7. Calculate stop loss and take profit prices:              │
│    ┌───────────────────────────────────────────────────┐    │
│    │ open_long:                                        │    │
│    │   SL = price × (1 - stop_loss_pct)               │    │
│    │   TP = price × (1 + take_profit_pct)             │    │
│    │ open_short:                                       │    │
│    │   SL = price × (1 + stop_loss_pct)               │    │
│    │   TP = price × (1 - take_profit_pct)             │    │
│    └───────────────────────────────────────────────────┘    │
│ 8. Create Decision object                                   │
│ 9. Call executor.ExecuteDecision()                          │
│ 10. Update final_decision:                                  │
│     - executed = true/false                                 │
│     - executed_at = timestamp                               │
│     - error message if failed                               │
└─────────────────────────────────────────────────────────────┘
```

### 6.2 Position Size Calculation

```go
// Calculate position value
position_size_usd := available_balance * position_pct

// Ensure minimum size for exchange
if position_size_usd < 12 {
    position_size_usd = 12
}

// Calculate quantity
quantity := position_size_usd / market_price
```

---

## 7. API Reference

### 7.1 Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/debates` | List all debates for user |
| GET | `/api/debates/personalities` | Get AI personality configs |
| GET | `/api/debates/:id` | Get debate with full details |
| POST | `/api/debates` | Create new debate |
| POST | `/api/debates/:id/start` | Start debate execution |
| POST | `/api/debates/:id/cancel` | Cancel running debate |
| POST | `/api/debates/:id/execute` | Execute consensus trade |
| DELETE | `/api/debates/:id` | Delete debate |
| GET | `/api/debates/:id/messages` | Get all messages |
| GET | `/api/debates/:id/votes` | Get all votes |
| GET | `/api/debates/:id/stream` | SSE live stream |

### 7.2 Create Debate Request

```json
POST /api/debates
{
  "name": "BTC Market Debate",
  "strategy_id": "strategy-uuid",
  "symbol": "BTCUSDT",
  "max_rounds": 3,
  "interval_minutes": 5,
  "prompt_variant": "balanced",
  "auto_execute": false,
  "trader_id": "trader-uuid",
  "enable_oi_ranking": true,
  "oi_ranking_limit": 10,
  "oi_duration": "1h",
  "participants": [
    {"ai_model_id": "deepseek-v3", "personality": "bull"},
    {"ai_model_id": "qwen-max", "personality": "bear"},
    {"ai_model_id": "gpt-5.2", "personality": "analyst"}
  ]
}
```

### 7.3 Create Debate Response

```json
{
  "id": "debate-uuid",
  "user_id": "user-uuid",
  "name": "BTC Market Debate",
  "strategy_id": "strategy-uuid",
  "status": "pending",
  "symbol": "BTCUSDT",
  "max_rounds": 3,
  "current_round": 0,
  "participants": [
    {
      "id": "participant-uuid",
      "ai_model_id": "deepseek-v3",
      "ai_model_name": "DeepSeek V3",
      "provider": "deepseek",
      "personality": "bull",
      "color": "#22C55E",
      "speak_order": 0
    }
  ],
  "messages": [],
  "votes": [],
  "created_at": "2025-12-15T12:00:00Z"
}
```

### 7.4 Execute Consensus Request

```json
POST /api/debates/:id/execute
{
  "trader_id": "trader-uuid"
}
```

---

## 8. Real-Time Updates (SSE)

### 8.1 SSE Endpoint

**Location:** `api/debate.go:HandleDebateStream()` (Lines 407-453)

```
GET /api/debates/:id/stream
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. Validate user ownership                                  │
│ 2. Set SSE headers:                                         │
│    Content-Type: text/event-stream                          │
│    Cache-Control: no-cache                                  │
│    Connection: keep-alive                                   │
│ 3. Send initial state                                       │
│ 4. Subscribe to events                                      │
│ 5. Stream updates until client disconnects                  │
└─────────────────────────────────────────────────────────────┘
```

### 8.2 Event Types

| Event | Trigger | Data |
|-------|---------|------|
| `initial` | Connection start | Full session state |
| `round_start` | Round begins | `{round, status}` |
| `message` | AI speaks | DebateMessage object |
| `round_end` | Round complete | `{round, status}` |
| `vote` | AI votes | DebateVote object |
| `consensus` | Debate complete | DebateDecision object |
| `error` | Error occurs | `{error: string}` |

### 8.3 SSE Message Format

```
event: message
data: {"id":"msg-uuid","session_id":"session-uuid","round":1,"ai_model_name":"DeepSeek V3","personality":"bull","content":"...","decision":{"action":"open_long","symbol":"BTCUSDT","confidence":75}}

event: vote
data: {"id":"vote-uuid","session_id":"session-uuid","ai_model_name":"DeepSeek V3","action":"open_long","symbol":"BTCUSDT","confidence":80,"reasoning":"..."}

event: consensus
data: {"action":"open_long","symbol":"BTCUSDT","confidence":75,"leverage":8,"position_pct":0.25,"stop_loss":0.03,"take_profit":0.06}
```

---

## 9. Database Schema

### 9.1 Tables

**debate_sessions:**
```sql
CREATE TABLE debate_sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  name TEXT NOT NULL,
  strategy_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  symbol TEXT NOT NULL,
  max_rounds INTEGER DEFAULT 3,
  current_round INTEGER DEFAULT 0,
  interval_minutes INTEGER DEFAULT 5,
  prompt_variant TEXT DEFAULT 'balanced',
  final_decision TEXT,
  final_decisions TEXT,
  auto_execute BOOLEAN DEFAULT 0,
  trader_id TEXT,
  enable_oi_ranking BOOLEAN DEFAULT 0,
  oi_ranking_limit INTEGER DEFAULT 10,
  oi_duration TEXT DEFAULT '1h',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**debate_participants:**
```sql
CREATE TABLE debate_participants (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  ai_model_id TEXT NOT NULL,
  ai_model_name TEXT NOT NULL,
  provider TEXT NOT NULL,
  personality TEXT NOT NULL,
  color TEXT NOT NULL,
  speak_order INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (session_id) REFERENCES debate_sessions(id) ON DELETE CASCADE
);
```

**debate_messages:**
```sql
CREATE TABLE debate_messages (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  round INTEGER NOT NULL,
  ai_model_id TEXT NOT NULL,
  ai_model_name TEXT NOT NULL,
  provider TEXT NOT NULL,
  personality TEXT NOT NULL,
  message_type TEXT NOT NULL,
  content TEXT NOT NULL,
  decision TEXT,
  decisions TEXT,
  confidence INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (session_id) REFERENCES debate_sessions(id) ON DELETE CASCADE
);
```

**debate_votes:**
```sql
CREATE TABLE debate_votes (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  ai_model_id TEXT NOT NULL,
  ai_model_name TEXT NOT NULL,
  action TEXT NOT NULL,
  symbol TEXT NOT NULL,
  confidence INTEGER DEFAULT 0,
  leverage INTEGER DEFAULT 5,
  position_pct REAL DEFAULT 0.2,
  stop_loss_pct REAL DEFAULT 0.03,
  take_profit_pct REAL DEFAULT 0.06,
  reasoning TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (session_id) REFERENCES debate_sessions(id) ON DELETE CASCADE
);
```

### 9.2 Key Store Methods

| Method | Description |
|--------|-------------|
| `CreateSession()` | Create new debate session |
| `GetSession()` | Get session by ID |
| `GetSessionWithDetails()` | Get session with participants, messages, votes |
| `UpdateSessionStatus()` | Update session status |
| `UpdateSessionRound()` | Update current round |
| `UpdateSessionFinalDecisions()` | Store consensus decisions |
| `AddParticipant()` | Add AI participant |
| `AddMessage()` | Store debate message |
| `AddVote()` | Store final vote |

---

## 10. Frontend Components

### 10.1 Page Structure

**Location:** `web/src/pages/DebateArenaPage.tsx`

```
DebateArenaPage
├── Left Sidebar (w-56)
│   ├── New Debate Button
│   ├── Debate Sessions List
│   │   └── SessionItem (status, name, timestamp)
│   └── Online Traders List
│       └── TraderItem (name, status, AI model)
│
├── Main Content
│   ├── Header Bar
│   │   ├── Session Info (name, status, symbol)
│   │   ├── Participants Avatars
│   │   └── Vote Summary
│   │
│   ├── Content Area (two-column)
│   │   ├── Left: Discussion Records
│   │   │   ├── Round Headers
│   │   │   └── MessageCards (expandable)
│   │   │
│   │   └── Right: Final Votes
│   │       └── VoteCards (action, confidence, reasoning)
│   │
│   └── Consensus Bar
│       ├── Final Decision Display
│       └── Execute Button (if auto_execute disabled)
│
└── Modals
    ├── CreateModal
    │   ├── Name Input
    │   ├── Strategy Selector
    │   ├── Symbol Input (auto-filled)
    │   ├── Max Rounds Selector
    │   └── Participant Picker (AI model + personality)
    │
    └── ExecuteModal
        └── Trader Selector
```

### 10.2 UI Components

**MessageCard:**
- Expandable message display
- Shows AI avatar, personality emoji, decision
- Parses reasoning/analysis sections from content
- Displays decision details (leverage, position, SL/TP)
- Supports multi-coin decisions

**VoteCard:**
- Confidence bar visualization
- Action indicator (long/short/hold/wait)
- Leverage and position size display
- Stop loss and take profit display
- Reasoning preview

### 10.3 Status Colors

```typescript
const STATUS_COLOR = {
  pending: 'bg-gray-500',
  running: 'bg-blue-500 animate-pulse',
  voting: 'bg-yellow-500 animate-pulse',
  completed: 'bg-green-500',
  cancelled: 'bg-red-500',
}
```

### 10.4 Action Styling

```typescript
const ACT = {
  open_long: {
    color: 'text-green-400',
    bg: 'bg-green-500/20',
    icon: <TrendingUp />,
    label: 'LONG'
  },
  open_short: {
    color: 'text-red-400',
    bg: 'bg-red-500/20',
    icon: <TrendingDown />,
    label: 'SHORT'
  },
  hold: {
    color: 'text-blue-400',
    bg: 'bg-blue-500/20',
    icon: <Minus />,
    label: 'HOLD'
  },
  wait: {
    color: 'text-gray-400',
    bg: 'bg-gray-500/20',
    icon: <Clock />,
    label: 'WAIT'
  },
}
```

### 10.5 Personality Colors

```typescript
const PERS = {
  bull: { emoji: '🐂', color: '#22C55E', name: '多头', nameEn: 'Bull' },
  bear: { emoji: '🐻', color: '#EF4444', name: '空头', nameEn: 'Bear' },
  analyst: { emoji: '📊', color: '#3B82F6', name: '分析', nameEn: 'Analyst' },
  contrarian: { emoji: '🔄', color: '#F59E0B', name: '逆势', nameEn: 'Contrarian' },
  risk_manager: { emoji: '🛡️', color: '#8B5CF6', name: '风控', nameEn: 'Risk Mgr' },
}
```

---

## 11. Integration Points

### 11.1 Strategy System

Debate sessions depend on saved strategies for:
- **Coin source configuration:** static/pool/OI top
- **Market data indicators:** K-lines, timeframes, technical indicators
- **Risk control parameters:** leverage limits, position sizing
- **Custom prompts:** role definition, trading rules

### 11.2 AI Model System

Each participant requires:
- AI model configuration (provider, API key, custom URL)
- Supported providers: deepseek, qwen, openai, claude, gemini, grok, kimi
- Client initialization with timeout handling (60s per call)

### 11.3 Trader System

For auto-execution:
- Requires active trader with running status
- Trader must have valid exchange connection
- Executor interface: `ExecuteDecision()`, `GetBalance()`

### 11.4 Market Data

Market context building uses:
- Market data service (K-lines, OHLCV)
- Quantitative data (capital flow, position changes)
- OI ranking data (market-wide position changes)

---

## 12. Error Handling

### 12.1 Cleanup on Startup

**Location:** `debate/engine.go:cleanupStaleDebates()` (Lines 58-71)

```go
// On server restart, cancel all running/voting debates
func cleanupStaleDebates() {
    sessions := debateStore.ListAllSessions()
    for _, session := range sessions {
        if session.Status == running || session.Status == voting {
            debateStore.UpdateSessionStatus(session.ID, cancelled)
        }
    }
}
```

### 12.2 AI Call Timeout

```go
// 60 seconds per participant response
select {
case res := <-resultCh:
    response = res.response
case <-time.After(60 * time.Second):
    return nil, fmt.Errorf("AI call timeout")
}
```

### 12.3 Symbol Validation

```go
// Force all decisions to use session symbol if specified
if session.Symbol != "" {
    for _, d := range decisions {
        if d.Symbol == "" || d.Symbol != session.Symbol {
            logger.Warnf("Fixing invalid symbol '%s' -> '%s'", d.Symbol, session.Symbol)
            d.Symbol = session.Symbol
        }
    }
}
```

### 12.4 Panic Recovery

```go
defer func() {
    if r := recover(); r != nil {
        logger.Errorf("Debate panic: %v", r)
        debateStore.UpdateSessionStatus(sessionID, cancelled)
        if OnError != nil {
            OnError(sessionID, fmt.Errorf("panic: %v", r))
        }
    }
}()
```

---

## Summary

The Debate Arena module provides a sophisticated multi-AI collaborative decision system with:

- **Multi-Personality Debate:** 5 distinct AI personalities (Bull, Bear, Analyst, Contrarian, Risk Manager) with unique trading biases
- **Consensus Mechanism:** Weighted voting based on confidence levels to determine final decisions
- **Real-Time Updates:** SSE streaming for live debate progress
- **Auto-Execution:** Optional automatic trade execution based on consensus
- **Strategy Integration:** Deep integration with strategy configuration for market data and risk parameters
- **Multi-Coin Support:** Ability to analyze and decide on multiple coins simultaneously

The system enables users to leverage multiple AI perspectives for more robust trading decisions while maintaining full control over execution.
