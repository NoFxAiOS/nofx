#!/usr/bin/env python3
"""
功能: 数据采集 + SQLite 存储 + 定时轮询
版本: v1.0 Minimal
日期: 2025-11-02

特性:
- ✅ 自动抓取中文经济日历数据
- ✅ SQLite 数据库存储和 UPSERT 去重
- ✅ 定时轮询（可配置间隔）
- ✅ 智能代理检测
- ✅ 日志输出
- ❌ 无 Rich UI（纯后台运行）
- ❌ 无 Discord 功能

依赖:
    pip install requests lxml pytz python-dotenv

使用:
    # 默认配置运行
    python3 economic_calendar_minimal.py

    # 自定义轮询间隔（秒）
    python3 economic_calendar_minimal.py --interval 60

    # 后台运行
    nohup python3 economic_calendar_minimal.py &
"""

import os
import sys
import time
import sqlite3
import signal
import socket
import urllib.request
import argparse
from datetime import datetime, timedelta
from typing import List, Dict, Tuple, Optional
from copy import deepcopy

# ============================================================================
# 依赖检查和导入
# ============================================================================

try:
    import requests
    from lxml.html import fromstring
    import pytz
    from dotenv import load_dotenv
except ImportError as e:
    print(f"❌ 缺少依赖: {e}")
    print("请运行: pip install requests lxml pytz python-dotenv")
    sys.exit(1)

load_dotenv()

# ============================================================================
# 命令行参数
# ============================================================================

parser = argparse.ArgumentParser(description='经济日历超精简版 - 数据库 + 定时轮询')
parser.add_argument('--interval', type=int, default=None,
                    help='轮询间隔（秒），默认300秒')
parser.add_argument('--days', type=int, default=7,
                    help='获取未来天数，默认7天')
parser.add_argument('--verbose', action='store_true',
                    help='详细日志输出')
args = parser.parse_args()

# ============================================================================
# 全局配置
# ============================================================================

# 数据库配置
DB_PATH = os.getenv('DATABASE_URL', 'economic_calendar.db').replace('sqlite+pysqlite:///', '')
DAYS_AHEAD = args.days

# 轮询间隔配置
if args.interval:
    POLL_INTERVAL = args.interval
else:
    POLL_INTERVAL = int(os.getenv('POLL_INTERVAL', '300'))  # 默认5分钟

# 代理配置
PROXY_MODE = os.getenv('PROXY_MODE', 'auto').lower()  # auto/always/never
PROXY_URL = os.getenv('HTTP_PROXY', os.getenv('http_proxy', 'http://127.0.0.1:9910'))
NETWORK_TIMEOUT = int(os.getenv('NETWORK_TEST_TIMEOUT', '5'))

# API 配置
CN_CALENDAR_URL = "https://cn.investing.com/economic-calendar/Service/getCalendarFilteredData"
IMPORTANCE_MAP = {"1": "高", "2": "中", "3": "低", None: None}
TIMEZONE_CONFIGS = {
    "GMT+8": ("Asia/Shanghai", 28),
    "GMT+0": ("GMT", 0),
    "GMT-5": ("America/New_York", 1)
}

# 数据归一化常量
ALL_DAY_SENTINEL = "全天"
TENTATIVE_SENTINEL = "待定"
ALL_DAY_TOKENS = {"all day", "全天", "--", "--:--", ""}
TENTATIVE_TOKENS = {"tentative", "待定", "暂定"}

# 全局状态
running = True
update_count = 0
db_write_count = 0
_network_cache = {"result": None, "timestamp": None, "duration": 300}

VERBOSE = args.verbose

# ============================================================================
# 日志工具
# ============================================================================

def log(message: str, level: str = "INFO"):
    """简单的日志输出"""
    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    print(f"[{timestamp}] [{level}] {message}")


def verbose_log(message: str):
    """详细日志（仅在 --verbose 时输出）"""
    if VERBOSE:
        log(message, "DEBUG")


# ============================================================================
# 网络工具
# ============================================================================

def test_network_connection(url: str = "https://discord.com", use_proxy: bool = False, timeout: int = None) -> Tuple[bool, str]:
    """测试网络连接"""
    if timeout is None:
        timeout = NETWORK_TIMEOUT

    try:
        req = urllib.request.Request(url, headers={'User-Agent': 'Mozilla/5.0'})

        if use_proxy:
            proxy_handler = urllib.request.ProxyHandler({'http': PROXY_URL, 'https': PROXY_URL})
            opener = urllib.request.build_opener(proxy_handler)
        else:
            proxy_handler = urllib.request.ProxyHandler({})
            opener = urllib.request.build_opener(proxy_handler)

        response = opener.open(req, timeout=timeout)
        status_code = response.getcode()

        if 200 <= status_code < 400:
            return (True, f"连接成功 (HTTP {status_code})")
        else:
            return (False, f"HTTP {status_code}")

    except Exception as e:
        return (False, f"错误: {str(e)[:30]}")


