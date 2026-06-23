package config

import (
	"sort"
	"strings"
	"sync"
)

const DefaultRedactionText = "[REDACTED]"

var DefaultSensitiveWords = []string{
	"password",
	"passwd",
	"pwd",
	"secret",
	"token",
	"access_token",
	"refresh_token",
	"id_token",
	"api_key",
	"apikey",
	"private_key",
	"client_secret",
	"credential",
	"credentials",
	"dsn",
	"connection_string",
	"auth_header",
	"authorization",
	"cookie",
	"session",
}

type SensitivePolicy struct {
	Words         []string
	Paths         []string
	EnvVars       []string
	Redaction     string
	MatchContains bool
}

func DefaultSensitivePolicy() SensitivePolicy {
	return SensitivePolicy{Words: append([]string(nil), DefaultSensitiveWords...), Redaction: DefaultRedactionText, MatchContains: true}
}

type SensitiveSnapshot struct {
	Words         []string `json:"words"`
	Paths         []string `json:"paths"`
	EnvVars       []string `json:"env_vars"`
	Redaction     string   `json:"redaction"`
	MatchContains bool     `json:"match_contains"`
}

type SensitiveMatcher struct {
	mu            sync.RWMutex
	words         map[string]struct{}
	paths         map[string]struct{}
	envVars       map[string]struct{}
	redaction     string
	matchContains bool
}

func NewSensitiveMatcher(policy SensitivePolicy) *SensitiveMatcher {
	if policy.Redaction == "" {
		policy.Redaction = DefaultRedactionText
	}
	m := &SensitiveMatcher{
		words:         map[string]struct{}{},
		paths:         map[string]struct{}{},
		envVars:       map[string]struct{}{},
		redaction:     policy.Redaction,
		matchContains: policy.MatchContains,
	}
	m.AddWords(policy.Words...)
	m.AddPaths(policy.Paths...)
	m.AddEnvVars(policy.EnvVars...)
	return m
}

func normalizeSensitiveWord(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "-", "_")
	return s
}
func normalizeSensitivePath(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.Trim(s, ".")
	s = strings.ReplaceAll(s, "[", ".")
	s = strings.ReplaceAll(s, "]", "")
	return s
}
func normalizeEnvName(s string) string {
	return strings.TrimSpace(strings.ToUpper(s))
}

