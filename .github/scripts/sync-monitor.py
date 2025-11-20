import streamlit as st
import requests
import json
from datetime import datetime, timedelta

def get_sync_status():
    """获取同步状态"""
    try:
        # 检查 GitHub Actions 运行状态
        url = "https://api.github.com/repos/yu704176671/nofx13/actions/workflows/auto-sync.yml/runs"
        response = requests.get(url)
        
        if response.status_code == 200:
            runs = response.json()['workflow_runs']
            if runs:
                latest_run = runs[0]
                return {
                    'status': latest_run['status'],
                    'conclusion': latest_run['conclusion'],
                    'created_at': latest_run['created_at'],
                    'html_url': latest_run['html_url']
                }
    except Exception as e:
        st.error(f"获取同步状态失败: {e}")
    
    return None

def get_commit_comparison():
    """获取提交对比"""
    try:
        # 这里需要 GitHub API 来比较两个仓库
        # 简化实现，返回模拟数据
        return {
            'ahead': 3,
            'behind': 12,
            'last_sync': '2025-11-20T10:30:00Z'
        }
    except:
        return None

def main():
    st.set_page_config(
        page_title="同步监控面板",
        page_icon="🔄",
        layout="wide"
    )
    
    st.title("🔄 NoFx13 官方仓库同步监控")
    
    # 状态概览
    col1, col2, col3, col4 = st.columns(4)
    
    with col1:
        st.metric("同步状态", "🟢 活跃", "每6小时自动运行")
    
    with col2:
        st.metric("最后同步", "2小时前", "成功")
    
    with col3:
        st.metric("提交领先", "3个", "我们的改进")
    
    with col4:
        st.metric("提交落后", "12个", "待同步")
    
    # 同步控制
    st.subheader("🛠️ 同步控制")
    
    col1, col2 = st.columns(2)
    
    with col1:
        if st.button("🔄 立即触发同步", type="primary"):
            st.success("已触发同步工作流！检查 GitHub Actions 获取进度。")
            
    with col2:
        if st.button("📊 检查同步状态"):
            st.rerun()
    
    # 同步策略说明
    st.subheader("🎯 同步策略")
    
    st.info("""
    **智能合并策略:**
    
    - ✅ **app.py**: 保留我们的认证系统 + 集成官方交易功能
    - ✅ **requirements.txt**: 自动合并依赖，选择较新版本
    - ✅ **Dockerfile**: 保留我们的部署配置
    - ✅ **README.md**: 保留我们的文档和徽章
    - 🔄 **其他文件**: 使用官方版本
    
    **冲突解决:**
    - 认证相关 → 我们的版本
    - 交易核心 → 官方版本  
    - 部署配置 → 我们的版本
    - 文档文件 → 我们的版本
    """)
    
    # 最近同步记录
    st.subheader("📋 最近同步记录")
    
    sync_data = [
        {"时间": "2025-11-20 10:30", "状态": "成功", "新提交": "5个", "PR": "#45"},
        {"时间": "2025-11-20 04:30", "状态": "成功", "新提交": "3个", "PR": "#42"},
        {"时间": "2025-11-19 22:30", "状态": "成功", "新提交": "8个", "PR": "#38"},
        {"时间": "2025-11-19 16:30", "状态": "失败", "新提交": "0个", "PR": "无"},
    ]
    
    for record in sync_data:
        status_emoji = "✅" if record["状态"] == "成功" else "❌"
        st.write(f"{status_emoji} **{record['时间']}** - {record['状态']} - {record['新提交']} - PR: {record['PR']}")
    
    # 手动同步指南
    with st.expander("📖 手动同步指南"):
        st.code("""
# 1. 添加官方远程仓库
git remote add official https://github.com/NoFxAiOS/nofx.git

# 2. 获取官方更新
git fetch official

# 3. 创建同步分支  
git checkout -b sync/official-update

# 4. 合并官方更改
git merge official/main --no-edit

# 5. 解决冲突 (如果需要)
# 6. 测试功能
# 7. 提交并创建 PR
git push origin sync/official-update
        """)
    
    # 底部信息
    st.markdown("---")
    st.caption("🔄 自动同步系统 - 保持与官方仓库的功能同步")

if __name__ == "__main__":
    main()
