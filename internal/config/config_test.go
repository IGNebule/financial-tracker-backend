package config

import (
	"os"
	"strings"
	"testing"
)

// clearDBEnv unsets every APP_DB_* variable so a test starts from a known
// state regardless of the developer's shell. t.Setenv restores the original
// values when the test finishes.
func clearDBEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"ADDR", "APP_DB_HOST", "APP_DB_PORT",
		"APP_DB_USER", "APP_DB_NAME", "APP_DB_PASSWORD", "APP_DB_SSLMODE",
	} {
		t.Setenv(k, "")
	}
}

func setValid(t *testing.T) {
	t.Helper()
	t.Setenv("APP_DB_HOST", "localhost")
	t.Setenv("APP_DB_PORT", "5432")
	t.Setenv("APP_DB_USER", "blanc")
	t.Setenv("APP_DB_NAME", "financial_app")
	t.Setenv("APP_DB_PASSWORD", "hunter2")
}

func TestLoadValid(t *testing.T) {
	clearDBEnv(t)
	setValid(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Fatalf("Addr = %q, want \":8080\"", cfg.Addr)
	}
	if cfg.DB.Port != 5432 {
		t.Fatalf("DB.Port = %d, want 5432", cfg.DB.Port)
	}
	if cfg.DB.SSLMode != "disable" {
		t.Fatalf("DB.SSLMode = %q, want \"disable\"", cfg.DB.SSLMode)
	}
	if cfg.DB.Password != "hunter2" {
		t.Fatal("DB.Password not populated")
	}
}

func TestLoadMissingRequired(t *testing.T) {
	clearDBEnv(t)
	setValid(t)
	t.Setenv("APP_DB_USER", "") // simulate unset

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error, want missing-variable error")
	}
	if !strings.Contains(err.Error(), "APP_DB_USER") {
		t.Fatalf("error %q should mention APP_DB_USER", err.Error())
	}
}

func TestLoadInvalidPort(t *testing.T) {
	clearDBEnv(t)
	setValid(t)
	t.Setenv("APP_DB_PORT", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error, want invalid-port error")
	}
	if !strings.Contains(err.Error(), "APP_DB_PORT") {
		t.Fatalf("error %q should mention APP_DB_PORT", err.Error())
	}
}

func TestLoadDotEnvDoesNotOverrideEnv(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/.env"
	if err := os.WriteFile(path, []byte("APP_DB_HOST=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	clearDBEnv(t)
	t.Setenv("APP_DB_HOST", "from-shell")

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv: %v", err)
	}
	if got := os.Getenv("APP_DB_HOST"); got != "from-shell" {
		t.Fatalf("APP_DB_HOST = %q, want \"from-shell\" (env must win over file)", got)
	}
}
