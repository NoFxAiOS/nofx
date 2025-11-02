#!/usr/bin/env python3
"""
使用系统环境变量代理登录 - 不使用 PySocks
"""
import asyncio
import os
from dotenv import load_dotenv
from telethon import TelegramClient
from telethon.errors import SessionPasswordNeededError, PhoneCodeInvalidError

load_dotenv()

async def login():
    phone = os.getenv("TELEGRAM_PHONE_NUMBER")
    api_id = int(os.getenv("TELEGRAM_API_ID"))
    api_hash = os.getenv("TELEGRAM_API_HASH")
    password = os.getenv("TELEGRAM_PASSWORD", "")
    session_name = os.getenv("TELEGRAM_SESSION_NAME", "telegram_monitor_optimized")

    # 设置系统代理环境变量
    os.environ['HTTP_PROXY'] = 'http://127.0.0.1:9910'
    os.environ['HTTPS_PROXY'] = 'http://127.0.0.1:9910'

    print("=" * 70)
    print("🔐 Telegram 登录工具 (系统代理模式)")
    print("=" * 70)
    print()
    print(f"📱 手机号: {phone}")
    print(f"🔧 系统代理: {os.environ.get('HTTP_PROXY')}")
    print(f"💾 会话: data/sessions/{session_name}.session")
    print()

    # 不传 proxy 参数，让它使用系统环境变量
    client = TelegramClient(
        f"data/sessions/{session_name}",
        api_id,
        api_hash,
        proxy=None  # 使用系统环境变量代理
    )

    try:
        print("🔗 正在连接 Telegram (使用系统代理)...")
        await client.connect()
        print("✅ 连接成功!")
        print()

        # 检查是否已登录
        if await client.is_user_authorized():
            me = await client.get_me()
            print(f"✅ 已登录: {me.first_name} (@{me.username})")
            print("✅ 无需重新认证!")
            return

        # 发送验证码
        print("📤 正在发送验证码...")
        sent_code = await client.send_code_request(phone)

        code_type = type(sent_code.type).__name__
        print()
        print("=" * 70)
        print("📥 验证码发送成功!")
        print(f"   方式: {code_type}")

        if hasattr(sent_code.type, 'length'):
            print(f"   长度: {sent_code.type.length} 位")

        if sent_code.next_type:
            print(f"   可切换: {type(sent_code.next_type).__name__}")

        print("=" * 70)
        print()

        if 'App' in code_type:
            print("📱 验证码在 Telegram 应用中 (不是短信)")
            print("   打开手机/电脑 Telegram 查看")
        elif 'Sms' in code_type:
            print("✅ 短信验证码已发送!")

        print()

        # 等待输入
        max_attempts = 5
        for attempt in range(max_attempts):
            try:
                user_input = input(f"📱 输入验证码 (s=切换, r=重发) [{attempt + 1}/{max_attempts}]: ").strip().lower()

                if user_input == 's' and sent_code.next_type:
                    print("🔄 切换中...")
                    sent_code = await client.resend_code(phone, sent_code.phone_code_hash)
                    print(f"✅ 已切换到: {type(sent_code.type).__name__}")
                    continue

                if user_input == 'r':
                    print("🔄 重发中...")
                    sent_code = await client.send_code_request(phone)
                    print(f"✅ 已重发: {type(sent_code.type).__name__}")
                    continue

                code = user_input
                if not code or not code.isdigit():
                    print("❌ 请输入数字!")
                    continue

                print("🔐 验证中...")
                await client.sign_in(phone, code)
                print("✅ 验证码正确!")
                break

            except PhoneCodeInvalidError:
                print(f"❌ 验证码错误! 剩余 {max_attempts - attempt - 1} 次")
                if attempt == max_attempts - 1:
                    return

            except SessionPasswordNeededError:
                print("🔐 需要两步验证密码...")
                if password:
                    await client.sign_in(password=password)
                    print("✅ 密码验证通过!")
                    break
                else:
                    pwd = input("🔐 输入密码: ").strip()
                    await client.sign_in(password=pwd)
                    print("✅ 密码验证通过!")
                    break

            except Exception as e:
                print(f"❌ 错误: {e}")
                return

        # 验证
        print()
        me = await client.get_me()
        print("=" * 70)
        print("✅ 登录成功!")
        print(f"   👤 {me.first_name} (@{me.username})")
        print(f"   📞 {me.phone}")
        print("=" * 70)

    except Exception as e:
        print(f"❌ 错误: {e}")
        import traceback
        traceback.print_exc()

    finally:
        await client.disconnect()
        print("👋 完成")

if __name__ == "__main__":
    asyncio.run(login())
