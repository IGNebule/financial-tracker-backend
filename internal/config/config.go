// Package config reads the application configuration from the environment.
//
// A .env file in the working directory is loaded first if it exists; existing
// environment variables take precedence over values in the file. This matches
// the convention used by most Go services and by every hosting platform:
// platform-injected environment variables must win over a file that happened
// to be on disk.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

// Config is the validated application configuration. All fields are populated
// by Load; a partially-filled Config is never returned alongside a nil error.
type Config struct {
	Addr string
	DB   DBConfig
}

// DBConfig holds the PostgreSQL connection settings.
type DBConfig struct {
	Host     string
	Port     int
	User     string
	Name     string
	Password string
	SSLMode  string
}

// Load reads configuration from .env (if present) and the environment, and
// validates that every required variable is present. It returns an error if
// any required value is missing, empty, or malformed.
func Load() (*Config, error) {
	if err := loadDotEnv(".env"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	addr := getDefault("ADDR", ":8080")

	host := os.Getenv("APP_DB_HOST")
	portStr := os.Getenv("APP_DB_PORT")
	user := os.Getenv("APP_DB_USER")
	name := os.Getenv("APP_DB_NAME")
	password := os.Getenv("APP_DB_PASSWORD")
	sslMode := getDefault("APP_DB_SSLMODE", "disable")

	var missing []string
	for _, v := range []struct{ name, val string }{
		{"APP_DB_HOST", host},
		{"APP_DB_PORT", portStr},
		{"APP_DB_USER", user},
		{"APP_DB_NAME", name},
		{"APP_DB_PASSWORD", password},
	} {
		if v.val == "" {
			missing = append(missing, v.name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("APP_DB_PORT must be a valid port (1-65535), got %q", portStr)
	}

	return &Config{
		Addr: addr,
		DB: DBConfig{
			Host:     host,
			Port:     port,
			User:     user,
			Name:     name,
			Password: password,
			SSLMode:  sslMode,
		},
	}, nil
}

// getDefault returns the environment value, or fallback if unset or empty.
func getDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv reads path as a .env file and sets any variables that are not
// already present in the environment. Lines may be comments (starting with #),
// blank, or KEY=value. Values are not trimmed of internal spaces; leading and
// trailing whitespace around the key and value is stripped.
//
// It returns fs.ErrNotExist when the file does not exist; callers may choose
// to ignore that case, as Load does.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}
