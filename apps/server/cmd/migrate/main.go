package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

const (
	legacyBaseline = "0025_integration_jobs.sql"
	// baseline.sql is a schema-only snapshot after migration 0028.
	baselineVersion = "0028_notification_delivery.sql"
)

var migrationName = regexp.MustCompile(`^([0-9]{4})_[a-z0-9_]+\.sql$`)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	config := mysql.NewConfig()
	config.User = os.Getenv("MYSQL_USER")
	config.Passwd = os.Getenv("MYSQL_PASSWORD")
	config.Net = "tcp"
	config.Addr = os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT")
	config.DBName = os.Getenv("MYSQL_DATABASE")
	config.MultiStatements = true
	config.Timeout = 10 * time.Second
	config.ReadTimeout = 5 * time.Minute
	config.WriteTimeout = 5 * time.Minute
	if config.User == "" || config.Passwd == "" || config.DBName == "" || os.Getenv("DB_HOST") == "" || os.Getenv("DB_PORT") == "" {
		return fmt.Errorf("missing MySQL connection environment")
	}

	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	var locked int
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK('taskly_schema_migrations', 60)`).Scan(&locked); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	if locked != 1 {
		return fmt.Errorf("timed out waiting for migration lock")
	}
	defer conn.ExecContext(context.Background(), `SELECT RELEASE_LOCK('taskly_schema_migrations')`)

	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "/app/migrations"
	}
	files, err := migrationFiles(migrationsDir)
	if err != nil {
		return err
	}
	var tableCount int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()`).Scan(&tableCount); err != nil {
		return fmt.Errorf("inspect database tables: %w", err)
	}
	if tableCount == 0 {
		contents, err := os.ReadFile(filepath.Join(filepath.Dir(migrationsDir), "baseline.sql"))
		if err != nil {
			return fmt.Errorf("read empty-database baseline: %w", err)
		}
		if _, err := conn.ExecContext(ctx, string(contents)); err != nil {
			return fmt.Errorf("apply empty-database baseline: %w (MySQL DDL may be partially applied; inspect before retrying)", err)
		}
		// The snapshot contains these migrations. Record them only after every CREATE succeeds.
		for _, name := range []string{legacyBaseline, "0027_remove_task_search_outbox.sql", baselineVersion} {
			if _, err := conn.ExecContext(ctx, `INSERT INTO migrations(name) VALUES (?)`, name); err != nil {
				return fmt.Errorf("record baseline migration %s: %w", name, err)
			}
		}
		log.Printf("created empty database from schema baseline through %s", baselineVersion)
	} else {
		var baselineApplied int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM migrations WHERE name = ?`, legacyBaseline).Scan(&baselineApplied); err != nil {
			return fmt.Errorf("read legacy migration baseline: %w", err)
		}
		if baselineApplied != 1 {
			return fmt.Errorf("existing database is not reconciled through %s; refusing to apply the empty-database baseline", legacyBaseline)
		}
	}
	for _, name := range files {
		var applied int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM migrations WHERE name = ?`, name).Scan(&applied); err != nil {
			return fmt.Errorf("check %s: %w", name, err)
		}
		if applied != 0 {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if strings.TrimSpace(string(contents)) == "" {
			return fmt.Errorf("migration %s is empty", name)
		}
		if _, err := conn.ExecContext(ctx, string(contents)); err != nil {
			return fmt.Errorf("apply %s: %w (MySQL DDL may already be partially applied; inspect before retrying)", name, err)
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO migrations(name) VALUES (?)`, name); err != nil {
			return fmt.Errorf("record %s: %w", name, err)
		}
		log.Printf("applied migration %s", name)
	}
	log.Printf("database migrations complete")
	return nil
}

func migrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	versions := make(map[int]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := migrationName.FindStringSubmatch(entry.Name())
		if matches == nil {
			if strings.HasSuffix(entry.Name(), ".sql") {
				return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
			}
			continue
		}
		version, _ := strconv.Atoi(matches[1])
		if version <= 25 {
			continue // Historical migrations are represented by the verified baseline.
		}
		if previous := versions[version]; previous != "" {
			return nil, fmt.Errorf("duplicate migration version %04d: %s and %s", version, previous, entry.Name())
		}
		versions[version] = entry.Name()
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	return files, nil
}
