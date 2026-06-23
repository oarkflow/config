package config

import (
	"fmt"
	"strings"
	"time"
)

type EntryMeta struct {
	Path            string
	Env             string
	Kind            ValueKind
	Default         any
	DescriptionText string
	Secret          bool
	Reload          string
	Required        bool
	Validation      string
	Classification  Classification
	Deprecated      string
	Replacement     string
	Owner           string
}

func (e EntryMeta) Description() string { return e.DescriptionText }

type Entry struct {
	section *Section
	meta    EntryMeta
}

func (e *Entry) Description(text string) *Entry {
	e.meta.DescriptionText = text
	e.section.m.meta[e.meta.Path] = e.meta
	return e
}
func (e *Entry) Reload(policy string) *Entry {
	e.meta.Reload = policy
	e.section.m.meta[e.meta.Path] = e.meta
	return e
}
func (e *Entry) Required() *Entry {
	e.meta.Required = true
	e.section.m.meta[e.meta.Path] = e.meta
	return e
}
func (e *Entry) Validate(rules string) *Entry {
	e.meta.Validation = rules
	e.section.m.meta[e.meta.Path] = e.meta
	return e
}
func (e *Entry) Classification(c Classification) *Entry {
	e.meta.Classification = c
	if c == ClassSecret || c == ClassRestricted {
		e.Secret()
	}
	e.section.m.meta[e.meta.Path] = e.meta
	return e
}
func (e *Entry) Deprecated(message string) *Entry {
	e.meta.Deprecated = message
	e.section.m.meta[e.meta.Path] = e.meta
	return e
}
func (e *Entry) MigrateTo(path string) *Entry {
	e.meta.Replacement = path
	e.section.m.meta[e.meta.Path] = e.meta
	return e
}
func (e *Entry) Owner(owner string) *Entry {
	e.meta.Owner = owner
	e.section.m.meta[e.meta.Path] = e.meta
	return e
}
func (e *Entry) Immutable() *Entry {
	e.section.m.SetRuntimePolicies(RuntimePolicy{Path: e.meta.Path, Mutable: false, Sensitive: e.meta.Secret})
	return e
}
func (e *Entry) LiveReloadable(roles ...string) *Entry {
	e.section.m.SetRuntimePolicies(RuntimePolicy{Path: e.meta.Path, Mutable: true, Roles: roles, Sensitive: e.meta.Secret})
	return e
}
func (e *Entry) RestartRequired() *Entry {
	e.section.m.SetRuntimePolicies(RuntimePolicy{Path: e.meta.Path, Mutable: true, RequiresRestart: true, Sensitive: e.meta.Secret})
	return e
}
func (e *Entry) Secret() *Entry {
	e.meta.Secret = true
	e.section.m.secretPaths[e.meta.Path] = true
	e.section.m.sensitive.AddPaths(e.meta.Path)
	e.section.m.meta[e.meta.Path] = e.meta
	return e
}

type Section struct {
	m      *Manager
	prefix string
	values map[string]any
}

func (s *Section) path(key string) string {
	key = strings.Trim(key, ".")
	if s.prefix == "" {
		return key
	}
	return s.prefix + "." + key
}
func (s *Section) put(key, env string, kind ValueKind, def any, value any, secret bool) *Entry {
	s.values[key] = value
	meta := EntryMeta{Path: s.path(key), Env: env, Kind: kind, Default: def, Secret: secret, Classification: ClassInternal}
	s.m.meta[meta.Path] = meta
	if secret {
		s.m.secretPaths[meta.Path] = true
		s.m.sensitive.AddPaths(meta.Path)
	}
	return &Entry{section: s, meta: meta}
}
func (s *Section) Any(key string, value any) *Entry {
	return s.put(key, "", KindAny, value, value, false)
}
func (s *Section) String(key, env, def string) *Entry {
	return s.put(key, env, KindString, def, s.m.EnvString(env, def), false)
}
func (s *Section) Bool(key, env string, def bool) *Entry {
	return s.put(key, env, KindBool, def, s.m.EnvBool(env, def), false)
}
func (s *Section) Int(key, env string, def int) *Entry {
	return s.put(key, env, KindInt, def, s.m.EnvInt(env, def), false)
}
func (s *Section) Int64(key, env string, def int64) *Entry {
	return s.put(key, env, KindInt64, def, s.m.EnvInt64(env, def), false)
}
func (s *Section) Float64(key, env string, def float64) *Entry {
	return s.put(key, env, KindFloat64, def, s.m.EnvFloat64(env, def), false)
}
func (s *Section) Duration(key, env string, def time.Duration) *Entry {
	return s.put(key, env, KindDuration, def, s.m.EnvDuration(env, def), false)
}
func (s *Section) Size(key, env string, def Size) *Entry {
	return s.put(key, env, KindSize, def, s.m.EnvSize(env, def), false)
}
func (s *Section) StringSlice(key, env string, def []string) *Entry {
	return s.put(key, env, KindStringSlice, def, s.m.EnvStringSlice(env, def), false)
}
func (s *Section) SecretString(key, env, def string) *Entry {
	return s.put(key, env, KindSecret, "[REDACTED]", s.m.EnvSecret(env, def), true)
}
func (s *Section) Map(key string, values map[string]any) *Entry {
	return s.put(key, "", KindMap, values, values, false)
}
func (s *Section) Commit() error {
	if s.prefix == "" {
		return fmt.Errorf("empty section prefix")
	}
	return s.m.Add(s.prefix, s.values)
}
