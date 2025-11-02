#!/usr/bin/env python3
# -*- coding: utf-8 -*-

############################################################
# 📘 文件说明：
# 本文件是 JT Bot 的核心主程序，实现了 Telegram 消息监听、过滤、
# 处理和转发的完整功能。将原项目的配置、监听器、消息处理器、
# 认证工具以及辅助脚本统一到单个 Python 文件中，方便在不同环境
# 下快速部署和调用。
#
# 核心功能：
# - Telegram 频道消息实时监听
# - 消息内容过滤与格式化（支持黑名单/白名单）
# - 消息转发到 Bot API 或直接发送
# - 数据持久化到 SQLite 数据库
# - CoinGlass 警报专用格式化
# - 交互式认证终端
#
# 📋 程序整体伪代码（中文）：
# 1. 加载环境变量与配置（.env文件、命令行参数）
# 2. 初始化 Telegram 客户端（Telethon）和消息处理器
# 3. 建立数据库连接（SQLite）用于消息持久化
# 4. 注册频道消息监听事件处理器
# 5. 进入事件循环：
#    5.1. 接收新消息事件
#    5.2. 过滤消息（黑名单、白名单、内容规则）
#    5.3. 格式化消息（清理、时间戳、Markdown 转换）
#    5.4. 持久化到数据库（保存原始消息）
#    5.5. 转发到目标 Bot/Chat（可选）
# 6. 异常处理与日志记录
# 7. 优雅关闭（断开连接、释放资源）
#
# 🔄 程序流程图（逻辑流）：
# ┌──────────────────┐
# │  加载环境配置      │
# │  (.env / 命令行)  │
# └────────┬─────────┘
#          ↓
# ┌──────────────────┐
# │  初始化组件        │
# │ - TelegramClient │
# │ - MessageProcessor│
# │ - SQLite Database│
# └────────┬─────────┘
#          ↓
# ┌──────────────────┐
# │ 注册事件监听器     │
# │ (NewMessage)     │
# └────────┬─────────┘
#          ↓
# ┌──────────────────────────┐
# │      事件循环开始          │
# └────────┬─────────────────┘
#          ↓
# ┌──────────────────┐
# │  接收频道新消息    │
# └────────┬─────────┘
#          ↓
# ┌──────────────────┐
# │   内容过滤        │
# │ (黑名单/白名单)   │
# └────┬────┬────────┘
#      ↓ 拒绝 ↓ 通过
#   丢弃     ↓
#    ┌───────────────┐
#    │   格式化处理   │
#    │ - 清理特殊字符 │
#    │ - 添加时间戳   │
#    │ - CoinGlass转换│
#    └───────┬───────┘
#            ↓
#    ┌───────────────┐
#    │ 保存到数据库   │
#    │ (messages表)  │
#    └───────┬───────┘
#            ↓
#    ┌───────────────┐
#    │   转发消息     │
#    │ (Bot API可选) │
#    └───────────────┘
#
# 📊 数据管道说明：
# 数据流向：
# Telegram频道消息 → [监听器] → [过滤器] → [格式化器] → [SQLite数据库]
#
# 输入源：
# - Telegram 订阅频道（通过环境变量配置）
# - 频道白名单/黑名单配置
#
# 处理流程：
# 1. 消息接收：Telethon NewMessage 事件
# 2. 内容过滤：正则表达式黑/白名单
# 3. 格式转换：Markdown → 纯文本，链接提取
# 4. 数据持久化：SQLite (jtbot.db)
# 5. 下游消费：其他服务可从数据库读取消息
#
# 输出目标：
# - SQLite 数据库文件 (./jtbot.db)
# - Bot API 发送 (可选)
# - 日志文件 (./logs/*.log)
#
# 🧩 文件结构：
# - 模块1：环境变量与配置加载
#   ├── _get_env, _get_env_int, _get_env_bool, _get_env_list
#   ├── TelegramConfig, BotConfig, ProxyConfig
#   └── Config (统一配置容器)
#
# - 模块2：消息处理器
#   └── SimpleMessageProcessor
#       ├── 内容过滤 (filter_patterns, blacklist_patterns)
#       ├── 格式化 (_format_message_with_timestamp)
#       ├── CoinGlass 警报处理 (_normalize_coinglass_alert)
#       └── Bot API 发送 (send_message)
#
# - 模块3：Telegram 监听器
#   └── SimpleTelegramMonitor
#       ├── 事件注册 (handle_new_message)
#       ├── 数据库持久化 (_save_message_to_db)
#       └── 连接管理 (connect, disconnect)
#
# - 模块4：认证工具
#   ├── authenticate_telegram (交互式认证)
#   └── list_my_channels (频道列表工具)
#
# - 模块5：数据库管理
#   └── SQLite 初始化与消息存储
#
# - 模块6：命令行入口
#   ├── main (主函数)
#   ├── parse_args (参数解析)
#   └── dispatch_command (命令分发)
#
# 🕒 创建时间：2024-09
############################################################

from __future__ import annotations

import argparse
import asyncio
import json
import logging
import os
import re
import sqlite3
import sys
import time
from dataclasses import dataclass
from datetime import datetime, timezone, timedelta
from functools import lru_cache
from pathlib import Path
from typing import Any, Callable, Dict, Iterable, List, Optional, Set, Tuple

from colorama import Fore, Style, init
from telethon import TelegramClient, events
from telethon.errors import (
    AuthKeyDuplicatedError,
    PhoneCodeExpiredError,
    PhoneCodeInvalidError,
    SessionPasswordNeededError,
)
from telethon.sessions import StringSession
from telethon.tl.types import PeerChannel, PeerChat, PeerUser

try:  # 可选依赖，缺失时工具功能会自动降级
    from dotenv import load_dotenv
except ImportError:  # pragma: no cover
    load_dotenv = None

# 初始化终端颜色
init(autoreset=True)

# 日志配置，与原 main.py 保持一致的输出样式
logging.basicConfig(
    level=logging.DEBUG,
    format=f"{Fore.CYAN}%(asctime)s{Style.RESET_ALL} │ {Fore.GREEN}%(levelname)s{Style.RESET_ALL} │ {Fore.WHITE}%(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
    handlers=[logging.StreamHandler(sys.stdout)],
)

LOGGER = logging.getLogger("jt_bot.monitor")
PROCESSOR_LOGGER = logging.getLogger("jt_bot.processor")
CLIENT_LOGGER = logging.getLogger("jt_bot.client")

PROJECT_ROOT = Path(__file__).resolve().parent


class UnauthorizedSessionError(Exception):
    """Raised when a valid Telegram session is not available in non-interactive mode."""

# 加载 .env（若存在），保持工具脚本兼容性
if load_dotenv:
    try:
        load_dotenv(dotenv_path=PROJECT_ROOT / ".env", override=False)
        load_dotenv(dotenv_path=PROJECT_ROOT / "config" / ".env", override=True)
    except Exception:  # pragma: no cover - 忽略缺失
        pass


def _get_env(name: str, default: Optional[str] = None, *, required: bool = False) -> Optional[str]:
    value = os.getenv(name)
    if value is None or not str(value).strip():
        if required and default is None:
            raise ValueError(f"环境变量 {name} 未设置且没有默认值")
        return default
    return value


def _get_env_int(name: str, default: Optional[int] = None, *, required: bool = False) -> Optional[int]:
    raw = _get_env(name, default=None, required=required)
    if raw is None:
        return default
    try:
        return int(str(raw).strip())
    except (TypeError, ValueError) as exc:  # pragma: no cover - 配置错误
        raise ValueError(f"环境变量 {name} 的值无效: {raw}") from exc


def _get_env_bool(name: str, default: bool) -> bool:
    raw = _get_env(name)
    if raw is None:
        return default
    return str(raw).strip().lower() in {"1", "true", "yes", "on", "y"}


def _get_env_list(
    name: str,
    *,
    cast: Optional[Callable[[str], Any]] = None,
) -> List:
    raw = _get_env(name)
    if raw is None:
        return []

    values: List = []
    for part in raw.split(","):
        item = part.strip()
        if not item:
            continue
        if cast is not None:
            try:
                values.append(cast(item))
            except Exception:
                continue
        else:
            values.append(item)
    return values


def _expand_identifier(value: Optional[object]) -> Set[str]:
    """Normalize an identifier string for sender/channel matching."""

    identifiers: Set[str] = set()
    if value is None:
        return identifiers

    raw = str(value).strip()
    if not raw:
        return identifiers

    lowered = raw.lower()
    identifiers.add(lowered)

    if lowered.startswith("@"):
        stripped = lowered[1:]
        if stripped:
            identifiers.add(stripped)
        numeric_candidate = stripped
    else:
        numeric_candidate = lowered

    if numeric_candidate:
        if numeric_candidate.startswith("-100") and numeric_candidate[4:].isdigit():
            identifiers.add(numeric_candidate[4:])
        if (numeric_candidate.startswith("-") and numeric_candidate[1:].isdigit()) or numeric_candidate.isdigit():
            identifiers.add(numeric_candidate)

    return {item for item in identifiers if item}


