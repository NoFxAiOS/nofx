############################################################
# 📘 文件说明：
# 本文件是 JT Bot 系统的统一配置模块，提供项目级的配置管理
# 和环境变量加载功能。通过数据类（dataclass）组织配置，
# 支持多层次配置结构和灵活的环境变量覆盖。
#
# 核心功能：
# - 统一的环境变量加载与类型转换
# - 路径管理（数据目录、日志目录）
# - 功能开关配置（FeatureFlags）
# - Telegram、AI 等模块配置
# - 消息过滤规则（黑名单/白名单/正则表达式）
# - AI Prompt 模板管理
#
# 📋 程序整体伪代码（中文）：
# 1. 定义环境变量辅助函数（_env_bool, _env_int）
# 2. 定义路径配置类（Paths）
#    2.1. 初始化项目根目录
#    2.2. 创建数据和日志目录
# 3. 定义功能开关类（FeatureFlags）
# 4. 定义各模块配置类：
#    4.1. TelegramSettings（Bot Token、频道ID等）
#    4.2. AISettings（AI 配置与 Prompt）
#    4.3. AISettings（Gemini API 配置）
#    4.4. MonitorConfig（网页监控配置）
#    4.5. FilterConfig（消息过滤规则）
# 5. 定义主配置类（Config）
#    5.1. 加载网站配置（金十数据、新浪财经等）
#    5.2. 加载过滤规则（filter_patterns, blacklist_patterns）
#    5.3. 提供配置查询方法
# 6. 实例化全局配置对象（config, PATHS, AI等）
#
# 🔄 程序流程图（逻辑流）：
# ┌──────────────────┐
# │  导入环境变量     │
# │  (os.getenv)     │
# └────────┬─────────┘
#          ↓
# ┌──────────────────┐
# │  类型转换         │
# │  _env_bool/_int  │
# └────────┬─────────┘
#          ↓
# ┌──────────────────┐
# │ 初始化 Paths      │
# │ 创建目录结构      │
# └────────┬─────────┘
#          ↓
# ┌──────────────────┐
# │ 加载功能开关      │
# │ FeatureFlags     │
# └────────┬─────────┘
#          ↓
# ┌──────────────────┐
# │ 加载模块配置      │
# │ Telegram         │
# └────────┬─────────┘
#          ↓
# ┌──────────────────┐
# │ 实例化 Config     │
# │ 加载过滤规则      │
# └────────┬─────────┘
#          ↓
# ┌──────────────────┐
# │ 导出全局配置对象  │
# └──────────────────┘
#
# 📊 数据管道说明：
# 数据流向：
# 环境变量(.env) → [类型转换] → [配置类] → [全局对象] →
# → 其他模块（jt_bot.py）
#
# 输入源：
# - 环境变量文件：.env, config/.env
# - 系统环境变量
#
# 处理流程：
# 1. 环境变量读取：os.getenv()
# 2. 类型转换：字符串 → bool/int/list
# 3. 数据类初始化：@dataclass 自动构造
# 4. 验证与默认值：__post_init__
# 5. 导出全局对象：供其他模块导入
#
# 输出目标：
# - 全局配置对象：config, PATHS, AI
# - 配置查询接口：get_enabled_websites(), get_website_by_name()
#
# 🧩 文件结构：
# - 模块1：环境变量工具函数
#   ├── _env_bool (字符串 → bool)
#   └── _env_int (字符串 → int)
#
# - 模块2：路径配置
#   └── Paths (dataclass)
#       ├── root, data_dir, logs_dir
#       ├── 各种数据文件路径
#       └── __post_init__ (自动创建目录)
#
# - 模块3：功能开关
#   └── FeatureFlags (dataclass)
#       ├── enable_web_monitor
#       ├── enable_ai_processor
#       └── enable_telegram_bot
#
# - 模块4：网站配置
#   └── WebsiteConfig (dataclass)
#       ├── name, url, selectors
#       └── enabled, check_interval
#
# - 模块5：模块配置类
#   ├── MonitorConfig (网页监控配置)
#   ├── FilterConfig (消息过滤配置)
#   ├── OutputConfig (输出格式配置)
#   └── AISettings    (AI 配置与 Prompt)
#
# - 模块6：主配置类
#   └── Config (统一配置容器)
#       ├── websites (网站列表)
#       ├── filter_patterns (过滤正则)
#       ├── blacklist_patterns (黑名单)
#       └── 配置查询方法
#
# - 模块7：全局配置对象
#   ├── PATHS
#   ├── FEATURE_FLAGS
#   ├── AI
#   └── config
#
# 🕒 创建时间：2024-09
############################################################

from __future__ import annotations

import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List


def _env_bool(name: str, default: bool) -> bool:
    return os.getenv(name, str(default).lower()).lower() in {"1", "true", "yes", "on"}


