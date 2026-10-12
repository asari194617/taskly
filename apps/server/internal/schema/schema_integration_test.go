package schema_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"testing"

	"github.com/asari194617/taskly/apps/server/internal/legal"
	"github.com/go-sql-driver/mysql"
)

// CI runs this after Atlas applies schema.sql to a disposable MySQL 8.4 database.
func TestDeclarativeSchema(t *testing.T) {
	if os.Getenv("TEST_SCHEMA_DB") != "1" {
		t.Skip("requires a disposable MySQL 8.4 database")
	}
	ctx := context.Background()
	config := mysql.NewConfig()
	config.User = os.Getenv("MYSQL_USER")
	config.Passwd = os.Getenv("MYSQL_PASSWORD")
	config.Net = "tcp"
	config.Addr = os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT")
	config.DBName = os.Getenv("MYSQL_DATABASE")
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var tables int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 38 {
		t.Fatalf("expected 38 application tables, got %d", tables)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	user, err := tx.ExecContext(ctx, `INSERT INTO users(email) VALUES ('schema-smoke@example.test')`)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := user.LastInsertId()
	if _, err := tx.ExecContext(ctx, `INSERT INTO policy_consents(user_id, terms_version, privacy_version, security_version, display_language, accepted_at)
		VALUES (?, ?, ?, ?, 'ja', NOW(6))`, userID, legal.Version, legal.Version, legal.Version); err != nil {
		t.Fatal(err)
	}
	subject, err := tx.ExecContext(ctx, `INSERT INTO subjects(name_ja) VALUES ('Atlas schema validation')`)
	if err != nil {
		t.Fatal(err)
	}
	subjectID, _ := subject.LastInsertId()
	task, err := tx.ExecContext(ctx, `INSERT INTO tasks(added_user_id, title, subject_id, deadline) VALUES (?, 'smoke', ?, '2026-10-01')`, userID, subjectID)
	if err != nil {
		t.Fatal(err)
	}
	taskID, _ := task.LastInsertId()
	if _, err := tx.ExecContext(ctx, `INSERT INTO tasks_users_completions(user_id, task_id) VALUES (?, ?)`, userID, taskID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO notification_deliveries(delivery_key, channel, user_id, status) VALUES ('schema-smoke', 'email', ?, 'sending')`, userID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	if os.Getenv("TEST_API_SMOKE") == "1" {
		user, err := db.ExecContext(ctx, `INSERT INTO users(email) VALUES ('atlas-api@example.test')`)
		if err != nil {
			t.Fatal(err)
		}
		userID, _ := user.LastInsertId()
		if _, err := db.ExecContext(ctx, `INSERT INTO policy_consents(user_id, terms_version, privacy_version, security_version, display_language, accepted_at)
			VALUES (?, ?, ?, ?, 'ja', NOW(6))`, userID, legal.Version, legal.Version, legal.Version); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO subjects(id, name_ja) VALUES (359, 'API smoke')`); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte("taskly-baseline-ci-key"))
		if _, err := db.ExecContext(ctx, `INSERT INTO api_keys(user_id, hashed_token) VALUES (?, ?)`, userID, hex.EncodeToString(hash[:])); err != nil {
			t.Fatal(err)
		}
	}
}
