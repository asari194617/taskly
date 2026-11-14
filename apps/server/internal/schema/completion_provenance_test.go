package schema_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func TestMySQLGuidanceCompletionProvenanceSchema(t *testing.T) {
	if os.Getenv("TEST_SCHEMA_DB") != "1" {
		t.Skip("requires a disposable MySQL schema")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cfg := mysql.NewConfig()
	cfg.User, cfg.Passwd = os.Getenv("MYSQL_USER"), os.Getenv("MYSQL_PASSWORD")
	cfg.Net = "tcp"
	cfg.Addr = os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT")
	cfg.DBName = os.Getenv("MYSQL_DATABASE")
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for name, length := range map[string]int64{"auto_event_id": 128, "auto_source_task_id": 0, "auto_assignment_scope": 64} {
		var nullable, datatype string
		var defaultValue sql.NullString
		var columnLength sql.NullInt64
		if err := db.QueryRowContext(ctx, `SELECT IS_NULLABLE, COLUMN_DEFAULT, DATA_TYPE, CHARACTER_MAXIMUM_LENGTH
			FROM information_schema.columns WHERE TABLE_SCHEMA=DATABASE()
			AND TABLE_NAME='tasks_users_completions' AND COLUMN_NAME=?`, name).
			Scan(&nullable, &defaultValue, &datatype, &columnLength); err != nil {
			t.Fatalf("metadata column %s: %v", name, err)
		}
		if nullable != "YES" || defaultValue.Valid {
			t.Fatalf("%s must remain nullable with NULL default", name)
		}
		if name == "auto_source_task_id" {
			if datatype != "bigint" {
				t.Fatalf("%s type=%s", name, datatype)
			}
		} else if !columnLength.Valid || columnLength.Int64 != length {
			t.Fatalf("%s length=%v", name, columnLength)
		}
	}
	t.Run("ordinary_completion_stays_manual", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		user, err := tx.ExecContext(ctx, "INSERT INTO users(email) VALUES ('synthetic-provenance-schema@example.test')")
		if err != nil {
			t.Fatal(err)
		}
		userID, err := user.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		subject, err := tx.ExecContext(ctx, "INSERT INTO subjects(name_ja) VALUES ('Synthetic provenance schema fixture')")
		if err != nil {
			t.Fatal(err)
		}
		subjectID, err := subject.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		task, err := tx.ExecContext(ctx, "INSERT INTO tasks(added_user_id,title,subject_id,deadline) VALUES (?,'Synthetic manual completion',?,'2030-05-14')", userID, subjectID)
		if err != nil {
			t.Fatal(err)
		}
		taskID, err := task.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO tasks_users_completions(user_id,task_id) VALUES (?,?)", userID, taskID); err != nil {
			t.Fatal(err)
		}
		var event, scope sql.NullString
		var source sql.NullInt64
		if err := tx.QueryRowContext(ctx, "SELECT auto_event_id,auto_source_task_id,auto_assignment_scope FROM tasks_users_completions WHERE user_id=? AND task_id=?", userID, taskID).Scan(&event, &source, &scope); err != nil {
			t.Fatal(err)
		}
		if event.Valid || source.Valid || scope.Valid {
			t.Fatal("ordinary completion accidentally acquired automatic provenance")
		}
	})
	t.Run("additive_columns_preserve_existing_rows", func(t *testing.T) {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if _, err := conn.ExecContext(ctx, `CREATE TEMPORARY TABLE synthetic_legacy_completion_upgrade (
			user_id bigint unsigned NOT NULL, task_id bigint unsigned NOT NULL,
			completed_at datetime NOT NULL, PRIMARY KEY(user_id,task_id))`); err != nil {
			t.Fatal(err)
		}
		defer conn.ExecContext(context.Background(), "DROP TEMPORARY TABLE IF EXISTS synthetic_legacy_completion_upgrade")
		if _, err := conn.ExecContext(ctx, "INSERT INTO synthetic_legacy_completion_upgrade VALUES(1,2,'2030-05-14 09:00:00')"); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.ExecContext(ctx, `ALTER TABLE synthetic_legacy_completion_upgrade
			ADD COLUMN auto_event_id varchar(128) DEFAULT NULL,
			ADD COLUMN auto_source_task_id bigint DEFAULT NULL,
			ADD COLUMN auto_assignment_scope char(64) DEFAULT NULL`); err != nil {
			t.Fatal(err)
		}
		var userID, taskID int
		var completedAt string
		var event, scope sql.NullString
		var source sql.NullInt64
		if err := conn.QueryRowContext(ctx, `SELECT user_id,task_id,DATE_FORMAT(completed_at,'%Y-%m-%d %H:%i:%s'),
			auto_event_id,auto_source_task_id,auto_assignment_scope FROM synthetic_legacy_completion_upgrade`).
			Scan(&userID, &taskID, &completedAt, &event, &source, &scope); err != nil {
			t.Fatal(err)
		}
		if userID != 1 || taskID != 2 || completedAt != "2030-05-14 09:00:00" || event.Valid || source.Valid || scope.Valid {
			t.Fatal("additive provenance columns changed a pre-existing manual row")
		}
	})
}
