import streamlit as st
import os
import requests
import socket
import json
from datetime import datetime
import hashlib
import jwt
from supabase import create_client

st.set_page_config(
    page_title="NoFx13 Trading",
    page_icon="📈", 
    layout="wide"
)

# 初始化 Supabase
@st.cache_resource
def init_supabase():
    try:
        url = os.environ.get('SUPABASE_URL')
        key = os.environ.get('SUPABASE_ANON_KEY')
        if url and key:
            return create_client(url, key)
        return None
    except Exception as e:
        st.error(f"Supabase 初始化失败: {e}")
        return None

# 密码加密
def hash_password(password):
    return hashlib.sha256(password.encode()).hexdigest()

# 初始化会话状态
def init_session():
    if 'user' not in st.session_state:
        st.session_state.user = None
    if 'authenticated' not in st.session_state:
        st.session_state.authenticated = False
    if 'page' not in st.session_state:
        st.session_state.page = "login"

# 用户注册
def register_user(email, password, username):
    try:
        supabase = init_supabase()
        if not supabase:
            return False, "数据库连接失败"
        
        # 检查用户是否已存在
        existing_user = supabase.table('users').select('*').eq('email', email).execute()
        if existing_user.data:
            return False, "邮箱已被注册"
        
        # 创建新用户
        user_data = {
            'email': email,
            'password_hash': hash_password(password),
            'username': username,
            'created_at': datetime.now().isoformat(),
            'last_login': datetime.now().isoformat()
        }
        
        result = supabase.table('users').insert(user_data).execute()
        if result.data:
            return True, "注册成功"
        else:
            return False, "注册失败"
    except Exception as e:
        return False, f"注册错误: {str(e)}"

# 用户登录
def login_user(email, password):
    try:
        supabase = init_supabase()
        if not supabase:
            return False, "数据库连接失败"
        
        # 查询用户
        user_data = supabase.table('users').select('*').eq('email', email).execute()
        if not user_data.data:
            return False, "用户不存在"
        
        user = user_data.data[0]
        if user['password_hash'] == hash_password(password):
            # 更新最后登录时间
            supabase.table('users').update({'last_login': datetime.now().isoformat()}).eq('id', user['id']).execute()
            return True, user
        else:
            return False, "密码错误"
    except Exception as e:
        return False, f"登录错误: {str(e)}"

# 登录页面
def show_login():
    st.title("🔐 用户登录")
    
    with st.form("login_form"):
        email = st.text_input("📧 邮箱")
        password = st.text_input("🔑 密码", type="password")
        submit = st.form_submit_button("登录")
        
        if submit:
            if email and password:
                success, result = login_user(email, password)
                if success:
                    st.session_state.user = result
                    st.session_state.authenticated = True
                    st.session_state.page = "dashboard"
                    st.success("登录成功！")
                    st.rerun()
                else:
                    st.error(result)
            else:
                st.error("请填写所有字段")
    
    st.write("---")
    if st.button("📝 没有账号？立即注册"):
        st.session_state.page = "register"
        st.rerun()

# 注册页面
def show_register():
    st.title("📝 用户注册")
    
    with st.form("register_form"):
        username = st.text_input("👤 用户名")
        email = st.text_input("📧 邮箱")
        password = st.text_input("🔑 密码", type="password")
        confirm_password = st.text_input("✅ 确认密码", type="password")
        submit = st.form_submit_button("注册")
        
        if submit:
            if all([username, email, password, confirm_password]):
                if password != confirm_password:
                    st.error("密码不一致")
                elif len(password) < 6:
                    st.error("密码至少6位")
                else:
                    success, message = register_user(email, password, username)
                    if success:
                        st.success(message)
                        st.session_state.page = "login"
                        st.rerun()
                    else:
                        st.error(message)
            else:
                st.error("请填写所有字段")
    
    st.write("---")
    if st.button("🔙 返回登录"):
        st.session_state.page = "login"
        st.rerun()

# 用户仪表板
def show_dashboard():
    st.title(f"🎯 欢迎回来，{st.session_state.user['username']}！")
    
    # 用户信息卡片
    col1, col2, col3 = st.columns(3)
    with col1:
        st.metric("👤 用户名", st.session_state.user['username'])
    with col2:
        st.metric("📧 邮箱", st.session_state.user['email'])
    with col3:
        last_login = st.session_state.user.get('last_login', '未知')
        st.metric("🕒 最后登录", last_login[:10] if last_login != '未知' else '未知')
    
    # 功能区域
    st.subheader("🚀 交易功能")
    tab1, tab2, tab3 = st.tabs(["账户概览", "交易面板", "设置"])
    
    with tab1:
        st.write("### 📊 账户信息")
        st.info("""
        - **账户状态**: 🟢 正常
        - **会员等级**: 标准用户
        - **交易权限**: 基础功能
        """)
        
        # 模拟账户数据
        col1, col2, col3 = st.columns(3)
        with col1:
            st.metric("💰 账户余额", "$10,000")
        with col2:
            st.metric("📈 总收益", "+$250")
        with col3:
            st.metric("🔢 交易次数", "15")
    
    with tab2:
        st.write("### 💹 交易面板")
        st.warning("交易功能开发中...")
        
        # 简单的交易模拟
        symbol = st.selectbox("选择交易对", ["BTC/USDT", "ETH/USDT", "BNB/USDT"])
        amount = st.number_input("交易数量", min_value=0.0, value=100.0)
        
        col1, col2 = st.columns(2)
        with col1:
            if st.button("🟢 买入", use_container_width=True):
                st.success(f"已买入 {amount} {symbol}")
        with col2:
            if st.button("🔴 卖出", use_container_width=True):
                st.error(f"已卖出 {amount} {symbol}")
    
    with tab3:
        st.write("### ⚙️ 账户设置")
        
        # 密码修改
        with st.expander("🔒 修改密码"):
            current_pwd = st.text_input("当前密码", type="password")
            new_pwd = st.text_input("新密码", type="password")
            confirm_pwd = st.text_input("确认新密码", type="password")
            if st.button("更新密码"):
                if new_pwd == confirm_pwd:
                    st.success("密码更新成功")
                else:
                    st.error("密码不一致")
        
        # 退出登录
        st.write("---")
        if st.button("🚪 退出登录"):
            st.session_state.authenticated = False
            st.session_state.user = None
            st.session_state.page = "login"
            st.rerun()

# 主应用
def main():
    init_session()
    
    # 如果未认证，显示登录/注册页面
    if not st.session_state.authenticated:
        if st.session_state.page == "login":
            show_login()
        else:
            show_register()
        return
    
    # 已认证用户显示主界面
    show_dashboard()

if __name__ == "__main__":
    main()
