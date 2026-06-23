package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

func (m *Manager) Env(key string, fallback any) any {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	switch fallback.(type) {
	case bool:
		b, e := strconv.ParseBool(v)
		if e == nil {
			return b
		}
	case int:
		i, e := strconv.Atoi(v)
		if e == nil {
			return i
		}
	case int64:
		i, e := strconv.ParseInt(v, 10, 64)
		if e == nil {
			return i
		}
	case float64:
		f, e := strconv.ParseFloat(v, 64)
		if e == nil {
			return f
		}
	case time.Duration:
		d, e := time.ParseDuration(v)
		if e == nil {
			return d
		}
	case Size:
		s, e := ParseSize(v)
		if e == nil {
			return s
		}
	case []string:
		return splitCSV(v)
	case SecretString:
		return NewSecretString(v)
	}
	return v
}
func (m *Manager) EnvString(key, fallback string) string { return m.Env(key, fallback).(string) }
func (m *Manager) EnvBool(key string, fallback bool) bool {
	v := m.Env(key, fallback)
	b, _ := ToBool(v)
	return b
}
func (m *Manager) EnvInt(key string, fallback int) int {
	v := m.Env(key, fallback)
	i, _ := ToInt(v)
	return i
}
func (m *Manager) EnvInt64(key string, fallback int64) int64 {
	v := m.Env(key, fallback)
	i, _ := ToInt64(v)
	return i
}
func (m *Manager) EnvFloat64(key string, fallback float64) float64 {
	v := m.Env(key, fallback)
	f, _ := ToFloat64(v)
	return f
}
func (m *Manager) EnvDuration(key string, fallback time.Duration) time.Duration {
	v := m.Env(key, fallback)
	d, _ := ToDuration(v)
	return d
}
func (m *Manager) EnvSize(key string, fallback Size) Size {
	v := m.Env(key, fallback)
	s, _ := ToSize(v)
	return s
}
func (m *Manager) EnvStringSlice(key string, fallback []string) []string {
	v := m.Env(key, fallback)
	s, _ := ToStringSlice(v)
	return s
}
func (m *Manager) EnvSecret(key string, fallback string) SecretString {
	v, ok := os.LookupEnv(key)
	if !ok {
		v = fallback
	}
	return NewSecretString(v)
}
func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
