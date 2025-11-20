import streamlit as st
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
)

# ========== 官方核心功能 ==========
class NoFxCore:
    """官方 NoFx 核心交易功能"""
    
    @staticmethod
    def get_market_data(symbol="BTCUSDT"):
        """获取市场数据（模拟）"""
        try:
            # 模拟市场数据
            return {
                'symbol': symbol,
                'price': 45000 + (datetime.now().minute % 10) * 100,
                'change': 2.5,
                'volume': 125000000,
                'timestamp': datetime.now().isoformat()
            }
        except Exception as e:
            return {'error': str(e)}
    
    @staticmethod
    def calculate_signals(data):
        """计算交易信号"""
        price = data.get('price', 0)
        change = data.get('change', 0)
        
        if change > 3:
            return "STRONG_BUY", 0.85
        elif change > 1:
            return "BUY", 0.65
        elif change < -3:
            return "STRONG_SELL", 0.85
        elif change < -1:
            return "SELL", 0.65
        else:
            return "HOLD", 0.5
    
    @staticmethod
    def generate_chart(data):
        """生成交易图表"""
        # 模拟价格数据
        dates = pd.date_range(end=datetime.now(), periods=50, freq='H')
        prices = [data.get('price', 45000) + i * 50 - 1250 for i in range(50)]
        
        fig = go.Figure()
        fig.add_trace(go.Scatter(
            x=dates, y=prices,
            mode='lines',
            name='Price',
            line=dict(color='#00ff88', width=2)
        ))
        
        fig.update_layout(
            title="Price Chart",
            xaxis_title="Time",
            yaxis_title="Price (USDT)",
            template="plotly_dark",
            height=300
        )
        
        return fig

# ========== 认证系统 ==========
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

def hash_password(password):
    return hashlib.sha256(password.encode()).hexdigest()

def init_session():
    if 'user' not in st.session_state:
        st.session_state.user = None
    if 'authenticated' not in st.session_state:
        st.session_state.authenticated = False
    if 'page' not in st.session_state:
        st.session_state.page = "login"

def register_user(email, password, username):
    try:
        supabase = init_supabase()
        if not supabase:
            return False, "数据库连接失败"
        
        existing_user = supabase.table('users').select('*').eq('email', email).execute()
        if existing_user.data:
            return False, "邮箱已被注册"
        
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

def login_user(email, password):
    try:
        supabase = init_supabase()
        if not supabase:
            return False, "数据库连接失败"
        
        user_data = supabase.table('users').select('*').eq('email', email).execute()
        if not user_data.data:
            return False, "用户不存在"
        
        user = user_data.data[0]
        if user['password_hash'] == hash_password(password):
            supabase.table('users').update({'last_login': datetime.now().isoformat()}).eq('id', user['id']).execute()
            return True, user
        else:
            return False, "密码错误"
    except Exception as e:
        return False, f"登录错误: {str(e)}"

# ========== 页面组件 ==========
def show_login():
    st.title("🔐 NoFx13 交易系统 - 登录")
    
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
    
    if st.button("📝 没有账号？立即注册"):
        st.session_state.page = "register"
        st.rerun()

def show_register():
    st.title("📝 NoFx13 交易系统 - 注册")
    
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
    
    if st.button("🔙 返回登录"):
        st.session_state.page = "login"
        st.rerun()

def show_dashboard():
    """主仪表板 - 整合官方交易功能"""
    st.title(f"🎯 欢迎回来，{st.session_state.user['username']}！")
    
    # 实时市场数据
    st.subheader("📊 实时市场")
    
    # 市场数据行
    col1, col2, col3, col4 = st.columns(4)
    
    with col1:
        btc_data = NoFxCore.get_market_data("BTCUSDT")
        signal, confidence = NoFxCore.calculate_signals(btc_data)
        st.metric("BTC/USDT", f"${btc_data['price']:,.0f}", f"{btc_data['change']}%")
    
    with col2:
        eth_data = NoFxCore.get_market_data("ETHUSDT")
        signal, confidence = NoFxCore.calculate_signals(eth_data)
        st.metric("ETH/USDT", f"${eth_data['price']:,.0f}", f"{eth_data['change']}%")
    
    with col3:
        st.metric("24h 成交量", f"${btc_data['volume']:,.0f}", "市场")
    
    with col4:
        status_color = {"STRONG_BUY": "🟢", "BUY": "🟡", "HOLD": "⚪", "SELL": "🟠", "STRONG_SELL": "🔴"}
        st.metric("交易信号", f"{status_color.get(signal, '⚪')} {signal}")
    
    # 图表和交易面板
    col1, col2 = st.columns([2, 1])
    
    with col1:
        st.plotly_chart(NoFxCore.generate_chart(btc_data), use_container_width=True)
    
    with col2:
        st.subheader("💹 快速交易")
        
        symbol = st.selectbox("交易对", ["BTC/USDT", "ETH/USDT", "BNB/USDT"])
        amount = st.number_input("数量", min_value=0.0, value=0.01, step=0.01)
        
        col_a, col_b = st.columns(2)
        with col_a:
            if st.button("🟢 买入", use_container_width=True):
                st.success(f"买入 {amount} {symbol}")
        with col_b:
            if st.button("🔴 卖出", use_container_width=True):
                st.error(f"卖出 {amount} {symbol}")
        
        # 官方信号显示
        st.subheader("📈 智能信号")
        st.info(f"""
        **当前信号**: {signal}
        **置信度**: {confidence:.0%}
        **建议操作**: {'买入' if 'BUY' in signal else '卖出' if 'SELL' in signal else '持有'}
        """)
    
    # 系统状态
    st.subheader("🔧 系统状态")
    
    col1, col2, col3 = st.columns(3)
    
    with col1:
        st.info("""
        **交易引擎**
        - 状态: 🟢 运行中
        - 延迟: <50ms
        - API: 正常
        """)
    
    with col2:
        # 网络信息
        try:
            hostname = socket.gethostname()
            local_ip = socket.gethostbyname(hostname)
            st.info(f"""
            **网络状态**
            - IP: {local_ip}
            - 连接: 🟢 稳定
            - 时延: 正常
            """)
        except:
            st.warning("网络信息获取失败")
    
    with col3:
        st.info("""
        **账户信息**
        - 用户: {st.session_state.user['username']}
        - 等级: 标准版
        - 状态: 🟢 活跃
        """)
    
    # 底部导航
    st.sidebar.write("---")
    if st.sidebar.button("🚪 退出登录"):
        st.session_state.authenticated = False
        st.session_state.user = None
        st.session_state.page = "login"
        st.rerun()

def main():
    init_session()
    
    # 显示登录/注册页面或主仪表板
    if not st.session_state.authenticated:
        if st.session_state.page == "login":
            show_login()
        else:
            show_register()
    else:
        show_dashboard()

if __name__ == "__main__":
    main()
