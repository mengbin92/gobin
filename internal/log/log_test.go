package log

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWriter(t *testing.T) {
	if got := resolveWriter("stdout"); got != os.Stdout {
		t.Errorf("resolveWriter(stdout) = %v, want os.Stdout", got)
	}
	if got := resolveWriter("stderr"); got != os.Stderr {
		t.Errorf("resolveWriter(stderr) = %v, want os.Stderr", got)
	}
	if got := resolveWriter(""); got != os.Stderr {
		t.Errorf("resolveWriter(\"\") = %v, want os.Stderr", got)
	}
}

func TestResolveWriterFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")
	w := resolveWriter(path)
	f, ok := w.(*os.File)
	if !ok {
		t.Fatalf("resolveWriter(path) returned %T, want *os.File", w)
	}
	defer f.Close()
	if _, err := f.WriteString("hello\n"); err != nil {
		t.Fatalf("write to log file failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file failed: %v", err)
	}
	if string(data) != "hello\n" {
		t.Errorf("log file content = %q, want %q", string(data), "hello\n")
	}
}

func TestResolveWriterBadPathFallsBackToStderr(t *testing.T) {
	// A path inside a non-existent directory cannot be opened.
	bad := filepath.Join(t.TempDir(), "nope", "deeper", "x.log")
	if got := resolveWriter(bad); got != os.Stderr {
		t.Errorf("resolveWriter(bad) = %v, want os.Stderr fallback", got)
	}
}

// newTestLogger builds a logger writing into buf with the given format/level.
func newTestLogger(buf *bytes.Buffer, format Format, level slog.Level) *slog.Logger {
	var h slog.Handler
	opts := &slog.HandlerOptions{Level: level}
	if format == FormatJSON {
		h = slog.NewJSONHandler(buf, opts)
	} else {
		h = slog.NewTextHandler(buf, opts)
	}
	return slog.New(h)
}

func TestParseLevel(t *testing.T) {
	cases := map[Level]slog.Level{
		LevelDebug:    slog.LevelDebug,
		LevelInfo:     slog.LevelInfo,
		LevelWarn:     slog.LevelWarn,
		LevelError:    slog.LevelError,
		Level("bogus"): slog.LevelInfo, // unknown -> info
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestNewTextHandlerFiltersByLevel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.log")
	logger := New(Options{Level: LevelInfo, Format: FormatText, Output: path})
	logger.Debug("debug-msg")
	logger.Info("info-msg")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if strings.Contains(out, "debug-msg") {
		t.Errorf("INFO logger should not emit debug, got: %s", out)
	}
	if !strings.Contains(out, "info-msg") {
		t.Errorf("INFO logger should emit info, got: %s", out)
	}
}

func TestNewJSONFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	logger := New(Options{Level: LevelInfo, Format: FormatJSON, Output: path})
	logger.Info("hello", "k", "v")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(bytes.TrimRight(data, "\n"), &rec); err != nil {
		t.Fatalf("output is not valid JSON: %v (%q)", err, string(data))
	}
	if rec["msg"] != "hello" || rec["k"] != "v" {
		t.Errorf("unexpected JSON record: %v", rec)
	}
}

func TestNewFromCLIVerboseEnablesDebug(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v.log")
	logger := NewFromCLI(true, "text", path)
	logger.Debug("dbg-line")
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "dbg-line") {
		t.Errorf("verbose logger should emit debug, got: %s", string(data))
	}
}

func TestNewFromCLIDefaultsHideDebug(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nv.log")
	logger := NewFromCLI(false, "", path)
	logger.Debug("dbg-line")
	logger.Info("info-line")
	data, _ := os.ReadFile(path)
	out := string(data)
	if strings.Contains(out, "dbg-line") {
		t.Errorf("non-verbose logger should hide debug, got: %s", out)
	}
	if !strings.Contains(out, "info-line") {
		t.Errorf("non-verbose logger should show info, got: %s", out)
	}
}

func TestWithComponentAddsField(t *testing.T) {
	var buf bytes.Buffer
	base := newTestLogger(&buf, FormatJSON, slog.LevelInfo)
	logger := WithComponent(base, "parser")
	logger.Info("scanned")
	var rec map[string]any
	json.Unmarshal(bytes.TrimRight(buf.Bytes(), "\n"), &rec)
	if rec["component"] != "parser" {
		t.Errorf("component field = %v, want parser", rec["component"])
	}
}

func TestSetAndGetDefault(t *testing.T) {
	orig := GetDefault()
	defer SetDefault(orig)

	var buf bytes.Buffer
	custom := newTestLogger(&buf, FormatText, slog.LevelInfo)
	SetDefault(custom)
	if GetDefault() != custom {
		t.Error("GetDefault did not return the logger set by SetDefault")
	}
	Info("via-convenience")
	if !strings.Contains(buf.String(), "via-convenience") {
		t.Errorf("convenience Info did not route to default logger, got: %s", buf.String())
	}
	Warn("warn-convenience")
	if !strings.Contains(buf.String(), "warn-convenience") {
		t.Errorf("convenience Warn did not route to default logger, got: %s", buf.String())
	}
	Error("error-convenience")
	if !strings.Contains(buf.String(), "error-convenience") {
		t.Errorf("convenience Error did not route to default logger, got: %s", buf.String())
	}
}

func TestContextRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf, FormatText, slog.LevelInfo)
	ctx := IntoContext(context.Background(), logger)
	if FromContext(ctx) != logger {
		t.Error("FromContext did not return the logger put via IntoContext")
	}
}

func TestFromContextFallsBackToDefault(t *testing.T) {
	if FromContext(context.Background()) != GetDefault() {
		t.Error("FromContext on empty context should return the default logger")
	}
}
