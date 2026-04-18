# Vanessa C2

<p align="center">
  <img src="./assets/README-img/vanissa-c2.jpeg" alt="Vanessa C2 Logo" width="400"/>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-in%20development-yellow" alt="Status"/>
  <img src="https://img.shields.io/badge/python-3.10+-blue" alt="Python"/>
  <img src="https://img.shields.io/badge/go-1.21+-00ADD8" alt="Go"/>
  <img src="https://img.shields.io/badge/platform-windows%20%7C%20linux-orange" alt="Platform"/>
  <img src="https://img.shields.io/badge/license-MIT-green" alt="License"/>
  <img src="https://img.shields.io/badge/encryption-AES--256--GCM-red" alt="Encryption"/>
</p>

> ⚠️ **Educational PoC only.** This project is part of a master's thesis on *Living off Trusted Services* (LoTS) C2 evasion techniques. Use only on systems you own or have explicit written permission to test.

---

## What is Vanessa C2?

**Vanessa C2** is a proof-of-concept Command & Control framework that leverages **Cloud-based and Public Legitimate Services (CPLS)** — specifically **Telegram** and **Discord** — as its communication channels. All C2 traffic rides on legitimate HTTPS APIs, making it indistinguishable from normal platform usage to network defenders.

### Key Capabilities

- **Multi-channel C2** — Telegram + Discord with runtime channel switching
- **AES-256-GCM encryption** — All protocol messages encrypted end-to-end
- **Jitter & evasion** — Randomized beacon intervals to defeat NDR fingerprinting
- **Persistent state** — SQLite-backed agent registry survives server restarts
- **File exfiltration** — Download files from targets
- **Interactive shell** — Full operator console with agent selection
- **Agent health monitoring** — Automatic reaper marks dead agents

---

## Architecture
<img src="./assets/README-img/diagram.png" alt="Vanessa C2 Logo" width="600"/>

---

## Tech Stack

| Component | Technology |
|-----------|------------|
| **Server** | Python 3.10+, Flask, Telethon (MTProto), discord.py |
| **Client** | Go 1.21+, discordgo |
| **Database** | SQLite (`vanessa.db`) |
| **Encryption** | AES-256-GCM (SHA-256 key derivation) |
| **Channels** | Telegram Bot API, Discord Bot API |

---

## Project Structure

```
vanessa-c2/
├── server/
│   ├── app.py                  # Operator console + Flask API
│   ├── core/
│   │   ├── agent.py            # Agent registry + reaper
│   │   ├── channel.py          # Abstract C2Channel interface
│   │   ├── crypto.py           # AES-256-GCM encryption (Python)
│   │   └── database.py         # SQLite persistence layer
│   ├── channels/
│   │   ├── telegram.py         # Telegram channel (Telethon)
│   │   └── discord.py          # Discord channel (discord.py)
│   ├── .env                    # API credentials + C2_SECRET
│   └── requirements.txt
│
├── client/
│   ├── main.go                 # Agent entry point + .env loader
│   ├── core/
│   │   ├── config.go           # AgentConfig (jitter, encryption key)
│   │   ├── identity.go         # Deterministic agent ID generation
│   │   ├── interfaces.go       # C2Channel interface
│   │   └── handlers.go         # Shared command handlers
│   ├── channels/
│   │   ├── telegram/client.go  # Telegram channel (HTTP polling)
│   │   └── discord/client.go   # Discord channel (discordgo)
│   ├── utils/
│   │   └── encoding.go         # AES-256-GCM encryption (Go)
│   ├── .env                    # Bot tokens + C2_SECRET
│   ├── go.mod / go.sum
│
│
└── assets/
    └── README-img/
```

---

## Setup

### Prerequisites

- Python 3.10+ with `pip`
- Go 1.21+
- A Telegram account + API credentials from [my.telegram.org/apps](https://my.telegram.org/apps)
- A Telegram bot (via [@BotFather](https://t.me/BotFather))
- A Discord bot + server (via [Discord Developer Portal](https://discord.com/developers/applications))

### 1. Clone

```bash
git clone https://github.com/SaadAzil3/Vanessa-C2.git
cd Vanessa-C2
```

### 2. Server Setup

```bash
cd server
python -m venv env
source env/bin/activate
pip install -r requirements.txt
```

Create `server/.env`:
```env
API_ID=12345678
API_HASH=your_api_hash
PHONE=+213xxxxxxxxx
BOT_TOKEN=your_telegram_bot_token
CHAT_ID=-100xxxxxxxxxx
DISCORD_TOKEN=your_discord_bot_token
DISCORD_CHANNEL_ID=your_discord_channel_id
C2_SECRET=your_shared_encryption_key
```

### 3. Client Setup

```bash
cd client
go mod tidy
```

Create `client/.env`:
```env
BOT_TOKEN=your_telegram_bot_token
CHAT_ID=-100xxxxxxxxxx
DISCORD_TOKEN=your_discord_bot_token
C2_SECRET=your_shared_encryption_key
```

> ⚠️ The `C2_SECRET` **must match** on both server and client for encrypted comms to work.

---

## Usage

### Start the Server

```bash
cd server
source env/bin/activate
python app.py
```

```
[+] Encryption: ENABLED (AES-256-GCM)
[+] Telegram channel configured
[+] Discord channel configured
[+] Flask API running on http://127.0.0.1:5000
[+] Active channels: ['telegram', 'discord']

╔══════════════════════════════════════════╗
║  VANESSA C2 — Operator Console           ║
║  Type 'help' for available commands      ║
╚══════════════════════════════════════════╝

vanessa>
```

### Start the Agent

```bash
cd client
go run main.go
```

```
Encryption: ENABLED (AES-256-GCM)
Agent ID: abc123de
Telegram channel available
Discord channel available
```

### Operator Commands

| Command | Description |
|---------|-------------|
| `agents` | List all connected agents |
| `use <id>` | Select an agent to interact with |
| `back` | Deselect current agent |
| `switch <channel>` | Switch agent to telegram/discord |
| `sleep <seconds>` | Put agent to sleep |
| `jitter <min> <max>` | Set randomized beacon interval |
| `download <path>` | Exfiltrate a file from target |
| `sysinfo` | Get system information |
| `ifconfig` / `netstat` | Network info (auto-detects OS) |
| `persist` | Install persistence mechanism |
| `kill` | Terminate the agent |
| `exit` | Shut down the server |
| `<any command>` | Execute shell command on target |

---

## Disclaimer

This tool is intended for **educational purposes** and **authorized penetration testing only**. It was developed as part of a master's thesis on network evasion techniques using legitimate cloud services. Unauthorized use against systems you do not own or have explicit permission to test is **illegal and unethical**.

---

## Author

**Saad Azil** — Cybersecurity student

---

## License

MIT License — see [LICENSE](LICENSE) for details.
