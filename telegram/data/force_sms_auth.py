#!/usr/bin/env python3
"""
强制使用短信验证码登录
"""
import asyncio
import sys
from pathlib import Path
import os
from dotenv import load_dotenv
from telethon import TelegramClient
from telethon.errors import SessionPasswordNeededError, PhoneCodeInvalidError
import socks

# 加载环境变量
load_dotenv()

async def force_sms_login():
    print("=" * 70)
    print("📱 强制短信验证码登录工具")
    print("=" * 70)
    print()

    phone = os.getenv("TELEGRAM_PHONE_NUMBER")
    api_id = int(os.getenv("TELEGRAM_API_ID"))
    api_hash = os.getenv("TELEGRAM_API_HASH")
    password = os.getenv("TELEGRAM_PASSWORD", "")

    proxy_host = os.getenv("PROXY_HOST", "127.0.0.1")
    proxy_port = int(os.getenv("PROXY_PORT", "9910"))

    print(f"📱 手机号: {phone}")
    print(f"🔧 使用代理: http://{proxy_host}:{proxy_port}")
    print()

    # 使用代理连接
    proxy_config = (
        socks.HTTP,
        proxy_host,
        int(proxy_port),
    )

    session_name = os.getenv("TELEGRAM_SESSION_NAME", "sms_login_session")
    client = TelegramClient(
        session_name,
        api_id,
        api_hash,
        proxy=proxy_config
    )

    try:
        print("🔗 正在连接 Telegram...")
        await client.connect()
        print("✅ 连接成功!")
        print()

        # 检查是否已登录
        if await client.is_user_authorized():
            me = await client.get_me()
            print(f"✅ 已登录: {me.first_name} (@{me.username})")
            print("无需重新认证")
            return

        # 强制使用短信发送验证码
        print("📤 正在发送短信验证码...")
        print("⚠️  强制使用 SMS 模式")
        print()

        sent_code = await client.send_code_request(phone, force_sms=True)

        print("=" * 70)
        print("📥 验证码发送结果:")
        print(f"   类型: {type(sent_code.type).__name__}")

        if hasattr(sent_code.type, 'length'):
            print(f"   验证码长度: {sent_code.type.length} 位")

        if sent_code.next_type:
            print(f"   下一种方式: {type(sent_code.next_type).__name__}")

        if sent_code.timeout:
            print(f"   超时时间: {sent_code.timeout} 秒")

        print("=" * 70)
        print()

        if 'Sms' in type(sent_code.type).__name__:
            print("✅ 短信验证码已发送!")
            print("📱 请检查手机短信")
        elif 'App' in type(sent_code.type).__name__:
            print("⚠️  仍然是 App 内验证码")
            print("📱 请在 Telegram 应用中查看")
        else:
            print(f"ℹ️  验证码类型: {type(sent_code.type).__name__}")

        print()

        # 等待输入验证码
        max_attempts = 3
        for attempt in range(max_attempts):
            try:
                code = input(f"📱 请输入收到的验证码 ({attempt + 1}/{max_attempts}): ").strip()

                if not code:
                    print("❌ 验证码不能为空!")
                    continue

                if not code.isdigit():
                    print("❌ 验证码只能是数字!")
                    continue

                print(f"🔐 验证中...")
                await client.sign_in(phone, code)
                print("✅ 验证码正确!")
                break

            except PhoneCodeInvalidError:
                print(f"❌ 验证码错误! 剩余 {max_attempts - attempt - 1} 次机会")
                if attempt < max_attempts - 1:
                    continue
                else:
                    print("❌ 验证失败次数过多")
                    return

            except SessionPasswordNeededError:
                print("🔐 需要两步验证密码...")

                if password:
                    print("🔑 使用配置文件中的密码...")
                    try:
                        await client.sign_in(password=password)
                        print("✅ 两步验证通过!")
                        break
                    except Exception as e:
                        print(f"❌ 配置密码错误: {e}")

                pwd = input("🔐 请输入两步验证密码: ").strip()
                try:
                    await client.sign_in(password=pwd)
                    print("✅ 两步验证通过!")
                    break
                except Exception as e:
                    print(f"❌ 密码错误: {e}")
                    return

            except Exception as e:
                print(f"❌ 登录失败: {e}")
                return

        # 验证登录
        print()
        print("🔍 验证登录状态...")
        me = await client.get_me()

        print()
        print("=" * 70)
        print("✅ 登录成功!")
        print(f"   👤 姓名: {me.first_name}")
        if me.username:
            print(f"   🆔 用户名: @{me.username}")
        print(f"   📞 手机: {me.phone}")
        print(f"   🔢 User ID: {me.id}")
        print("=" * 70)

    except Exception as e:
        print()
        print(f"❌ 错误: {e}")
        import traceback
        traceback.print_exc()

    finally:
        await client.disconnect()
        print()
        print("👋 连接已关闭")

if __name__ == "__main__":
    asyncio.run(force_sms_login())
