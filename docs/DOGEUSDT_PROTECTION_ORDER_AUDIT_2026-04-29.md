# DOGEUSDT 残留保护单只读审计（2026-04-29 13:06）

## 当前持仓

- Symbol: `DOGEUSDT`
- Side: LONG
- 当前交易所持仓：`0.13` contract（约 `130` qty）
- 当前 runtime 状态：
  - Break-even: armed，monitor 持续 `already armed`
  - Drawdown/native trailing: `native_partial_trailing_armed`，monitor 持续 `already in native_partial_trailing`
  - Reconciler: verified，`stopOwner=breakeven profitOwner=drawdown`

## 当前 open order 总览（本地 sync 视图）

`trader_orders` 中 `DOGEUSDT status=NEW` 共 `31` 条；运行日志当前交易所 `GetOpenOrders` 返回约 `20` 条。

### 明确应保留 / 当前有效候选

1. 当前 BE stop（qty=130, stop=0.0996）
   - 候选 order ids：
     - `3521079756131557376`
     - `3521079721033621504`
   - 二者 stop/qty 相同，其中较新的两条来自 12:59 重启窗口；理论上只需要一条，但取消前需 live query 确认哪个仍在交易所。

2. 当前 native trailing（qty=130）
   - `3521088488269574144`
   - type: `TRAILING_STOP_MARKET`
   - quantity: `130`
   - 当前仓位量匹配，应保留。

### 高风险残留候选（不应自动取消，需人工确认）

这些订单 qty 大于当前仓位 `130`，疑似来自旧仓位/竞态窗口：

- 旧 full/fallback/BE 类 stop，qty `650` 或 `330`：
  - `3519608935018029056` qty=650 stop=0.09679
  - `3519608809021136896` qty=330 stop=0.09788
  - `3519608786539667456` qty=330 stop=0.09848
  - 多条 qty=650 stop=0.0996：
    - `3520980951147110400`
    - `3520980913968799744`
    - `3520992450049630208`
    - `3520992408777678848`
    - `3520999520572432384`
    - `3520999472254050304`
    - `3521005832429527040`
    - `3521005783507165184`
    - `3521028598641876992`
    - `3521028547572031488`
    - `3521034876374114304`
    - `3521034835035054080`
    - `3521043817153847296`
    - `3521043780512407552`

- 旧 native trailing，qty `520` 或 `650`：
  - `3519609193688879104` qty=650 stop=0.1008
  - `3520982462539124736` qty=520 stop=0.10122
  - `3520981715550359552` qty=520 stop=0.10123
  - `3520981114691153920` qty=520 stop=0.10127
  - `3521002880746549248` qty=520 stop=0.10136（仍是 dynamic record 中旧 native_partial owner；但当前仓位已缩为 130，语义已过期）
  - `3521001555413585920` qty=520 stop=0.10142
  - `3521000195251130368` qty=520 stop=0.10144
  - `3520997214577381376` qty=520 stop=0.10153
  - `3520995875218681856` qty=520 stop=0.10159
  - `3520993190931230720` qty=520 stop=0.1016
  - `3520994535625084928` qty=520 stop=0.10167

## 建议下一步

1. **不要直接 bulk cancel**：先用 live exchange query 重新确认这些 ids 是否仍存在。
2. 若确认仍存在，建议只保留：
   - 1 条 qty=130 的 BE stop；
   - 1 条 qty=130 的 native trailing；
   - 其他 qty>130 的 DOGE reduce-only/conditional 保护单列入取消候选。
3. 清理前最好暂停 trader 或至少确认本轮 AI/reconciler 不会并发改单。
4. 清理后需要再次 `GetOpenOrders(DOGEUSDT)` 验证：
   - open order 数量下降；
   - reconciler 仍 `verified=true`；
   - DOGE 当前持仓仍有 BE + drawdown protection。

## 当前代码状态

- BE/native 重复下单链路已收住：13:00-13:07 日志未再出现新的 `Stop loss price set` / `Trailing stop set`。
- 当前问题主要是历史残留订单清理，而不是继续生成新订单。
