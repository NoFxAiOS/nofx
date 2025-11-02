#!/usr/bin/env python3
"""
调试验证码发送问题
"""
import asyncio
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

from telethon import TelegramClient
import socks
import os
from dotenv import load_dotenv

# 加载环境变量
load_dotenv()

async def test_send_code():
    print("=" * 70)
    print("🔍 Telegram 验证码发送诊断工具")
    print("=" * 70)
    print()

    phone = os.getenv("TELEGRAM_PHONE_NUMBER")
    api_id = int(os.getenv("TELEGRAM_API_ID"))
    api_hash = os.getenv("TELEGRAM_API_HASH")

    proxy_host = os.getenv("PROXY_HOST", "127.0.0.1")
    proxy_port = int(os.getenv("PROXY_PORT", "9910"))

    print(f"📱 手机号: {phone}")
    print(f"🔑 API ID: {api_id}")
    print(f"🔑 API Hash: {api_hash[:10]}...")
    print()

    # 测试1: 不使用代理
    print("=" * 70)
    print("测试 1: 直连 Telegram (不使用代理)")
    print("=" * 70)

    client = TelegramClient(
        "debug_session_no_proxy",
        api_id,
        api_hash,
        proxy=None
    )

    try:
        print("🔗 正在连接...")
        await asyncio.wait_for(client.connect(), timeout=10)
        print("✅ 连接成功!")

        print("📤 正在发送验证码...")
        sent_code = await client.send_code_request(phone)

        print("✅ 验证码发送请求已提交!")
        print(f"   类型: {type(sent_code.type).__name__}")
        print(f"   Phone Hash: {sent_code.phone_code_hash[:20]}...")

        if hasattr(sent_code.type, 'length'):
            print(f"   验证码长度: {sent_code.type.length} 位")

        if sent_code.next_type:
            print(f"   下一种方式: {type(sent_code.next_type).__name__}")

        if sent_code.timeout:
            print(f"   超时时间: {sent_code.timeout} 秒")

        print()
        print("🎯 直连模式验证码发送成功!")

    except asyncio.TimeoutError:
        print("❌ 连接超时 (10秒)")
    except Exception as e:
        print(f"❌ 直连失败: {type(e).__name__}: {e}")
    finally:
        try:
            await client.disconnect()
        except:
            pass

    print()

    # 测试2: 使用代理
    print("=" * 70)
    print("测试 2: 通过代理连接 Telegram")
    print("=" * 70)

    proxy_config = (
        socks.HTTP,
        proxy_host,
        int(proxy_port),
    )

    print(f"🔧 代理配置: http://{proxy_host}:{proxy_port}")

    client = TelegramClient(
        "debug_session_with_proxy",
        api_id,
        api_hash,
        proxy=proxy_config
    )

    try:
        print("🔗 正在通过代理连接...")
        await asyncio.wait_for(client.connect(), timeout=10)
        print("✅ 代理连接成功!")

        print("📤 正在通过代理发送验证码...")
        sent_code = await client.send_code_request(phone)

        print("✅ 验证码发送请求已提交!")
        print(f"   类型: {type(sent_code.type).__name__}")
        print(f"   Phone Hash: {sent_code.phone_code_hash[:20]}...")

        if hasattr(sent_code.type, 'length'):
            print(f"   验证码长度: {sent_code.type.length} 位")

        if sent_code.next_type:
            print(f"   下一种方式: {type(sent_code.next_type).__name__}")

        if sent_code.timeout:
            print(f"   超时时间: {sent_code.timeout} 秒")

        print()
        print("🎯 代理模式验证码发送成功!")

    except asyncio.TimeoutError:
        print("❌ 代理连接超时 (10秒)")
    except Exception as e:
        print(f"❌ 代理连接失败: {type(e).__name__}: {e}")
        import traceback
        traceback.print_exc()
    finally:
        try:
            await client.disconnect()
        except:
            pass

    print()
    print("=" * 70)
    print("🔍 诊断完成")
    print("=" * 70)
    print()
    print("💡 建议:")
    print("   1. 如果两个测试都失败,说明网络有问题")
    print("   2. 如果直连成功但代理失败,说明代理配置有问题")
    print("   3. 如果都成功但收不到验证码,可能是:")
    print("      - Telegram 限流 (短时间内请求太多次)")
    print("      - 手机号被 Telegram 限制")
    print("      - API ID/Hash 有问题")
    print()

if __name__ == "__main__":
    asyncio.run(test_send_code())