def detect_network() -> Tuple[bool, str]:
    """智能检测网络环境"""
    global _network_cache

    # 检查缓存
    now = time.time()
    if _network_cache["result"] is not None and _network_cache["timestamp"] is not None:
        if now - _network_cache["timestamp"] < _network_cache["duration"]:
            verbose_log("使用缓存的网络检测结果")
            return _network_cache["result"]

    # 强制模式
    if PROXY_MODE == "always":
        result = (True, "配置: 总是使用代理")
        _network_cache["result"] = result
        _network_cache["timestamp"] = now
        return result

    if PROXY_MODE == "never":
        result = (False, "配置: 从不使用代理")
        _network_cache["result"] = result
        _network_cache["timestamp"] = now
        return result

    # 自动检测
    verbose_log("正在检测网络环境...")

    # 测试本地网络
    success, msg = test_network_connection(use_proxy=False, timeout=3)
    if success:
        verbose_log("本地网络可用，不使用代理")
        result = (False, "本地网络可用")
        _network_cache["result"] = result
        _network_cache["timestamp"] = now
        return result

    # 测试代理网络
    verbose_log(f"本地网络不可用 ({msg})，尝试代理...")
    success, msg = test_network_connection(use_proxy=True, timeout=3)
    if success:
        verbose_log(f"代理网络可用: {PROXY_URL}")
        result = (True, "代理网络可用")
        _network_cache["result"] = result
        _network_cache["timestamp"] = now
        return result

    # 两者都不可用
    log(f"网络不可用: {msg}", "ERROR")
    result = (False, "网络不可用")
    _network_cache["result"] = result
    _network_cache["timestamp"] = now
    return result


def setup_proxy():
    """根据网络检测结果设置代理"""
    use_proxy, reason = detect_network()

    if use_proxy:
        os.environ['HTTP_PROXY'] = PROXY_URL
        os.environ['HTTPS_PROXY'] = PROXY_URL
        log(f"已配置代理: {PROXY_URL}")
    else:
        os.environ.pop('HTTP_PROXY', None)
        os.environ.pop('HTTPS_PROXY', None)
        log("使用本地网络")

    return use_proxy, reason


# ============================================================================
# 数据归一化
# ============================================================================

def normalize_time(raw: Optional[str]) -> str:
    """归一化时间字段"""
    if raw is None:
        return ALL_DAY_SENTINEL
    value = str(raw).strip().lower()
    if value in ALL_DAY_TOKENS:
        return ALL_DAY_SENTINEL
    if value in TENTATIVE_TOKENS:
        return TENTATIVE_SENTINEL
    return raw.strip()


def normalize_text(raw: Optional[str]) -> Optional[str]:
    """归一化文本字段"""
    if raw is None:
        return None
    value = str(raw).strip()
    return value or None


def normalize_event_for_db(event: Dict) -> Dict:
    """归一化事件数据"""
    normalized = deepcopy(event)
    normalized["date"] = normalize_text(event.get("date"))
    if not normalized["date"]:
        raise ValueError("event date is required")
    normalized["event"] = normalize_text(event.get("event"))
    if not normalized["event"]:
        raise ValueError("event name is required")
    normalized["time"] = normalize_time(event.get("time"))
    normalized["zone"] = normalize_text(event.get("zone"))
    normalized["currency"] = normalize_text(event.get("currency"))
    normalized["importance"] = normalize_text(event.get("importance"))
    return normalized


# ============================================================================
# 数据采集
# ============================================================================

