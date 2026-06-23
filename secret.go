package config

import (
	"bytes"

	json "github.com/oarkflow/fastjson"
)

func (m *Manager) MarkSecret(paths ...string)   { m.AddSensitivePaths(paths...) }
func (m *Manager) UnmarkSecret(paths ...string) { m.RemoveSensitivePaths(paths...) }
func (m *Manager) Redacted() map[string]any {
	return redactMap(map[string]any(m.cur.Load().Tree.Clone()), "", nil, m.sensitive)
}
func (m *Manager) RedactedJSON() []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m.Redacted()); err != nil {
		return []byte("{}")
	}
	out := buf.Bytes()
	if len(out) > 0 && out[len(out)-1] == '\n' {
		out = out[:len(out)-1]
	}
	return append([]byte(nil), out...)
}
func redactMap(mp map[string]any, prefix string, explicit map[string]bool, matcher *SensitiveMatcher) map[string]any {
	out := map[string]any{}
	redaction := DefaultRedactionText
	if matcher != nil {
		redaction = matcher.RedactionText()
	}
	for k, v := range mp {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		if isSensitivePath(path, explicit, matcher) {
			out[k] = redaction
			continue
		}
		switch x := v.(type) {
		case map[string]any:
			out[k] = redactMap(x, path, explicit, matcher)
		case Tree:
			out[k] = redactMap(map[string]any(x), path, explicit, matcher)
		case SecretString:
			out[k] = x.String()
		default:
			out[k] = v
		}
	}
	return out
}