def _collect_message_identifiers(
    sender_username: Optional[str],
    sender_id: Optional[int],
    channel_username: Optional[str],
    channel_id: Optional[str],
) -> Set[str]:
    """Collect normalized identifiers for the current message."""

    identifiers: Set[str] = set()
    for candidate in (
        sender_username,
        sender_id,
        channel_username,
        channel_id,
    ):
        identifiers.update(_expand_identifier(candidate))

    # 对频道 ID 额外处理，兼容 Telethon -100 前缀
    if channel_id and channel_id.startswith("-100") and channel_id[4:].isdigit():
        identifiers.update(_expand_identifier(channel_id[4:]))

    return identifiers


@dataclass
class TelegramConfig:
    """Telegram API 配置"""

    api_id: int
    api_hash: str
    phone_number: str
    password: str
    session_name: str


@dataclass
class ProxyConfig:
    """网络代理配置"""

    type: str = ""
    host: str = ""
    port: int = 0
    username: str = ""
    password: str = ""


@dataclass
class PerformanceConfig:
    """性能配置"""

    max_message_length: int = 4000
    batch_size: int = 8
    timeout: int = 15
    retry_count: int = 3
    queue_size: int = 500
    cache_cleanup_interval: int = 3600


@dataclass
class AuthConfig:
    """认证配置"""

    force_reauth: bool = False
    auto_reset_on_duplicate: bool = True


class Config:
    """主配置类"""

    def __init__(self) -> None:
        self.project_root = PROJECT_ROOT
        self.data_dir = self.project_root / "data"
        self.sessions_dir = self.data_dir / "sessions"
        self.logs_dir = self.project_root / "logs"
        self.config_dir = self.project_root / "config"

        self.sessions_dir.mkdir(parents=True, exist_ok=True)
        self.logs_dir.mkdir(parents=True, exist_ok=True)
        self.config_dir.mkdir(parents=True, exist_ok=True)
        self.data_dir.mkdir(parents=True, exist_ok=True)

        telegram_api_id = _get_env_int("TELEGRAM_API_ID", required=True)
        if telegram_api_id is None:
            raise ValueError("TELEGRAM_API_ID 配置无效")
        telegram_api_hash = _get_env("TELEGRAM_API_HASH", required=True)
        telegram_phone = _get_env("TELEGRAM_PHONE_NUMBER", required=True)
        telegram_password = _get_env("TELEGRAM_PASSWORD", default="") or ""
        telegram_session = _get_env(
            "TELEGRAM_SESSION_NAME",
            default="telegram_monitor_optimized",
        ) or "telegram_monitor_optimized"

        self.telegram = TelegramConfig(
            api_id=telegram_api_id,
            api_hash=telegram_api_hash,
            phone_number=telegram_phone,
            password=telegram_password,
            session_name=telegram_session,
        )

        self.performance = PerformanceConfig(
            max_message_length=4000,
            batch_size=8,
            timeout=15,
            retry_count=3,
            queue_size=500,
            cache_cleanup_interval=3600,
        )

        self.proxy = ProxyConfig(
            type=_get_env("PROXY_TYPE", default="http") or "",
            host=_get_env("PROXY_HOST", default="127.0.0.1") or "",
            port=_get_env_int("PROXY_PORT", default=9910) or 0,
            username=_get_env("PROXY_USERNAME", default="") or "",
            password=_get_env("PROXY_PASSWORD", default="") or "",
        )

        self.auth = AuthConfig()

        self.no_translation_channels: List[str] = [
            
        ]

        env_allowed_channels = _get_env_list("CHANNEL_ALLOWLIST")
        if env_allowed_channels:
            normalized_channels: List[str] = []
            for entry in env_allowed_channels:
                normalized_entry = str(entry).strip()
                if normalized_entry.startswith("@"):
                    normalized_entry = normalized_entry[1:]
                normalized_channels.append(normalized_entry)
            if normalized_channels:
                self.no_translation_channels = normalized_channels

        self.listen_all_subscribed_channels = _get_env_bool(
            "LISTEN_ALL_SUBSCRIBED_CHANNELS",
            True,
        )
        self.listen_archived_only = _get_env_bool(
            "LISTEN_ARCHIVED_ONLY",
            False,
        )
        self.archived_refresh_interval = _get_env_int(
            "ARCHIVED_REFRESH_INTERVAL",
            300,
        )
        self.block_private_messages = _get_env_bool(
            "BLOCK_PRIVATE_MESSAGES",
            False,
        )
        blocked_sender_ids_env = _get_env("BLOCKED_SENDER_IDS")
        if blocked_sender_ids_env:
            parsed_ids: List[int] = []
            for item in blocked_sender_ids_env.split(","):
                item = item.strip()
                if not item:
                    continue
                try:
                    parsed_ids.append(int(item))
                except ValueError:
                    LOGGER.warning("忽略无效的 BLOCKED_SENDER_IDS 项: %s", item)
            self.blocked_sender_ids = parsed_ids
        else:
            self.blocked_sender_ids = [777000]
        self.channel_mapping: Dict[str, str] = {}
        self.channel_sender_whitelist: Dict[str, Dict[str, List]] = {}
        self.enable_sender_whitelist = _get_env_bool(
            "ENABLE_SENDER_WHITELIST",
            False,
        )
        if self.enable_sender_whitelist:
            default_ids = [8174663699]
            default_usernames = ["Givin9505"]
            whitelist_ids = _get_env_list("GLOBAL_WHITELIST_IDS", cast=int) or default_ids
            whitelist_usernames = _get_env_list("GLOBAL_WHITELIST_USERNAMES") or default_usernames
            normalized_usernames = [name.lstrip("@") for name in whitelist_usernames]
            self.global_sender_whitelist: Dict[str, List] = {
                "ids": whitelist_ids,
                "usernames": normalized_usernames,
            }
        else:
            self.global_sender_whitelist = {"ids": [], "usernames": []}
        self.channel_blocklist: List[str] = []

        self.filter_patterns: List[str] = [
            r'\[.*?\]\(https://t\.me/.*?\)',  # 移除Telegram链接
        ]

        self.blacklist_patterns: List[str] = [
            r'\b铝\b',
        ]

    def get_session_path(self, session_name: Optional[str] = None) -> str:
        if session_name is None:
            session_name = self.telegram.session_name
        return str(self.sessions_dir / session_name)

    def get_database_path(self, filename: str = "jtbot.db") -> Path:
        return self.data_dir / filename

    def cleanup_session_files(self, session_name: Optional[str] = None) -> List[Path]:
        if session_name is None:
            session_name = self.telegram.session_name
        base_path = Path(self.get_session_path(session_name))
        removed: List[Path] = []
        suffixes = ["", ".session", ".session-journal", ".session-shm", ".session-wal"]
        for suffix in suffixes:
            candidate = Path(f"{base_path}{suffix}")
            try:
                if candidate.exists():
                    candidate.unlink()
                    removed.append(candidate)
            except Exception as exc:
                LOGGER.warning("删除会话文件失败 %s: %s", candidate, exc)
        return removed

    def get_telethon_proxy(self):
        if not self.proxy.type or not self.proxy.host or not self.proxy.port:
            return None
        try:
            import socks  # type: ignore
        except Exception:
            return None

        proxy_type = None
        if self.proxy.type in ("socks5", "socks"):
            proxy_type = socks.SOCKS5
        elif self.proxy.type in ("http", "https"):
            proxy_type = socks.HTTP
        else:
            return None

        if self.proxy.username or self.proxy.password:
            return (
                proxy_type,
                self.proxy.host,
                int(self.proxy.port),
                True,
                self.proxy.username or None,
                self.proxy.password or None,
            )
        return (
            proxy_type,
            self.proxy.host,
            int(self.proxy.port),
        )


config = Config()


