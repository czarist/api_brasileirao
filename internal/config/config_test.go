package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRequiresPostgresURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("empty DATABASE_URL was accepted")
	}
	t.Setenv("DATABASE_URL", "mysql://localhost/db")
	if _, err := Load(); err == nil {
		t.Fatal("non-postgres URL was accepted")
	}
	t.Setenv("DATABASE_URL", "postgres://user:secret@localhost/db")
	t.Setenv("SYNC_COMPETITION", "serie-b")
	if _, err := Load(); err == nil {
		t.Fatal("unknown competition was accepted")
	}
	t.Setenv("SYNC_COMPETITION", "brasileirao-serie-a")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SyncInterval.String() != "1h0m0s" || cfg.SourceBaseURL != "https://www.cbf.com.br" {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestLoadDotEnvDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("DATABASE_URL=postgres://from-file/db\nSYNC_YEAR=2026\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", "postgres://already/db")
	os.Unsetenv("SYNC_YEAR")
	LoadDotEnv(path)
	if os.Getenv("DATABASE_URL") != "postgres://already/db" {
		t.Fatalf("dotenv overrode DATABASE_URL: %s", os.Getenv("DATABASE_URL"))
	}
	if os.Getenv("SYNC_YEAR") != "2026" {
		t.Fatalf("SYNC_YEAR = %q", os.Getenv("SYNC_YEAR"))
	}
}
