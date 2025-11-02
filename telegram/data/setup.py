"""
Setup script for telegram_collector package
"""

from setuptools import setup, find_packages
from pathlib import Path

# Read README
readme_file = Path(__file__).parent / "README.md"
long_description = readme_file.read_text(encoding="utf-8") if readme_file.exists() else ""

# Read requirements
requirements_file = Path(__file__).parent / "requirements.txt"
requirements = []
if requirements_file.exists():
    requirements = [
        line.strip()
        for line in requirements_file.read_text(encoding="utf-8").splitlines()
        if line.strip() and not line.startswith("#")
    ]

setup(
    name="telegram_collector",
    version="1.0.0",
    description="Telegram Message Monitoring and Archival Tool (JT Bot)",
    long_description=long_description,
    long_description_content_type="text/markdown",
    author="JT Bot Team",
    python_requires=">=3.8",
    packages=find_packages(exclude=["test", "test.*", "archive", ".venv"]),
    install_requires=requirements,
    entry_points={
        "console_scripts": [
            "jt-bot=telegram_collector.jt_bot:main",
        ],
    },
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
    ],
)
