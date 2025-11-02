#!/usr/bin/env python3
"""测试归档频道过滤功能"""

import asyncio
import sys
from pathlib import Path

# 添加项目路径
project_root = Path(__file__).parent
sys.path.insert(0, str(project_root))

from telegram_collector.jt_bot import SimpleTelegramMonitor, config


async def test_archived_channels():
    """测试获取归档频道功能"""
    print("=" * 60)
    print("📋 Telegram 归档频道过滤功能测试")
    print("=" * 60)

    # 显示当前配置
    print(f"\n当前配置:")
    print(f"  LISTEN_ALL_SUBSCRIBED_CHANNELS: {config.listen_all_subscribed_channels}")
    print(f"  LISTEN_ARCHIVED_ONLY: {config.listen_archived_only}")
    print(f"  CHANNEL_ALLOWLIST 数量: {len(config.no_translation_channels)}")

    # 创建监听器实例
    monitor = SimpleTelegramMonitor()

    print(f"\n正在连接 Telegram...")
    if not await monitor.init_client():
        print("❌ 无法连接到 Telegram")
        return

    print(f"✅ 连接成功\n")

    # 测试 1: 获取所有频道
    print("-" * 60)
    print("测试 1: 获取所有订阅频道（不过滤归档）")
    print("-" * 60)

    # 临时禁用归档过滤
    original_setting = config.listen_archived_only
    config.listen_archived_only = False

    all_channels = await monitor.get_subscribed_channels()
    print(f"✅ 找到 {len(all_channels)} 个频道/群组")

    archived_count = sum(1 for ch in all_channels if ch.get("is_archived", False))
    normal_count = len(all_channels) - archived_count

    print(f"\n统计:")
    print(f"  📋 主界面频道: {normal_count} 个")
    print(f"  📂 归档频道: {archived_count} 个")

    if all_channels:
        print(f"\n前 10 个频道列表:")
        for idx, channel in enumerate(all_channels[:10], 1):
            username = f"@{channel['username']}" if channel['username'] else "无用户名"
            status = "📂 [归档]" if channel.get('is_archived', False) else "📋 [主界面]"
            print(f"  {idx:2d}. {channel['name'][:30]:<30} ({username:<20}) {status}")

    # 测试 2: 只获取归档频道
    print("\n" + "-" * 60)
    print("测试 2: 只获取归档频道（LISTEN_ARCHIVED_ONLY=true）")
    print("-" * 60)

    config.listen_archived_only = True

    archived_channels = await monitor.get_subscribed_channels()
    print(f"✅ 找到 {len(archived_channels)} 个归档频道")

    if archived_channels:
        print(f"\n归档频道列表:")
        for idx, channel in enumerate(archived_channels[:15], 1):
            username = f"@{channel['username']}" if channel['username'] else "无用户名"
            folder_id = channel.get('folder_id', None)
            print(f"  {idx:2d}. {channel['name'][:30]:<30} ({username:<20}) folder_id={folder_id}")
    else:
        print("⚠️  未找到归档频道，请先在 Telegram 中将一些频道归档")

    # 恢复原始设置
    config.listen_archived_only = original_setting

    # 清理
    await monitor.client.disconnect()

    print("\n" + "=" * 60)
    print("✅ 测试完成")
    print("=" * 60)

    # 输出使用说明
    print("\n📖 使用说明:")
    print("1. 在 .env 中设置 LISTEN_ARCHIVED_ONLY=true 启用归档过滤")
    print("2. 设置 LISTEN_ALL_SUBSCRIBED_CHANNELS=true 监听所有频道")
    print("3. 两个选项同时启用时，只监听归档的频道")
    print("\n示例配置:")
    print("  LISTEN_ALL_SUBSCRIBED_CHANNELS=true   # 监听所有订阅")
    print("  LISTEN_ARCHIVED_ONLY=true             # 只监听归档的")
    print("  # 结果：只监听归档文件夹中的频道")


if __name__ == "__main__":
    try:
        asyncio.run(test_archived_channels())
    except KeyboardInterrupt:
        print("\n\n⚠️  测试被用户中断")
    except Exception as e:
        print(f"\n❌ 测试失败: {e}")
        import traceback
        traceback.print_exc()
