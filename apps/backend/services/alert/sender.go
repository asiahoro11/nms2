// Made by YTSworks
// YTS工作室製作
package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"
)

// SendAlert dispatches an alert message to the specified channel
func SendAlert(alertType string, configJSON string, message string) error {
	switch alertType {
	case "email":
		return sendEmail(configJSON, message)
	case "line":
		return sendLine(configJSON, message)
	case "telegram":
		return sendTelegram(configJSON, message)
	case "discord":
		return sendDiscord(configJSON, message)
	case "slack":
		return sendSlack(configJSON, message)
	case "whatsapp":
		return fmt.Errorf("WhatsApp integration not implemented yet")
	default:
		return fmt.Errorf("unknown alert type: %s", alertType)
	}
}

// ... existing email/line/telegram ...

// --- Discord Sender ---

type DiscordConfig struct {
	WebhookURL string `json:"webhook_url"`
}

func sendDiscord(configJSON string, message string) error {
	var cfg DiscordConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("invalid discord config: %v", err)
	}

	if cfg.WebhookURL == "" {
		return fmt.Errorf("missing discord webhook url")
	}

	payload := map[string]string{"content": message}
	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", cfg.WebhookURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord api returned status: %d", resp.StatusCode)
	}
	return nil
}

// --- Slack Sender ---

type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
}

func sendSlack(configJSON string, message string) error {
	var cfg SlackConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("invalid slack config: %v", err)
	}

	if cfg.WebhookURL == "" {
		return fmt.Errorf("missing slack webhook url")
	}

	payload := map[string]string{"text": message}
	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", cfg.WebhookURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack api returned status: %d", resp.StatusCode)
	}
	return nil
}

// --- Email Sender ---

type EmailConfig struct {
	SMTPHost    string `json:"smtp_host"`
	SMTPPort    int    `json:"smtp_port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	FromAddress string `json:"from_address"`
	ToAddresses string `json:"to_addresses"` // Comma separated
}

func sendEmail(configJSON string, message string) error {
	var cfg EmailConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("invalid email config: %v", err)
	}

	if cfg.SMTPHost == "" || cfg.FromAddress == "" || cfg.ToAddresses == "" {
		return fmt.Errorf("missing required email config fields")
	}

	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)

	// Prepare headers (RFC 2822 compliant)
	subject := "Subject: NMS Alert Notification\r\n"
	mime := "MIME-version: 1.0;\r\nContent-Type: text/plain; charset=\"UTF-8\";\r\n\r\n"
	body := subject + mime + message

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	to := splitCSV(cfg.ToAddresses)

	err := smtp.SendMail(addr, auth, cfg.FromAddress, to, []byte(body))
	if err != nil {
		return fmt.Errorf("smtp send failed: %v", err)
	}
	return nil
}

// --- LINE Sender ---

type LineConfig struct {
	AccessToken string `json:"access_token"`
}

func sendLine(configJSON string, message string) error {
	var cfg LineConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("invalid line config: %v", err)
	}

	if cfg.AccessToken == "" {
		return fmt.Errorf("missing line access token")
	}

	apiURL := "https://notify-api.line.me/api/notify"

	form := url.Values{}
	form.Set("message", message)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+cfg.AccessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("line api returned status: %d", resp.StatusCode)
	}
	return nil
}

// --- Telegram Sender ---

type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

func sendTelegram(configJSON string, message string) error {
	var cfg TelegramConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("invalid telegram config: %v", err)
	}

	if cfg.BotToken == "" || cfg.ChatID == "" {
		return fmt.Errorf("missing telegram bot token or chat id")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	payload := map[string]string{
		"chat_id": cfg.ChatID,
		"text":    message,
	}
	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("telegram api returned status: %d", resp.StatusCode)
	}
	return nil
}

// Helper
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	var res []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}

// SendDirectEmail sends an email to a specific recipient using the configured SMTP settings
func SendDirectEmail(configJSON string, toEmail string, subject string, message string) error {
	var cfg EmailConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("invalid email config: %v", err)
	}

	if cfg.SMTPHost == "" || cfg.FromAddress == "" {
		return fmt.Errorf("missing required email config fields (SMTPHost or FromAddress)")
	}

	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)

	// Prepare headers (RFC 2822 compliant)
	headerSubject := fmt.Sprintf("Subject: %s\r\n", subject)
	mime := "MIME-version: 1.0;\r\nContent-Type: text/plain; charset=\"UTF-8\";\r\n\r\n"
	body := headerSubject + mime + message

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	to := []string{toEmail}

	err := smtp.SendMail(addr, auth, cfg.FromAddress, to, []byte(body))
	if err != nil {
		return fmt.Errorf("smtp send failed: %v", err)
	}
	return nil
}
