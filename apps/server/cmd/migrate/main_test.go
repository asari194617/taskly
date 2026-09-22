package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMigrationFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"0025_integration_jobs.sql", "0027_second.sql", "0026_first.sql", "README.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	files, err := migrationFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"0026_first.sql", "0027_second.sql"}; !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %v, want %v", files, want)
	}
}

func TestRepositoryMigrationFiles(t *testing.T) {
	files, err := migrationFiles(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no repository migrations found")
	}
}

func TestMigrationFilesRejectsDuplicateVersion(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"0026_first.sql", "0026_second.sql"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := migrationFiles(dir); err == nil || !strings.Contains(err.Error(), "duplicate migration version") {
		t.Fatalf("expected duplicate version error, got %v", err)
	}
}

func TestMigrationFilesRejectsInvalidNewFilename(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "0026-new.sql"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := migrationFiles(dir); err == nil || !strings.Contains(err.Error(), "invalid migration filename") {
		t.Fatalf("expected invalid filename error, got %v", err)
	}
}
