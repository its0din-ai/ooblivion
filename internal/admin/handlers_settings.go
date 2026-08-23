package admin

import (
	"net/http"
	"strconv"

	"ooblivion/internal/models"
	"ooblivion/internal/scheduler"
)

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "dashboard", map[string]any{"Title": "Dashboard"})
}

func (s *Server) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "settings", map[string]any{
		"Title":            "Settings",
		"PublicURL":        scheduler.ReadSetting(s.db, "public_url"),
		"RetentionDays":    s.sched.RetentionDays(),
		"AutoFlush":        scheduler.ReadSetting(s.db, "auto_flush_enabled") != "0",
		"TelegramToken":    s.cfg.TelegramToken != "",
		"TelegramChatID":   s.cfg.TelegramChatID,
		"TelegramThreadID": s.cfg.TelegramThreadID,
		"LastFlushAt":      s.sched.LastFlushAt(),
	})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"public_url":         scheduler.ReadSetting(s.db, "public_url"),
		"retention_days":     s.sched.RetentionDays(),
		"auto_flush_enabled": scheduler.ReadSetting(s.db, "auto_flush_enabled") != "0",
		"telegram_enabled":   s.cfg.TelegramToken != "" && s.cfg.TelegramChatID != "",
		"last_flush_at":      s.sched.LastFlushAt(),
	})
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PublicURL        *string `json:"public_url"`
		RetentionDays    *int    `json:"retention_days"`
		AutoFlushEnabled *bool   `json:"auto_flush_enabled"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid body"})
		return
	}
	if body.RetentionDays != nil && (*body.RetentionDays <= 0 || *body.RetentionDays > 3650) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "retention_days out of range"})
		return
	}
	if body.PublicURL != nil {
		if err := scheduler.WriteSetting(s.db, "public_url", *body.PublicURL); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
			return
		}
	}
	if body.RetentionDays != nil {
		if err := scheduler.WriteSetting(s.db, "retention_days", strconv.Itoa(*body.RetentionDays)); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
			return
		}
	}
	if body.AutoFlushEnabled != nil {
		val := "0"
		if *body.AutoFlushEnabled {
			val = "1"
		}
		if err := scheduler.WriteSetting(s.db, "auto_flush_enabled", val); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
			return
		}
	}
	s.logAudit("settings_update", "settings changed", authClientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAuditPage(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(
		`SELECT id, actor, action, detail, ip, created_at FROM audit_log ORDER BY id DESC LIMIT 200`)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	items := []models.AuditEntry{}
	for rows.Next() {
		var e models.AuditEntry
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Detail, &e.IP, &e.CreatedAt); err != nil {
			continue
		}
		items = append(items, e)
	}
	s.render(w, r, "audit", map[string]any{"Title": "Audit", "Items": items})
}
