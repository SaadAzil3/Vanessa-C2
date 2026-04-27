"""
Simple Base64 Protocol Encoding for Vanessa C2 Server.
Mirrors client/utils/encoding.go for cross-language compatibility.
"""

import base64
import hashlib
import logging

log = logging.getLogger("vanessa.crypto")

def derive_key(secret: str) -> bytes:
    """Derive a 32-byte key from a shared secret using SHA-256.
    Kept for compatibility with existing code."""
    return hashlib.sha256(secret.encode()).digest()

def encode_message(message: str, key: bytes) -> str:
    """Encode a protocol message using simple base64."""
    return base64.b64encode(message.encode('utf-8')).decode('utf-8')

def decode_message(message: str, key: bytes) -> str:
    """Decode a base64 encoded message back to original protocol format."""
    if isinstance(message, str) and message.startswith("ENC|"):
        message = message[4:]
        
    try:
        # Handle padding if needed
        padding = 4 - len(message) % 4
        if padding != 4:
            message += "=" * padding
            
        decoded = base64.b64decode(message)
        return decoded.decode('utf-8')
    except Exception:
        return message
