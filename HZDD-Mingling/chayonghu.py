import sqlite3
import os

# --- 核心配置 ---
DB_PATH = "/root/nofx_project/data/data.db"

# --- 荧光绿配色 ---
NEON_GREEN = "\033[92m"
BOLD = "\033[1m"
RESET = "\033[0m"

def print_row(col1, col2, is_header=False):
    """ 打印表格行 (使用纯英文以确保边框完美对齐) """
    v = "║" 
    
    # 格式化: ID列宽38, 邮箱列宽30 (根据UUID和常见邮箱长度设定)
    c1_txt = f"{col1:<38}"
    c2_txt = f"{col2:<30}"
    
    if is_header:
        # 顶部边框
        print(f"{NEON_GREEN}╔{'═'*40}╦{'═'*32}╗{RESET}")
        print(f"{NEON_GREEN}║{'USER DATABASE':^73}║{RESET}") # 居中标题
        print(f"{NEON_GREEN}╠{'═'*40}╬{'═'*32}╣{RESET}")
        # 表头内容
        print(f"{NEON_GREEN}{v} {BOLD}{c1_txt} {RESET}{NEON_GREEN}{v} {BOLD}{c2_txt} {RESET}{NEON_GREEN}{v}{RESET}")
        print(f"{NEON_GREEN}╠{'═'*40}╬{'═'*32}╣{RESET}")
    else:
        # 数据行
        print(f"{NEON_GREEN}{v} {c1_txt} {v} {c2_txt} {v}{RESET}")

def run():
    if not os.path.exists(DB_PATH):
        print(f"{NEON_GREEN}❌ ERROR: Database not found at {DB_PATH}{RESET}")
        return

    try:
        conn = sqlite3.connect(DB_PATH)
        cursor = conn.cursor()
        
        cursor.execute("SELECT id, email FROM users")
        users = cursor.fetchall()
        
        # 打印表头 (纯英文)
        print_row("USER ID", "EMAIL ADDRESS", is_header=True)
        
        if not users:
            print(f"{NEON_GREEN}║ {'(NO DATA FOUND)':^71} ║{RESET}")
        else:
            for user in users:
                user_id = str(user[0])
                email = str(user[1])
                print_row(user_id, email)
        
        # 底部边框
        print(f"{NEON_GREEN}╚{'═'*40}╩{'═'*32}╝{RESET}\n")

    except Exception as e:
        print(f"{NEON_GREEN}❌ ERROR: {e}{RESET}")
    finally:
        if 'conn' in locals():
            conn.close()

if __name__ == "__main__":
    run()
