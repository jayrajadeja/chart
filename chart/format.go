package chart

import "strconv"

// itoa formats a base-10 integer.
func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// numWidth is the printed width of n in base 10 (including a leading '-').
func numWidth(n int64) int { return len(itoa(n)) }

// spaces returns n spaces.
func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}

// padLeft right-aligns n in a field of width w.
func padLeft(n int64, w int) string {
	s := itoa(n)
	if len(s) >= w {
		return s
	}
	return spaces(w-len(s)) + s
}
