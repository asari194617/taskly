package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// CI runs this against a disposable MySQL 8.4 database.
func TestEmptyDatabaseBaseline(t *testing.T) {
	if os.Getenv("TEST_MIGRATION_DB") != "1" {
		t.Skip("requires a disposable MySQL 8.4 database")
	}
	ctx := context.Background()
	if err := run(ctx); err != nil {
		t.Fatalf("initialize empty database: %v", err)
	}
	if err := run(ctx); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
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
	var tables, migrations int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 35 {
		t.Fatalf("expected 35 application tables, got %d", tables)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM migrations WHERE name IN (?, ?, ?)`, legacyBaseline, "0027_remove_task_search_outbox.sql", baselineVersion).Scan(&migrations); err != nil {
		t.Fatal(err)
	}
	if migrations != 3 {
		t.Fatalf("expected three baseline migration records, got %d", migrations)
	}

	// Exercise the user, subject, task, completion, and delivery schema together.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	user, err := tx.ExecContext(ctx, `INSERT INTO users(email) VALUES ('baseline-smoke@example.test')`)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := user.LastInsertId()
	subject, err := tx.ExecContext(ctx, `INSERT INTO subjects(name_ja) VALUES ('ベースライン検証')`)
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
	if _, err := tx.ExecContext(ctx, `INSERT INTO notification_deliveries(delivery_key, channel, user_id, status) VALUES ('baseline-smoke', 'email', ?, 'sending')`, userID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks t JOIN subjects s ON s.id=t.subject_id JOIN tasks_users_completions c ON c.task_id=t.id WHERE t.id=? AND c.user_id=?`, taskID, userID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected task join and completion, got %d", count)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	// A new migration must join the same history after the snapshot.
	newDir := filepath.Join(t.TempDir(), "migrations")
	if err := os.Mkdir(newDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "0029_baseline_probe.sql"), []byte(`CREATE TABLE baseline_probe (id INT PRIMARY KEY)`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MIGRATIONS_DIR", newDir)
	if err := run(ctx); err != nil {
		t.Fatalf("apply subsequent migration: %v", err)
	}
	if err := run(ctx); err != nil {
		t.Fatalf("repeat subsequent migration: %v", err)
	}
	var applied int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM migrations WHERE name = '0029_baseline_probe.sql'`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Fatalf("expected subsequent migration once, got %d", applied)
	}

	// An existing database without the reconciled legacy marker must be rejected.
	if _, err := db.ExecContext(ctx, `DELETE FROM migrations WHERE name = ?`, legacyBaseline); err != nil {
		t.Fatal(err)
	}
	if err := run(ctx); err == nil {
		t.Fatal("accepted a nonempty database without its legacy migration marker")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO migrations(name) VALUES (?)`, legacyBaseline); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("TEST_API_SMOKE") == "1" {
		user, err := db.ExecContext(ctx, `INSERT INTO users(email) VALUES ('baseline-api@example.test')`)
		if err != nil {
			t.Fatal(err)
		}
		userID, _ := user.LastInsertId()
		if _, err := db.ExecContext(ctx, `INSERT INTO subjects(id, name_ja) VALUES (359, 'API smoke')`); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte("taskly-baseline-ci-key"))
		if _, err := db.ExecContext(ctx, `INSERT INTO api_keys(user_id, hashed_token) VALUES (?, ?)`, userID, hex.EncodeToString(hash[:])); err != nil {
			t.Fatal(err)
		}
	}
}
