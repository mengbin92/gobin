package shortcode

import "strings"

// span is a half-open byte range [start, end) within the source.
type span struct {
	start int
	end   int
}

// protectedRanges returns the byte ranges of the source that must not be
// scanned for shortcode delimiters: fenced code blocks (``` or ~~~) and inline
// code spans (`...`). Ranges are returned in ascending order and do not
// overlap meaningfully for the purposes of inProtected.
func protectedRanges(src string) []span {
	var spans []span

	inFence := false
	var fenceChar byte
	fenceLen := 0
	fenceStart := 0

	offset := 0
	for line := range strings.SplitSeq(src, "\n") {
		lineStart := offset
		lineEnd := lineStart + len(line)
		trimmed := strings.TrimLeft(line, " \t")

		if inFence {
			if c, l := fenceMarker(trimmed); l > 0 && c == fenceChar && l >= fenceLen {
				inFence = false
				spans = append(spans, span{fenceStart, lineEnd})
			}
		} else if c, l := fenceMarker(trimmed); l > 0 {
			inFence = true
			fenceChar = c
			fenceLen = l
			fenceStart = lineStart
		} else {
			spans = append(spans, inlineCodeSpans(line, lineStart)...)
		}

		offset = lineEnd + 1 // account for the split '\n'
	}

	if inFence {
		spans = append(spans, span{fenceStart, len(src)})
	}

	return spans
}

// fenceMarker reports the fence character and run length when s begins with a
// run of at least three '`' or '~' characters. Otherwise it returns a zero
// length.
func fenceMarker(s string) (byte, int) {
	if len(s) < 3 {
		return 0, 0
	}
	c := s[0]
	if c != '`' && c != '~' {
		return 0, 0
	}
	n := 0
	for n < len(s) && s[n] == c {
		n++
	}
	if n < 3 {
		return 0, 0
	}
	return c, n
}

// inlineCodeSpans returns the protected ranges for inline code spans within a
// single line. base is the byte offset of the line within the full source. An
// inline span opened by a run of N backticks is closed by the next run of
// exactly N backticks on the same line; an unterminated run is not protected.
func inlineCodeSpans(line string, base int) []span {
	var spans []span
	i := 0
	for i < len(line) {
		if line[i] != '`' {
			i++
			continue
		}
		runLen := 0
		for i+runLen < len(line) && line[i+runLen] == '`' {
			runLen++
		}
		openStart := i
		j := i + runLen
		closeFound := false
		for j < len(line) {
			if line[j] != '`' {
				j++
				continue
			}
			closeLen := 0
			for j+closeLen < len(line) && line[j+closeLen] == '`' {
				closeLen++
			}
			if closeLen == runLen {
				spans = append(spans, span{base + openStart, base + j + closeLen})
				i = j + closeLen
				closeFound = true
				break
			}
			j += closeLen
		}
		if !closeFound {
			i += runLen
		}
	}
	return spans
}

// inProtected reports whether the opening delimiter at byte offset pos falls
// inside any protected range.
func inProtected(pos int, protected []span) bool {
	for _, s := range protected {
		if pos >= s.start && pos < s.end {
			return true
		}
	}
	return false
}
