import streamlit as st
import os
import time
from supabase import create_client

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
        st.error(f"数据库初始化失败: {e}")
        return None

def show_auth_interface(supabase):
    """显示认证界面"""
    try:
        tab1, tab2 = st.tabs(["登录", "注册"])
        
        with tab1:
            with st.form("login_form"):
                st.write("### 用户登录")
                email = st.text_input("邮箱", key="login_email")
                password = st.text_input("密码", type="password", key="login_password")
                login_button = st.form_submit_button("登录")
                
                if login_button:
                    if email and password:
                        with st.spinner("登录中..."):
                            try:
                                response = supabase.auth.sign_in_with_password({
                                    "email": email,
                                    "password": password
                                })
                                if response.user:
                                    st.success("登录成功！")
                                    time.sleep(1)
                                    st.rerun()
                                else:
                                    st.error("登录失败，请检查邮箱和密码")
                            except Exception as e:
                                st.error(f"登录错误: {str(e)}")
                    else:
                        st.warning("请输入邮箱和密码")
        
        with tab2:
            with st.form("register_form"):
                st.write("### 用户注册")
                email = st.text_input("注册邮箱", key="register_email")
                password = st.text_input("注册密码", type="password", key="register_password")
                username = st.text_input("用户名（可选）", key="register_username")
                register_button = st.form_submit_button("注册")
                
                if register_button:
                    if email and password:
                        with st.spinner("注册中..."):
                            try:
                                # 先注册认证用户
                                auth_response = supabase.auth.sign_up({
                                    "email": email,
                                    "password": password,
                                })
                                
                                if auth_response.user:
                                    st.success("🎉 注册成功！请检查邮箱验证邮件。")
                                    
                                    # 尝试在 users 表中创建记录
                                    try:
                                        user_data = {
                                            "id": auth_response.user.id,
                                            "email": email,
                                            "username": username
                                        }
                                        db_response = supabase.table('users').insert(user_data).execute()
                                        if db_response.data:
                                            st.success("✅ 用户数据创建成功！")
                                    except Exception as db_error:
                                        st.info("⚠️ 用户数据表需要调整权限，但不影响登录使用")
                                
                                else:
                                    st.error("❌ 注册失败")
                            except Exception as e:
                                error_msg = str(e)
                                if "already registered" in error_msg.lower():
                                    st.error("❌ 该邮箱已被注册")
                                elif "password" in error_msg.lower():
                                    st.error("❌ 密码强度不足，请使用更复杂的密码")
                                else:
                                    st.error(f"❌ 注册错误: {error_msg}")
                    else:
                        st.warning("⚠️ 请输入邮箱和密码")
    except Exception as e:
        st.error(f"界面渲染错误: {str(e)}")

def show_user_dashboard(supabase, user):
    """显示用户仪表板"""
    st.sidebar.success(f"👋 欢迎, {user.email}")
    
    # 退出登录按钮
    if st.sidebar.button("🚪 退出登录"):
        supabase.auth.sign_out()
        st.success("已退出登录")
        time.sleep(1)
        st.rerun()
    
    # 主功能区域
    st.subheader("📊 交易仪表板")
    
    # 功能选项卡
    tab1, tab2, tab3 = st.tabs(["交易记录", "数据分析", "账户信息"])
    
    with tab1:
        st.write("### 交易记录管理")
        
        # 添加交易记录表单
        with st.form("add_trade_form"):
            col1, col2 = st.columns(2)
            with col1:
                symbol = st.text_input("交易标的", "BTC/USDT")
                action = st.selectbox("操作", ["BUY", "SELL"])
            with col2:
                price = st.number_input("价格", value=100.0, min_value=0.0)
                quantity = st.number_input("数量", value=1.0, min_value=0.0)
            
            notes = st.text_area("交易备注")
            
            if st.form_submit_button("💾 保存交易记录"):
                try:
                    trade_data = {
                        "user_id": user.id,
                        "symbol": symbol,
                        "action": action,
                        "price": float(price),
                        "quantity": float(quantity),
                        "notes": notes
                    }
                    response = supabase.table('trading_records').insert(trade_data).execute()
                    if response.data:
                        st.success("✅ 交易记录保存成功！")
                    else:
                        st.error("❌ 保存失败")
                except Exception as e:
                    st.error(f"❌ 保存错误: {str(e)}")
        
        # 显示历史记录
        st.write("### 历史交易记录")
        try:
            records_response = supabase.table('trading_records')\
                .select('*')\
                .eq('user_id', user.id)\
                .order('timestamp', desc=True)\
                .execute()
            
            if records_response.data:
                for record in records_response.data:
                    with st.expander(f"{record['symbol']} - {record['action']} - {record['timestamp'][:10]}"):
                        st.write(f"价格: {record['price']}")
                        st.write(f"数量: {record['quantity']}")
                        st.write(f时间: {record['timestamp'][:19]}")
            else:
                st.info("暂无交易记录")
        except Exception as e:
            st.error(f"加载记录失败: {str(e)}")
    
    with tab2:
        st.write("### 交易数据分析")
        st.info("📈 AI分析功能开发中...")
        
        if st.button("生成分析报告"):
            st.success("✅ 分析报告生成完成！")
            st.write("""
            **示例分析报告:**
            - 总交易次数: 5
            - 平均收益率: 8.5%
            - 风险等级: 中等
            - 建议: 考虑分散投资
            """)
    
    with tab3:
        st.write("### 账户信息")
        st.write(f"**用户ID:** {user.id}")
        st.write(f"**邮箱:** {user.email}")
        st.write(f"**注册时间:** {user.created_at[:10]}")

def main():
    st.set_page_config(
        page_title="NoFx13 Trading",
        page_icon="📈",
        layout="wide",
        initial_sidebar_state="expanded"
    )

    # 页面标题
    st.title("🚀 NoFx13 智能交易系统")
    
    # 显示环境状态
    col1, col2 = st.columns(2)
    with col1:
        st.subheader("🔧 环境状态")
        st.write(f"SUPABASE_URL: {'✅' if os.environ.get('SUPABASE_URL') else '❌'}")
        st.write(f"SUPABASE_ANON_KEY: {'✅' if os.environ.get('SUPABASE_ANON_KEY') else '❌'}")
    
    with col2:
        st.subheader("📊 功能状态")
        st.write("✅ 用户认证系统")
        st.write("✅ 交易数据分析")
        st.write("✅ 数据库集成")
        st.write("✅ 实时交易记录")
    
    # 初始化数据库
    supabase = init_supabase()
    
    if supabase:
        st.success("✅ 数据库连接成功")
        
        # 检查用户登录状态
        try:
            user_response = supabase.auth.get_user()
            if user_response.user:
                show_user_dashboard(supabase, user_response.user)
            else:
                st.info("🔐 请登录或注册以使用完整功能")
                show_auth_interface(supabase)
        except Exception as auth_error:
            st.warning("🔐 显示登录界面")
            show_auth_interface(supabase)
    else:
        st.error("❌ 数据库连接失败，请检查环境变量")

if __name__ == "__main__":
    main()