def parse_event_row(row, current_date: str) -> Optional[Dict]:
    """解析单条事件行"""
    try:
        event = {
            'date': current_date, 'time': None, 'zone': None, 'currency': None,
            'event': None, 'importance': None, 'actual': None, 'forecast': None, 'previous': None,
        }

        row_id = row.get("id", "").replace("eventRowId_", "")
        cells = row.xpath("td")

        for cell in cells:
            cell_class = cell.get("class", "")

            if "first left" in cell_class and "time" in cell_class:
                time_text = cell.text_content().strip()
                if time_text and time_text.lower() != "all day":
                    event['time'] = time_text

            elif "flagCur" in cell_class:
                span = cell.xpath(".//span[@title]")
                if span:
                    event['zone'] = span[0].get("title", "").lower()
                event['currency'] = cell.text_content().strip()

            elif "sentiment" in cell_class:
                data_img_key = cell.get("data-img_key", "")
                if data_img_key:
                    importance_level = data_img_key.replace("bull", "")
                    event['importance'] = IMPORTANCE_MAP.get(importance_level)

            elif cell.get("class") == "left event":
                event['event'] = cell.text_content().strip()
                if "(" in event['event']:
                    event['event'] = event['event'].split("(")[0].strip()

            elif cell.get("id") == f"eventActual_{row_id}":
                actual_text = cell.text_content().strip()
                event['actual'] = actual_text if actual_text != "–" else None

            elif cell.get("id") == f"eventForecast_{row_id}":
                forecast_text = cell.text_content().strip()
                event['forecast'] = forecast_text if forecast_text != "–" else None

            elif cell.get("id") == f"eventPrevious_{row_id}":
                previous_text = cell.text_content().strip()
                event['previous'] = previous_text if previous_text != "–" else None

        if event['event']:
            return event
        return None

    except Exception as e:
        verbose_log(f"解析事件行失败: {str(e)[:50]}")
        return None


def get_chinese_calendar(from_date: str = None, to_date: str = None, timezone: str = "GMT+8") -> List[Dict]:
    """获取中文经济日历数据"""
    try:
        if from_date is None or to_date is None:
            today = datetime.now()
            from_date = today.strftime('%d/%m/%Y')
            to_date = today.strftime('%d/%m/%Y')

        if timezone not in TIMEZONE_CONFIGS:
            timezone = "GMT+8"

        _, tz_id = TIMEZONE_CONFIGS[timezone]

        headers = {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
            "X-Requested-With": "XMLHttpRequest",
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
            "Accept-Encoding": "gzip, deflate",
            "Connection": "keep-alive",
            "Referer": "https://cn.investing.com/economic-calendar/"
        }

        start_date = datetime.strptime(from_date, "%d/%m/%Y")
        end_date = datetime.strptime(to_date, "%d/%m/%Y")

        data = {
            "dateFrom": start_date.strftime("%Y-%m-%d"),
            "dateTo": end_date.strftime("%Y-%m-%d"),
            "timeZone": tz_id,
            "timeFilter": "timeOnly",
            "currentTab": "custom",
            "submitFilters": 1,
            "limit_from": 0,
        }

        events = []
        last_id = None
        iteration = 0
        max_iterations = 50

        while iteration < max_iterations:
            iteration += 1

            try:
                response = requests.post(CN_CALENDAR_URL, headers=headers, data=data, timeout=10)
                response.raise_for_status()

                json_data = response.json()
                html_content = json_data.get("data", "")

                if not html_content:
                    break

                root = fromstring(html_content)
                rows = root.xpath(".//tr")

                if not rows:
                    break

                current_last_id = None
                current_date = None

                for row in rows:
                    row_id = row.get("id")

                    if row_id is None:
                        try:
                            day_id = row.xpath("td")[0].get("id", "").replace("theDay", "")
                            if day_id:
                                timestamp = int(day_id)
                                current_date = datetime.fromtimestamp(timestamp, tz=pytz.UTC).strftime("%d/%m/%Y")
                        except:
                            continue

                    elif "eventRowId_" in row_id:
                        current_last_id = row_id.replace("eventRowId_", "")
                        event = parse_event_row(row, current_date)
                        if event:
                            events.append(event)

                if current_last_id == last_id:
                    break

                last_id = current_last_id
                data["limit_from"] += 1

            except requests.RequestException as e:
                log(f"网络请求失败: {str(e)[:50]}", "ERROR")
                break
            except Exception as e:
                log(f"解析失败: {str(e)[:50]}", "ERROR")
                break

        return events

    except Exception as e:
        log(f"获取中文经济日历失败: {str(e)[:100]}", "ERROR")
        return []


def fetch_calendar(days_ahead: int = DAYS_AHEAD) -> List[Dict]:
    """获取经济日历数据"""
    try:
        today = datetime.now()
        future_date = today + timedelta(days=days_ahead)
        from_date = today.strftime('%d/%m/%Y')
        to_date = future_date.strftime('%d/%m/%Y')

        verbose_log(f"获取日期范围: {from_date} - {to_date}")
        events = get_chinese_calendar(from_date=from_date, to_date=to_date, timezone="GMT+8")

        return events
    except Exception as e:
        log(f"获取失败: {type(e).__name__}", "ERROR")
        return []


# ============================================================================
# 数据库操作
# ============================================================================

