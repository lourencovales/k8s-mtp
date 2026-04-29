package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Run("valid test with explicit fields", func(t *testing.T) {
		cfg := `{"db_url": "postgresql://test:test@localhost", "listen_addr": ":9090", "log_level": "debug"}`
		file := writeTempConfig(t, cfg)
		got, err := Load(file)
		if err != nil {
			t.Fatal(err)
		}
		if got.DBURL != "postgresql://test:test@localhost" {
			t.Errorf("expected `postgresql://test:test@localhost, got %s`", got.DBURL)
		}
		if got.ListenAddr != ":9090" {
			t.Errorf("expected :9090, got %s", got.ListenAddr)
		}
		if got.LogLevel != "debug" {
			t.Errorf("expected `debug`, got %s", got.LogLevel)
		}
	})
	t.Run("empty listen_addr returns error", func(t *testing.T) {
		cfg := `{"db_url": "postgresql://test:test@localhost", "listen_addr": ""}`
		file := writeTempConfig(t, cfg)
		_, err := Load(file)
		if err == nil {
			t.Errorf("expected error for empty listen_addr")
		}
	})
	t.Run("empty db_url returns error", func(t *testing.T) {
		cfg := `{"db_url": ""}`
		file := writeTempConfig(t, cfg)
		_, err := Load(file)
		if err == nil {
			t.Errorf("expected error for empty db_url")
		}
	})
	t.Run("non-existing file returns error", func(t *testing.T) {
		_, err := Load("/this/path/is/fake")
		if err == nil {
			t.Errorf("expected error for missing file")
		}
	})
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "conf")
	err := os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSlogLevel(t *testing.T) {
	type testCase struct {
		logLevel string
		expected slog.Level
	}
	for _, tc := range []testCase{
		{"debug", slog.LevelDebug},
		{"warn", slog.LevelWarn},
		{"info", slog.LevelInfo},
		{"error", slog.LevelError},
		{"bad", slog.LevelInfo},  // default case
		{"WARN", slog.LevelWarn}, // testing if strings.ToLower works
		{"", slog.LevelInfo},
	} {
		cfg := &Config{LogLevel: tc.logLevel}
		got := cfg.slogLevel()
		if got != tc.expected {
			t.Errorf("%s: expected %v, got %v", tc.logLevel, tc.expected, got)
		}
	}
}
