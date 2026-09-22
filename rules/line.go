package rules

import "strings"

// lineOf — 1-based номер строки первого вхождения needle; если нет — 1.
func lineOf(src, needle string) int {
	if needle == "" {
		return 1
	}
	idx := strings.Index(src, needle)
	if idx < 0 {
		return 1
	}
	return strings.Count(src[:idx], "\n") + 1
}

// lineOfAny — первая из needles, которая нашлась.
func lineOfAny(src string, needles ...string) int {
	best := -1
	bestLine := 1
	for _, n := range needles {
		idx := strings.Index(src, n)
		if idx < 0 {
			continue
		}
		if best < 0 || idx < best {
			best = idx
			bestLine = strings.Count(src[:idx], "\n") + 1
		}
	}
	return bestLine
}
