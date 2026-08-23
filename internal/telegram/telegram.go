// Package telegram sends alerts through the Telegram Bot API.
package telegram

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ooblivion/internal/logx"
)

type Job struct {
	RuleID    int64
	RequestID int64
	RuleName  string
	Text      string
}

type Sender struct {
	token    string
	chatID   string
	threadID string
	db       *sql.DB
	jobs     chan Job
	logger   *logx.Logger
	client   *http.Client
}

func New(token, chatID, threadID string, db *sql.DB, logger *logx.Logger) *Sender {
	s := &Sender{
		token:    token,
		chatID:   chatID,
		threadID: threadID,
		db:       db,
		jobs:     make(chan Job, 256),
		logger:   logger,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
	if token != "" {
		go s.run()
	}
	return s
}

func (s *Sender) Enqueue(j Job) {
	if s.token == "" {
		return
	}
	select {
	case s.jobs <- j:
	default:
		s.logger.Warnf("telegram queue full, dropping alert for request %d", j.RequestID)
	}
}

func (s *Sender) run() {
	for j := range s.jobs {
		s.send(j, s.chatID)
	}
}

func (s *Sender) SendTest(chatID string) error {
	if s.token == "" {
		return fmt.Errorf("telegram disabled: TELEGRAM_BOT_TOKEN not set")
	}
	if chatID == "" {
		return fmt.Errorf("telegram chat id not set")
	}
	return s.post(chatID, "ooblivion test message - telegram notifications are working", 0, 0, "")
}

func (s *Sender) send(j Job, chatID string) {
	if chatID == "" {
		s.logEvent(j, false, "telegram chat id not set")
		return
	}
	if err := s.post(chatID, j.Text, j.RuleID, j.RequestID, j.RuleName); err != nil {
		s.logEvent(j, false, err.Error())
	}
}

func (s *Sender) post(chatID, text string, ruleID, requestID int64, ruleName string) error {
	payload := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}
	if s.threadID != "" {
		payload["message_thread_id"] = s.threadID
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := "https://api.telegram.org/bot" + s.token + "/sendMessage"
	resp, err := s.client.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		s.logger.Errorf("telegram send error: %v", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body := make([]byte, 256)
		n, _ := resp.Body.Read(body)
		return fmt.Errorf("telegram status %d: %s", resp.StatusCode, strings.TrimSpace(string(body[:n])))
	}
	return nil
}

func (s *Sender) logEvent(j Job, ok bool, response string) {
	_, err := s.db.Exec(
		"INSERT INTO notification_log (rule_id, request_id, ok, response, sent_at) VALUES (?, ?, ?, ?, ?)",
		nullInt(j.RuleID), nullInt(j.RequestID), boolToInt(ok), response, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		s.logger.Errorf("notification_log insert error: %v", err)
	}
}

func nullInt(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
