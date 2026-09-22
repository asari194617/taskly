package main

import (
	"compress/gzip"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/jlaffaye/ftp"
)

var backupName = regexp.MustCompile(`^backup-\d{8}-\d{6}\.sql\.gz\.age$`)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	recipient, err := age.ParseX25519Recipient(os.Getenv("BACKUP_AGE_RECIPIENT"))
	if err != nil {
		return fmt.Errorf("BACKUP_AGE_RECIPIENT is required: %w", err)
	}
	if os.Getenv("FTP_HOST") == "" || os.Getenv("FTP_USER") == "" || os.Getenv("FTP_PASSWORD") == "" {
		return errors.New("FTP_HOST, FTP_USER and FTP_PASSWORD are required")
	}
	dir := valueOr(os.Getenv("BACKUP_DIR"), os.TempDir())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("backup-%s.sql.gz.age", time.Now().UTC().Format("20060102-150405")))
	if err := runDump(path, recipient); err != nil {
		return err
	}
	return uploadWithRetry(path)
}

func runDump(outFile string, recipient age.Recipient) (err error) {
	pass := os.Getenv("MYSQL_PASSWORD")
	if os.Getenv("MYSQL_USER") == "" || os.Getenv("MYSQL_DATABASE") == "" || pass == "" {
		return errors.New("MYSQL_USER, MYSQL_DATABASE and MYSQL_PASSWORD are required")
	}
	if strings.ContainsAny(pass, "\r\n") {
		return errors.New("MYSQL_PASSWORD contains a line break")
	}
	// MySQL reads credentials from a private option file; only its path is in argv.
	optionFile, err := os.CreateTemp(filepath.Dir(outFile), ".mysqldump-*")
	if err != nil {
		return err
	}
	defer os.Remove(optionFile.Name())
	escaped := strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(pass)
	if _, err = fmt.Fprintf(optionFile, "[client]\npassword=\"%s\"\n", escaped); err != nil {
		optionFile.Close()
		return err
	}
	if err = optionFile.Close(); err != nil {
		return err
	}
	f, err := os.OpenFile(outFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(outFile)
		}
	}()
	enc, err := age.Encrypt(f, recipient)
	if err != nil {
		f.Close()
		return err
	}
	gz := gzip.NewWriter(enc)
	cmd := exec.Command("mysqldump",
		"--defaults-extra-file="+optionFile.Name(),
		"-h", valueOr(os.Getenv("DB_HOST"), "localhost"),
		"-P", valueOr(os.Getenv("DB_PORT"), "3306"),
		"-u", os.Getenv("MYSQL_USER"),
		"--single-transaction", "--quick", "--skip-lock-tables", "--no-tablespaces",
		"--routines", "--events", os.Getenv("MYSQL_DATABASE"))
	cmd.Stdout, cmd.Stderr = gz, os.Stderr
	runErr := cmd.Run()
	closeErr := errors.Join(gz.Close(), enc.Close(), f.Close())
	if runErr != nil {
		return fmt.Errorf("mysqldump: %w", runErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close backup: %w", closeErr)
	}
	return nil
}

func uploadWithRetry(path string) error {
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		err = uploadAndPrune(path)
		if err == nil {
			return nil
		}
		log.Printf("backup upload attempt %d/3 failed: %v", attempt, err)
		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * 5 * time.Second)
		}
	}
	return fmt.Errorf("backup upload failed after 3 attempts: %w", err)
}

func uploadAndPrune(path string) error {
	host := os.Getenv("FTP_HOST")
	c, err := ftp.Dial(net.JoinHostPort(host, valueOr(os.Getenv("FTP_PORT"), "21")),
		ftp.DialWithTimeout(30*time.Second),
		ftp.DialWithExplicitTLS(ftpsTLSConfig(host)))
	if err != nil {
		return err
	}
	defer c.Quit()
	if err := c.Login(os.Getenv("FTP_USER"), os.Getenv("FTP_PASSWORD")); err != nil {
		return err
	}
	dir := valueOr(os.Getenv("FTP_DIR"), "/backups")
	if err := c.MakeDir(dir); err != nil {
		log.Printf("FTPS mkdir %s: %v", dir, err)
	}
	if err := c.ChangeDir(dir); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	name := filepath.Base(path)
	tmp := name + ".part"
	defer c.Delete(tmp)
	if err := c.Stor(tmp, f); err != nil {
		return fmt.Errorf("FTPS STOR %s: %w", tmp, err)
	}
	if err := verifyRemote(c, tmp, path); err != nil {
		return err
	}
	if err := c.Rename(tmp, name); err != nil {
		return err
	}
	log.Printf("Verified FTPS backup %s", name)
	return pruneRemote(c, name)
}

func ftpsTLSConfig(host string) *tls.Config {
	// The FTPS server requires the data connection to resume the control
	// connection's TLS session. Both connections share this config during a dial.
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         host,
		ClientSessionCache: tls.NewLRUClientSessionCache(32),
	}
}

func verifyRemote(c *ftp.ServerConn, remote, local string) error {
	lf, err := os.Open(local)
	if err != nil {
		return err
	}
	defer lf.Close()
	lh := sha256.New()
	size, err := io.Copy(lh, lf)
	if err != nil {
		return err
	}
	remoteSize, err := c.FileSize(remote)
	if err != nil {
		return err
	}
	if size != remoteSize {
		return fmt.Errorf("remote size mismatch: %d != %d", remoteSize, size)
	}
	r, err := c.Retr(remote)
	if err != nil {
		return err
	}
	rh := sha256.New()
	readSize, readErr := io.Copy(rh, r)
	closeErr := r.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return err
	}
	if readSize != size || string(lh.Sum(nil)) != string(rh.Sum(nil)) {
		return errors.New("remote SHA-256 mismatch")
	}
	return nil
}

func pruneRemote(c *ftp.ServerConn, current string) error {
	retention, err := strconv.Atoi(valueOr(os.Getenv("BACKUP_RETENTION_DAYS"), "14"))
	if err != nil || retention < 1 {
		return errors.New("BACKUP_RETENTION_DAYS must be a positive integer")
	}
	entries, err := c.List("")
	if err != nil {
		return err
	}
	cutoff := time.Now().UTC().Add(-time.Duration(retention) * 24 * time.Hour)
	var names []string
	for _, e := range entries {
		if e.Type != ftp.EntryTypeFile || !backupName.MatchString(e.Name) {
			continue
		}
		names = append(names, e.Name)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for i, name := range names {
		if i < 3 || name == current {
			continue
		}
		stamp, err := time.Parse("20060102-150405", strings.TrimSuffix(strings.TrimPrefix(name, "backup-"), ".sql.gz.age"))
		if err == nil && stamp.Before(cutoff) {
			if err := c.Delete(name); err != nil {
				return fmt.Errorf("delete old backup %s: %w", name, err)
			}
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
