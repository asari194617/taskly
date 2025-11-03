package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
)

func main() {
	backupDir := valueOr(os.Getenv("BACKUP_DIR"), os.TempDir())
	// Ensure the backup directory exists to avoid file creation failures
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		log.Fatal(err)
	}
	file := filepath.Join(backupDir, fmt.Sprintf("backup-%s.sql.gz", time.Now().Format("20060102-150405")))
	if err := runDump(file); err != nil {
		log.Fatal(err)
	}

	if err := uploadAndPrune(file); err != nil {
		log.Fatal(err)
	}
	// Ensure group-readable (640) by default
	if err := os.Chmod(file, 0o640); err != nil {
		log.Println("chmod:", err)
	}
}

func runDump(outFile string) error {
	host := valueOr(os.Getenv("DB_HOST"), "localhost")
	port := valueOr(os.Getenv("DB_PORT"), "3306")
	user := os.Getenv("MYSQL_USER")
	pass := os.Getenv("MYSQL_PASSWORD")
	name := os.Getenv("MYSQL_DATABASE")

	args := []string{
		fmt.Sprintf("-h%s", host),
		fmt.Sprintf("-P%s", port),
		fmt.Sprintf("-u%s", user),
		fmt.Sprintf("-p%s", pass),
		// safer defaults for production-ish dumps without elevated privileges
		"--single-transaction",
		"--quick",
		"--skip-lock-tables",
		"--no-tablespaces",
		name,
	}
	cmd := exec.Command("mysqldump", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysqldump: %w", err)
	}

	f, err := os.OpenFile(outFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	defer gw.Close()
	if _, err := gw.Write(stdout.Bytes()); err != nil {
		return err
	}
	return nil
}

func uploadAndPrune(localFile string) error {
	host := valueOr(os.Getenv("FTP_HOST"), "")
	if host == "" {
		log.Printf("FTP_HOST missing; skipping upload and prune for %s", localFile)
		return nil
	}
	port := valueOr(os.Getenv("FTP_PORT"), "21")
	user := os.Getenv("FTP_USER")
	pass := os.Getenv("FTP_PASSWORD")
	dir := valueOr(os.Getenv("FTP_DIR"), "/backups")

	log.Printf("Connecting FTP %s:%s ...", host, port)
	c, err := ftp.Dial(fmt.Sprintf("%s:%s", host, port), ftp.DialWithTimeout(10*time.Second))
	if err != nil {
		return err
	}
	defer c.Quit()
	if err := c.Login(user, pass); err != nil {
		return err
	}
	_ = c.MakeDir(dir)
	if err := c.ChangeDir(dir); err != nil {
		return err
	}

	lf, err := os.Open(localFile)
	if err != nil {
		return err
	}
	defer lf.Close()
	base := filepath.Base(localFile)
	log.Printf("Uploading %s to ftp://%s:%s%s/%s ...", localFile, host, port, dir, base)
	if err := c.Stor(base, lf); err != nil {
		return err
	}
	log.Printf("Uploaded %s to ftp://%s:%s%s/%s", base, host, port, dir, base)

	// prune older than 7 days
	entries, err := c.List("")
	if err != nil {
		return err
	}
	cut := time.Now().Add(-7 * 24 * time.Hour)
	for _, e := range entries {
		if e.Type != ftp.EntryTypeFile {
			continue
		}
		if e.Time.Before(cut) {
			_ = c.Delete(e.Name)
		}
	}
	return nil
}

func valueOr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
