# ═══════════════════════════════════════════
#  Vanessa C2 — Makefile
# ═══════════════════════════════════════════

.PHONY: help install uninstall

help:
	@echo ""
	@echo "  ╔══════════════════════════════════════════════╗"
	@echo "  ║  Vanessa C2 — Setup                         ║"
	@echo "  ╠══════════════════════════════════════════════╣"
	@echo "  ║  make install     — full install + CLI       ║"
	@echo "  ║  make uninstall   — remove CLI from system   ║"
	@echo "  ╚══════════════════════════════════════════════╝"
	@echo ""
	@echo "  After install, use the 'vanessa' command:"
	@echo "    vanessa server    — start C2 server"
	@echo "    vanessa attach    — operator console"
	@echo "    vanessa generate  — generate agent payload"
	@echo "    vanessa stop      — stop server"
	@echo ""

install:
	@chmod +x install.sh
	@bash install.sh

uninstall:
	@echo "[*] Removing vanessa CLI..."
	@sudo rm -f /usr/local/bin/vanessa
	@echo "[✓] Uninstalled. Project files preserved."
