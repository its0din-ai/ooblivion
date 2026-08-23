// Package scheduler runs periodic maintenance jobs such as flush and pruning.
package scheduler

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"ooblivion/internal/auth"
	"ooblivion/internal/logx"
)

type Scheduler struct {
	db          *sql.DB
	defaultDays int
	logger      *logx.Logger
	flushLock   chan struct{}
	lastFlushAt string
}

func New(db *sql.DB, defaultDays int, logger *logx.Logger) *Scheduler {
	return &Scheduler{
		db:          db,
		defaultDays: defaultDays,
		logger:      logger,
		flushLock:   make(chan struct{}, 1),
		lastFlushAt: readSetting(db, "last_flush_at"),
	}
}

func (s *Scheduler) Start(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.RunOnce()
			}
		}
	}()
}

func (s *Scheduler) RunOnce() {
	flushed, err := s.Flush(false, "system")
	if err != nil {
		s.logger.Errorf("scheduler flush error: %v", err)
	} else if flushed > 0 {
		s.logger.Infof("scheduler flushed %d requests", flushed)
	}
	if err := auth.DenylistPrune(s.db); err != nil {
		s.logger.Errorf("scheduler denylist prune error: %v", err)
	}
	s.pruneNotificationLog()
}

func (s *Scheduler) RetentionDays() int {
	raw := readSetting(s.db, "retention_days")
	days, err := strconv.Atoi(raw)
	if err != nil || days <= 0 {
		return s.defaultDays
	}
	return days
}

func (s *Scheduler) AutoFlushEnabled() bool {
	return readSetting(s.db, "auto_flush_enabled") != "0"
}

func (s *Scheduler) LastFlushAt() string {
	return s.lastFlushAt
}

func (s *Scheduler) Flush(manual bool, ip string) (int64, error) {
	select {
	case s.flushLock <- struct{}{}:
		defer func() { <-s.flushLock }()
	default:
		return 0, nil
	}

	if !manual && !s.AutoFlushEnabled() {
		return 0, nil
	}

	var res sql.Result
	var err error
	if manual {
		res, err = s.db.Exec("DELETE FROM requests WHERE saved = 0")
	} else {
		boundary := time.Now().UTC().Add(-time.Duration(s.RetentionDays()) * 24 * time.Hour).Format(time.RFC3339)
		res, err = s.db.Exec("DELETE FROM requests WHERE saved = 0 AND created_at < ?", boundary)
	}
	if err != nil {
		return 0, err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	s.lastFlushAt = now
	if err := writeSetting(s.db, "last_flush_at", now); err != nil {
		return 0, err
	}

	action := "flush_auto"
	if manual {
		action = "flush_manual"
	}
	_, err = s.db.Exec(
		"INSERT INTO audit_log (actor, action, detail, ip, created_at) VALUES ('admin', ?, ?, ?, ?)",
		action, strconv.FormatInt(count, 10)+" requests", ip, now,
	)
	return count, err
}

func (s *Scheduler) pruneNotificationLog() {
	if _, err := s.db.Exec("DELETE FROM notification_log WHERE id NOT IN (SELECT id FROM notification_log ORDER BY id DESC LIMIT 5000)"); err != nil {
		s.logger.Errorf("scheduler notification log prune error: %v", err)
	}
}

func readSetting(db *sql.DB, key string) string {
	var value string
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		return ""
	}
	return value
}

func writeSetting(db *sql.DB, key, value string) error {
	_, err := db.Exec(
		"INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	return err
}

func WriteSetting(db *sql.DB, key, value string) error {
	return writeSetting(db, key, value)
}

func ReadSetting(db *sql.DB, key string) string {
	return readSetting(db, key)
}
