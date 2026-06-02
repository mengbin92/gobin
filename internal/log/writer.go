package log

import (
	"io"
	"os"
)

// resolveWriter maps an Options.Output string to an io.Writer.
// "stdout" -> os.Stdout, "stderr"/"" -> os.Stderr, otherwise a file
// opened in append mode. On open failure it warns and falls back to stderr.
func resolveWriter(output string) io.Writer {
	switch output {
	case "stdout":
		return os.Stdout
	case "stderr", "":
		return os.Stderr
	default:
		f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			os.Stderr.WriteString("warning: cannot open log file " + output + ", falling back to stderr\n")
			return os.Stderr
		}
		return f
	}
}
