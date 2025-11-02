"""Configuration management for telegram_collector"""

from pathlib import Path
from typing import Optional
import os


class Config:
    """Configuration manager for telegram_collector package"""

    def __init__(self):
        # Project root directory (parent of telegram_collector package)
        self.PROJECT_ROOT = Path(__file__).resolve().parent.parent

        # Standard directory paths
        self.CONFIG_DIR = self.PROJECT_ROOT / "config"
        self.DATA_DIR = self.PROJECT_ROOT / "data"
        self.LOG_DIR = self.PROJECT_ROOT / "logs"

        # Ensure directories exist
        self.CONFIG_DIR.mkdir(exist_ok=True)
        self.DATA_DIR.mkdir(exist_ok=True)
        self.LOG_DIR.mkdir(exist_ok=True)

        # Load environment variables from .env if it exists
        env_file = self.PROJECT_ROOT / ".env"
        if env_file.exists():
            try:
                from dotenv import load_dotenv
                load_dotenv(env_file)
            except ImportError:
                pass  # dotenv not installed, skip

    def get_config_path(self, filename: str) -> Path:
        """Get full path to a config file

        Args:
            filename: Name of config file

        Returns:
            Full path to the config file
        """
        return self.CONFIG_DIR / filename

    def get_data_path(self, filename: str) -> Path:
        """Get full path to a data file

        Args:
            filename: Name of data file

        Returns:
            Full path to the data file
        """
        return self.DATA_DIR / filename

    def get_log_path(self, filename: str) -> Path:
        """Get full path to a log file

        Args:
            filename: Name of log file

        Returns:
            Full path to the log file
        """
        return self.LOG_DIR / filename

    def get_database_path(self, filename: str = "jtbot.db") -> Path:
        """Get full path to a database file in data directory

        Args:
            filename: Name of database file (default: jtbot.db)

        Returns:
            Full path to the database file in data/
        """
        return self.PROJECT_ROOT / "data" / filename

    def get_json_path(self, filename: str) -> Path:
        """Get full path to a JSON file in project root

        Args:
            filename: Name of JSON file

        Returns:
            Full path to the JSON file
        """
        return self.PROJECT_ROOT / filename


# Global config instance
config = Config()


# Convenience functions for common operations
def get_project_root() -> Path:
    """Get the project root directory"""
    return config.PROJECT_ROOT


def get_config_path(filename: str) -> Path:
    """Get full path to a config file"""
    return config.get_config_path(filename)


def get_data_path(filename: str) -> Path:
    """Get full path to a data file"""
    return config.get_data_path(filename)


def get_database_path(filename: str = "jtbot.db") -> Path:
    """Get full path to a database file"""
    return config.get_database_path(filename)


def get_log_path(filename: str) -> Path:
    """Get full path to a log file"""
    return config.get_log_path(filename)


def get_json_path(filename: str) -> Path:
    """Get full path to a JSON file"""
    return config.get_json_path(filename)