def _env_int(name: str, default: int) -> int:
    value = os.getenv(name)
    if value is None:
        return default
    try:
        return int(value)
    except ValueError:
        return default


@dataclass
class Paths:
    """Filesystem layout used across the project."""

    root: Path = field(default_factory=lambda: Path(__file__).resolve().parent)
    data_dir: Path = field(init=False)
    logs_dir: Path = field(init=False)
    raw_news_file: Path = field(init=False)
    ai_news_file: Path = field(init=False)
    processed_ids_file: Path = field(init=False)
    sent_messages_file: Path = field(init=False)
    monitor_log: Path = field(init=False)
    ai_processor_log: Path = field(init=False)
    telegram_log: Path = field(init=False)

    def __post_init__(self) -> None:
        self.data_dir = self.root / "data"
        self.logs_dir = self.root / "logs"
        self.raw_news_file = self.data_dir / "financial_news.json"
        self.ai_news_file = self.data_dir / "financial_news_ai.json"
        self.processed_ids_file = self.data_dir / "processed_news_ids.json"
        self.sent_messages_file = self.data_dir / "sent_telegram_messages.json"
        self.monitor_log = self.logs_dir / "monitor.log"
        self.ai_processor_log = self.logs_dir / "ai_processor.log"
        self.telegram_log = self.logs_dir / "telegram_bot.log"

        self.data_dir.mkdir(parents=True, exist_ok=True)
        self.logs_dir.mkdir(parents=True, exist_ok=True)


@dataclass
class FeatureFlags:
    enable_web_monitor: bool = _env_bool("ENABLE_WEB_MONITOR", True)
    enable_ai_processor: bool = _env_bool("ENABLE_AI_PROCESSOR", False)
    enable_telegram_bot: bool = _env_bool("ENABLE_TELEGRAM_BOT", True)


@dataclass
class WebsiteConfig:
    name: str
    url: str
    selectors: List[str]
    enabled: bool = True
    check_interval: int = 2


@dataclass
class MonitorConfig:
    page_load_timeout: int = 30
    headless: bool = True
    auto_refresh: bool = True
    refresh_interval: int = 300


@dataclass
class FilterConfig:
    enable_filter: bool = True
    enable_blacklist: bool = True
    enable_whitelist: bool = False
    min_content_length: int = 10
    max_content_length: int = 500
    remove_duplicates: bool = True


@dataclass
class OutputConfig:
    show_timestamp: bool = True
    timestamp_format: str = "%H:%M:%S"
    show_source: bool = True
    save_to_file: bool = False
    log_file_path: Path = Path("financial_news.log")
    max_log_size: int = 10


@dataclass
class AISettings:
    enabled: bool = _env_bool("ENABLE_AI_PROCESSOR", False)
    api_key: str = os.getenv("GEMINI_API_KEY", "AIzaSyA27reYRj0LnIZwSPOWhpO8wrSS6HVyzmo")
    rate_limit_delay: int = _env_int("GEMINI_RATE_LIMIT_DELAY", 1)
    file_watch_cooldown: int = _env_int("FINANCIAL_FILE_WATCH_COOLDOWN", 2)

    @property
    def prompt_template(self) -> str:
        return PROMPT_TEMPLATE


