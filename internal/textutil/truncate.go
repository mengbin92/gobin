package textutil

// TruncateRunes limits s to max runes and appends suffix when truncation occurs.
func TruncateRunes(s string, max int, suffix string) string {
	if max < 0 {
		max = 0
	}

	count := 0
	for idx := range s {
		if count == max {
			return s[:idx] + suffix
		}
		count++
	}

	return s
}
