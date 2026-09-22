// e2e-seed resets the dedicated, disposable E2E database. Never point it at a shared database.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if os.Getenv("TASKLY_E2E_RESET") != "1" || os.Getenv("MYSQL_DATABASE") != "taskly_e2e" {
		log.Fatal("refusing to reset a database other than taskly_e2e")
	}
	cfg := mysql.NewConfig()
	cfg.User, cfg.Passwd, cfg.DBName = os.Getenv("MYSQL_USER"), os.Getenv("MYSQL_PASSWORD"), os.Getenv("MYSQL_DATABASE")
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%s", os.Getenv("DB_HOST"), os.Getenv("DB_PORT"))
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	rdb := redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_ADDR"), Username: os.Getenv("REDIS_USERNAME"), Password: os.Getenv("REDIS_PASSWORD")})
	defer rdb.Close()
	if err := rdb.FlushDB(ctx).Err(); err != nil {
		log.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()
	for _, table := range []string{"tasks_users_completions", "integration_jobs", "tasks_teams_logs", "tasks", "sessions", "role_users", "subjects", "users"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM `"+table+"`"); err != nil {
			log.Fatalf("clear %s: %v", table, err)
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("e2e-password"), bcrypt.MinCost)
	if err != nil {
		log.Fatal(err)
	}
	for _, user := range []struct {
		email string
		admin bool
	}{{"e2e-admin@example.test", true}, {"e2e-user@example.test", false}} {
		res, err := tx.ExecContext(ctx, "INSERT INTO users(email, display_name, hashed_password, email_code_allowed, is_registered, is_agreed_policies) VALUES (?, ?, ?, 0, 1, 1)", user.email, user.email, hash)
		if err != nil {
			log.Fatal(err)
		}
		if user.admin {
			id, err := res.LastInsertId()
			if err != nil {
				log.Fatal(err)
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO role_users(user_id, role) VALUES (?, 'admin')", id); err != nil {
				log.Fatal(err)
			}
		}
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO subjects(name_ja, name_en) VALUES ('数学', 'Math')"); err != nil {
		log.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
}
