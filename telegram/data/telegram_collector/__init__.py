"""
Telegram Collector - JT Bot

Telegram message monitoring and filtering toolkit.

Modules:
- jt_bot: Main bot implementation with message monitoring
- settings: Configuration dataclasses and settings
- config_manager: Path and configuration management
"""

__version__ = "1.0.0"
__author__ = "JT Bot Team"

# Import config for easy access
from .config_manager import (
    config,
    get_project_root,
    get_config_path,
    get_data_path,
    get_database_path,
    get_log_path,
    get_json_path,
)

__all__ = [
    "config",
    "get_project_root",
    "get_config_path",
    "get_data_path",
    "get_database_path",
    "get_log_path",
    "get_json_path",
]
