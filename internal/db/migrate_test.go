package db

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"financialtracker/migrations"
)

func TestMigrationVersion(t *testing.T) {
	cases := map[string]string{
		"0001_create_users.sql":  "0001",
		"0002_add_index.sql":     "0002",
		"2026_10_30_add_col.sql": "2026",
		"single.sql":             "single",
	}
	for name, want := range cases {
		if got := migrationVersion(name); got != want {
			t.Fatalf("migrationVersion(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestListMigrations(t *testing.T) {
	fsys := fstest.MapFS{
		"0002_second.sql":   {Data: []byte("SELECT 1;")},
		"0001_first.sql":    {Data: []byte("SELECT 1;")},
		"README.md":         {Data: []byte("not a migration")},
		"sub/0003_nest.sql": {Data: []byte("SELECT 1;")},
	}
	got, err := listMigrations(fsys)
	if err != nil {
		t.Fatalf("listMigrations: %v", err)
	}
	want := []string{"0001_first.sql", "0002_second.sql"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMigrateIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped in -short mode")
	}

	// This test runs against the dev database from .env. It is the smallest
	// useful integration test; it applies migrations a second time and
	// asserts that they are a no-op.
	// NOTE: running it modifies the dev database — but only by applying
	// migrations that are supposed to be applied anyway. Do not run it
	// against a production database.
	t.Skip("integration test not yet implemented; see Checkpoint 4b")
}

func TestMigrationsEmbedded(t *testing.T) {
	names, err := listMigrations(migrations.FS)
	if err != nil {
		t.Fatalf("listMigrations: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no migrations found in the embedded FS — check //go:embed in migrations/embed.go")
	}

	for _, name := range names {
		data, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if len(data) == 0 {
			t.Fatalf("%s is empty", name)
		}
	}
}
