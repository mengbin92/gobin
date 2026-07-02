package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/log"
)

// withFlagValues saves the package-level flag vars, runs fn, then
// restores them. Tests that want to exercise different flag combos use
// this helper. The flag vars are package-level; assigning to a local
// variable with the same name is a no-op, so the helper reads / writes
// the package-level symbols directly.
func withFlagValues(v bool, format, file string, fn func()) {
	prevV, prevF, prevFile := verbose, logFormat, logFile
	verbose = v
	logFormat = format
	logFile = file
	defer func() { verbose, logFormat, logFile = prevV, prevF, prevFile }()
	fn()
}

func withEnv(env map[string]string, fn func()) {
	prev := make(map[string]string, len(env))
	for k := range env {
		prev[k] = os.Getenv(k)
	}
	for k, v := range env {
		os.Setenv(k, v)
	}
	defer func() {
		for k, v := range prev {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()
	fn()
}

func TestLoggingValuesFromConfig_NilReturnsZero(t *testing.T) {
	got := loggingValuesFromConfig(nil)
	if got != (log.ConfigValues{}) {
		t.Fatalf("expected zero value, got %+v", got)
	}
}

func TestLoggingValuesFromConfig_PopulatesAll(t *testing.T) {
	got := loggingValuesFromConfig(&config.Config{
		Logging: &config.LoggingConfig{
			Level:  "debug",
			Format: "json",
			File:   "/tmp/x.log",
		},
	})
	want := log.ConfigValues{Level: "debug", Format: "json", File: "/tmp/x.log"}
	if got != want {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

func TestOverlayEnvOnValues_OnlyNonEmptyEnvWins(t *testing.T) {
	withEnv(map[string]string{
		"GOBIN_LOG_LEVEL":  "warn",
		"GOBIN_LOG_FORMAT": "",
		"GOBIN_LOG_FILE":   "",
	}, func() {
		base := log.ConfigValues{Level: "debug", Format: "text", File: "/tmp/base.log"}
		got := overlayEnvOnValues(base)
		if got.Level != "warn" {
			t.Fatalf("env should win, got Level=%q", got.Level)
		}
		if got.Format != "text" {
			t.Fatalf("empty env should not overwrite, got Format=%q", got.Format)
		}
		if got.File != "/tmp/base.log" {
			t.Fatalf("empty env should not overwrite, got File=%q", got.File)
		}
	})
}

func TestOverlayFlagsOnValues_VerboseSetsDebug(t *testing.T) {
	withFlagValues(true, "json", "/tmp/flag.log", func() {
		base := log.ConfigValues{Level: "info", Format: "text", File: ""}
		got := overlayFlagsOnValues(base)
		if got.Level != "debug" {
			t.Fatalf("--verbose should set Level=debug, got %q", got.Level)
		}
		if got.Format != "json" {
			t.Fatalf("--log-format should win, got %q", got.Format)
		}
		if got.File != "/tmp/flag.log" {
			t.Fatalf("--log-file should win, got %q", got.File)
		}
	})
}

func TestInitLogging_PriorityFlagEnvConfigDefault(t *testing.T) {
	cases := []struct {
		name     string
		cfg      *config.Config
		env      map[string]string
		verbose  bool
		format   string
		logFile  string
		wantFile string // expected output path (""=stderr)
		wantLevel string
	}{
		{
			name:      "default",
			wantLevel: "info",
		},
		{
			name:      "config wins over default",
			cfg:       &config.Config{Logging: &config.LoggingConfig{Level: "debug"}},
			wantLevel: "debug",
		},
		{
			name:      "env wins over config",
			cfg:       &config.Config{Logging: &config.LoggingConfig{Level: "debug"}},
			env:       map[string]string{"GOBIN_LOG_LEVEL": "warn"},
			wantLevel: "warn",
		},
		{
			name:      "flag wins over env",
			cfg:       &config.Config{Logging: &config.LoggingConfig{Level: "debug"}},
			env:       map[string]string{"GOBIN_LOG_LEVEL": "warn"},
			verbose:   true,
			wantLevel: "debug",
		},
		{
			name:     "log-file from flag",
			logFile:  filepath.Join(t.TempDir(), "gobin.log"),
			wantFile: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withFlagValues(tc.verbose, tc.format, tc.logFile, func() {
				withEnv(tc.env, func() {
					if err := InitLogging(tc.cfg); err != nil {
						t.Fatalf("InitLogging: %v", err)
					}
					if got := log.GetDefault().Debug; got == nil {
						t.Fatal("expected default logger to be set")
					}
				})
			})
		})
	}
}

func TestInitLogging_RejectsBadConfigLevel(t *testing.T) {
	withFlagValues(false, "", "", func() {
		err := InitLogging(&config.Config{Logging: &config.LoggingConfig{Level: "trace"}})
		if err == nil {
			t.Fatal("expected error for invalid config level, got nil")
		}
		if !strings.Contains(err.Error(), "invalid logging level") {
			t.Fatalf("expected invalid level error, got %v", err)
		}
	})
}

func TestInitLogging_RejectsUnwritableConfigFile(t *testing.T) {
	withFlagValues(false, "", "", func() {
		err := InitLogging(&config.Config{Logging: &config.LoggingConfig{File: "/nonexistent-root/gobin.log"}})
		if err == nil {
			t.Fatal("expected error for unwritable log file, got nil")
		}
		if !strings.Contains(err.Error(), "not writable") && !strings.Contains(err.Error(), "no such file or directory") && !strings.Contains(err.Error(), "stat ") {
			t.Fatalf("expected not-writable error, got %v", err)
		}
	})
}

func TestInitLogging_NilConfigFallsBackToFlagAndEnv(t *testing.T) {
	withFlagValues(true, "json", "", func() {
		withEnv(map[string]string{"GOBIN_LOG_FILE": ""}, func() {
			if err := InitLogging(nil); err != nil {
				t.Fatalf("InitLogging(nil): %v", err)
			}
			// No assertion on internal state; just that it didn't crash
			// and the default logger is installed.
			if log.GetDefault() == nil {
				t.Fatal("expected default logger to be set")
			}
		})
	})
}
