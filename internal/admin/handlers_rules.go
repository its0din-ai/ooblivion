package admin

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ooblivion/internal/models"
)

var matchOns = []string{"host", "path", "query", "method", "header", "body", "user_agent", "source_ip"}
var matchTypes = []string{"contains", "equals", "prefix", "suffix", "regex", "exists"}

type rulePayload struct {
	Name       string  `json:"name"`
	MatchOn    string  `json:"match_on"`
	MatchType  string  `json:"match_type"`
	Pattern    string  `json:"pattern"`
	HeaderName *string `json:"header_name"`
	Enabled    *bool   `json:"enabled"`
	Priority   *int    `json:"priority"`
}

func (p rulePayload) validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if !contains(matchOns, p.MatchOn) {
		return fmt.Errorf("invalid match_on")
	}
	if !contains(matchTypes, p.MatchType) {
		return fmt.Errorf("invalid match_type")
	}
	if strings.TrimSpace(p.Pattern) == "" && p.MatchType != "exists" {
		return fmt.Errorf("pattern is required")
	}
	if p.MatchOn == "header" && (p.HeaderName == nil || strings.TrimSpace(*p.HeaderName) == "") {
		return fmt.Errorf("header_name is required for header match")
	}
	if p.MatchType == "regex" {
		if _, err := regexp.Compile(p.Pattern); err != nil {
			return fmt.Errorf("invalid regex: %v", err)
		}
	}
	return nil
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func (s *Server) handleScopesPage(w http.ResponseWriter, r *http.Request) {
	items, err := s.listScopes()
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	s.render(w, "scopes", map[string]any{"Title": "Scopes", "Items": items})
}

func (s *Server) handleNotificationsPage(w http.ResponseWriter, r *http.Request) {
	items, err := s.listRules()
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	s.render(w, "notifications", map[string]any{
		"Title":   "Notifications",
		"Items":   items,
		"Enabled": s.cfg.TelegramToken != "" && s.cfg.TelegramChatID != "",
	})
}

func (s *Server) handleListScopes(w http.ResponseWriter, r *http.Request) {
	items, err := s.listScopes()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateScope(w http.ResponseWriter, r *http.Request) {
	var p rulePayload
	if err := readJSON(r, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid body"})
		return
	}
	if err := p.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	priority := 0
	enabled := true
	if p.Priority != nil {
		priority = *p.Priority
	}
	if p.Enabled != nil {
		enabled = *p.Enabled
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var headerName any
	if p.HeaderName != nil {
		headerName = *p.HeaderName
	}
	res, err := s.db.Exec(
		`INSERT INTO scopes (name, match_on, match_type, pattern, header_name, enabled, priority, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.MatchOn, p.MatchType, p.Pattern, headerName, boolToInt64(enabled), priority, now, now,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}
	id, _ := res.LastInsertId()
	s.logAudit("scope_create", fmt.Sprintf("scope %d %q created", id, p.Name), authClientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

func (s *Server) handleUpdateScope(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	var p rulePayload
	if err := readJSON(r, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid body"})
		return
	}
	if err := p.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	priority := 0
	enabled := true
	if p.Priority != nil {
		priority = *p.Priority
	}
	if p.Enabled != nil {
		enabled = *p.Enabled
	}
	var headerName any
	if p.HeaderName != nil {
		headerName = *p.HeaderName
	}
	if _, err := s.db.Exec(
		`UPDATE scopes SET name = ?, match_on = ?, match_type = ?, pattern = ?, header_name = ?,
		 enabled = ?, priority = ?, updated_at = ? WHERE id = ?`,
		p.Name, p.MatchOn, p.MatchType, p.Pattern, headerName, boolToInt64(enabled), priority,
		time.Now().UTC().Format(time.RFC3339), id,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}
	s.logAudit("scope_update", fmt.Sprintf("scope %d updated", id), authClientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteScope(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	if _, err := s.db.Exec("DELETE FROM scopes WHERE id = ?", id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}
	s.logAudit("scope_delete", fmt.Sprintf("scope %d deleted", id), authClientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) listScopes() ([]models.Scope, error) {
	rows, err := s.db.Query(
		`SELECT id, name, match_on, match_type, pattern, header_name, enabled, priority, created_at, updated_at
		 FROM scopes ORDER BY priority DESC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []models.Scope{}
	for rows.Next() {
		var sc models.Scope
		var headerName sqlNullString
		var enabled int
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.MatchOn, &sc.MatchType, &sc.Pattern, &headerName,
			&enabled, &sc.Priority, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, err
		}
		sc.Enabled = enabled == 1
		if headerName.Valid {
			sc.HeaderName = &headerName.String
		}
		items = append(items, sc)
	}
	return items, rows.Err()
}

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	items, err := s.listRules()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var p rulePayload
	if err := readJSON(r, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid body"})
		return
	}
	if err := p.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	enabled := true
	if p.Enabled != nil {
		enabled = *p.Enabled
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var headerName any
	if p.HeaderName != nil {
		headerName = *p.HeaderName
	}
	res, err := s.db.Exec(
		`INSERT INTO notification_rules (name, match_on, match_type, pattern, header_name, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.MatchOn, p.MatchType, p.Pattern, headerName, boolToInt64(enabled), now, now,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}
	id, _ := res.LastInsertId()
	s.logAudit("rule_create", fmt.Sprintf("rule %d %q created", id, p.Name), authClientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

func (s *Server) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	var p rulePayload
	if err := readJSON(r, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid body"})
		return
	}
	if err := p.validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	enabled := true
	if p.Enabled != nil {
		enabled = *p.Enabled
	}
	var headerName any
	if p.HeaderName != nil {
		headerName = *p.HeaderName
	}
	if _, err := s.db.Exec(
		`UPDATE notification_rules SET name = ?, match_on = ?, match_type = ?, pattern = ?, header_name = ?,
		 enabled = ?, updated_at = ? WHERE id = ?`,
		p.Name, p.MatchOn, p.MatchType, p.Pattern, headerName, boolToInt64(enabled),
		time.Now().UTC().Format(time.RFC3339), id,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}
	s.logAudit("rule_update", fmt.Sprintf("rule %d updated", id), authClientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	if _, err := s.db.Exec("DELETE FROM notification_rules WHERE id = ?", id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}
	s.logAudit("rule_delete", fmt.Sprintf("rule %d deleted", id), authClientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleTestRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	var name string
	if err := s.db.QueryRow("SELECT name FROM notification_rules WHERE id = ?", id).Scan(&name); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "rule not found"})
		return
	}
	chatID := s.cfg.TelegramChatID
	if chatID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "TELEGRAM_CHAT_ID not set"})
		return
	}
	if err := s.telegram.SendTest(chatID); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) listRules() ([]models.NotificationRule, error) {
	rows, err := s.db.Query(
		`SELECT id, name, match_on, match_type, pattern, header_name, enabled, created_at, updated_at
		 FROM notification_rules ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []models.NotificationRule{}
	for rows.Next() {
		var rule models.NotificationRule
		var headerName sqlNullString
		var enabled int
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.MatchOn, &rule.MatchType, &rule.Pattern,
			&headerName, &enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rule.Enabled = enabled == 1
		if headerName.Valid {
			rule.HeaderName = &headerName.String
		}
		items = append(items, rule)
	}
	return items, rows.Err()
}
