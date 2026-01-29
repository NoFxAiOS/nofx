import sys
import re
import json
import datetime
import os

# === 配置部分 ===
LOG_FILE_PATH = "/root/nofx_project/ai_commands.txt"

# 动作翻译字典
ACTION_MAP = {
    "open_short": "📉 开空 (做空)",
    "open_long": "📈 开多 (做多)",
    "close_short": "💰 平空 (止盈/止损)",
    "close_long": "💰 平多 (止盈/止损)",
    "wait": "👀 观望",
    "hold": "✊ 持仓",
}

def clean_file_if_needed():
    if not os.path.exists(LOG_FILE_PATH): return
    try:
        last_mtime = datetime.datetime.fromtimestamp(os.path.getmtime(LOG_FILE_PATH))
        if last_mtime.date() < datetime.datetime.now().date():
            with open(LOG_FILE_PATH, 'w', encoding='utf-8') as f:
                f.write(f"=== 日志自动清理: {datetime.datetime.now()} ===\n\n")
    except: pass

def write_to_log(text):
    """通用的写入函数"""
    try:
        clean_file_if_needed()
        with open(LOG_FILE_PATH, 'a', encoding='utf-8') as f:
            f.write(text + "\n")
    except: pass

def translate_and_save_json(json_str, timestamp):
    try:
        clean_json = json_str.replace("```json", "").replace("```", "").strip()
        data = json.loads(clean_json)
        
        output_lines = [f"⏰ 时间: {timestamp}"]
        items = data if isinstance(data, list) else [data]
        
        for item in items:
            symbol = item.get("symbol", "未知币种")
            action = item.get("action", "unknown")
            action_cn = ACTION_MAP.get(action, action)
            price = item.get("price", "市价")
            reason = item.get("reason", "")
            leverage = item.get("leverage", "")
            
            # 判断订单类型
            order_type = ""
            if action.startswith("open"):
                if str(price).replace('.', '', 1).isdigit() and float(price) > 0:
                    order_type = " [🎯 限价单]"
                else:
                    order_type = " [⚡ 市价单]"
            
            line = f"  👉 {symbol} | {action_cn}{order_type}"
            if str(price) not in ["0", "", "market", "市价"] or action.startswith("open"):
                line += f" | 价格: {price}"
            if leverage:
                line += f" | {leverage}x"
                
            output_lines.append(line)
            if reason:
                output_lines.append(f"     📝 理由: {reason}")

        output_lines.append("-" * 40)
        write_to_log("\n".join(output_lines))
            
    except Exception as e:
        write_to_log(f"⚠️ JSON解析错误: {str(e)}")

def process_log_line(line):
    """处理普通日志行，提取关键信息"""
    current_time = datetime.datetime.now().strftime("%m-%d %H:%M:%S")
    
    # 1. 拦截“死扛模式”
    if "死扛模式" in line:
        # 提取关键信息，通常在 ] 后面
        msg = line.split("死扛模式]")[-1].strip()
        write_to_log(f"🛡️ 【触发死扛】 {current_time} | {msg}")
        write_to_log("-" * 40)

    # 2. 拦截“嫌赚得少”
    elif "嫌赚得少" in line:
        msg = line.split("嫌赚得少]")[-1].strip()
        write_to_log(f"🤏 【嫌赚得少】 {current_time} | {msg}")
        write_to_log("-" * 40)

    # 3. 拦截“止盈时刻”
    elif "止盈时刻" in line:
        msg = line.split("止盈时刻]")[-1].strip()
        write_to_log(f"💰 【止盈触发】 {current_time} | {msg}")
        write_to_log("-" * 40)

def main():
    buffer = ""
    recording = False
    # 强制无缓冲，确保实时性
    sys.stdout.reconfigure(line_buffering=True) if hasattr(sys.stdout, 'reconfigure') else None

    for line in sys.stdin:
        # 优先检查是否有特殊关键词（死扛/止盈等）
        process_log_line(line)

        # 处理 JSON 块
        if "RAW JSON >>>" in line:
            recording = True
            match = re.search(r'(\d{2}-\d{2} \d{2}:\d{2}:\d{2})', line)
            current_time = match.group(1) if match else datetime.datetime.now().strftime("%m-%d %H:%M:%S")
            parts = line.split("RAW JSON >>>")
            if len(parts) > 1: buffer += parts[1]
        
        elif recording:
            if "<<<" in line:
                recording = False
                buffer += line.split("<<<")[0]
                translate_and_save_json(buffer, current_time)
                buffer = ""
            else:
                buffer += line

if __name__ == "__main__":
    main()
