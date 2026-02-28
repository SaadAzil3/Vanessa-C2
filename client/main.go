package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"telegram-client/telegram"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("❌ Error loading .env file")
	}

	token     := os.Getenv("BOT_TOKEN")
	chatIDStr := os.Getenv("CHAT_ID")

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		log.Fatalf("❌ Invalid CHAT_ID: %v", err)
	}

	client := telegram.NewClient(token, chatID)
	fmt.Println("🤖 Go client started. Waiting for commands...")

	offset := -1

	for {
		updates, err := client.GetUpdates(offset)
		if err != nil {
			log.Printf("❌ Error fetching updates: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1

			msg := update.Message
			if msg.Text == "" || msg.Chat.ID != chatID {
				continue
			}

			if !strings.HasPrefix(msg.Text, "INSTRUCTION|") {
				continue
			}

			parts := strings.SplitN(msg.Text, "|", 3)
			if len(parts) != 3 {
				continue
			}

			instructionID := parts[1]
			command        := parts[2]

			fmt.Printf("📥 [%s] $ %s\n", instructionID, command)

			// Execute in goroutine — don't block polling
			go func(id, cmd string) {
				output := executeCommand(cmd)

				// Telegram has a 4096 char message limit — truncate if needed
				if len(output) > 3800 {
					output = output[:3800] + "\n... (truncated)"
				}

				result := fmt.Sprintf("RESULT|%s|%s", id, output)
				if err := client.SendMessage(result); err != nil {
					log.Printf("❌ Failed to send result: %v", err)
				} else {
					fmt.Printf("✅ [%s] Result sent\n", id)
				}
			}(instructionID, command)
		}
	}
}

// executeCommand runs a shell command and returns its output (stdout + stderr)
func executeCommand(cmd string) string {
	var command *exec.Cmd

	// Support both Linux/Mac and Windows
	if runtime.GOOS == "windows" {
		command = exec.Command("cmd", "/C", cmd)
	} else {
		command = exec.Command("sh", "-c", cmd)
	}

	// Capture both stdout and stderr
	out, err := command.CombinedOutput()

	if err != nil {
		// Still return output even if command failed (e.g. non-zero exit)
		if len(out) > 0 {
			return fmt.Sprintf("%s\n⚠️ Exit error: %s", strings.TrimSpace(string(out)), err.Error())
		}
		return fmt.Sprintf("❌ Error: %s", err.Error())
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return "✅ Command executed (no output)"
	}
	return output
}
