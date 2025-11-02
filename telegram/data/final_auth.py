#!/usr/bin/env python3
"""
最终登录脚本 - 使用工作的代理配置
"""
import asyncio
import os
from dotenv import load_dotenv
from telethon import TelegramClient
from telethon.errors import SessionPasswordNeededError, PhoneCodeInvalidError
import socks

load_dotenv()

async def login():
    phone = os.getenv("TELEGRAM_PHONE_NUMBER")
    api_id = int(os.getenv("TELEGRAM_API_ID"))
    api_hash = os.getenv("TELEGRAM_API_HASH")
    password = os.getenv("TELEGRAM_PASSWORD", "")
    session_name = os.getenv("TELEGRAM_SESSION_NAME", "telegram_monitor_optimized")

    # 使用测试确认可用的代理
    proxy = (socks.HTTP, "127.0.0.1", 9910)

    print("=" * 70)
    print("🔐 Telegram 登录工具")
    print("=" * 70)
    print()
    print(f"📱 手机号: {phone}")
    print(f"🔧 代理: HTTP 127.0.0.1:9910")
    print(f"💾 会话: data/sessions/{session_name}.session")
    print()

    client = TelegramClient(
        f"data/sessions/{session_name}",
        api_id,
        api_hash,
        proxy=proxy
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
            print("✅ 无需重新认证,可以直接运行程序!")
            print()
            print("💡 运行: source .venv/bin/activate && python telegram_collector/jt_bot.py monitor")
            return

        # 发送验证码
        print("📤 正在发送验证码到 Telegram...")
        sent_code = await client.send_code_request(phone)

        code_type = type(sent_code.type).__name__
        print()
        print("=" * 70)
        print("📥 验证码发送成功!")
        print(f"   发送方式: {code_type}")

        if hasattr(sent_code.type, 'length'):
            print(f"   验证码长度: {sent_code.type.length} 位")

        print("=" * 70)
        print()

        if 'App' in code_type:
            print("⚠️  验证码发送到 Telegram 应用中 (不是短信!)")
            print()
            print("📱 查看验证码的方法:")
            print("   1. 打开手机/电脑上已登录的 Telegram 应用")
            print("   2. 查看 'Telegram' 或 '服务通知'")
            print("   3. 应该能看到一个5位数的验证码")
            print()
            print("💡 如果没有其他设备登录:")
            print("   - 等待60-120秒,Telegram可能会自动切换到短信")
            print("   - 或者在这里输入 's' 请求切换到短信")
        elif 'Sms' in code_type:
            print("✅ 短信验证码已发送!")
            print("📱 请检查手机短信")
        else:
            print(f"ℹ️  验证码类型: {code_type}")

        if sent_code.next_type:
            print(f"   可以输入 's' 切换到: {type(sent_code.next_type).__name__}")

        print()

        # 等待用户输入
        max_attempts = 5
        for attempt in range(max_attempts):
            try:
                user_input = input(f"📱 请输入验证码 (或 's'切换, 'r'重发) [{attempt + 1}/{max_attempts}]: ").strip().lower()

                # 切换发送方式
                if user_input == 's' and sent_code.next_type:
                    print()
                    print("🔄 正在切换验证码发送方式...")
                    sent_code = await client.resend_code(phone, sent_code.phone_code_hash)
                    new_type = type(sent_code.type).__name__
                    print(f"✅ 已切换到: {new_type}")

                    if 'Sms' in new_type:
                        print("📱 短信验证码已发送,请检查手机!")
                    elif 'Call' in new_type:
                        print("📞 将通过电话告知验证码!")
                    else:
                        print(f"ℹ️  方式: {new_type}")

                    print()
                    continue

                # 重新发送
                if user_input == 'r':
                    print()
                    print("🔄 正在重新发送...")
                    sent_code = await client.send_code_request(phone)
                    print(f"✅ 已重新发送! (方式: {type(sent_code.type).__name__})")
                    print()
                    continue

                code = user_input

                if not code or not code.isdigit():
                    print("❌ 请输入数字验证码!")
                    continue

                print("🔐 验证中...")
                await client.sign_in(phone, code)
                print("✅ 验证码正确!")
                break

            except PhoneCodeInvalidError:
                print(f"❌ 验证码错误! 剩余 {max_attempts - attempt - 1} 次")
                if attempt == max_attempts - 1:
                    print("❌ 失败次数过多,请稍后重试")
                    return

            except SessionPasswordNeededError:
                print()
                print("🔐 需要两步验证密码...")

                if password:
                    print("🔑 使用配置文件中的密码...")
                    try:
                        await client.sign_in(password=password)
                        print("✅ 两步验证通过!")
                        break
                    except Exception as e:
                        print(f"❌ 密码错误: {e}")

                pwd = input("🔐 请输入两步验证密码: ").strip()
                try:
                    await client.sign_in(password=pwd)
                    print("✅ 两步验证通过!")
                    break
                except Exception as e:
                    print(f"❌ 密码错误: {e}")
                    return

            except Exception as e:
                print(f"❌ 错误: {e}")
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
        print(f"   🔢 ID: {me.id}")
        print("=" * 70)
        print()
        print("💡 接下来:")
        print("   1. 会话已保存,下次无需重新登录")
        print("   2. 运行程序: source .venv/bin/activate && python telegram_collector/jt_bot.py monitor")
        print()

    except Exception as e:
        print()
        print(f"❌ 错误: {e}")
        import traceback
        traceback.print_exc()

    finally:
        await client.disconnect()
        print("👋 连接已关闭")

if __name__ == "__main__":
    asyncio.run(login())
