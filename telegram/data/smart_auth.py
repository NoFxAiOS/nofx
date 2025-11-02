#!/usr/bin/env python3
"""
智能验证码登录 - 尝试所有可用的验证码发送方式
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

async def smart_login():
    print("=" * 70)
    print("🧠 智能验证码登录工具")
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

    session_name = os.getenv("TELEGRAM_SESSION_NAME", "telegram_monitor_optimized")
    client = TelegramClient(
        f"data/sessions/{session_name}",
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

        # 第一次发送验证码
        print("📤 正在发送验证码...")
        sent_code = await client.send_code_request(phone)

        print("=" * 70)
        print("📥 验证码发送结果:")
        code_type_name = type(sent_code.type).__name__
        print(f"   当前类型: {code_type_name}")

        if hasattr(sent_code.type, 'length'):
            print(f"   验证码长度: {sent_code.type.length} 位")

        if sent_code.next_type:
            next_type_name = type(sent_code.next_type).__name__
            print(f"   ✅ 可切换到: {next_type_name}")
        else:
            print(f"   ⚠️  无其他发送方式")

        if sent_code.timeout:
            print(f"   超时时间: {sent_code.timeout} 秒")

        print("=" * 70)
        print()

        # 提示用户
        if 'App' in code_type_name:
            print("⚠️  当前是 Telegram App 内验证码")
            print("📱 请在已登录的 Telegram 应用中查看验证码")
            print()

            if sent_code.next_type:
                print(f"💡 你可以输入 's' 切换到 {next_type_name}")
                print()
        elif 'Sms' in code_type_name:
            print("✅ 短信验证码已发送!")
            print("📱 请检查手机短信")
            print()
        else:
            print(f"ℹ️  验证码类型: {code_type_name}")
            print()

        # 等待输入验证码
        max_attempts = 5
        for attempt in range(max_attempts):
            try:
                user_input = input(f"📱 请输入验证码 或 's'切换发送方式 ({attempt + 1}/{max_attempts}): ").strip().lower()

                # 处理切换发送方式
                if user_input == 's' and sent_code.next_type:
                    print()
                    print("🔄 正在切换验证码发送方式...")
                    try:
                        # 使用 resend_code 切换到 next_type
                        sent_code = await client.resend_code(phone, sent_code.phone_code_hash)
                        new_type = type(sent_code.type).__name__
                        print(f"✅ 已切换到: {new_type}")

                        if 'Sms' in new_type:
                            print("📱 短信验证码已发送,请检查手机!")
                        elif 'Call' in new_type:
                            print("📞 将通过电话告知验证码!")
                        elif 'FlashCall' in new_type:
                            print("📞 将通过闪存呼叫发送验证码!")
                        else:
                            print(f"ℹ️  新方式: {new_type}")

                        print()
                        continue
                    except Exception as e:
                        print(f"❌ 切换失败: {e}")
                        print()
                        continue

                # 处理重新发送
                if user_input == 'r':
                    print()
                    print("🔄 正在重新发送验证码...")
                    try:
                        sent_code = await client.send_code_request(phone)
                        print(f"✅ 验证码已重新发送! (方式: {type(sent_code.type).__name__})")
                        print()
                        continue
                    except Exception as e:
                        print(f"❌ 重新发送失败: {e}")
                        print()
                        continue

                code = user_input

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
                print()

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
                import traceback
                traceback.print_exc()
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
        print(f"   💾 会话文件: data/sessions/{session_name}.session")
        print("=" * 70)
        print()
        print("💡 现在你可以运行: ./start.sh monitor")

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
    asyncio.run(smart_login())
