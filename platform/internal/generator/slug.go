package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a string to a url-safe slug.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		s = s[:80]
		// trim to last hyphen to avoid breaking words
		if i := strings.LastIndex(s, "-"); i > 30 {
			s = s[:i]
		}
	}
	return s
}

// Hash returns a short deterministic hash suitable for idempotency keys.
func Hash(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}
