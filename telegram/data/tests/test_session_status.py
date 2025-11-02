import os
import unittest
from unittest.mock import patch

# Preload minimal environment variables before importing the module under test.
os.environ.setdefault("TELEGRAM_API_ID", "123456")
os.environ.setdefault("TELEGRAM_API_HASH", "testhash")
os.environ.setdefault("TELEGRAM_PHONE_NUMBER", "+10000000000")
os.environ.setdefault("TELEGRAM_SESSION_NAME", "test_session")
os.environ.setdefault("DATABASE_PATH", "./data/test.db")

from telegram_collector import jt_bot  # noqa: E402  pylint: disable=wrong-import-position


class _DummyClient:
    async def disconnect(self):
        return None


class SessionStatusTests(unittest.IsolatedAsyncioTestCase):
    async def test_run_session_status_authorized(self):
        async def fake_init_client(self):
            self.client = _DummyClient()
            return True

        with patch.object(jt_bot.SimpleTelegramMonitor, "init_client", new=fake_init_client):
            result = await jt_bot.run_session_status()
            self.assertEqual(result, 0)

    async def test_run_session_status_requires_auth(self):
        async def fake_init_client(_self):
            raise jt_bot.UnauthorizedSessionError("no session")

        with patch.object(jt_bot.SimpleTelegramMonitor, "init_client", new=fake_init_client):
            result = await jt_bot.run_session_status()
            self.assertEqual(result, 2)


if __name__ == "__main__":
    unittest.main()
