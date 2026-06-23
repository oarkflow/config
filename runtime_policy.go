package config

import (
	"context"
	"strings"
)

type RuntimePolicy struct {
	Path              string   `json:"path"`
	Mutable           bool     `json:"mutable"`
	RequiresRestart   bool     `json:"requires_restart,omitempty"`
	Roles             []string `json:"roles,omitempty"`
	DenyEnvironments  []string `json:"deny_environments,omitempty"`
	AllowEnvironments []string `json:"allow_environments,omitempty"`
	Sensitive         bool     `json:"sensitive,omitempty"`
}

func Mutable(path string) RuntimePolicy   { return RuntimePolicy{Path: path, Mutable: true} }
func Immutable(path string) RuntimePolicy { return RuntimePolicy{Path: path, Mutable: false} }

func (m *Manager) SetRuntimePolicies(policies ...RuntimePolicy) {
	if m.runtimePolicies == nil {
		m.runtimePolicies = map[string]RuntimePolicy{}
	}
	for _, p := range policies {
		p.Path = normalizeSensitivePath(p.Path)
		if p.Path != "" {
			m.runtimePolicies[p.Path] = p
			if p.Sensitive {
				m.AddSensitivePaths(p.Path)
			}
		}
	}
}
func (m *Manager) RuntimePolicies() []RuntimePolicy {
	out := make([]RuntimePolicy, 0, len(m.runtimePolicies))
	for _, p := range m.runtimePolicies {
		out = append(out, p)
	}
	return out
}
func (m *Manager) Policy(path string) (RuntimePolicy, bool) {
	p, ok := m.runtimePolicies[normalizeSensitivePath(path)]
	return p, ok
}

func (m *Manager) authorizeMutation(path string, actor Actor) error {
	npath := normalizeSensitivePath(path)
	pol, ok := m.runtimePolicies[npath]
	if !ok {
		for p, rp := range m.runtimePolicies {
			if strings.HasSuffix(p, ".*") && strings.HasPrefix(npath, strings.TrimSuffix(p, "*")) {
				pol = rp
				ok = true
				break
			}
		}
	}
	if !ok {
		return nil
	}
	if !pol.Mutable {
		return &ConfigError{Kind: ErrSecurity, Path: path, Message: "runtime mutation is not allowed"}
	}
	if pol.RequiresRestart {
		return RestartRequired(path)
	}
	env := strings.ToLower(m.String("app.env", m.String("env")))
	if env != "" {
		for _, d := range pol.DenyEnvironments {
			if strings.EqualFold(env, d) {
				return &ConfigError{Kind: ErrSecurity, Path: path, Message: "runtime mutation denied in environment " + env}
			}
		}
		if len(pol.AllowEnvironments) > 0 {
			allowed := false
			for _, a := range pol.AllowEnvironments {
				if strings.EqualFold(env, a) {
					allowed = true
					break
				}
			}
			if !allowed {
				return &ConfigError{Kind: ErrSecurity, Path: path, Message: "runtime mutation not allowed in environment " + env}
			}
		}
	}
	if len(pol.Roles) > 0 {
		have := map[string]bool{}
		for _, r := range actor.Roles {
			have[strings.ToLower(r)] = true
		}
		okRole := false
		for _, r := range pol.Roles {
			if have[strings.ToLower(r)] {
				okRole = true
				break
			}
		}
		if !okRole {
			return &ConfigError{Kind: ErrSecurity, Path: path, Message: "actor does not have required role"}
		}
	}
	return nil
}

func (m *Manager) SetWithActor(ctx context.Context, actor Actor, path string, value any) error {
	if m.security.DenyUnknownSet {
		if _, ok := m.meta[path]; !ok {
			return &ConfigError{Kind: ErrSecurity, Path: path, Message: "unknown runtime key"}
		}
	}
	if err := m.authorizeMutation(path, actor); err != nil {
		m.emitAudit(ctx, AuditEvent{Event: EventReloadRejected, Actor: actor, Path: path, Error: err.Error()})
		return err
	}
	old := m.cur.Load()
	nt := old.Tree.Clone()
	if err := nt.Set(path, value); err != nil {
		return err
	}
	return m.commitTreeWithContext(ctx, nt, old.Sources, old.Warnings, actor, EventRuntimeSet, path)
}

func (m *Manager) DeleteWithActor(ctx context.Context, actor Actor, path string) error {
	if err := m.authorizeMutation(path, actor); err != nil {
		m.emitAudit(ctx, AuditEvent{Event: EventReloadRejected, Actor: actor, Path: path, Error: err.Error()})
		return err
	}
	old := m.cur.Load()
	nt := old.Tree.Clone()
	if err := nt.Delete(path); err != nil {
		return err
	}
	return m.commitTreeWithContext(ctx, nt, old.Sources, old.Warnings, actor, EventRuntimeDelete, path)
}