def init_database():
    """初始化数据库"""
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()

    cursor.execute("""
        CREATE TABLE IF NOT EXISTS events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            date TEXT NOT NULL,
            time TEXT,
            zone TEXT,
            currency TEXT,
            event TEXT NOT NULL,
            importance TEXT,
            actual TEXT,
            forecast TEXT,
            previous TEXT,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            UNIQUE(date, time, zone, event)
        )
    """)

    cursor.execute("CREATE INDEX IF NOT EXISTS idx_date ON events(date)")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_importance ON events(importance)")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_currency ON events(currency)")

    conn.commit()
    conn.close()


def write_to_database(events: List[Dict]) -> int:
    """批量写入数据库"""
    global db_write_count

    if not events:
        return 0

    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    now = datetime.now().isoformat()
    success_count = 0

    for event in events:
        try:
            normalized = normalize_event_for_db(event)
            cursor.execute("""
                INSERT INTO events
                (date, time, zone, currency, event, importance,
                 actual, forecast, previous, created_at, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT(date, time, zone, event) DO UPDATE SET
                    actual = excluded.actual,
                    forecast = excluded.forecast,
                    previous = excluded.previous,
                    importance = excluded.importance,
                    updated_at = excluded.updated_at
            """, (
                normalized.get('date'),
                normalized.get('time'),
                normalized.get('zone'),
                normalized.get('currency'),
                normalized.get('event'),
                normalized.get('importance'),
                normalized.get('actual'),
                normalized.get('forecast'),
                normalized.get('previous'),
                now,
                now
            ))
            success_count += 1
        except Exception as e:
            verbose_log(f"数据库写入失败: {str(e)[:50]}")

    conn.commit()
    conn.close()
    db_write_count += 1

    return success_count


# ============================================================================
# 主程序逻辑
# ============================================================================

def signal_handler(sig, frame):
    """处理中断信号"""
    global running
    log("接收到停止信号，正在退出...", "INFO")
    running = False


signal.signal(signal.SIGINT, signal_handler)
signal.signal(signal.SIGTERM, signal_handler)


def update_data():
    """更新数据"""
    global update_count

    log("开始获取数据...")
    events = fetch_calendar()

    if events:
        event_count = len(events)
        log(f"获取到 {event_count} 条事件")

        db_count = write_to_database(events)
        update_count += 1

        log(f"数据库已更新: {db_count} 条 (总更新次数: {update_count})")

        # 统计信息
        high = len([e for e in events if e.get('importance') == '高'])
        medium = len([e for e in events if e.get('importance') == '中'])
        low = len([e for e in events if e.get('importance') == '低'])

        log(f"事件统计: 总数={event_count}, 高={high}, 中={medium}, 低={low}")
    else:
        log("数据获取失败", "ERROR")


def main():
    """主函数"""
    global running

    log("=" * 60)
    log("经济日历超精简版 - 启动中...")
    log("=" * 60)
    log(f"数据库路径: {DB_PATH}")
    log(f"轮询间隔: {POLL_INTERVAL} 秒")
    log(f"数据范围: 未来 {DAYS_AHEAD} 天")
    log(f"详细日志: {'开启' if VERBOSE else '关闭'}")
    log("=" * 60)

    # 网络检测
    setup_proxy()

    # 初始化数据库
    log("初始化数据库...")
    init_database()
    log("数据库已就绪")

    # 首次更新
    log("执行首次数据更新...")
    update_data()

    # 主循环
    log(f"进入轮询循环 (间隔: {POLL_INTERVAL}秒)")
    log("按 Ctrl+C 停止程序")
    log("=" * 60)

    next_update_time = time.time() + POLL_INTERVAL

    while running:
        try:
            now = time.time()
            remaining = int(next_update_time - now)

            if now >= next_update_time:
                update_data()
                next_update_time = now + POLL_INTERVAL
            else:
                # 等待时显示倒计时（每30秒显示一次）
                if remaining % 30 == 0 and remaining > 0:
                    verbose_log(f"下次更新: {remaining} 秒后")

            time.sleep(1)

        except KeyboardInterrupt:
            break
        except Exception as e:
            log(f"发生错误: {e}", "ERROR")
            time.sleep(10)  # 错误后等待10秒再继续

    log("=" * 60)
    log(f"程序已停止 (总更新次数: {update_count}, 数据库写入次数: {db_write_count})")
    log("=" * 60)


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        log("程序被用户中断", "INFO")
        sys.exit(0)
    except Exception as e:
        log(f"致命错误: {e}", "ERROR")
        import traceback
        traceback.print_exc()
        sys.exit(1)
