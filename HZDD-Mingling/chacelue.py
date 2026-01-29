import sqlite3
import os
import unicodedata

DB_PATH = "/root/nofx_project/data/data.db"

# --- 荧光绿配色 ---
NEON_GREEN = "\033[92m"
BOLD = "\033[1m"
RESET = "\033[0m"

def get_display_width(text):
    """
    关键函数：计算字符串在屏幕上的真实显示宽度
    中文算2格，英文算1格
    """
    width = 0
    for char in str(text):
        # 'W' = Wide (汉字), 'F' = Fullwidth (全角符号)
        if unicodedata.east_asian_width(char) in ('W', 'F'):
            width += 2
        else:
            width += 1
    return width

def pad_string(text, target_width):
    """
    智能填充：根据真实显示宽度来补充空格
    """
    text = str(text)
    current_width = get_display_width(text)
    padding_len = target_width - current_width
    if padding_len < 0:
        padding_len = 0 # 如果超长就不补了
    return text + " " * padding_len

def print_row(col1, col2, is_header=False):
    v = "║"
    
    # 设定列宽：策略名宽45，时间宽25
    # 这里不再用 f"{col1:<45}" 这种傻瓜写法，而是用我们的智能函数
    c1_txt = pad_string(col1, 45)
    c2_txt = pad_string(col2, 25)
    
    # 计算总宽度用于画横线: 45 + 25 + 边框空间
    # 45(col1) + 1(space) + 1(v) + 1(space) + 25(col2) + ...
    # 实际上横线长度是固定的，只要内容对齐就行
    
    if is_header:
        print(f"{NEON_GREEN}╔{'═'*47}╦{'═'*27}╗{RESET}")
        # 标题居中比较麻烦，简单处理直接写死长度或者用空行
        print(f"{NEON_GREEN}║{'STRATEGY LIBRARY':^75}║{RESET}")
        print(f"{NEON_GREEN}╠{'═'*47}╬{'═'*27}╣{RESET}")
        print(f"{NEON_GREEN}{v} {BOLD}{c1_txt} {RESET}{NEON_GREEN}{v} {BOLD}{c2_txt} {RESET}{NEON_GREEN}{v}{RESET}")
        print(f"{NEON_GREEN}╠{'═'*47}╬{'═'*27}╣{RESET}")
    else:
        print(f"{NEON_GREEN}{v} {c1_txt} {v} {c2_txt} {v}{RESET}")

def run():
    if not os.path.exists(DB_PATH):
        print(f"❌ DB Not Found: {DB_PATH}")
        return

    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    
    try:
        # 查询策略名和创建时间
        cursor.execute("SELECT name, created_at FROM strategies ORDER BY created_at DESC")
        rows = cursor.fetchall()
        
        print_row("STRATEGY NAME (策略名)", "CREATED AT (创建时间)", is_header=True)
        
        if not rows:
            print(f"{NEON_GREEN}║ {'(NO STRATEGIES FOUND)':^73} ║{RESET}")
        else:
            for row in rows:
                # 截取时间字符串，防止过长
                time_str = str(row[1])[:19]
                print_row(row[0], time_str)
        
        print(f"{NEON_GREEN}╚{'═'*47}╩{'═'*27}╝{RESET}\n")
        
    finally:
        conn.close()

if __name__ == "__main__":
    run()