class Config:
    """Application-wide configuration container."""

    def __init__(self, paths: Paths):
        JIN10_ENABLED = os.getenv("JIN10_ENABLED", "y").lower()
        SINA_ENABLED = os.getenv("SINA_ENABLED", "y").lower()
        WALLSTREETCN_ENABLED = os.getenv("WALLSTREETCN_ENABLED", "n").lower()
        MARKETWATCH_ENABLED = os.getenv("MARKETWATCH_ENABLED", "n").lower()
        CNBC_ENABLED = os.getenv("CNBC_ENABLED", "n").lower()

        self.paths = paths

        self.websites = [
            WebsiteConfig(
                name="金十数据",
                url="https://www.jin10.com/",
                selectors=[
                    ".jin-flash-item",
                    ".flash-item",
                    "[data-flash-id]",
                    ".news-item",
                    ".live-item",
                ],
                enabled=(JIN10_ENABLED == "y"),
                check_interval=_env_int("JIN10_CHECK_INTERVAL", 2),
            ),
            WebsiteConfig(
                name="新浪财经7x24",
                url="https://finance.sina.com.cn/7x24/?tag=0",
                selectors=[
                    "p",
                    "[data-id]",
                    "[data-time]",
                    "[class*='d_list']",
                    "[class*='list']",
                    "li",
                    ".content",
                    "[class*='item']",
                    "article",
                ],
                enabled=(SINA_ENABLED == "y"),
                check_interval=_env_int("SINA_CHECK_INTERVAL", 10),
            ),
            WebsiteConfig(
                name="华尔街见闻",
                url="https://wallstreetcn.com/live/global",
                selectors=[
                    ".live-item",
                    ".live-content",
                    "[class*='content']",
                    "[class*='text']",
                    "[class*='body']",
                    ".article-content",
                    ".news-content",
                    "[data-content]",
                    "p",
                    "article",
                    "[class*='item']",
                    "[class*='list']",
                    ".message",
                    ".detail",
                ],
                enabled=(WALLSTREETCN_ENABLED == "y"),
                check_interval=_env_int("WALLSTREETCN_CHECK_INTERVAL", 3),
            ),
            WebsiteConfig(
                name="MarketWatch",
                url="https://www.marketwatch.com/latest-news",
                selectors=[
                    ".article__content",
                    ".article__headline",
                    ".article__summary",
                    ".barron-news-item",
                    ".element--article",
                    ".headline",
                    ".summary",
                    "[class*='article']",
                    "[class*='headline']",
                    "[class*='news']",
                    "[class*='story']",
                    "h3.article__headline",
                    "p.article__summary",
                    ".latest-news__headline",
                    ".latest-news__item",
                ],
                enabled=(MARKETWATCH_ENABLED == "y"),
                check_interval=_env_int("MARKETWATCH_CHECK_INTERVAL", 5),
            ),
            WebsiteConfig(
                name="CNBC",
                url="https://www.cnbc.com/markets/",
                selectors=[
                    ".InlineVideo-container",
                    ".RiverHeadline-headline",
                    ".Card-title",
                    ".LatestNews-headline",
                    ".MarketsBanner-text",
                    ".QuickLinks-story",
                    "[class*='headline']",
                    "[class*='title']",
                    "[class*='story']",
                    "[class*='news']",
                    "[data-module='stories']",
                    "div.story-headline",
                    "h2.LatestNews-headline",
                    "h3.InlineVideo-title",
                    ".SecondaryCard-headline",
                ],
                enabled=(CNBC_ENABLED == "y"),
                check_interval=_env_int("CNBC_CHECK_INTERVAL", 5),
            ),
        ]

        self.monitor = MonitorConfig(
            page_load_timeout=_env_int("PAGE_TIMEOUT", 30),
            headless=_env_bool("HEADLESS", True),
            auto_refresh=_env_bool("AUTO_REFRESH", True),
            refresh_interval=_env_int("REFRESH_INTERVAL", 300),
        )

        self.filter = FilterConfig(
            enable_filter=_env_bool("ENABLE_FILTER", True),
            enable_blacklist=_env_bool("ENABLE_BLACKLIST", True),
            enable_whitelist=_env_bool("ENABLE_WHITELIST", False),
            min_content_length=_env_int("MIN_LENGTH", 10),
            max_content_length=_env_int("MAX_LENGTH", 500),
            remove_duplicates=_env_bool("REMOVE_DUPLICATES", True),
        )

        self.output = OutputConfig(
            show_timestamp=_env_bool("SHOW_TIMESTAMP", True),
            timestamp_format=os.getenv("TIMESTAMP_FORMAT", "%H:%M:%S"),
            show_source=_env_bool("SHOW_SOURCE", True),
            save_to_file=_env_bool("SAVE_TO_FILE", False),
            log_file_path=paths.logs_dir / os.getenv("LOG_FILE_NAME", "financial_news.log"),
            max_log_size=_env_int("MAX_LOG_SIZE", 10),
        )

        self.filter_patterns: List[str] = [
            r"\【.*?\】",
            r"\(来源:.*?\)",
            r"\s+",
            r"^交易时钟",
            r"北京时间\d{2}:\d{2}",
            r"周[一二三四五六日]\（.*?\）",
            r"^\d{2}:\d{2}",  # 只删除行首的时间戳
            r"^:\d{2}\s+",    # 删除金十数据特有的 ":18 " 格式
        ]

        self.blacklist_patterns: List[str] = [
            "查看更多$",
            "解锁VIP快讯",
            "热点头条.*查看更多$",
            r":\d{2}\s+热点头条.*查看更多$",
            "协议.*隐私政策.*白皮书.*官方验证.*Cookie.*博客",
            "API.*ZH.*亮色.*安装客户端.*登录.*注册",
            "搜索.*SSI.*Mag7.*Meme.*ETF.*币种.*指数.*图表.*研报",
            "筛选.*仅展示重要.*选择代币.*搜索关键词",
            "全部.*1年.*90天.*30天.*7天.*24小时",
            "推荐.*新闻.*机构.*推文.*研报.*链上.*查看",
            "Twitter.*Substack.*Mirror.*Medium.*Crypto Media.*SoSo Original",
            "全部.*ETF.*Fundraising.*DeFi.*Macro.*Bitcoin.*Layer1.*Layer2.*NFT.*GameFi.*SocialFi",
            "TokenBar.*®",
            "Share to$",
            "10s.*洞悉市场.*AI.*帮我总结",
            "ForesightNews",
            "SoSo Price Bot",
            "SSI Price Bot",
            r"^#[A-Za-z]+\s*\$[A-Z]+",
            r"^\$[A-Z]+\s+Share to$",
            r"^[A-Z]+:\s*\$[\d,.]+ [-+]?\d+\.\d+%$",
            r"总市值:\$[\d,.]+(B|M)[-+]?\d+\.\d+%",
            r"^ssi(MAG7|Meme):\s*\$[\d,.]+ [-+]?\d+\.\d+%$",
            r"^\$[\d,.]+ [-+]?\d+\.\d+%$",
            r"^:\s*\$[\d,.]+ [-+]?\d+\.\d+%$",
            r"^:\s*\$0\.0…\d+ [-+]?\d+\.\d+%$",
            r"SoSo每日播客.*重复.*",
            r"^[A-Z]{2,5}:\s*\$[\d,.]+",
            r"跌幅异动.*现价.*\$",
            r"^\d+$",
            r"^[A-Za-z\s]+\d+$",
            r".*协议.*隐私政策.*白皮书.*官方验证.*Cookie.*博客.*$",
            r".*跌幅异动.*现价.*\$.*协议.*隐私政策.*$",
            r"Qubic.*计划掌控.*Monero.*51%.*算力.*协议.*隐私政策.*$",
        ]

        self.whitelist_patterns: List[str] = []
        self.replacement_rules: Dict[str, str] = {}
        self.keyword_tags: Dict[str, List[str]] = {}
        self.priority_keywords: List[str] = []
        self.time_filter: Dict[str, object] = {
            "enable": False,
            "allowed_hours": list(range(9, 18)),
            "blocked_hours": [],
            "weekend_filter": False,
        }

    def get_enabled_websites(self) -> List[WebsiteConfig]:
        return [site for site in self.websites if site.enabled]

    def get_website_by_name(self, name: str) -> WebsiteConfig | None:
        for site in self.websites:
            if site.name == name:
                return site
        return None


