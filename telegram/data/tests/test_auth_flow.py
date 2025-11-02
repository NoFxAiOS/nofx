import os
import unittest
from pathlib import Path
from unittest.mock import patch

os.environ.setdefault("TELEGRAM_API_ID", "123456")
os.environ.setdefault("TELEGRAM_API_HASH", "testhash")
os.environ.setdefault("TELEGRAM_PHONE_NUMBER", "+10000000000")
os.environ.setdefault("TELEGRAM_SESSION_NAME", "test_session_force")
os.environ.setdefault("DATABASE_PATH", "./data/test_force.db")

from telegram_collector import jt_bot  # noqa: E402


class _SentCodeTypeAppDummy:
    length = 5


class _SentCodeTypeSmsDummy:
    length = 5


class _SentCode:
    def __init__(self, type_obj, *, next_type=None):
        self.type = type_obj
        self.phone_code_hash = "hash123"
        self.next_type = next_type


class _FakeClient:
    def __init__(self):
        self.force_sms_calls = []
        self.sign_in_calls = []

    async def connect(self):
        return None

    async def disconnect(self):
        return None

    async def is_user_authorized(self):
        return False

    async def send_code_request(self, phone, *, force_sms=False):
        self.force_sms_calls.append(force_sms)
        if force_sms:
            return _SentCode(_SentCodeTypeSmsDummy())
        return _SentCode(_SentCodeTypeAppDummy())

    async def resend_code(self, phone, phone_code_hash):
        return _SentCode(_SentCodeTypeSmsDummy())

    async def sign_in(self, phone, code=None, password=None):
        self.sign_in_calls.append((phone, code, password))
        if code == "12345" or password == "pwd":
            return True
        raise ValueError("invalid code")

    async def get_me(self):
        class Dummy:
            first_name = "Tester"
            username = "tester"
            phone = "+10000000000"
            id = 1

        return Dummy()


class AuthFlowTests(unittest.IsolatedAsyncioTestCase):
    async def test_force_sms_option_triggers(self):
        fake_client = _FakeClient()
        original_session = jt_bot.config.telegram.session_name
        original_phone = jt_bot.config.telegram.phone_number
        jt_bot.config.telegram.session_name = "test_session_auth_flow"
        jt_bot.config.telegram.phone_number = "+10000000000"

        async def run_auth():
            with patch("telegram_collector.jt_bot.TelegramClient", return_value=fake_client):
                with patch("builtins.input", side_effect=["f", "12345"]):
                    return await jt_bot.authenticate_telegram()

        try:
            result = await run_auth()
            self.assertTrue(result)
            self.assertGreaterEqual(len(fake_client.force_sms_calls), 2)
            self.assertIn(True, fake_client.force_sms_calls)
            self.assertIn(False, fake_client.force_sms_calls)
            expected_phone = jt_bot.config.telegram.phone_number
            self.assertIn((expected_phone, "12345", None), fake_client.sign_in_calls)
        finally:
            session_base = Path(jt_bot.config.get_session_path())
            suffixes = ["", ".session", ".session-journal", ".session-wal", ".session-shm"]
            for suffix in suffixes:
                candidate = Path(f"{session_base}{suffix}")
                if candidate.exists():
                    candidate.unlink()
            jt_bot.config.telegram.session_name = original_session
            jt_bot.config.telegram.phone_number = original_phone


if __name__ == "__main__":
    unittest.main()
