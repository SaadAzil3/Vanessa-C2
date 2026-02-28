package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
// Client
// ─────────────────────────────────────────

type Client struct {
	token  string
	chatID int64
	apiURL string
}

func NewClient(token string, chatID int64) *Client {
	return &Client{
		token:  token,
		chatID: chatID,
		apiURL: fmt.Sprintf("https://api.telegram.org/bot%s", token),
	}
}

// GetUpdates fetches new updates starting from offset.
func (c *Client) GetUpdates(offset int) ([]Update, error) {
	endpoint := fmt.Sprintf("%s/getUpdates", c.apiURL)

	params := url.Values{}
	params.Set("timeout", "10")
	params.Set("allowed_updates", `["message"]`)
	params.Set("offset", strconv.Itoa(offset)) // always send offset

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

// SendMessage sends a text message to the group.
func (c *Client) SendMessage(text string) error {
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
