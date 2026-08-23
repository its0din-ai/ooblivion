package capture

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"ooblivion/internal/matcher"
	"ooblivion/internal/models"
	"ooblivion/internal/scheduler"
	"ooblivion/internal/telegram"
)

type Classifier struct {
	db *sql.DB
	tg *telegram.Sender
}

func NewClassifier(db *sql.DB, tg *telegram.Sender) *Classifier {
	return &Classifier{db: db, tg: tg}
}

func (c *Classifier) Classify(req models.Request, subj matcher.Subject) error {
	if err := c.applyScope(req, subj); err != nil {
		return err
	}
	return c.applyNotifications(req, subj)
}

func (c *Classifier) applyScope(req models.Request, subj matcher.Subject) error {
	rows, err := c.db.Query(
		`SELECT id, name, match_on, match_type, pattern, header_name
		 FROM scopes WHERE enabled = 1 ORDER BY priority DESC, id ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var matchedID *int64
	for rows.Next() {
		var scope models.Scope
		var headerName sql.NullString
		if err := rows.Scan(&scope.ID, &scope.Name, &scope.MatchOn, &scope.MatchType, &scope.Pattern, &headerName); err != nil {
			return err
		}
		if headerName.Valid {
			scope.HeaderName = &headerName.String
		}
		if matcher.Matches(subj, ruleFrom(scope.MatchOn, scope.MatchType, scope.Pattern, scope.HeaderName)) {
			id := scope.ID
			matchedID = &id
			break
		}
	}
	if rows.Err() != nil {
		return rows.Err()
	}
	if matchedID != nil {
		_, err = c.db.Exec("UPDATE requests SET saved = 1, scope_id = ? WHERE id = ?", *matchedID, req.ID)
		return err
	}
	return nil
}

func (c *Classifier) applyNotifications(req models.Request, subj matcher.Subject) error {
	rows, err := c.db.Query(
		`SELECT id, name, match_on, match_type, pattern, header_name
		 FROM notification_rules WHERE enabled = 1 ORDER BY id ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	fired := false
	publicURL := scheduler.ReadSetting(c.db, "public_url")
	for rows.Next() {
		var rule models.NotificationRule
		var headerName sql.NullString
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.MatchOn, &rule.MatchType, &rule.Pattern, &headerName); err != nil {
			return err
		}
		if headerName.Valid {
			rule.HeaderName = &headerName.String
		}
		if !matcher.Matches(subj, ruleFrom(rule.MatchOn, rule.MatchType, rule.Pattern, rule.HeaderName)) {
			continue
		}
		fired = true
		c.tg.Enqueue(telegram.Job{
			RuleID:    rule.ID,
			RequestID: req.ID,
			RuleName:  rule.Name,
			Text:      buildAlert(req, rule.Name, publicURL),
		})
	}
	if rows.Err() != nil {
		return rows.Err()
	}
	if fired {
		_, err = c.db.Exec("UPDATE requests SET notified = 1 WHERE id = ?", req.ID)
		return err
	}
	return nil
}

func ruleFrom(matchOn, matchType, pattern string, headerName *string) matcher.Rule {
	r := matcher.Rule{MatchOn: matchOn, MatchType: matchType, Pattern: pattern}
	if headerName != nil {
		r.HeaderName = *headerName
	}
	return r
}

func formatTimeID(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.Local().Format("02/01/2006 15.04.05")
}

func buildAlert(req models.Request, ruleName, publicURL string) string {
	line := fmt.Sprintf("%s %s%s", req.Method, req.Host, req.Path)
	if req.Query != "" {
		line += "?" + req.Query
	}
	endpoint := strings.ReplaceAll(line, "`", "\\`")
	ip := strings.ReplaceAll(req.SourceIP, "`", "\\`")

	var view string
	if publicURL != "" {
		view = fmt.Sprintf(
			"[%s/admin/requests/%d](%s/admin/requests/%d)",
			telegram.EscapeMD(publicURL), req.ID, publicURL, req.ID,
		)
	} else {
		view = telegram.EscapeMD(fmt.Sprintf("/admin/requests/%d", req.ID))
	}

	ipLine := ""
	if req.SourceIP != "" {
		ipLine = fmt.Sprintf("IP: `%s`\n", ip)
	}

	return fmt.Sprintf(
		"*OOBlivion Alert*\n\nAlert Name: %s\nTime: %s\n\n%s`%s`\n\nView Url: %s",
		telegram.EscapeMD(ruleName),
		telegram.EscapeMD(formatTimeID(req.CreatedAt)),
		ipLine,
		endpoint,
		view,
	)
}
