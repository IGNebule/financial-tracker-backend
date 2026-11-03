package db

import (
	"strings"
	"testing"

	"financialtracker/internal/config"
)

func TestBuildDSNEncodesSpecialCharacters(t *testing.T) {
	// A password with @, :, and / would break a naive "host=... password=..."
	// DSN. url.URL percent-encodes them.
	dsn := buildDSN(config.DBConfig{
		Host: "localhost", Port: 5432,
		User: "blanc", Name: "financial_app",
		Password: "p@ss:w/rd", SSLMode: "disable",
	})

	if strings.Contains(dsn, "p@ss:w/rd") {
		t.Fatalf("password not encoded in DSN: %s", dsn)
	}
	if !strings.Contains(dsn, "p%40ss") {
		t.Fatalf("expected percent-encoded @ in DSN, got: %s", dsn)
	}
	if !strings.Contains(dsn, "sslmode=disable") {
		t.Fatalf("sslmode missing from DSN: %s", dsn)
	}
	if !strings.Contains(dsn, "/financial_app") {
		t.Fatalf("database name missing from DSN: %s", dsn)
	}
}
