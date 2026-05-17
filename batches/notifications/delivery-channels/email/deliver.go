package main

import (
	"database/sql"
	"fmt"
	"strings"
)

func deliverGroupEmail(db *sql.DB, mailer Mailer, uid int64, group []QueueItem, opts DeliverOptions) error {
	to, err := resolveRecipientEmail(db, sql.NullInt64{Valid: true, Int64: uid})
	if err != nil || strings.TrimSpace(to) == "" {
		return fmt.Errorf("no recipient for uid=%d: %w", uid, err)
	}

	locale := valueOr(opts.Locale, "ja")
	appName := valueOr(opts.AppName, "Taskly")
	html, text, err := composeTaskDigestGRPC(locale, appName, group)
	if err != nil {
		return fmt.Errorf("compose failed: %w", err)
	}

	subject := fmt.Sprintf("[%s]期限が近い課題", appName)
	if err := mailer.Send(to, subject, text, html); err != nil {
		return fmt.Errorf("failed to send email to %s: %w", to, err)
	}

	return nil
}
