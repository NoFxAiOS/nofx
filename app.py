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

# ========== 修复的 GitHub 连接函数 ==========
def get_github_info():
    """修复的 GitHub 信息获取函数"""
    try:
        # 使用更稳定的 GitHub API 端点
        repo_url = "https://api.github.com/repos/yu704176671/nofx13"
        
        # 添加超时和重试机制
        headers = {
            'User-Agent': 'NoFx13-Trading-App',
            'Accept': 'application/vnd.github.v3+json'
        }
        
        response = requests.get(repo_url, headers=headers, timeout=15)
        
        if response.status_code == 200:
            repo_data = response.json()
            return {
                'stars': repo_data.get('stargazers_count', 0),
                'forks': repo_data.get('forks_count', 0),
                'last_update': repo_data.get('updated_at', ''),
                'description': repo_data.get('description', 'NoFx13 Trading System'),
                'language': repo_data.get('language', 'Python'),
                'size': repo_data.get('size', 0)
            }
        elif response.status_code == 403:
            # GitHub API 限制，使用备用数据
            return get_fallback_github_info()
        else:
            st.warning(f"GitHub API 返回状态码: {response.status_code}")
            return get_fallback_github_info()
            
    except requests.exceptions.Timeout:
        st.warning("GitHub API 请求超时")
        return get_fallback_github_info()
    except requests.exceptions.ConnectionError:
        st.warning("网络连接错误")
        return get_fallback_github_info()
    except Exception as e:
        st.warning(f"GitHub API 错误: {e}")
        return get_fallback_github_info()

def get_fallback_github_info():
    """备用 GitHub 信息（当 API 不可用时）"""
    return {
        'stars': 1,
        'forks': 0,
        'last_update': datetime.now().isoformat(),
        'description': 'NoFx13 Trading System - 智能交易平台',
        'language': 'Python',
        'size': 1024
    }

def get_github_actions_status():
    """获取 GitHub Actions 状态（修复版）"""
    try:
        actions_url = "https://api.github.com/repos/yu704176671/nofx13/actions/runs"
        headers = {
            'User-Agent': 'NoFx13-Trading-App',
            'Accept': 'application/vnd.github.v3+json'
        }
        
        response = requests.get(actions_url, headers=headers, timeout=10)
        
        if response.status_code == 200:
            actions_data = response.json()
            if actions_data['workflow_runs']:
                latest_run = actions_data['workflow_runs'][0]
                return latest_run
        return None
    except:
        return None

# ========== 网络测试函数 ==========
def test_network_connections():
    """测试各种网络连接"""
    results = {}
    
    # 测试 GitHub API
    try:
        response = requests.get('https://api.github.com', timeout=5)
        results['github_api'] = response.status_code == 200
    except:
        results['github_api'] = False
    
    # 测试外部网络
    try:
        response = requests.get('https://httpbin.org/ip', timeout=5)
        results['external_network'] = response.status_code == 200
    except:
        results['external_network'] = False
    
    # 测试 Supabase 连接
    try:
        supabase = init_supabase()
        results['supabase'] = supabase is not None
    except:
        results['supabase'] = False
    
    return results

# ========== 其他现有函数保持不变 ==========
class NoFxCore:
    """官方 NoFx 核心交易功能"""
    
    @staticmethod
    def get_market_data(symbol="BTCUSDT"):
        """获取市场数据（模拟）"""
        try:
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

# ========== 更新侧边栏显示 ==========
def show_sidebar():
    """显示侧边栏信息"""
    with st.sidebar:
        st.header("🔗 GitHub 连接")
        st.write(f"**仓库:** yu704176671/nofx13")
        
        github_info = get_github_info()
        if github_info:
            st.write(f"⭐ **Stars:** {github_info['stars']}")
            st.write(f"🍴 **Forks:** {github_info['forks']}")
            st.write(f"🕒 **最后更新:** {github_info['last_update'][:10]}")
            st.write(f"💻 **语言:** {github_info['language']}")
        else:
            st.write("⚠️ 使用备用数据")
            st.write("⭐ **Stars:** 1")
            st.write("🍴 **Forks:** 0")
            st.write("💻 **语言:** Python")
        
        st.markdown("[📂 查看仓库](https://github.com/yu704176671/nofx13)")
        st.markdown("[🐛 报告问题](https://github.com/yu704176671/nofx13/issues)")
        
        # 部署信息
        st.header("🚀 部署信息")
        st.write(f"**平台:** Hugging Face")
        st.write(f"**方式:** Dockerfile")
        st.write(f"**状态:** 🟢 运行中")
        
        # 获取 IP 地址
        try:
            response = requests.get('https://api.ipify.org?format=json', timeout=5)
            ip_address = response.json()['ip'] if response.status_code == 200 else "未知"
        except:
            ip_address = "未知"
            
        st.write(f"**IPv4:** `{ip_address}`")
        
        # 网络测试
        if st.button("🔍 测试网络连接"):
            with st.spinner("测试中..."):
                results = test_network_connections()
                
                st.write("**网络测试结果:**")
                for service, status in results.items():
                    emoji = "✅" if status else "❌"
                    st.write(f"{emoji} {service}: {'正常' if status else '失败'}")

def show_dashboard():
    """主仪表板"""
    st.title("🚀 NoFx13 智能交易系统")
    
    # 显示侧边栏
    show_sidebar()
    
    # 实时市场数据
    st.subheader("📊 实时市场")
    
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
    
    # GitHub 集成标签页
    st.subheader("📊 GitHub 集成")
    
    tab1, tab2 = st.tabs(["仓库状态", "系统信息"])
    
    with tab1:
        github_info = get_github_info()
        if github_info:
            col1, col2, col3 = st.columns(3)
            with col1:
                st.metric("Stars", github_info['stars'])
            with col2:
                st.metric("Forks", github_info['forks'])
            with col3:
                st.metric("语言", github_info['language'])
            
            st.write(f"**描述:** {github_info['description']}")
            st.write(f"**最后更新:** {github_info['last_update'][:10]}")
        else:
            st.info("使用模拟 GitHub 数据")
            col1, col2, col3 = st.columns(3)
            with col1:
                st.metric("Stars", 1)
            with col2:
                st.metric("Forks", 0)
            with col3:
                st.metric("状态", "活跃")
        
        if st.button("🔄 刷新 GitHub 数据"):
            st.rerun()
    
    with tab2:
        st.write("**系统信息**")
        
        # 网络状态
        results = test_network_connections()
        st.write("**服务状态:**")
        for service, status in results.items():
            emoji = "✅" if status else "❌"
            st.write(f"{emoji} {service}: {'正常' if status else '失败'}")
        
        # 环境信息
        st.write("**环境变量状态:**")
        env_status = {
            'SUPABASE_URL': '✅ 已设置' if os.environ.get('SUPABASE_URL') else '❌ 未设置',
            'SUPABASE_KEY': '✅ 已设置' if os.environ.get('SUPABASE_ANON_KEY') else '❌ 未设置'
        }
        st.json(env_status)

def show_login():
    """登录页面"""
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
    """注册页面"""
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

def main():
    init_session()
    
    if not st.session_state.authenticated:
        if st.session_state.page == "login":
            show_login()
        else:
            show_register()
    else:
        show_dashboard()

if __name__ == "__main__":
    main()
