"""
Agent model and thread-safe registry for tracking multiple C2 agents.
"""

import threading
import time
from dataclasses import dataclass, field
from typing import Dict, Optional, List


# ─────────────────────────────────────────
# Agent Model
# ─────────────────────────────────────────

@dataclass
class Agent:
    """Represents a connected C2 agent (implant)."""
    agent_id:   str
    hostname:   str   = "unknown"
    os:         str   = "unknown"
    user:       str   = "unknown"
    ip:         str   = "unknown"
    channel:    str   = "unknown"          # current active channel
    first_seen: float = field(default_factory=time.time)
    last_seen:  float = field(default_factory=time.time)
    status:     str   = "active"           # active | dormant | dead

    def update_seen(self):
        self.last_seen = time.time()

    def elapsed(self) -> str:
        """Human-readable time since last seen."""
        delta = int(time.time() - self.last_seen)
        if delta < 60:
            return f"{delta}s ago"
        elif delta < 3600:
            return f"{delta // 60}m ago"
        else:
            return f"{delta // 3600}h {(delta % 3600) // 60}m ago"

    def summary(self) -> str:
        return (
            f"  ID:       {self.agent_id}\n"
            f"  Host:     {self.hostname}\n"
            f"  OS:       {self.os}\n"
            f"  User:     {self.user}\n"
            f"  IP:       {self.ip}\n"
            f"  Channel:  {self.channel}\n"
            f"  Status:   {self.status}\n"
            f"  Last seen: {self.elapsed()}"
        )


# ─────────────────────────────────────────
# Agent Registry (thread-safe)
# ─────────────────────────────────────────

class AgentRegistry:
    """Thread-safe registry for tracking multiple C2 agents."""

    def __init__(self):
        self._agents: Dict[str, Agent] = {}
        self._lock = threading.Lock()

    def register(self, agent_id: str, hostname: str = "unknown",
                 os_name: str = "unknown", user: str = "unknown",
                 ip: str = "unknown", channel: str = "unknown") -> Agent:
        """Register a new agent or update an existing one."""
        with self._lock:
            if agent_id in self._agents:
                agent = self._agents[agent_id]
                agent.hostname = hostname
                agent.os       = os_name
                agent.user     = user
                agent.ip       = ip
                agent.channel  = channel
                agent.status   = "active"
                agent.update_seen()
            else:
                agent = Agent(
                    agent_id=agent_id,
                    hostname=hostname,
                    os=os_name,
                    user=user,
                    ip=ip,
                    channel=channel,
                )
                self._agents[agent_id] = agent
            return agent

    def get(self, agent_id: str) -> Optional[Agent]:
        with self._lock:
            return self._agents.get(agent_id)

    def list_all(self) -> List[Agent]:
        with self._lock:
            return list(self._agents.values())

    def update_last_seen(self, agent_id: str, channel: str = None):
        with self._lock:
            agent = self._agents.get(agent_id)
            if agent:
                agent.update_seen()
                agent.status = "active"
                if channel:
                    agent.channel = channel

    def update_channel(self, agent_id: str, channel: str):
        """Update the active channel for an agent (after SWITCHACK)."""
        with self._lock:
            agent = self._agents.get(agent_id)
            if agent:
                agent.channel = channel
                agent.update_seen()

    def remove(self, agent_id: str):
        with self._lock:
            self._agents.pop(agent_id, None)

    def count(self) -> int:
        with self._lock:
            return len(self._agents)

    def auto_register(self, agent_id: str, channel: str) -> Agent:
        """Lightweight registration from a RESULT — no metadata yet."""
        with self._lock:
            if agent_id not in self._agents:
                agent = Agent(agent_id=agent_id, channel=channel)
                self._agents[agent_id] = agent
                return agent
            else:
                self._agents[agent_id].update_seen()
                self._agents[agent_id].channel = channel
                return self._agents[agent_id]
