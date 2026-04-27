package utils

import (
	"crypto/sha256"
	"encoding/base64"
)

// DeriveKey derives a 32-byte key from a shared secret using SHA-256.
func DeriveKey(secret string) []byte {
	hash := sha256.Sum256([]byte(secret))
	return hash[:]
}

// EncodeMessage takes a plaintext message and returns base64 encoding.
func EncodeMessage(message string, key []byte) (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(message)), nil
}

// DecodeMessage takes a base64 encoded message and returns the plaintext.
// Falls back to returning the original string if decoding fails.
func DecodeMessage(message string, key []byte) (string, error) {
	// Strip legacy "ENC|" if present
	if len(message) > 4 && message[:4] == "ENC|" {
		message = message[4:]
	}

	decoded, err := base64.StdEncoding.DecodeString(message)
	if err != nil {
		// Might not be base64 encoded, return as-is
		return message, nil
	}
	return string(decoded), nil
}

// Base64Encode is a convenience wrapper.
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode is a convenience wrapper.
func Base64Decode(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}
