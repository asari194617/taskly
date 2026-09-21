package main

import (
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/jlaffaye/ftp"
)

func TestIntegrationFTPS(t *testing.T) {
	if os.Getenv("BACKUP_FTPS_TEST") != "1" {
		t.Skip("requires FTPS credentials")
	}
	host := os.Getenv("FTP_HOST")
	c, err := ftp.Dial(net.JoinHostPort(host, valueOr(os.Getenv("FTP_PORT"), "21")),
		ftp.DialWithTimeout(30*time.Second),
		ftp.DialWithExplicitTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Quit()
	if err := c.Login(os.Getenv("FTP_USER"), os.Getenv("FTP_PASSWORD")); err != nil {
		t.Fatal(err)
	}
	dir := valueOr(os.Getenv("FTP_DIR"), "/backups")
	if err := c.MakeDir(dir); err != nil {
		t.Logf("mkdir: %v", err)
	}
	if err := c.ChangeDir(dir); err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("issue-279-check-%d.part", time.Now().UnixNano())
	defer c.Delete(name)
	local := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(local, []byte("FTPS encrypted data connection check"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(local)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Stor(name, f); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	if err := verifyRemote(c, name, local); err != nil {
		t.Fatal(err)
	}
}

// Run with BACKUP_RESTORE_TEST=1 against a disposable MySQL instance.
func TestIntegrationRestore(t *testing.T) {
	if os.Getenv("BACKUP_RESTORE_TEST") != "1" {
		t.Skip("requires disposable MySQL")
	}
	t.Setenv("MYSQL_USER", "root")
	t.Setenv("MYSQL_DATABASE", "backup_source")
	t.Setenv("MYSQL_PASSWORD", "restore_test_password")
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", "33379")
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "backup.sql.gz.age")
	if err := runDump(out, identity.Recipient()); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	decrypted, err := age.Decrypt(f, identity)
	if err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(decrypted)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("mysql", "-h", "127.0.0.1", "-P", "33379", "-u", "root", "restore_target")
	cmd.Env = append(os.Environ(), "MYSQL_PWD=restore_test_password")
	cmd.Stdin = gz
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("restore: %v: %s", err, output)
	}
	check := exec.Command("mysql", "-h", "127.0.0.1", "-P", "33379", "-u", "root", "-N", "-e", "SELECT value_text FROM restore_target.sample WHERE id=1")
	check.Env = append(os.Environ(), "MYSQL_PWD=restore_test_password")
	output, err := check.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "restored" {
		t.Fatalf("restored row: %v: %s", err, output)
	}
}

func TestRunDumpEncryptsStreamAndHidesPassword(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "mysqldump")
	// A fake dump checks that the password never appears in argv and emits SQL.
	script := "#!/bin/sh\ncase \"$*\" in *supersecret*) exit 4;; esac\nprintf 'CREATE TABLE restore_check (id INT);\\nINSERT INTO restore_check VALUES (42);\\n'\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MYSQL_USER", "test")
	t.Setenv("MYSQL_DATABASE", "restore_test")
	t.Setenv("MYSQL_PASSWORD", "supersecret")
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "backup.sql.gz.age")
	if err := runDump(out, identity.Recipient()); err != nil {
		t.Fatal(err)
	}
	encrypted, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encrypted), "restore_check") {
		t.Fatal("plaintext in backup")
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	decrypted, err := age.Decrypt(f, identity)
	if err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(decrypted)
	if err != nil {
		t.Fatal(err)
	}
	sql, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sql), "INSERT INTO restore_check VALUES (42)") {
		t.Fatalf("unexpected SQL: %s", sql)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".mysqldump-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("credential file remains: %v, %v", matches, err)
	}
}
