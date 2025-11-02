#!/bin/bash
cd "$(dirname "$0")"
source .venv/bin/activate

python3 << 'PYTHON_EOF'
import asyncio, os, socks
from dotenv import load_dotenv
from telethon import TelegramClient
from telethon.errors import SessionPasswordNeededError, PhoneCodeInvalidError

load_dotenv()

async def main():
    phone = os.getenv("TELEGRAM_PHONE_NUMBER")
    api_id = int(os.getenv("TELEGRAM_API_ID"))
    api_hash = os.getenv("TELEGRAM_API_HASH")
    password = os.getenv("TELEGRAM_PASSWORD", "")
    
    proxy = (socks.HTTP, "127.0.0.1", 9910)
    client = TelegramClient("data/sessions/telegram_monitor_optimized", api_id, api_hash, proxy=proxy)
    
    await client.connect()
    
    if await client.is_user_authorized():
        me = await client.get_me()
        print(f"✅ 已登录: {me.first_name}")
        await client.disconnect()
        return
    
    sent_code = await client.send_code_request(phone)
    print(f"\n验证码已发送 (类型: {type(sent_code.type).__name__})")
    print(f"请在Telegram APP中查看验证码\n")
    
    for i in range(3):
        code = input(f"请输入验证码 [{i+1}/3]: ").strip()
        try:
            await client.sign_in(phone, code)
            break
        except PhoneCodeInvalidError:
            print("❌ 验证码错误")
        except SessionPasswordNeededError:
            pwd = password or input("需要两步验证密码: ").strip()
            await client.sign_in(password=pwd)
            break
    
    me = await client.get_me()
    print(f"\n✅ 登录成功: {me.first_name} (@{me.username})")
    await client.disconnect()

asyncio.run(main())
PYTHON_EOF
