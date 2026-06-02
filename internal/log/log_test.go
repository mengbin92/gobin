package log

import (
	"os"
	"path/filepath"
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