class SimpleMessageProcessor:
    """精简版消息处理器"""

    def __init__(self) -> None:
        PROCESSOR_LOGGER.debug("初始化SimpleMessageProcessor...")

        PROCESSOR_LOGGER.debug("预编译过滤正则表达式...")
        self._filter_patterns = [
            re.compile(pattern, re.MULTILINE | re.DOTALL) for pattern in config.filter_patterns
        ]
        self._blacklist_patterns = [
            re.compile(pattern, re.MULTILINE | re.DOTALL) for pattern in config.blacklist_patterns
        ]

        PROCESSOR_LOGGER.debug("预编译常用正则表达式...")
        self._emoji_patterns = [

        ]

        self._promotion_patterns = [

        ]

        self._link_pattern = re.compile(r"\[([^\]]+)\]\(([^)]+)\)")
        self._separator_patterns = [
            
        ]

        self._coinglass_header_pattern = re.compile(r"^(?:📡|📢)?\s*CoinGlass警报", re.IGNORECASE)
        self._coinglass_source_pattern = re.compile(r"^(?:📢\s*)?来源[:：]", re.IGNORECASE)
        self._coinglass_relative_time_pattern = re.compile(r"^(今天|昨日|昨天)\s*\d{1,2}:\d{2}")

        self._timestamp_pattern = re.compile(r"\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}")
        self._separator_line_pattern = re.compile(r"^[—]{10,}$")
        self._has_separator_pattern = re.compile(r"^[—\-=]{6,}$", re.MULTILINE)

        self.stats = {
            "processed": 0,
            "filtered": 0,
            "blacklisted": 0,
            "promotion_filtered": 0,
        }
        PROCESSOR_LOGGER.debug("SimpleMessageProcessor初始化完成")

    def _is_blacklisted(self, text: str) -> bool:
        if not text:
            return False
        for pattern in self._blacklist_patterns:
            if pattern.search(text):
                self.stats["blacklisted"] += 1
                PROCESSOR_LOGGER.debug(f"消息被黑名单过滤: {text[:50]}...")
                return True
        return False

    def _is_pure_promotion_message(self, text: str) -> bool:
        if not text or not text.strip():
            return False

        cleaned_text = self._remove_emojis(text).strip()

        for pattern in self._promotion_patterns:
            if pattern.match(cleaned_text):
                PROCESSOR_LOGGER.debug(f"检测到纯推广消息: {text[:50]}...")
                return True

        if len(cleaned_text) < 50:
            matches = self._link_pattern.findall(cleaned_text)
            if matches:
                total_link_text = sum(len(match[0]) for match in matches)
                link_ratio = total_link_text / len(cleaned_text)
                PROCESSOR_LOGGER.debug(
                    f"短消息链接文本占比: {link_ratio:.2f}, 文本长度: {len(cleaned_text)}"
                )
                if link_ratio > 0.7:
                    PROCESSOR_LOGGER.debug(f"检测到主要为链接的短消息: {text[:50]}...")
                    return True

        return False

    def _remove_emojis(self, text: str) -> str:
        if not text:
            return ""

        result = text
        for pattern in self._emoji_patterns:
            result = pattern.sub("", result)

        if len(result) != len(text):
            PROCESSOR_LOGGER.debug(
                f"已移除Emoji，原长度: {len(text)}, 新长度: {len(result)}"
            )

        return result

    def _apply_filter_rules(self, text: str) -> str:
        filtered = text
        for pattern in self._filter_patterns:
            filtered = pattern.sub("", filtered)
        return filtered

    def _standardize_separator_format(self, text: str) -> str:
        result = text
        for pattern, separator in self._separator_patterns:
            result = pattern.sub(separator, result)
        return result

    def _convert_markdown_links(self, text: str) -> str:
        if not text:
            return ""

        def replacer(match: re.Match) -> str:
            url = (match.group(2) or "").strip()
            return url

        return self._link_pattern.sub(replacer, text)

    def _normalize_coinglass_alert(self, lines: List[str]) -> str:
        normalized_lines: List[str] = ["📢 CoinGlass警报"]
        body_lines: List[str] = []

        for raw_line in lines[1:]:
            stripped = raw_line.strip()
            if not stripped:
                if body_lines and body_lines[-1]:
                    body_lines.append("")
                continue

            if self._coinglass_source_pattern.match(stripped):
                continue

            if self._coinglass_relative_time_pattern.match(stripped):
                continue

            if self._separator_line_pattern.match(stripped):
                continue

            if self._timestamp_pattern.match(stripped):
                continue

            body_lines.append(stripped)

        while body_lines and not body_lines[0]:
            body_lines.pop(0)

        while body_lines and not body_lines[-1]:
            body_lines.pop()

        if body_lines:
            normalized_lines.append("")
            normalized_lines.extend(body_lines)

        return "\n".join(normalized_lines)

    def _format_message_with_timestamp(self, text: str) -> str:
        if text is None:
            return ""

        cleaned_text = text.strip()
        if not cleaned_text:
            return ""

        lines = [line.rstrip() for line in cleaned_text.splitlines()]

        if lines and self._coinglass_header_pattern.match(lines[0]):
            return self._normalize_coinglass_alert(lines)

        if self._has_separator_pattern.search(cleaned_text):
            return cleaned_text

        timestamp = datetime.now(timezone.utc).astimezone().strftime("%Y-%m-%d %H:%M:%S")
        separator = "——————————"
        return f"{cleaned_text}\n\n{separator}\n{timestamp}"

    def _format_for_telegram(self, text: str) -> str:
        return text

    async def process_message(
        self, message, channel_name: str, pre_filtered: bool = False
    ) -> Tuple[bool, str]:
        self.stats["processed"] += 1

        raw_text = getattr(message, "text", message)
        if raw_text is None:
            PROCESSOR_LOGGER.debug(f"空消息跳过 | 来源: {channel_name}")
            return False, ""

        if not isinstance(raw_text, str):
            raw_text = str(raw_text)

        if not raw_text.strip():
            PROCESSOR_LOGGER.debug(f"空消息跳过 | 来源: {channel_name}")
            return False, ""

        PROCESSOR_LOGGER.debug(
            f"开始处理消息 | 来源: {channel_name} | 字数: {len(raw_text)}"
        )

        if self._is_blacklisted(raw_text):
            PROCESSOR_LOGGER.debug(f"消息命中黑名单，已丢弃 | 来源: {channel_name}")
            return False, ""

        if self._is_pure_promotion_message(raw_text):
            self.stats["promotion_filtered"] += 1
            PROCESSOR_LOGGER.debug(f"消息识别为推广，已丢弃 | 来源: {channel_name}")
            return False, ""

        filtered_text = self._apply_filter_rules(raw_text)

        if not filtered_text.strip():
            PROCESSOR_LOGGER.debug(f"过滤后文本为空，使用原始文本: {channel_name}")
            filtered_text = raw_text
            self.stats["filtered"] += 1

        PROCESSOR_LOGGER.debug(f"开始格式化阶段 | 来源: {channel_name}")

        formatted_text = self._standardize_separator_format(filtered_text)
        formatted_text = self._convert_markdown_links(formatted_text)
        formatted_text = self._format_message_with_timestamp(formatted_text)
        final_text = self._format_for_telegram(formatted_text)

        PROCESSOR_LOGGER.debug(
            f"消息处理完成 | 来源: {channel_name} | 最终长度: {len(final_text)} 字符"
        )
        return True, final_text

    def get_stats(self) -> Dict[str, Dict[str, int]]:
        PROCESSOR_LOGGER.debug(
            "返回处理器统计信息: 处理 %s，过滤 %s，黑名单 %s，推广 %s",
            self.stats["processed"],
            self.stats["filtered"],
            self.stats["blacklisted"],
            self.stats["promotion_filtered"],
        )
        return {"processor": self.stats}

    @lru_cache(maxsize=64)
    def _remove_html_and_links(self, text: str) -> str:
        if not text:
            return ""
        text = re.sub(r"<[^>]+>", "", text)
        return self._link_pattern.sub(r"\1", text)


