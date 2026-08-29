package sqlwhere

import (
	"fmt"
	"strings"
)

func quoteIdent(name string, d Dialect) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%w", ErrInvalidIdent)
	}
	parts := strings.Split(name, ".")
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		if !validIdentPart(part) {
			return "", fmt.Errorf("%w: %q", ErrInvalidIdent, name)
		}
		quoted = append(quoted, quotePart(part, d))
	}
	return strings.Join(quoted, "."), nil
}

func validIdentPart(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 0 {
			if c != '_' && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
				return false
			}
			continue
		}
		if c != '_' && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}
