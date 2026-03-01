# Vanissa C2

<p align="center">
  <img src="./assets/README-img/vanissa-c2.jpeg" alt="Vanissa C2 Logo" width="400"/>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-in%20development-yellow" alt="Status"/>
  <img src="https://img.shields.io/badge/python-3.13-blue" alt="Python"/>
  <img src="https://img.shields.io/badge/go-1.21+-00ADD8" alt="Go"/>
  <img src="https://img.shields.io/badge/platform-windows-blue" alt="Platform"/>
  <img src="https://img.shields.io/badge/license-MIT-green" alt="License"/>
</p>

> ⚠️ **This project is under active development. Features may be incomplete or change at any time.**

---

## What is Vanissa C2?

**Vanissa C2** is a Command & Control framework that uses **Telegram as its communication channel**. The server sends shell commands through a Telegram group, and the client receives, executes them, and sends the results back — all over Telegram's infrastructure.

Built as a learning project to explore C2 architecture, the Telegram Bot API, MTProto, and Go/Python interoperability.

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                                                         │
│   [Operator]                                            │
│   shell> whoami                                         │
│       │                                                 │
│       ▼                                                 │
│   [Flask Server]  ──INSTRUCTION|id|cmd──►  [Telegram]  │
│       ▲                                        │        │
│       │                                        ▼        │
│   RESULT|id|output  ◄──────────────  [Go Client]       │
│                                      (executes cmd)     │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

- **Server** (Python + Flask + Telethon): Interactive shell that sends commands to a Telegram group as a real user account
- **Client** (Go): Polls the group, executes received commands via `sh -c`, sends results back as a bot
- **Channel**: A private Telegram supergroup used as the communication medium

---

## Features

- [x] Interactive remote shell from the server terminal
- [x] Real-time command execution on the client machine
- [x] Results delivered back to the operator automatically
- [x] Telegram as C2 channel (blends into normal traffic)
- [x] Concurrent command handling via goroutines
- [x] Telethon MTProto for server-side messaging (bypasses bot restrictions)
- [ ] Multiple client support
- [ ] File upload / download
- [ ] Screenshot capture
- [ ] Persistence mechanism
- [ ] Encrypted payloads
- [ ] Client authentication & ID verification
- [ ] Web dashboard

---

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Server | Python 3.13, Flask, Telethon |
| Client | Go 1.21+ |
| C2 Channel | Telegram (MTProto + Bot API) |
---

---

## Prerequisites

- Python 3.10+
- Go 1.21+
- A Telegram account
- Two Telegram bots (created via [@BotFather](https://t.me/BotFather))
- Telegram API credentials from [my.telegram.org/apps](https://my.telegram.org/apps)

---

## Setup

### 1. Clone the repository

```bash
git clone https://github.com/SaadAzil3/Vanissa-C2.git
cd vanissa-c2
```

### 2. Create a Telegram group

- Create a private Telegram supergroup
- Add both bots to the group
- Disable privacy mode for both bots via @BotFather → Bot Settings → Group Privacy → **Turn OFF**
- Make both bots admins with send message permissions
- Get your group chat ID (negative number, e.g. `-1003728125166`)

### 3. Get Telegram API credentials

Go to [my.telegram.org/apps](https://my.telegram.org/apps), log in, create an app and grab your `api_id` and `api_hash`.

### 4. Configure the server

```bash
cd server
pip install -r requirements.txt
```

Create `server/.env`:
```env
API_ID=12345678
API_HASH=your_api_hash_here
PHONE=+xxxxxxxxx
CHAT_ID=-1003728125166
BOT_TOKEN=111:SERVER_BOT_TOKEN
```

### 5. Configure the client

```bash
cd client
go mod tidy
```

Create `client/.env`:
```env
BOT_TOKEN=222:CLIENT_BOT_TOKEN
CHAT_ID=-1003728125166
```

---

## Usage

### Start the client (on target machine)

```bash
cd client
go run main.go
```

```
Go client started. Waiting for commands...
Polling... got 0 updates
```

### Start the server (on operator machine)

```bash
cd server
python app.py
```

On first run, Telegram will ask you to authenticate with your phone number and code. A session file (`server_session.session`) will be saved for future runs.

```
Telethon user client connected!
Telethon polling loop started...

==================================================
REMOTE SHELL — type commands to execute on client
Type 'exit' to quit
==================================================

shell>
```

### Execute commands

```bash
shell> whoami
Output:
azil

shell> uname -a
Output:
Linux kali 6.x.x-kali #1 SMP PREEMPT_DYNAMIC ...

shell> ls /etc/passwd
Output:
/etc/passwd

shell> exit
Goodbye!
```

---

## How It Works

The communication protocol uses a simple pipe-delimited format:

| Direction | Format | Example |
|-----------|--------|---------|
| Server → Client | `INSTRUCTION\|id\|command` | `INSTRUCTION\|a3f9c1b2\|whoami` |
| Client → Server | `RESULT\|id\|output` | `RESULT\|a3f9c1b2\|azil` |

The `id` field ties each result back to its instruction, allowing the server to match responses correctly even under concurrency.

---

## Important Notes

> This tool is intended for **educational purposes** and **authorized penetration testing only**. Use it only on systems you own or have explicit written permission to test. Unauthorized use is illegal and unethical.

---

## Author

**SA3D00N** — Cybersecurity student

---
