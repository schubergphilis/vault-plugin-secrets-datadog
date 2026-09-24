package plugin

import "strings"

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// nonEmpty returns the trimmed, non-empty elements of s.
func nonEmpty(s []string) []string {
	out := []string{}
	for _, e := range s {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, e)
		}
	}
	return out
}
