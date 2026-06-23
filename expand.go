package config

import (
	"os"
	"strings"
)

func (m *Manager) expandTree(t Tree) Tree {
	if !m.security.AllowEnvExpansion {
		return t
	}
	return expandValue(t).(Tree)
}
func expandValue(v any) any {
	switch x := v.(type) {
	case string:
		return os.Expand(x, func(k string) string { return os.Getenv(k) })
	case map[string]any:
		out := map[string]any{}
		for k, v := range x {
			out[k] = expandValue(v)
		}
		return out
	case Tree:
		out := Tree{}
		for k, v := range x {
			out[k] = expandValue(v)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = expandValue(v)
		}
		return out
	case SecretString:
		return NewSecretString(os.Expand(x.Value(), func(k string) string { return os.Getenv(k) }))
	default:
		_ = strings.Builder{}
		return v
	}
}
