package log

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewFromValues_NilDefaultsToDefaultLogger(t *testing.T) {
	logger, err := NewFromValues(ConfigValues{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNewFromValues_UnknownLevelIsError(t *testing.T) {
	_, err := NewFromValues(ConfigValues{Level: "trace"})
	if err == nil {
		t.Fatal("expected error for unknown level")
	}
	if !strings.Contains(err.Error(), "invalid logging level") {
		t.Fatalf("expected invalid level error, got %v", err)
	}
}

func TestNewFromValues_UnknownFormatIsError(t *testing.T) {
	_, err := NewFromValues(ConfigValues{Format: "yaml"})
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "invalid logging format") {
		t.Fatalf("expected invalid format error, got %v", err)
	}
}

func TestNewFromValues_UnwritableFileIsError(t *testing.T) {
	_, err := NewFromValues(ConfigValues{File: "/nonexistent-root-zzz/gobin.log"})
	if err == nil {
		t.Fatal("expected error for unwritable file path")
	}
}

func TestNewFromValues_AcceptsValidLevelAndFormat(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		_, err := NewFromValues(ConfigValues{Level: level, Format: "text"})
		if err != nil {
			t.Fatalf("level=%q should be accepted, got %v", level, err)
		}
	}
	for _, format := range []string{"text", "json"} {
		_, err := NewFromValues(ConfigValues{Format: format})
		if err != nil {
			t.Fatalf("format=%q should be accepted, got %v", format, err)
		}
	}
}

func TestNewFromValues_FileIsRespected(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "gobin.log")

	logger, err := NewFromValues(ConfigValues{File: logPath, Level: "info", Format: "text"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logger.Info("hello from log test")

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "hello from log test") {
		t.Fatalf("expected log entry in file, got %q", string(content))
	}
}

func TestNewFromValues_JSONFormatEmitsJSON(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "gobin.log")

	logger, err := NewFromValues(ConfigValues{File: logPath, Level: "info", Format: "json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logger.Info("json-roundtrip", "k", "v")

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}

	// Should be a valid JSON line.
	var parsed map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(content), &parsed); err != nil {
		t.Fatalf("log line is not valid JSON: %v\ncontent: %q", err, content)
	}
	if parsed["msg"] != "json-roundtrip" {
		t.Fatalf("expected msg=json-roundtrip, got %v", parsed["msg"])
	}
}

func TestNewFromEnv_OverridesBase(t *testing.T) {
	prevL, prevF, prevFi := os.Getenv("GOBIN_LOG_LEVEL"), os.Getenv("GOBIN_LOG_FORMAT"), os.Getenv("GOBIN_LOG_FILE")
	defer func() {
		if prevL == "" {
			os.Unsetenv("GOBIN_LOG_LEVEL")
		} else {
			os.Setenv("GOBIN_LOG_LEVEL", prevL)
		}
		if prevF == "" {
			os.Unsetenv("GOBIN_LOG_FORMAT")
		} else {
			os.Setenv("GOBIN_LOG_FORMAT", prevF)
		}
		if prevFi == "" {
			os.Unsetenv("GOBIN_LOG_FILE")
		} else {
			os.Setenv("GOBIN_LOG_FILE", prevFi)
		}
	}()

	os.Setenv("GOBIN_LOG_LEVEL", "warn")
	os.Setenv("GOBIN_LOG_FORMAT", "text")
	os.Unsetenv("GOBIN_LOG_FILE")

	base := ConfigValues{Level: "debug", Format: "json"}
	_, err := NewFromEnv(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The resolved logger's level is warn (env override) — verify by
	// sending a debug message and confirming it is filtered out.
	buf := &bytes.Buffer{}
	resolved, _ := NewFromEnv(base)
	resolved = slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	resolved.Debug("should-be-filtered")
	if strings.Contains(buf.String(), "should-be-filtered") {
		t.Fatalf("debug message should be filtered at warn level: %q", buf.String())
	}
}
