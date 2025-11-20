#!/usr/bin/env python3
"""
智能合并脚本 - 专门处理 app.py 的合并冲突
保留我们的认证系统，同时集成官方的交易功能
"""

import re
import sys
from pathlib import Path

def extract_sections(content, section_markers):
    """从内容中提取特定部分"""
    sections = {}
    current_section = None
    current_content = []
    
    lines = content.split('\n')
    
    for line in lines:
        # 检查是否是章节开始
        for marker, section_name in section_markers.items():
            if marker in line:
                # 保存前一个章节
                if current_section:
                    sections[current_section] = '\n'.join(current_content)
                
                # 开始新章节
                current_section = section_name
                current_content = [line]
                break
        else:
            # 如果不是章节开始，添加到当前章节
            if current_section:
                current_content.append(line)
    
    # 保存最后一个章节
    if current_section:
        sections[current_section] = '\n'.join(current_content)
    
    return sections

def smart_merge_app_py(our_content, official_content):
    """智能合并 app.py 文件"""
    
    # 定义章节标记
    section_markers = {
        '# ========== 核心交易功能 ==========': 'trading_core',
        '# ========== 数据库和认证功能 ==========': 'auth_system', 
        '# ========== 用户管理功能 ==========': 'user_management',
        '# ========== 交易功能 ==========': 'trading_functions',
        '# ========== 页面组件 ==========': 'page_components',
        '# ========== 网络功能 ==========': 'network_functions',
        'class NoFxCore:': 'trading_class',
        'def init_supabase():': 'supabase_init',
        'def login_user(': 'login_function',
        'def register_user(': 'register_function',
        'def show_dashboard(': 'dashboard_function',
        'def show_login(': 'login_page',
        'def show_register(': 'register_page',
        'if __name__ == "__main__":': 'main_block'
    }
    
    print("🔧 开始智能合并 app.py...")
    
    # 提取我们的章节
    our_sections = extract_sections(our_content, section_markers)
    print(f"📁 我们的章节: {list(our_sections.keys())}")
    
    # 提取官方章节  
    official_sections = extract_sections(official_content, section_markers)
    print(f"📁 官方章节: {list(official_sections.keys())}")
    
    # 合并策略
    merged_sections = {}
    
    # 优先使用我们的认证系统
    auth_sections = ['auth_system', 'user_management', 'login_function', 
                    'register_function', 'login_page', 'register_page']
    
    for section in auth_sections:
        if section in our_sections:
            merged_sections[section] = our_sections[section]
            print(f"✅ 保留我们的: {section}")
        elif section in official_sections:
            merged_sections[section] = official_sections[section]
            print(f"📥 使用官方的: {section}")
    
    # 优先使用官方的交易核心
    trading_sections = ['trading_core', 'trading_class', 'trading_functions']
    
    for section in trading_sections:
        if section in official_sections:
            merged_sections[section] = official_sections[section]
            print(f"📥 使用官方的: {section}")
        elif section in our_sections:
            merged_sections[section] = our_sections[section]
            print(f"✅ 保留我们的: {section}")
    
    # 处理其他章节
    all_sections = set(our_sections.keys()) | set(official_sections.keys())
    for section in all_sections:
        if section not in merged_sections:
            if section in official_sections:
                merged_sections[section] = official_sections[section]
                print(f"📥 使用官方的: {section}")
            else:
                merged_sections[section] = our_sections[section]
                print(f"✅ 保留我们的: {section}")
    
    # 构建合并后的内容
    merged_content = []
    
    # 添加文件头
    header = '''import streamlit as st
import os
import requests
import socket
import json
from datetime import datetime
import hashlib
import jwt
from supabase import create_client
import pandas as pd
import plotly.graph_objects as go

st.set_page_config(
    page_title="NoFx13 Trading System",
    page_icon="📈", 
    layout="wide"
)'''
    
    merged_content.append(header)
    merged_content.append("\n# ========== 自动合并的应用 ==========")
    merged_content.append("# 🔄 集成官方交易功能 + 我们的认证系统")
    merged_content.append("")
    
    # 按逻辑顺序添加章节
    section_order = [
        'trading_core', 'trading_class', 'auth_system', 'supabase_init',
        'user_management', 'login_function', 'register_function', 
        'trading_functions', 'network_functions', 'page_components',
        'dashboard_function', 'login_page', 'register_page', 'main_block'
    ]
    
    for section in section_order:
        if section in merged_sections:
            merged_content.append("")
            merged_content.append(merged_sections[section])
    
    # 确保有主函数
    if 'main_block' not in merged_sections:
        merged_content.append('''
if __name__ == "__main__":
    main()''')
    
    return '\n'.join(merged_content)

def main():
    if len(sys.argv) != 2:
        print("用法: python smart-merge.py <file_path>")
        sys.exit(1)
    
    file_path = sys.argv[1]
    
    if not file_path.endswith('app.py'):
        print("❌ 此脚本仅用于合并 app.py 文件")
        sys.exit(1)
    
    # 读取当前文件(我们的版本)
    with open(file_path, 'r', encoding='utf-8') as f:
        our_content = f.read()
    
    # 读取官方版本 (假设在临时文件中)
    official_path = "official_app.py"
    if Path(official_path).exists():
        with open(official_path, 'r', encoding='utf-8') as f:
            official_content = f.read()
    else:
        print("❌ 找不到官方版本文件")
        sys.exit(1)
    
    # 执行智能合并
    try:
        merged_content = smart_merge_app_py(our_content, official_content)
        
        # 保存合并结果
        with open(file_path, 'w', encoding='utf-8') as f:
            f.write(merged_content)
        
        print("✅ app.py 智能合并完成！")
        
    except Exception as e:
        print(f"❌ 合并过程中出错: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
