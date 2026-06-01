package shortcode

import "fmt"

// token is a single scanned shortcode delimiter (an opening or closing tag).
type token struct {
	name    string
	form    form
	closing bool
	args    *args
	// rawStart is the byte offset of the leading "{{<"/"{{%".
	rawStart int
	// rawEnd is the byte offset just past the trailing ">}}"/"%}}".
	rawEnd int
}

// scanTokens scans src for shortcode delimiters that fall outside protected
// ranges, returning them in source order. It errors on an unterminated
// delimiter or malformed arguments.
func scanTokens(src string, protected []span) ([]token, error) {
	var tokens []token

	i := 0
	for i < len(src) {
		idx, fm := nextOpener(src, i)
		if idx < 0 {
			break
		}
		if inProtected(idx, protected) {
			i = idx + 3
			continue
		}

		closeStr := ">}}"
		if fm == formMarkdown {
			closeStr = "%}}"
		}

		end, ok := findCloser(src, idx+3, closeStr)
		if !ok {
			return nil, fmt.Errorf("unterminated shortcode tag at offset %d", idx)
		}

		name, closing, a, err := parseArgs(src[idx+3 : end])
		if err != nil {
			return nil, err
		}

		tokens = append(tokens, token{
			name:     name,
			form:     fm,
			closing:  closing,
			args:     a,
			rawStart: idx,
			rawEnd:   end + len(closeStr),
		})
		i = end + len(closeStr)
	}

	return tokens, nil
}

// nextOpener returns the offset of the next "{{<" or "{{%" at or after start,
// together with its form. It returns (-1, _) when none is found.
func nextOpener(src string, start int) (int, form) {
	for i := start; i+2 < len(src); i++ {
		if src[i] != '{' || src[i+1] != '{' {
			continue
		}
		switch src[i+2] {
		case '<':
			return i, formHTML
		case '%':
			return i, formMarkdown
		}
	}
	return -1, formHTML
}

// findCloser scans from start for the closing delimiter closeStr, ignoring any
// occurrence inside a double-quoted string. It returns the offset of the closer
// and whether it was found.
func findCloser(src string, start int, closeStr string) (int, bool) {
	inQuote := false
	for i := start; i < len(src); i++ {
		c := src[i]
		if c == '"' {
			inQuote = !inQuote
			continue
		}
		if !inQuote && i+len(closeStr) <= len(src) && src[i:i+len(closeStr)] == closeStr {
			return i, true
		}
	}
	return 0, false
}
