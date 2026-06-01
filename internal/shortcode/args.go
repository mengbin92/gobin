package shortcode

import (
	"fmt"
	"strings"
)

// form distinguishes the two shortcode delimiter styles.
type form int

const (
	// formHTML is the {{< name args >}} style; its template output is injected
	// verbatim as HTML (it bypasses the Markdown renderer via a sentinel).
	formHTML form = iota
	// formMarkdown is the {{% name args %}} style; its template output is
	// spliced back into the Markdown source and rendered as Markdown.
	formMarkdown
)

// args holds the parsed positional and named parameters of a shortcode
// invocation. Positional order is preserved.
type args struct {
	positional []string
	named      map[string]string
}

// parseArgs parses the inner text of a shortcode delimiter (the part between
// "{{<"/"{{%" and ">}}"/"%}}"). It returns the shortcode name, whether the
// token is a closing tag ("/name"), and the parsed arguments (nil for closing
// tags).
func parseArgs(inner string) (name string, closing bool, a *args, err error) {
	toks, err := tokenizeArgs(inner)
	if err != nil {
		return "", false, nil, err
	}
	if len(toks) == 0 {
		return "", false, nil, fmt.Errorf("empty shortcode tag %q", strings.TrimSpace(inner))
	}

	name = toks[0]
	if rest, ok := strings.CutPrefix(name, "/"); ok {
		name = rest
		if name == "" || len(toks) > 1 {
			return "", false, nil, fmt.Errorf("invalid closing shortcode tag %q", strings.TrimSpace(inner))
		}
		return name, true, nil, nil
	}

	a = &args{named: map[string]string{}}
	for _, tok := range toks[1:] {
		if key, val, ok := splitNamed(tok); ok {
			a.named[key] = val
			continue
		}
		a.positional = append(a.positional, unquote(tok))
	}

	return name, false, a, nil
}

// tokenizeArgs splits an argument string into logical words, keeping
// double-quoted spans (including their spaces) together. Quote characters are
// retained so splitNamed/unquote can interpret them.
func tokenizeArgs(s string) ([]string, error) {
	var toks []string
	var cur strings.Builder
	inQuote := false
	started := false

	flush := func() {
		if started {
			toks = append(toks, cur.String())
			cur.Reset()
			started = false
		}
	}

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuote = !inQuote
			cur.WriteByte(c)
			started = true
		case (c == ' ' || c == '\t' || c == '\n' || c == '\r') && !inQuote:
			flush()
		default:
			cur.WriteByte(c)
			started = true
		}
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quote in shortcode args: %q", strings.TrimSpace(s))
	}
	flush()

	return toks, nil
}

// splitNamed reports whether tok is a key=value pair (with the '=' outside any
// quotes and a valid identifier key), returning the key and unquoted value.
func splitNamed(tok string) (key, val string, ok bool) {
	inQuote := false
	for i := 0; i < len(tok); i++ {
		switch tok[i] {
		case '"':
			inQuote = !inQuote
		case '=':
			if inQuote {
				continue
			}
			key = tok[:i]
			if key == "" || !isIdent(key) {
				return "", "", false
			}
			return key, unquote(tok[i+1:]), true
		}
	}
	return "", "", false
}

// isIdent reports whether s is a valid named-argument key:
// a leading ASCII letter followed by letters, digits, '_' or '-'.
func isIdent(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
			// always allowed
		case (c >= '0' && c <= '9') || c == '_' || c == '-':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// unquote strips a surrounding pair of double quotes and unescapes \" and \\.
// Strings without surrounding quotes are returned unchanged.
func unquote(s string) string {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return s
	}
	inner := s[1 : len(s)-1]
	var b strings.Builder
	b.Grow(len(inner))
	for i := 0; i < len(inner); i++ {
		if inner[i] == '\\' && i+1 < len(inner) {
			next := inner[i+1]
			if next == '"' || next == '\\' {
				b.WriteByte(next)
				i++
				continue
			}
		}
		b.WriteByte(inner[i])
	}
	return b.String()
}