PROMPT_TEMPLATE = """你是一名资深的信息编辑，擅长用简洁精准的语言提炼复杂信息的核心内容。请阅读下方提供的信息，将其加工为一个结构清晰、语言流畅、内容准确的摘要段落，全面呈现关键信息、核心结果或主要影响，保持客观中立，不添加任何原文未提及的主观判断或外部内容。随后，请基于原始信息提炼出关键词，直接以"#关键词1 #关键词2 #关键词3 ... #关键词n"的格式紧跟在摘要段落末尾输出，且摘要与关键词必须在同一行内、不换行呈现，用于概括核心主题或重点要素。

请严格按照本提示中的格式要求进行输出，不得更改段落结构、关键词格式或输出顺序。所有输出必须为纯文本格式，不得包含任何 Markdown 标记、换行符、额外符号或格式化元素。

领域判断: 如果新闻内容与加密货币有直接或间接关系，请在所有关键词的最后面加上 #加密货币相关 标签。

如果新闻属于以下任何一类，请在所有关键词的最前面加上 #重要 标签：
1. 宏观决策与政策：主要经济体的央行关键决策（如利率变动）、重大财政或贸易政策变动、影响深远的司法判决或监管行动。
2. 地缘政治与安全：高级别政治声明、国家间军事冲突、重大恐怖袭击、地缘政治格局的显著变化。
3. 系统性风险与危机：可能引发连锁反应的金融市场事件（如大型机构倒闭、流动性危机）、影响全球或大范围区域的自然灾害或公共卫生危机。

特定人物标签：如果新闻内容提及唐纳德·特朗普(Donald Trump)或埃隆·马斯克(Elon Musk)，关键词中必须分别包含 #特朗普 或 #马斯克。

示例：

原始信息：
:21 美国总统特朗普重申，伊朗的所有三个核设施都已被完全摧毁。
处理结果：
美国总统特朗普重申，伊朗境内的三座核设施已被彻底摧毁，进一步加剧了中东地区的紧张局势。#特朗普 #伊朗 #核设施 #中东 #重要

原始信息：
:04 美国总统特朗普：我已要求司法部公布大陪审团有关杰弗里·爱泼斯坦的所有证词，但必须获得法院批准。话虽如此，即使法院给予充分而坚定的批准，对于提出这一要求的麻烦制造者和激进左翼疯子来说，什么都是不够的，他们会要求更多。
处理结果：
特朗普表示已要求司法部公开与杰弗里·爱泼斯坦相关的大陪审团证词，但需经法院批准，并批评反对者永不满足。#特朗普 #爱泼斯坦 #司法部 #大陪审团

请你处理：{news_content}"""


PATHS = Paths()
FEATURE_FLAGS = FeatureFlags()
AI = AISettings()
config = Config(PATHS)
