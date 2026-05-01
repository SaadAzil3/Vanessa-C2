#!/bin/bash
# ═══════════════════════════════════════════
#  Vanessa C2 — Installer
#  One-command setup: git clone → make install
# ═══════════════════════════════════════════

set -e

# Colors
RED='\033[0;91m'
GREEN='\033[0;92m'
YELLOW='\033[0;93m'
CYAN='\033[0;96m'
BOLD='\033[1m'
RESET='\033[0m'

PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"

echo -e "${CYAN}"
echo "  ╔══════════════════════════════════════════════════════════════════╗"
echo "  ║                    Vanessa C2 — Installer                       ║"
echo "  ╚══════════════════════════════════════════════════════════════════╝"
echo -e "${RESET}"

# ── Step 1: Check dependencies ────────────
echo -e "${BOLD}[1/5] Checking dependencies...${RESET}"

# Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}[!] Docker is not installed.${RESET}"
    echo -e "    Install: https://docs.docker.com/get-docker/"
    exit 1
fi
echo -e "  ${GREEN}✓ Docker$(docker --version | grep -oP 'version \K[0-9.]+')${RESET}"

# Docker Compose
if ! docker compose version &> /dev/null; then
    echo -e "${RED}[!] Docker Compose (V2) is not installed.${RESET}"
    exit 1
fi
echo -e "  ${GREEN}✓ Docker Compose$(docker compose version --short 2>/dev/null)${RESET}"

# Go (optional — used for local builds if Docker builder unavailable)
if command -v go &> /dev/null; then
    echo -e "  ${GREEN}✓ Go $(go version | grep -oP 'go[0-9.]+')${RESET}"
else
    echo -e "  ${YELLOW}○ Go not found (optional — Docker will handle builds)${RESET}"
fi

echo ""

# ── Step 2: Setup .env files ─────────────
echo -e "${BOLD}[2/5] Setting up environment files...${RESET}"

if [ ! -f "$PROJECT_ROOT/server/.env" ]; then
    if [ -f "$PROJECT_ROOT/server/.env.example" ]; then
        cp "$PROJECT_ROOT/server/.env.example" "$PROJECT_ROOT/server/.env"
        echo -e "  ${GREEN}✓ Created server/.env from template${RESET}"
        echo -e "  ${YELLOW}  ⚠ EDIT server/.env with your Telegram/Discord API keys!${RESET}"
    else
        echo -e "  ${RED}✗ server/.env.example not found${RESET}"
        exit 1
    fi
else
    echo -e "  ${GREEN}✓ server/.env already exists${RESET}"
fi

echo ""

# ── Step 3: Create directories ────────────
echo -e "${BOLD}[3/5] Creating directories...${RESET}"

mkdir -p "$HOME/.vanessa/payloads"
echo -e "  ${GREEN}✓ ~/.vanessa/payloads/${RESET}"

echo ""

# ── Step 4: Build Docker images ──────────
echo -e "${BOLD}[4/5] Building Docker images...${RESET}"

echo -e "  ${CYAN}[*] Building C2 server image...${RESET}"
cd "$PROJECT_ROOT"
docker build -t vanessa-c2-server:latest -f "$PROJECT_ROOT/server/Dockerfile" "$PROJECT_ROOT/server"
echo -e "  ${GREEN}✓ Server image built${RESET}"

# Build the Go cross-compiler image for payload generation
echo -e "  ${CYAN}[*] Building agent compiler image...${RESET}"
docker build -t vanessa-agent-builder -f "$PROJECT_ROOT/client/Dockerfile.build" "$PROJECT_ROOT/client"
echo -e "  ${GREEN}✓ Agent compiler image built${RESET}"

echo ""

# ── Step 5: Install CLI ──────────────────
echo -e "${BOLD}[5/5] Installing 'vanessa' command...${RESET}"

# Inject the project root path into the CLI wrapper
CLI_SCRIPT="$PROJECT_ROOT/vanessa"
chmod +x "$CLI_SCRIPT"
chmod +x "$PROJECT_ROOT/generator/generate.sh"

# Replace the placeholder with the actual project root
sed -i "s|__VANESSA_ROOT__|$PROJECT_ROOT|g" "$CLI_SCRIPT"

# Symlink to /usr/local/bin
if [ -L /usr/local/bin/vanessa ] || [ -f /usr/local/bin/vanessa ]; then
    sudo rm -f /usr/local/bin/vanessa
fi
sudo ln -s "$CLI_SCRIPT" /usr/local/bin/vanessa

echo -e "  ${GREEN}✓ 'vanessa' command installed → /usr/local/bin/vanessa${RESET}"

echo ""
echo -e "${GREEN}  ╔══════════════════════════════════════════════════════════════════╗"
echo -e "  ║                 ✓ Installation Complete!                         ║"
echo -e "  ╠══════════════════════════════════════════════════════════════════╣"
echo -e "  ║                                                                  ║"
echo -e "  ║  1. Edit server/.env with your API keys                         ║"
echo -e "  ║  2. vanessa server         — start C2 server                    ║"
echo -e "  ║  3. vanessa attach         — open operator console              ║"
echo -e "  ║  4. vanessa generate       — generate agent payload             ║"
echo -e "  ║                                                                  ║"
echo -e "  ╚══════════════════════════════════════════════════════════════════╝${RESET}"
echo ""