class SimpleTelegramMonitor:
    """Telegram 监听器"""

    def __init__(self, *, interactive: bool = True) -> None:
        self.client: Optional[TelegramClient] = None
        self.interactive = interactive
        self.processor = SimpleMessageProcessor()
        self.running = False
        self.stats = {"received": 0, "start_time": time.time()}
        default_db = str(config.get_database_path())
        self.db_path = os.getenv("DATABASE_PATH", default_db)
        self._init_database()

        # 归档频道动态跟踪
        self.archived_channel_ids: set[int] = set()  # 当前归档的频道ID集合
        self.last_archived_refresh = 0.0  # 上次刷新归档列表的时间戳
        self.archived_refresh_task: Optional[asyncio.Task] = None  # 定时刷新任务

    async def init_client(self) -> bool:
        duplicate_recovered = False
        while True:
            try:
                return await self._init_client_once()
            except UnauthorizedSessionError:
                raise
            except AuthKeyDuplicatedError as exc:  # pragma: no cover - 网络依赖
                LOGGER.error("检测到 Telegram 会话密钥冲突: %s", exc)
                if duplicate_recovered or not getattr(config.auth, "auto_reset_on_duplicate", False):
                    LOGGER.error("自动修复失败，请手动运行 `python jt_bot.py auth --force` 后重试")
                    return False
                duplicate_recovered = True
                LOGGER.warning("正在清理本地会话文件并重新发起认证流程…")
                await self._reset_session_after_duplicate()
            except Exception as exc:  # pragma: no cover - 网络依赖
                LOGGER.error(f"客户端初始化失败: {exc}")
                return False

    async def _reset_session_after_duplicate(self) -> None:
        if self.client:
            try:
                await self.client.disconnect()
            except Exception as exc:
                LOGGER.debug(f"断开旧客户端失败: {exc}")
        removed_files = config.cleanup_session_files()
        if removed_files:
            for path in removed_files:
                try:
                    display_path = path.relative_to(config.project_root)
                except ValueError:
                    display_path = path
                LOGGER.info("已删除会话文件: %s", display_path)
        else:
            LOGGER.info("未找到需要清理的会话文件")
        self.client = None

    def _init_database(self) -> None:
        """初始化数据库"""
        try:
            conn = sqlite3.connect(self.db_path)
            cursor = conn.cursor()
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS news (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    title TEXT,
                    content TEXT,
                    source TEXT,
                    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )
            """)
            conn.commit()
            conn.close()
            LOGGER.debug(f"数据库初始化成功: {self.db_path}")
        except Exception as e:
            LOGGER.error(f"数据库初始化失败: {e}")

    def _save_message_to_db(self, title: str, content: str, source: str) -> None:
        """保存消息到数据库"""
        try:
            conn = sqlite3.connect(self.db_path)
            cursor = conn.cursor()
            cursor.execute(
                "INSERT INTO news (title, content, source) VALUES (?, ?, ?)",
                (title, content, source)
            )
            conn.commit()
            conn.close()
            LOGGER.debug(f"消息已保存到数据库 | 来源: {source}")
        except Exception as e:
            LOGGER.error(f"保存消息到数据库失败: {e}")

    def _resolve_sender_display(
        self,
        sender_username: Optional[str],
        sender: Optional[object],
        sender_id: Optional[int],
        channel_name: str,
    ) -> str:
        if sender_username:
            return f"@{sender_username}"

        if sender is not None:
            first_name = getattr(sender, "first_name", "") or ""
            last_name = getattr(sender, "last_name", "") or ""
            full_name = " ".join(part for part in [first_name.strip(), last_name.strip()] if part)
            if full_name:
                return full_name

        if sender_id is not None:
            return f"ID:{sender_id}"

        if channel_name:
            return channel_name

        return "未知用户"

    async def _init_client_once(self) -> bool:
        LOGGER.debug("开始初始化Telegram客户端...")

        LOGGER.info("尝试直连Telegram服务器...")
        self.client = TelegramClient(
            config.get_session_path(),
            config.telegram.api_id,
            config.telegram.api_hash,
            system_version="4.16.30-vxSIMPLE",
            device_model="Desktop",
            app_version="3.1",
            lang_code="zh",
            proxy=None,
        )

        direct_success = False
        try:
            await asyncio.wait_for(self.client.connect(), timeout=5)
            LOGGER.info("✅ 直连Telegram成功！")
            direct_success = True
        except (asyncio.TimeoutError, Exception) as exc:
            if isinstance(exc, AuthKeyDuplicatedError):
                raise
            LOGGER.debug(f"直连失败: {type(exc).__name__}: {str(exc)[:100]}")

        if not direct_success:
            proxy = config.get_telethon_proxy()
            if proxy:
                LOGGER.info("直连失败，自动切换到代理模式...")
                await self.client.disconnect()
                self.client = TelegramClient(
                    config.get_session_path(),
                    config.telegram.api_id,
                    config.telegram.api_hash,
                    system_version="4.16.30-vxSIMPLE",
                    device_model="Desktop",
                    app_version="3.1",
                    lang_code="zh",
                    proxy=proxy,
                )

                proxy_success = False
                for attempt in range(1, 4):
                    try:
                        await self.client.connect()
                        LOGGER.info(
                            "✅ 自动切换到代理连接成功: %s://%s:%s",
                            config.proxy.type,
                            config.proxy.host,
                            config.proxy.port,
                        )
                        proxy_success = True
                        break
                    except Exception as exc:
                        if isinstance(exc, AuthKeyDuplicatedError):
                            raise
                        LOGGER.warning(f"代理连接第{attempt}次失败: {exc}")
                        if attempt < 3:
                            await asyncio.sleep(2 * attempt)

                if not proxy_success:
                    raise Exception("直连和代理都无法连接到Telegram，请检查网络设置")
            else:
                LOGGER.info("未配置代理，继续尝试直连...")
                for attempt in range(2, 4):
                    try:
                        await asyncio.sleep(2)
                        await self.client.connect()
                        LOGGER.info("✅ 第%s次直连成功！", attempt)
                        direct_success = True
                        break
                    except Exception as exc:
                        if isinstance(exc, AuthKeyDuplicatedError):
                            raise
                        LOGGER.warning(f"第{attempt}次直连失败: {exc}")

                if not direct_success:
                    raise Exception("无法连接到Telegram（直连失败且未配置代理）")

        if not await self.client.is_user_authorized():
            if not self.interactive:
                LOGGER.warning(
                    "当前会话未授权，必须先完成交互式登录。请运行 `./start.sh auth` 后重试。"
                )
                try:
                    await self.client.disconnect()
                except Exception:
                    pass
                raise UnauthorizedSessionError("未找到可用的 Telegram 会话")

            LOGGER.warning("客户端当前未授权，启动交互式登录流程")
            await self._handle_authorization()

        if not await self.client.is_user_authorized():
            LOGGER.error("登录流程完成后依旧未授权，无法启动监听")
            raise Exception("需要有效的Telegram会话文件才能自动化运行")

        LOGGER.info("使用现有会话登录成功")
        return True

    async def _handle_authorization(self) -> None:
        phone = config.telegram.phone_number

        if not sys.stdin or not sys.stdin.isatty():
            raise UnauthorizedSessionError(
                "当前终端不支持交互式认证，请在可交互终端运行 `./start.sh auth` 完成登录"
            )

        for attempt in range(1, 4):
            try:
                LOGGER.debug(f"向 {phone} 发送验证码请求 (第{attempt}次)...")
                await self.client.send_code_request(phone)

                print(
                    f"\n{Fore.YELLOW}需要验证您的Telegram账号。" f"验证码已发送到 {phone}。{Style.RESET_ALL}"
                )
                code = input(f"{Fore.GREEN}请输入验证码: {Style.RESET_ALL}").strip()
                if not code:
                    LOGGER.warning("未输入验证码，取消登录尝试。")
                    continue

                try:
                    LOGGER.debug("用户输入验证码，尝试登录...")
                    await self.client.sign_in(phone, code)
                except SessionPasswordNeededError:
                    LOGGER.info("账号启用了两步验证，正在输入密码...")
                    password = config.telegram.password
                    if not password:
                        from getpass import getpass

                        password = getpass("请输入Telegram两步验证密码: ")
                    await self.client.sign_in(password=password)

                LOGGER.info("登录成功")
                return

            except PhoneCodeInvalidError:
                LOGGER.warning("验证码错误，请重新输入。")
                continue
            except PhoneCodeExpiredError:
                LOGGER.warning("验证码已过期，准备重新请求新的验证码。")
                continue
            except Exception as exc:
                LOGGER.error(f"授权失败: {exc}")
                raise

        raise Exception("多次尝试登录失败，请稍后重试或使用手动认证。")

    async def message_handler(self, event) -> None:
        try:
            self.stats["received"] += 1

            chat = await event.get_chat()
            channel_name = getattr(chat, "title", "未知频道")
            channel_username = getattr(chat, "username", None)
            channel_id = str(event.chat_id)
            message_text = event.message.text or ""
            sender: Optional[object] = None
            sender_id = None
            sender_username = None
            try:
                sender = await event.get_sender()
                sender_id = getattr(sender, "id", None)
                sender_username = getattr(sender, "username", None)
            except Exception:
                sender = None
                sender_id = None
                sender_username = None

            is_private_chat = not hasattr(chat, "broadcast") and not hasattr(chat, "megagroup")
            if is_private_chat and getattr(config, "block_private_messages", True):
                LOGGER.debug(
                    "跳过私聊消息 | 来源: %s %s",
                    getattr(chat, "first_name", "用户"),
                    getattr(chat, "last_name", ""),
                )
                return

            # 归档模式检查：如果启用了只监听归档，则检查频道是否在归档列表中
            if (
                hasattr(config, "listen_archived_only")
                and config.listen_archived_only
                and config.listen_all_subscribed_channels
            ):
                numeric_channel_id = int(channel_id) if channel_id.lstrip('-').isdigit() else None
                if numeric_channel_id and numeric_channel_id not in self.archived_channel_ids:
                    LOGGER.debug(
                        f"频道不在归档列表中，跳过 | 频道: {channel_name} (@{channel_username}) [ID: {channel_id}]"
                    )
                    return

            if not config.listen_all_subscribed_channels and config.no_translation_channels:
                if (
                    channel_username not in config.no_translation_channels
                    and channel_id not in config.no_translation_channels
                ):
                    LOGGER.debug(
                        f"频道不在白名单中，跳过 | 频道: {channel_name} (@{channel_username})"
                    )
                    return

            blocklist = getattr(config, "channel_blocklist", []) or []
            if blocklist:
                normalized_ids = {str(item).strip() for item in blocklist if str(item).strip()}
                normalized_usernames = {
                    str(item).strip().lstrip("@").lower()
                    for item in blocklist
                    if str(item).strip() and str(item).strip().lstrip("@")
                }

                if channel_id in normalized_ids:
                    LOGGER.debug(
                        f"频道在黑名单中（ID），跳过 | 频道: {channel_name} ({channel_id})"
                    )
                    return

                if channel_username and channel_username.lower() in normalized_usernames:
                    LOGGER.debug(
                        f"频道在黑名单中（用户名），跳过 | 频道: {channel_name} (@{channel_username})"
                    )
                    return

            if sender_id is not None and sender_id in getattr(config, "blocked_sender_ids", []):
                LOGGER.info(f"跳过被屏蔽发送者 {sender_id} 的消息 | 来源: {channel_name}")
                return

            # 只转发来自特定发送主体（用户或频道），支持多种标识
            allowed_senders_env = os.getenv("ALLOWED_SENDER_USERNAME", "").strip()
            if allowed_senders_env:
                allowed_identifiers: Set[str] = set()
                for item in allowed_senders_env.split(","):
                    allowed_identifiers.update(_expand_identifier(item))

                message_identifiers = _collect_message_identifiers(
                    sender_username,
                    sender_id,
                    channel_username,
                    channel_id,
                )

                if not message_identifiers:
                    LOGGER.debug(
                        "无法获取消息主体标识，跳过 | 频道: %s",
                        channel_name,
                    )
                    return

                if allowed_identifiers.isdisjoint(message_identifiers):
                    LOGGER.debug(
                        "非目标发送主体，跳过 | 频道: %s | 发送者: %s | 标识: %s | 允许: %s",
                        channel_name,
                        f"@{sender_username}" if sender_username else "<未知>",
                        ", ".join(sorted(message_identifiers)),
                        ", ".join(sorted(allowed_identifiers)),
                    )
                    return

            if getattr(config, "enable_sender_whitelist", False):
                whitelist_map = getattr(config, "channel_sender_whitelist", {}) or {}
                whitelist_entry = None
                channel_key_username = channel_username.lower() if channel_username else None
                channel_key_id = channel_id

                if channel_key_username and channel_key_username in whitelist_map:
                    whitelist_entry = whitelist_map[channel_key_username]
                elif channel_key_id in whitelist_map:
                    whitelist_entry = whitelist_map[channel_key_id]

                global_whitelist = getattr(config, "global_sender_whitelist", {}) or {}
                allowed_ids = set(global_whitelist.get("ids") or [])
                allowed_usernames = {
                    name.lower() for name in (global_whitelist.get("usernames") or [])
                }

                if whitelist_entry:
                    allowed_ids.update(whitelist_entry.get("ids") or [])
                    allowed_usernames.update(
                        name.lower() for name in (whitelist_entry.get("usernames") or [])
                    )

                if allowed_ids or allowed_usernames:
                    matched = False
                    if allowed_ids and sender_id in allowed_ids:
                        matched = True
                    if (
                        allowed_usernames
                        and sender_username
                        and sender_username.lower() in allowed_usernames
                    ):
                        matched = True

                    if not matched:
                        LOGGER.debug(
                            "白名单未匹配，跳过消息 | 频道: %s | 发送者ID: %s | 用户名: %s",
                            channel_name,
                            sender_id,
                            sender_username,
                        )
                        return

            if not message_text.strip():
                LOGGER.debug(f"收到空消息，已跳过 | 来源: {channel_name}")
                return

            message_preview = message_text[:50] + ("..." if len(message_text) > 50 else "")
            LOGGER.debug(f"接收到新消息: {message_preview} | 来源: {channel_name}")
            print(f"\n{Fore.CYAN}📨 新消息 | {Fore.YELLOW}{channel_name}{Style.RESET_ALL}")
            LOGGER.debug(f"开始处理消息 | 来源: {channel_name}")
            success, processed_text = await self.processor.process_message(
                event.message,
                channel_name,
            )

            if success:
                final_message = processed_text or ""
                if not final_message.strip():
                    raw_text = getattr(event.message, "text", "") or ""
                    final_message = self.processor._format_message_with_timestamp(raw_text)

                if not final_message or not final_message.strip():
                    LOGGER.debug(f"格式化后文本为空，跳过写入 | 来源: {channel_name}")
                    return

                LOGGER.debug(
                    "消息处理成功，准备写入数据库 | 来源: %s | 长度: %s",
                    channel_name,
                    len(final_message),
                )
                self._save_message_to_db(
                    title=channel_name,
                    content=final_message,
                    source=sender_username or channel_name,
                )
                LOGGER.info(f"消息已写入数据库 | 来源: {channel_name}")
            else:
                LOGGER.debug(f"消息被过滤或处理失败 | 来源: {channel_name}")

        except Exception as exc:
            LOGGER.error(
                f"消息处理错误: {exc} | 来源: {channel_name if 'channel_name' in locals() else '未知频道'}"
            )

    async def get_subscribed_channels(self) -> List[Dict[str, object]]:
        try:
            LOGGER.debug("开始获取订阅的频道列表...")

            # 如果启用了只监听归档，则只获取归档对话（folder_id=1）
            if hasattr(config, "listen_archived_only") and config.listen_archived_only:
                LOGGER.info("已启用只监听归档频道模式 (LISTEN_ARCHIVED_ONLY=true)")
                dialogs = await self.client.get_dialogs(folder=1)
            else:
                dialogs = await self.client.get_dialogs()

            channels: List[Dict[str, object]] = []
            for dialog in dialogs:
                if hasattr(dialog.entity, "broadcast") or hasattr(dialog.entity, "megagroup"):
                    channel_name = getattr(dialog.entity, "title", "未知频道")
                    channel_username = getattr(dialog.entity, "username", None)
                    channel_id = dialog.entity.id
                    folder_id = getattr(dialog, "folder_id", None)
                    is_archived = folder_id == 1

                    channels.append(
                        {
                            "id": channel_id,
                            "name": channel_name,
                            "username": channel_username,
                            "folder_id": folder_id,
                            "is_archived": is_archived,
                        }
                    )
                    archive_status = "📂 [归档]" if is_archived else "📋 [主界面]"
                    LOGGER.debug(
                        f"发现频道: {channel_name} (@{channel_username}) [ID: {channel_id}] {archive_status}"
                    )

            LOGGER.info(f"成功获取到 {len(channels)} 个订阅频道")
            return channels

        except Exception as exc:
            LOGGER.error(f"获取订阅频道失败: {exc}")
            return []

    async def refresh_archived_channels(self) -> None:
        """刷新归档频道列表（仅在启用归档模式时）"""
        if not (hasattr(config, "listen_archived_only") and config.listen_archived_only):
            return  # 未启用归档模式，跳过

        try:
            LOGGER.debug("正在刷新归档频道列表...")

            # 获取归档对话
            dialogs = await self.client.get_dialogs(folder=1)

            new_archived_ids: set[int] = set()
            for dialog in dialogs:
                if hasattr(dialog.entity, "broadcast") or hasattr(dialog.entity, "megagroup"):
                    channel_id = dialog.entity.id
                    new_archived_ids.add(channel_id)

            # 检测变化
            if self.archived_channel_ids:  # 不是第一次刷新
                added = new_archived_ids - self.archived_channel_ids
                removed = self.archived_channel_ids - new_archived_ids

                if added:
                    LOGGER.info(f"📂 检测到新归档频道 ({len(added)} 个):")
                    for channel_id in added:
                        # 获取频道名称
                        try:
                            entity = await self.client.get_entity(channel_id)
                            channel_name = getattr(entity, "title", "未知")
                            channel_username = getattr(entity, "username", None)
                            username_str = f"@{channel_username}" if channel_username else f"ID:{channel_id}"
                            LOGGER.info(f"  ➕ {channel_name} ({username_str})")
                            print(f"{Fore.GREEN}📂 新归档频道: {channel_name} ({username_str}){Style.RESET_ALL}")
                        except Exception:
                            LOGGER.info(f"  ➕ 频道ID: {channel_id}")

                if removed:
                    LOGGER.info(f"📋 检测到取消归档频道 ({len(removed)} 个):")
                    for channel_id in removed:
                        try:
                            entity = await self.client.get_entity(channel_id)
                            channel_name = getattr(entity, "title", "未知")
                            channel_username = getattr(entity, "username", None)
                            username_str = f"@{channel_username}" if channel_username else f"ID:{channel_id}"
                            LOGGER.info(f"  ➖ {channel_name} ({username_str})")
                            print(f"{Fore.YELLOW}📋 取消归档: {channel_name} ({username_str}){Style.RESET_ALL}")
                        except Exception:
                            LOGGER.info(f"  ➖ 频道ID: {channel_id}")

            # 更新缓存
            self.archived_channel_ids = new_archived_ids
            self.last_archived_refresh = time.time()

            LOGGER.debug(f"归档频道列表已更新，当前 {len(new_archived_ids)} 个归档频道")

        except Exception as exc:
            LOGGER.error(f"刷新归档频道列表失败: {exc}")

    async def archived_refresh_loop(self) -> None:
        """归档频道列表定时刷新任务"""
        if not (hasattr(config, "listen_archived_only") and config.listen_archived_only):
            return

        interval = getattr(config, "archived_refresh_interval", 300)
        LOGGER.info(f"启动归档频道定时刷新任务，间隔: {interval} 秒")

        # 初始刷新
        await self.refresh_archived_channels()

        while self.running:
            try:
                await asyncio.sleep(interval)
                await self.refresh_archived_channels()
            except asyncio.CancelledError:
                LOGGER.info("归档刷新任务被取消")
                break
            except Exception as exc:
                LOGGER.error(f"归档刷新任务出错: {exc}")

    async def run(self) -> None:
        print(f"\n{Fore.CYAN}Telegram监听工具{Style.RESET_ALL}")

        LOGGER.debug("开始初始化监听服务...")
        if not await self.init_client():
            LOGGER.error("初始化客户端失败，监听工具无法启动")
            return

        if hasattr(config, "listen_all_subscribed_channels") and config.listen_all_subscribed_channels:
            archived_mode = hasattr(config, "listen_archived_only") and config.listen_archived_only
            mode_text = "归档" if archived_mode else "所有订阅"
            LOGGER.info(f"配置为监听{mode_text}频道，正在获取频道列表...")
            subscribed_channels = await self.get_subscribed_channels()
            channels_count = len(subscribed_channels)

            if subscribed_channels:
                title = f"📂 归档频道列表:" if archived_mode else "📡 订阅频道列表:"
                print(f"\n{Fore.GREEN}{title}{Style.RESET_ALL}")
                for idx, channel in enumerate(subscribed_channels[:10], 1):
                    username_display = f"@{channel['username']}" if channel["username"] else "无用户名"
                    archive_badge = " 📂" if channel.get("is_archived", False) else ""
                    print(f"{Fore.CYAN}{idx:2d}.{Style.RESET_ALL} {channel['name']} ({username_display}){archive_badge}")
                if len(subscribed_channels) > 10:
                    print(
                        f"{Fore.YELLOW}   ... 和其他 {len(subscribed_channels) - 10} 个频道{Style.RESET_ALL}"
                    )
        else:
            channels_count = len(config.no_translation_channels)
            channel_list = ", ".join(config.no_translation_channels) if config.no_translation_channels else "无"

        LOGGER.debug("注册消息事件处理器...")
        self.client.add_event_handler(self.message_handler, events.NewMessage())
        self.running = True

        # 启动归档频道定时刷新任务
        if hasattr(config, "listen_archived_only") and config.listen_archived_only:
            self.archived_refresh_task = asyncio.create_task(self.archived_refresh_loop())
            LOGGER.info(f"归档频道动态跟踪已启动（刷新间隔: {config.archived_refresh_interval} 秒）")

        if hasattr(config, "listen_all_subscribed_channels") and config.listen_all_subscribed_channels:
            archived_mode = hasattr(config, "listen_archived_only") and config.listen_archived_only
            mode_text = "归档频道" if archived_mode else "订阅频道"
            LOGGER.info(f"开始监听 {channels_count} 个{mode_text}")
            if archived_mode:
                print(f"{Fore.YELLOW}📱 手机端归档/取消归档频道会自动生效，无需重启程序{Style.RESET_ALL}")
        else:
            LOGGER.info(f"开始监听 {channels_count} 个频道: {channel_list}")
        print(f"{Fore.GREEN}系统就绪，等待新消息...{Style.RESET_ALL}")

        try:
            LOGGER.debug("进入主循环，定期显示状态报告...")
            while self.running:
                await asyncio.sleep(600)
                LOGGER.debug("准备显示状态报告...")
                self._print_status()
        except KeyboardInterrupt:
            LOGGER.info("用户中断，正在关闭...")
        finally:
            LOGGER.debug("关闭会话和资源...")

            # 取消归档刷新任务
            if self.archived_refresh_task:
                self.archived_refresh_task.cancel()
                try:
                    await self.archived_refresh_task
                except asyncio.CancelledError:
                    pass

            if self.client:
                await self.client.disconnect()
            LOGGER.info("已安全关闭")

    def _print_status(self) -> None:
        elapsed = time.time() - self.stats["start_time"]
        elapsed_hours = int(elapsed // 3600)
        elapsed_mins = int((elapsed % 3600) // 60)

        processor_stats = self.processor.get_stats().get("processor", {})

        LOGGER.debug("生成状态报告...")
        print(f"\n{Fore.CYAN}状态报告{Style.RESET_ALL}")
        print(f"运行时长: {elapsed_hours}小时 {elapsed_mins}分钟")
        print(f"接收消息: {self.stats['received']}条")
        print(f"处理消息: {processor_stats.get('processed', 0)}条")
        print(f"过滤消息: {processor_stats.get('filtered', 0)}条")
        print(f"黑名单过滤: {processor_stats.get('blacklisted', 0)}条")
        print(f"推广过滤: {processor_stats.get('promotion_filtered', 0)}条")
        print(Fore.CYAN + Style.RESET_ALL)


# 认证工具（来源于原 auth.py）
def print_status(message: str) -> None:
    print(f"🔧 {message}")


def print_success(message: str) -> None:
    print(f"✅ {message}")


def print_error(message: str) -> None:
    print(f"❌ {message}")


def print_warning(message: str) -> None:
    print(f"⚠️  {message}")


async def _request_login_code(
    client: TelegramClient,
    *,
    force_sms: bool = False,
    resend_hash: Optional[str] = None,
) -> Any:
    phone = config.telegram.phone_number
    if resend_hash is not None:
        return await client.resend_code(phone, resend_hash)
    return await client.send_code_request(phone, force_sms=force_sms)


async def authenticate_telegram() -> bool:
    print("=" * 60)
    print("    🤖 Telegram监听工具认证向导")
    print("=" * 60)
    print()

    print_status("开始Telegram认证过程...")

    print("📋 当前配置信息:")
    print(f"   📱 手机号: {config.telegram.phone_number}")
    print(f"   🔑 API ID: {config.telegram.api_id}")
    print(f"   📂 会话文件: {config.get_session_path()}")
    print()

    session_file = f"{config.get_session_path()}.session"
    force_reauth = config.auth.force_reauth or ("--force" in sys.argv)
    if os.path.exists(session_file):
        if force_reauth:
            try:
                os.remove(session_file)
                print_success("已删除现有会话文件，准备重新认证")
            except Exception as exc:
                print_warning(f"删除会话文件失败，将继续使用现有会话: {exc}")
        else:
            print_status("检测到现有会话文件，直接使用现有会话")

    print()
    print_status("正在自动选择最佳连接方式...")
    print_status("尝试直连Telegram服务器...")
    client = TelegramClient(
        config.get_session_path(),
        config.telegram.api_id,
        config.telegram.api_hash,
        proxy=None,
    )

    connected = False
    try:
        await asyncio.wait_for(client.connect(), timeout=5)
        print_success("直连Telegram成功！")
        connected = True
    except (asyncio.TimeoutError, Exception):
        print_warning("直连失败，自动尝试其他方式...")
        proxy = config.get_telethon_proxy()
        if proxy:
            print_status("自动切换到代理模式...")
            await client.disconnect()
            client = TelegramClient(
                config.get_session_path(),
                config.telegram.api_id,
                config.telegram.api_hash,
                proxy=proxy,
            )
            try:
                await client.connect()
                print_success(
                    f"自动切换到代理成功: {config.proxy.type}://{config.proxy.host}:{config.proxy.port}"
                )
                connected = True
            except Exception as proxy_error:
                print_error(f"代理连接也失败: {proxy_error}")
        else:
            print_status("再次尝试直连...")
            try:
                await asyncio.sleep(2)
                await client.connect()
                print_success("第二次直连成功！")
                connected = True
            except Exception as retry_error:
                print_error(f"无法连接: {retry_error}")

    if not connected:
        raise Exception("无法连接到Telegram（请检查网络或配置代理）")

    try:
        if not await client.is_user_authorized():
            print_status("需要进行认证...")
            print()
            print_status(f"正在向 {config.telegram.phone_number} 发送验证码...")
            try:
                sent_code = await _request_login_code(client)

                # 打印返回结果以便调试
                LOGGER.debug(f"send_code_request 返回: {sent_code}")
                LOGGER.debug(f"返回类型: {type(sent_code).__name__}")

                # 检查发送方式
                if hasattr(sent_code, 'type'):
                    code_type = sent_code.type
                    LOGGER.debug(f"验证码类型: {type(code_type).__name__}")

                    if hasattr(code_type, '__class__'):
                        type_name = code_type.__class__.__name__
                        if 'App' in type_name:
                            print()
                            print_success("=" * 60)
                            print_success("✅ 验证码已发送到 Telegram App!")
                            print_warning("⚠️  注意: 验证码在 Telegram 应用中,不是短信!")
                            print()
                            print_status("📱 请在手机上:")
                            print("   1. 打开 Telegram 应用")
                            print("   2. 查看验证码通知或登录页面")
                            print(f"   3. 验证码是 {code_type.length if hasattr(code_type, 'length') else '5'} 位数字")
                            print_success("=" * 60)
                        elif 'Sms' in type_name:
                            print_success("✅ 验证码已通过短信发送到你的手机!")
                        elif 'Call' in type_name:
                            print_success("✅ 将通过电话告知验证码!")
                        elif 'FlashCall' in type_name:
                            print_success("✅ 将通过闪存呼叫发送验证码!")
                        else:
                            print_success(f"✅ 验证码已发送 (方式: {type_name})!")
                            LOGGER.warning(f"未知的验证码类型: {type_name}")
                else:
                    print_success("✅ 验证码请求已发送!")

                print()

                # 添加重新发送选项
                print()
                print_warning("⚠️  如果没收到验证码,可以:")
                print("   1. 输入 'r' 或 'resend' - 重新发送验证码")
                print("   2. 输入 'f' 或 'force' - 尝试强制通过短信发送验证码")
                if sent_code.next_type:
                    print(f"   3. 输入 's' 或 'sms' - 改用其他方式 ({sent_code.next_type})")
                print("   4. 或直接输入验证码")
                print()

                while True:
                    verification_code = input("📱 请输入收到的验证码 (或输入 r 重发): ").strip().lower()

                    # 处理重新发送
                    if verification_code in ['r', 'resend', '重发']:
                        print_status("正在重新发送验证码...")
                        try:
                            sent_code = await _request_login_code(client)
                            print_success(f"✅ 验证码已重新发送! (方式: {type(sent_code.type).__name__})")
                            continue
                        except Exception as e:
                            print_error(f"重新发送失败: {e}")
                            continue

                    # 尝试强制短信
                    if verification_code in ['f', 'force', 'forcesms', 'sms!']:
                        print_status("尝试强制通过短信发送验证码...")
                        try:
                            sent_code = await _request_login_code(client, force_sms=True)
                            print_success(f"✅ 已尝试强制短信发送! (方式: {type(sent_code.type).__name__})")
                            continue
                        except Exception as e:
                            print_error(f"强制短信发送失败: {e}")
                            continue

                    # 处理改用其他方式
                    if verification_code in ['s', 'sms', '短信'] and sent_code.next_type:
                        print_status("正在请求改用其他方式...")
                        try:
                            sent_code = await _request_login_code(
                                client,
                                resend_hash=getattr(sent_code, "phone_code_hash", None),
                            )
                            print_success(f"✅ 已改用其他方式发送! (方式: {type(sent_code.type).__name__})")
                            continue
                        except Exception as e:
                            print_error(f"切换失败: {e}")
                            continue

                    if verification_code:
                        break
                    print_error("验证码不能为空，请重新输入")

                print_status("正在验证...")
                try:
                    await client.sign_in(config.telegram.phone_number, verification_code)
                    print_success("验证码验证成功!")

                except Exception as exc:
                    if "password" in str(exc).lower() or "两步验证" in str(exc):
                        if getattr(config.telegram, "password", ""):
                            print_status("检测到两步验证，使用配置中的密码...")
                            await client.sign_in(password=config.telegram.password)
                            print_success("两步验证通过!")
                        else:
                            print_warning("检测到两步验证，需要输入密码")
                            password = input("🔐 请输入两步验证密码: ").strip()
                            await client.sign_in(password=password)
                            print_success("两步验证通过!")
                    else:
                        raise exc

            except Exception as exc:
                print_error(f"认证失败: {exc}")
                return False
        else:
            print_success("已通过认证!")

        print()
        print_status("测试连接...")
        me = await client.get_me()
        print_success(f"认证成功! 欢迎, {me.first_name}!")
        print(f"   👤 用户名: @{me.username}")
        print(f"   📞 手机号: {me.phone}")
        print(f"   🆔 用户ID: {me.id}")
        print()

        print_status("保存认证会话...")
        await client.disconnect()
        print_success(f"会话已保存到: {session_file}")

        return True

    except Exception as exc:
        print_error(f"认证过程中发生错误: {exc}")
        await client.disconnect()
        return False


async def run_authentication_cli() -> None:
    result = await authenticate_telegram()
    if result:
        print()
        print("=" * 60)
        print("    🎉 认证完成!")
        print("=" * 60)
    else:
        print()
        print("=" * 60)
        print("    ❌ 认证失败")
        print("=" * 60)


# 频道列表工具（来源于 tools/list_channels.py）
async def list_my_channels() -> None:
    print("\n🤖 Telegram频道列表工具\n")
    monitor = SimpleTelegramMonitor()

    try:
        await monitor.init_client()
        channels = await monitor.get_subscribed_channels()

        if not channels:
            print("❌ 没有找到任何频道")
            return

        print(f"\n📡 找到 {len(channels)} 个频道:\n")
        channel_ids: List[str] = []
        channel_usernames: List[str] = []

        for idx, ch in enumerate(channels, 1):
            print(f"{idx}. 📢 {ch['name']}")
            if ch["username"]:
                print(f"   用户名: @{ch['username']}")
                channel_usernames.append(ch["username"])
            else:
                print("   用户名: 无")

            print(f"   ID: {ch['id']}")
            channel_ids.append(str(ch["id"]))
            print("-" * 40)

        print("\n" + "=" * 60)
        print("📝 配置建议：")
        print("=" * 60)

        print("\n方式1：使用频道ID（更可靠）：")
        print("ALLOWED_CHANNELS=" + ",".join(channel_ids[:5]))
        if len(channel_ids) > 5:
            print(f"# ... 还有{len(channel_ids) - 5}个频道")

        if channel_usernames:
            print("\n方式2：使用用户名（更易读）：")
            print("ALLOWED_CHANNELS=" + ",".join(channel_usernames[:5]))
            if len(channel_usernames) > 5:
                print(f"# ... 还有{len(channel_usernames) - 5}个频道")

        print("\n💡 提示：")
        print("1. 复制上面的ALLOWED_CHANNELS配置")
        print("2. 编辑 .env 文件：nano .env")
        print("3. 粘贴并保留你需要的频道")
        print("4. 删除不需要监听的频道")
        print("5. 保存后运行：./start_with_env.sh")

    except Exception as exc:
        print(f"❌ 错误: {exc}")
    finally:
        if monitor.client:
            await monitor.client.disconnect()


# Telegram Client 辅助工具（来源于 tools/telegram_client.py）
class TelegramMonitor:
    def __init__(self, api_id: int, api_hash: str, session_string: str | None = None) -> None:
        self.api_id = api_id
        self.api_hash = api_hash
        self.client = TelegramClient(
            StringSession(session_string) if session_string else "monitor_session",
            api_id,
            api_hash,
        )
        self.monitored_chats: set[int] = set()
        self.message_handlers: List = []

    async def initialize(self):
        await self.client.start()
        me = await self.client.get_me()
        CLIENT_LOGGER.info("已登录为: %s (%s)", me.username, me.phone)
        return me

    def add_message_handler(self, handler) -> None:
        self.message_handlers.append(handler)

    def add_monitored_chat(self, chat_id: int) -> None:
        self.monitored_chats.add(chat_id)
        CLIENT_LOGGER.info("添加监控聊天: %s", chat_id)

    @events.register(events.NewMessage)
    async def handle_new_message(self, event) -> None:
        try:
            chat = await event.get_chat()
            sender = await event.get_sender()

            message_data = {
                "message_id": event.message.id,
                "chat_id": event.chat_id,
                "chat_title": getattr(chat, "title", getattr(chat, "username", "Private")),
                "sender_id": sender.id if sender else None,
                "sender_name": self._get_user_name(sender) if sender else "Unknown",
                "text": event.message.text or "",
                "date": event.message.date.isoformat(),
                "is_private": isinstance(event.message.peer_id, PeerUser),
                "is_group": isinstance(event.message.peer_id, (PeerChat, PeerChannel)),
                "media_type": self._get_media_type(event.message),
                "raw_message": event.message.to_dict(),
            }

            if self.monitored_chats and event.chat_id not in self.monitored_chats:
                return

            for handler in self.message_handlers:
                try:
                    await handler(message_data)
                except Exception as exc:
                    CLIENT_LOGGER.error(f"消息处理器错误: {exc}")

            CLIENT_LOGGER.info(
                "新消息 [%s] %s: %s...",
                message_data["chat_title"],
                message_data["sender_name"],
                message_data["text"][:50],
            )

        except Exception as exc:
            CLIENT_LOGGER.error(f"处理消息时出错: {exc}")

    def _get_user_name(self, user) -> str:
        if hasattr(user, "username") and user.username:
            return f"@{user.username}"
        if hasattr(user, "first_name"):
            name = user.first_name
            if hasattr(user, "last_name") and user.last_name:
                name += f" {user.last_name}"
            return name
        return f"User_{user.id}" if user else "Unknown"

    def _get_media_type(self, message) -> str:
        if not message.media:
            return "text"
        media_type = type(message.media).__name__
        return media_type.replace("MessageMedia", "").lower()

    async def get_dialogs(self, limit: int = 100) -> List[Dict[str, object]]:
        dialogs = []
        async for dialog in self.client.iter_dialogs(limit=limit):
            dialogs.append(
                {
                    "id": dialog.id,
                    "name": dialog.name,
                    "is_user": dialog.is_user,
                    "is_group": dialog.is_group,
                    "is_channel": dialog.is_channel,
                    "unread_count": dialog.unread_count,
                    "last_message_date": dialog.date.isoformat() if dialog.date else None,
                }
            )
        return dialogs

    async def get_chat_history(self, chat_id: int, limit: int = 100) -> List[Dict[str, object]]:
        messages = []
        async for message in self.client.iter_messages(chat_id, limit=limit):
            sender = await message.get_sender()
            messages.append(
                {
                    "id": message.id,
                    "text": message.text or "",
                    "date": message.date.isoformat(),
                    "sender_name": self._get_user_name(sender) if sender else "Unknown",
                    "media_type": self._get_media_type(message),
                    "is_outgoing": message.out,
                }
            )
        return messages

    async def send_message(self, chat_id: int, text: str) -> None:
        try:
            await self.client.send_message(chat_id, text)
            CLIENT_LOGGER.info("消息已发送到 %s: %s...", chat_id, text[:50])
        except Exception as exc:
            CLIENT_LOGGER.error(f"发送消息失败: {exc}")

    async def start_monitoring(self) -> None:
        self.client.add_event_handler(self.handle_new_message)
        CLIENT_LOGGER.info("开始监控消息...")
        await self.client.run_until_disconnected()

    async def stop(self) -> None:
        await self.client.disconnect()
        CLIENT_LOGGER.info("客户端已断开连接")

    def get_session_string(self) -> str:
        return self.client.session.save()


async def log_message_handler(message_data) -> None:
    with open("messages.json", "a", encoding="utf-8") as file:
        json.dump(message_data, file, ensure_ascii=False)
        file.write("\n")


async def keyword_alert_handler(message_data) -> None:
    keywords = ["urgent", "紧急", "alert", "警告"]
    text = message_data["text"].lower()

    if any(keyword in text for keyword in keywords):
        alert = {
            "timestamp": datetime.now().isoformat(),
            "type": "keyword_alert",
            "chat": message_data["chat_title"],
            "sender": message_data["sender_name"],
            "message": message_data["text"],
            "keywords_found": [kw for kw in keywords if kw in text],
        }

        with open("alerts.json", "a", encoding="utf-8") as file:
            json.dump(alert, file, ensure_ascii=False)
            file.write("\n")

        CLIENT_LOGGER.warning(f"关键词报警: {alert}")


async def run_telegram_client_demo() -> None:
    api_id = os.getenv("TELEGRAM_API_ID")
    api_hash = os.getenv("TELEGRAM_API_HASH")
    session_string = os.getenv("TELEGRAM_SESSION_STRING", "")

    if not api_id or not api_hash:
        print("请设置环境变量 TELEGRAM_API_ID、TELEGRAM_API_HASH (可选 TELEGRAM_SESSION_STRING)")
        return

    monitor = TelegramMonitor(int(api_id), api_hash, session_string)

    try:
        await monitor.initialize()
        monitor.add_message_handler(log_message_handler)
        monitor.add_message_handler(keyword_alert_handler)

        dialogs = await monitor.get_dialogs(limit=10)
        print("\n最近的对话:")
        for dialog in dialogs:
            print(f"- {dialog['name']} (ID: {dialog['id']}, 未读: {dialog['unread_count']})")

        print(f"\n会话字符串: {monitor.get_session_string()}")
        print("设置环境变量 TELEGRAM_SESSION_STRING 以避免重复登录")

        await monitor.start_monitoring()

    except KeyboardInterrupt:
        print("\n正在停止监控...")
    except Exception as exc:
        CLIENT_LOGGER.error(f"运行错误: {exc}")
    finally:
        await monitor.stop()


# 统一 CLI 入口
async def run_monitor() -> int:
    monitor = SimpleTelegramMonitor(interactive=False)
    try:
        await monitor.run()
        return 0
    except UnauthorizedSessionError as exc:
        LOGGER.error("未检测到有效的Telegram会话: %s", exc)
        print_warning("请运行 `./start.sh auth` 在终端中完成登录后再启动监控服务。")
        return 2
    finally:
        if monitor.client:
            try:
                await monitor.client.disconnect()
            except Exception:
                pass


async def run_session_status(verbose: bool = True) -> int:
    monitor = SimpleTelegramMonitor(interactive=False)
    try:
        await monitor.init_client()
        if verbose:
            print_success("当前会话已授权，可直接启动监听。")
        return 0
    except UnauthorizedSessionError:
        if verbose:
            print_warning("未检测到有效的 Telegram 会话，需要先运行登录向导。")
        return 2
    except Exception as exc:
        if verbose:
            print_error(f"检查会话状态失败: {exc}")
        return 1
    finally:
        if monitor.client:
            try:
                await monitor.client.disconnect()
            except Exception:
                pass


def parse_args(argv: Optional[Iterable[str]] = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="JTPD Bot 一体化脚本")
    sub = parser.add_subparsers(dest="command", required=True)

    sub.add_parser("monitor", help="启动Telegram监听器")
    sub.add_parser("auth", help="运行交互式Telegram认证")
    sub.add_parser("list-channels", help="列出已订阅的Telegram频道")
    sub.add_parser("client-demo", help="运行Telegram Client监控示例")
    sub.add_parser("session-status", help="检查当前会话授权状态")

    return parser.parse_args(argv)


async def dispatch_command(args: argparse.Namespace) -> int:
    if args.command == "monitor":
        return await run_monitor()
    elif args.command == "auth":
        await run_authentication_cli()
        return 0
    elif args.command == "list-channels":
        await list_my_channels()
        return 0
    elif args.command == "client-demo":
        await run_telegram_client_demo()
        return 0
    elif args.command == "session-status":
        return await run_session_status()
    else:  # pragma: no cover - argparse 已限制
        raise ValueError(f"未知命令: {args.command}")


def main(argv: Optional[Iterable[str]] = None) -> int:
    args = parse_args(argv)
    try:
        result = asyncio.run(dispatch_command(args))
        return int(result) if isinstance(result, int) else 0
    except KeyboardInterrupt:
        print(f"\n{Fore.YELLOW}已取消操作{Style.RESET_ALL}")
        return 1
    except Exception as exc:
        print(f"{Fore.RED}执行失败: {exc}{Style.RESET_ALL}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
