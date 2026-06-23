package config

import (
	"strconv"
	"strings"
)

type pathPart struct {
	key   string
	index *int
}

func parsePath(path string) ([]pathPart, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, &ConfigError{Kind: ErrPath, Message: "empty path"}
	}
	var tokens []string
	var b strings.Builder
	esc := false
	for i := 0; i < len(path); i++ {
		c := path[i]
		if esc {
			b.WriteByte(c)
			esc = false
			continue
		}
		if c == '\\' {
			esc = true
			continue
		}
		if c == '.' {
			if b.Len() == 0 {
				return nil, &ConfigError{Kind: ErrPath, Path: path, Message: "empty segment"}
			}
			tokens = append(tokens, b.String())
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	if esc {
		return nil, &ConfigError{Kind: ErrPath, Path: path, Message: "dangling escape"}
	}
	if b.Len() == 0 {
		return nil, &ConfigError{Kind: ErrPath, Path: path, Message: "empty segment"}
	}
	tokens = append(tokens, b.String())
	parts := make([]pathPart, 0, len(tokens)*2)
	for _, tok := range tokens {
		for len(tok) > 0 {
			br := strings.IndexByte(tok, '[')
			if br < 0 {
				if isNumeric(tok) {
					idx, _ := strconv.Atoi(tok)
					parts = append(parts, pathPart{index: &idx})
				} else {
					parts = append(parts, pathPart{key: tok})
				}
				break
			}
			if br > 0 {
				parts = append(parts, pathPart{key: tok[:br]})
			}
			end := strings.IndexByte(tok[br:], ']')
			if end < 0 {
				return nil, &ConfigError{Kind: ErrPath, Path: path, Message: "missing ]"}
			}
			idxs := tok[br+1 : br+end]
			idx, err := strconv.Atoi(idxs)
			if err != nil || idx < 0 {
				return nil, &ConfigError{Kind: ErrPath, Path: path, Message: "invalid index"}
			}
			parts = append(parts, pathPart{index: &idx})
			tok = tok[br+end+1:]
		}
	}
	return parts, nil
}
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