func (m *SensitiveMatcher) Snapshot() SensitiveSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return SensitiveSnapshot{Words: sortedKeys(m.words), Paths: sortedKeys(m.paths), EnvVars: sortedKeys(m.envVars), Redaction: m.redaction, MatchContains: m.matchContains}
}
func (m *SensitiveMatcher) RedactionText() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.redaction == "" {
		return DefaultRedactionText
	}
	return m.redaction
}
func (m *SensitiveMatcher) SetRedactionText(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if text == "" {
		text = DefaultRedactionText
	}
	m.redaction = text
}
func (m *SensitiveMatcher) SetMatchContains(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.matchContains = enabled
}
func (m *SensitiveMatcher) AddWords(words ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, w := range words {
		w = normalizeSensitiveWord(w)
		if w != "" {
			m.words[w] = struct{}{}
		}
	}
}
func (m *SensitiveMatcher) RemoveWords(words ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, w := range words {
		delete(m.words, normalizeSensitiveWord(w))
	}
}
func (m *SensitiveMatcher) SetWords(words ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.words = map[string]struct{}{}
	for _, w := range words {
		w = normalizeSensitiveWord(w)
		if w != "" {
			m.words[w] = struct{}{}
		}
	}
}
func (m *SensitiveMatcher) AddPaths(paths ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range paths {
		p = normalizeSensitivePath(p)
		if p != "" {
			m.paths[p] = struct{}{}
		}
	}
}
func (m *SensitiveMatcher) RemovePaths(paths ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range paths {
		delete(m.paths, normalizeSensitivePath(p))
	}
}
func (m *SensitiveMatcher) SetPaths(paths ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.paths = map[string]struct{}{}
	for _, p := range paths {
		p = normalizeSensitivePath(p)
		if p != "" {
			m.paths[p] = struct{}{}
		}
	}
}
func (m *SensitiveMatcher) AddEnvVars(names ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, n := range names {
		n = normalizeEnvName(n)
		if n != "" {
			m.envVars[n] = struct{}{}
		}
	}
}
func (m *SensitiveMatcher) RemoveEnvVars(names ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, n := range names {
		delete(m.envVars, normalizeEnvName(n))
	}
}
func (m *SensitiveMatcher) SetEnvVars(names ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.envVars = map[string]struct{}{}
	for _, n := range names {
		n = normalizeEnvName(n)
		if n != "" {
			m.envVars[n] = struct{}{}
		}
	}
}
func (m *SensitiveMatcher) IsPathSensitive(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	path = normalizeSensitivePath(path)
	if path == "" {
		return false
	}
	if _, ok := m.paths[path]; ok {
		return true
	}
	parts := strings.Split(path, ".")
	for _, part := range parts {
		part = normalizeSensitiveWord(part)
		if part == "" {
			continue
		}
		if _, ok := m.words[part]; ok {
			return true
		}
		if m.matchContains {
			for w := range m.words {
				if w != "" && strings.Contains(part, w) {
					return true
				}
			}
		}
	}
	return false
}
func (m *SensitiveMatcher) IsEnvSensitive(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	name = normalizeEnvName(name)
	if name == "" {
		return false
	}
	if _, ok := m.envVars[name]; ok {
		return true
	}
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '_' || r == '-' || r == '.' })
	for _, part := range parts {
		part = normalizeSensitiveWord(part)
		if _, ok := m.words[part]; ok {
			return true
		}
		if m.matchContains {
			for w := range m.words {
				if w != "" && strings.Contains(part, w) {
					return true
				}
			}
		}
	}
	return false
}

func sortedKeys[V any](mp map[string]V) []string {
	keys := make([]string, 0, len(mp))
	for k := range mp {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (m *Manager) Sensitive() SensitiveSnapshot         { return m.sensitive.Snapshot() }
func (m *Manager) AddSensitiveWords(words ...string)    { m.sensitive.AddWords(words...) }
func (m *Manager) RemoveSensitiveWords(words ...string) { m.sensitive.RemoveWords(words...) }
func (m *Manager) SetSensitiveWords(words ...string)    { m.sensitive.SetWords(words...) }
func (m *Manager) AddSensitivePaths(paths ...string) {
	m.sensitive.AddPaths(paths...)
	for _, p := range paths {
		m.secretPaths[normalizeSensitivePath(p)] = true
	}
}
func (m *Manager) RemoveSensitivePaths(paths ...string) {
	m.sensitive.RemovePaths(paths...)
	for _, p := range paths {
		delete(m.secretPaths, normalizeSensitivePath(p))
	}
}
func (m *Manager) SetSensitivePaths(paths ...string) {
	m.sensitive.SetPaths(paths...)
	m.secretPaths = map[string]bool{}
	for _, p := range paths {
		m.secretPaths[normalizeSensitivePath(p)] = true
	}
}
func (m *Manager) AddSensitiveEnvVars(names ...string)    { m.sensitive.AddEnvVars(names...) }
func (m *Manager) RemoveSensitiveEnvVars(names ...string) { m.sensitive.RemoveEnvVars(names...) }
func (m *Manager) SetSensitiveEnvVars(names ...string)    { m.sensitive.SetEnvVars(names...) }
func (m *Manager) SetRedactionText(text string)           { m.sensitive.SetRedactionText(text) }
func (m *Manager) IsSensitive(path string) bool           { return m.sensitive.IsPathSensitive(path) }

func isSensitivePath(path string, explicit map[string]bool, matcher *SensitiveMatcher) bool {
	p := normalizeSensitivePath(path)
	if explicit[p] || explicit[path] {
		return true
	}
	if matcher != nil && matcher.IsPathSensitive(path) {
		return true
	}
	return false
}
