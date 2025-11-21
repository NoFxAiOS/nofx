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

# ========== 核心交易功能 ==========
class NoFxCore:
    """NoFx 核心交易引擎"""
    
    @staticmethod
    def get_market_data(symbol="BTCUSDT"):
        """获取市场数据"""
        try:
            # 模拟实时市场数据
            base_price = {
                "BTCUSDT": 45000,
                "ETHUSDT": 2500,
                "BNBUSDT": 300
            }
            base_price = base_price.get(symbol, 45000)
            
            # 基于时间波动
            minute = datetime.now().minute
            price_variation = (minute % 20) * 50 - 500
            current_price = base_price + price_variation
            
            return {
                'symbol': symbol,
                'price': current_price,
                'change': round((price_variation / base_price) * 100, 2),
                'volume': 125000000,
                'high': current_price + 500,
                'low': current_price - 500,
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
    def generate_chart(data, periods=50):
        """生成交易图表"""
        try:
            base_price = data.get('price', 45000)
            dates = pd.date_range(end=datetime.now(), periods=periods, freq='H')
            prices = [base_price + (i - periods/2) * 100 for i in range(periods)]
            
            fig = go.Figure()
            fig.add_trace(go.Candlestick(
                x=dates,
                open=[p * 0.99 for p in prices],
                high=[p * 1.02 for p in prices],
                low=[p * 0.98 for p in prices],
                close=prices,
                name="Price"
            ))
            
            fig.update_layout(
                title=f"{data.get('symbol', 'BTCUSDT')} Price Chart",
                xaxis_title="Time",
                yaxis_title="Price (USDT)",
                template="plotly_dark",
                height=400,
                showlegend=False
            )
            
            return fig
        except Exception as e:
            # 备用简单图表
            fig = go.Figure()
            fig.add_trace(go.Scatter(
                x=[1, 2, 3, 4, 5],
                y=[data.get('price', 45000) + i * 100 for i in range(5)],
                mode='lines',
                name='Price'
            ))
            return fig

# ========== 数据库和认证功能 ==========
@st.cache_resource
def init_supabase():
    """初始化 Supabase 客户端 - 增强错误处理版本"""
    try:
        url = os.environ.get('SUPABASE_URL')
        key = os.environ.get('SUPABASE_ANON_KEY')
        
        # 添加详细的调试信息
        st.write("🔧 Supabase 连接调试信息:")
        st.write(f"- SUPABASE_URL 存在: {bool(url)}")
        st.write(f"- SUPABASE_ANON_KEY 存在: {bool(key)}")
        
        if url:
            st.write(f"- URL 格式: {url[:30]}..." if len(url) > 30 else f"- URL 格式: {url}")
        if key:
            st.write(f"- Key 格式: {key[:10]}..." if len(key) > 10 else f"- Key 格式: {key}")
        
        if not url or not key:
            st.error("❌ Supabase 环境变量未设置完整")
            st.info("请在 Hugging Face Space 设置中添加 SUPABASE_URL 和 SUPABASE_ANON_KEY")
            return None
        
        # 验证 URL 格式
        if not url.startswith('https://') or 'supabase.co' not in url:
            st.error(f"❌ SUPABASE_URL 格式不正确: {url}")
            st.info("URL 应该是 https://your-project-id.supabase.co 格式")
            return None
        
        # 验证 Key 格式
        if not key.startswith('eyJ') or len(key) < 50:
            st.error(f"❌ SUPABASE_ANON_KEY 格式不正确")
            st.info("Key 应该是长的 JWT 令牌，以 'eyJ' 开头")
            return None
        
        # 尝试创建客户端
        client = create_client(url, key)
        
        # 测试连接 - 尝试一个简单的查询
        try:
            test_result = client.table('users').select('*').limit(1).execute()
            st.success("✅ Supabase 连接成功")
            return client
        except Exception as test_error:
            st.error(f"❌ Supabase 连接测试失败: {str(test_error)}")
            
            # 提供具体的错误解决建议
            if "Invalid API key" in str(test_error):
                st.error("""
                **API Key 错误解决方案:**
                1. 登录 Supabase 控制台 (app.supabase.com)
                2. 进入你的项目
                3. 点击 Settings → API
                4. 复制正确的 anon public key
                5. 更新 Hugging Face 中的 SUPABASE_ANON_KEY
                """)
            elif "JWT" in str(test_error):
                st.error("JWT 令牌格式错误，请检查 SUPABASE_ANON_KEY 的值")
            
            return None
            
    except Exception as e:
        st.error(f"❌ Supabase 初始化失败: {str(e)}")
        return None

def hash_password(password):
    """密码加密"""
    return hashlib.sha256(password.encode()).hexdigest()

def init_session():
    """初始化会话状态"""
    if 'user' not in st.session_state:
        st.session_state.user = None
    if 'authenticated' not in st.session_state:
        st.session_state.authenticated = False
    if 'page' not in st.session_state:
        st.session_state.page = "dashboard"
    if 'trade_history' not in st.session_state:
        st.session_state.trade_history = []

# ========== 用户管理功能 ==========
def register_user(email, password, username):
    """用户注册"""
    try:
        supabase = init_supabase()
        if not supabase:
            return False, "数据库连接失败"
        
        # 检查用户是否存在
        existing_user = supabase.table('users').select('*').eq('email', email).execute()
        if existing_user.data:
            return False, "邮箱已被注册"
        
        # 创建新用户
        user_data = {
            'email': email,
            'password_hash': hash_password(password),
            'username': username,
            'created_at': datetime.now().isoformat(),
            'last_login': datetime.now().isoformat(),
            'balance': 10000.00  # 初始余额
        }
        
        result = supabase.table('users').insert(user_data).execute()
        if result.data:
            return True, "注册成功"
        else:
            return False, "注册失败"
    except Exception as e:
        return False, f"注册错误: {str(e)}"

def login_user(email, password):
    """用户登录"""
    try:
        supabase = init_supabase()
        if not supabase:
            return False, "数据库连接失败"
        
        user_data = supabase.table('users').select('*').eq('email', email).execute()
        if not user_data.data:
            return False, "用户不存在"
        
        user = user_data.data[0]
        if user['password_hash'] == hash_password(password):
            # 更新最后登录时间
            supabase.table('users').update({
                'last_login': datetime.now().isoformat()
            }).eq('id', user['id']).execute()
            return True, user
        else:
            return False, "密码错误"
    except Exception as e:
        return False, f"登录错误: {str(e)}"

# ========== 交易功能 ==========
def execute_trade(user_id, symbol, side, amount, price):
    """执行交易"""
    try:
        supabase = init_supabase()
        if not supabase:
            return False, "数据库连接失败"
        
        trade_data = {
            'user_id': user_id,
            'symbol': symbol,
            'side': side,
            'amount': float(amount),
            'price': float(price),
            'timestamp': datetime.now().isoformat(),
            'status': 'completed'
        }
        
        result = supabase.table('trades').insert(trade_data).execute()
        return True, "交易执行成功"
    except Exception as e:
        return False, f"交易错误: {str(e)}"

# ========== 页面组件 ==========
def show_sidebar():
    """显示侧边栏"""
    with st.sidebar:
        st.title("🔗 NoFx13")
        
        if st.session_state.authenticated:
            st.success(f"👤 {st.session_state.user['username']}")
            st.write(f"💰 余额: ${st.session_state.user.get('balance', 0):,.2f}")
        
        st.write("---")
        st.header("📊 市场概览")
        
        # 实时市场数据
        btc_data = NoFxCore.get_market_data("BTCUSDT")
        eth_data = NoFxCore.get_market_data("ETHUSDT")
        
        st.metric("BTC/USDT", f"${btc_data['price']:,.0f}", f"{btc_data['change']}%")
        st.metric("ETH/USDT", f"${eth_data['price']:,.0f}", f"{eth_data['change']}%")
        
        st.write("---")
        st.header("🌐 系统状态")
        
        # 网络状态
        try:
            response = requests.get('https://api.ipify.org?format=json', timeout=5)
            ip_address = response.json()['ip']
            st.write(f"**IP:** `{ip_address}`")
        except:
            st.write("**IP:** 未知")
        
        st.write(f"**状态:** 🟢 运行中")
        st.write(f"**时间:** {datetime.now().strftime('%H:%M:%S')}")

def show_dashboard():
    """主仪表板"""
    st.title("🚀 NoFx13 智能交易系统")
    
    # 显示侧边栏
    show_sidebar()
    
    # 用户欢迎信息
    if st.session_state.authenticated:
        st.success(f"🎯 欢迎
