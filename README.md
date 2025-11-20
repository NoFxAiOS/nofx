---
title: NoFx13 Trading System
emoji: 📈
colorFrom: blue
colorTo: purple
sdk: docker
app_file: app.py
pinned: false
---

# NoFx13 智能交易系统

基于人工智能的智能交易平台，集成实时市场数据、交易信号和用户管理系统。

## 🚀 功能特性

### 交易功能
- 📊 实时市场数据监控
- 💹 智能交易信号生成
- 📈 交互式价格图表
- ⚡ 一键快速交易
- 📋 交易历史记录

### 用户系统
- 🔐 安全用户认证
- 👤 个人账户管理
- 💰 虚拟资金交易
- 🛡️ 数据安全保障

### 技术架构
- 🐳 Docker 容器化部署
- 🔗 Supabase 后端服务
- 📊 Plotly 数据可视化
- 🌐 RESTful API 集成

## 🛠️ 快速开始

### 环境要求
- Python 3.11+
- Docker
- Supabase 账户

### 本地运行
```bash
# 克隆仓库
git clone https://github.com/yu704176671/nofx13.git
cd nofx13

# 安装依赖
pip install -r requirements.txt

# 设置环境变量
export SUPABASE_URL=your_supabase_url
export SUPABASE_ANON_KEY=your_supabase_key

# 运行应用
streamlit run app.py
