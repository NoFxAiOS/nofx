import pandas as pd
import numpy as np

class TradingAI:
    def __init__(self):
        pass
    
    def analyze_trades(self, trades):
        """分析交易数据"""
        if not trades:
            return {"error": "无交易数据"}
        
        df = pd.DataFrame(trades)
        
        # 基础分析
        total_trades = len(df)
        buy_trades = len(df[df['action'] == 'BUY'])
        sell_trades = len(df[df['action'] == 'SELL'])
        
        return {
            "total_trades": total_trades,
            "buy_trades": buy_trades,
            "sell_trades": sell_trades,
            "buy_ratio": buy_trades / total_trades if total_trades > 0 else 0,
            "most_traded": df['symbol'].mode()[0] if not df.empty else "N/A"
        }
