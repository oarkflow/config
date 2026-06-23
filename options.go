package config

import "time"

type FailMode int

const (
	KeepPrevious FailMode = iota
	FailClosed
)

type ReloadPolicy struct {
	Debounce             time.Duration
	MinInterval          time.Duration
	ValidateBeforeCommit bool
	FailMode             FailMode
}

func DefaultReloadPolicy() ReloadPolicy {
	return ReloadPolicy{Debounce: 250 * time.Millisecond, MinInterval: time.Second, ValidateBeforeCommit: true, FailMode: KeepPrevious}
}

type SecurityPolicy struct {
	RedactSecrets       bool
	DenyUnknownSet      bool
	AllowEnvExpansion   bool
	AllowFileExpansion  bool
	DisableRawDump      bool
	DisableRawJSON      bool
	RequireRedactedDump bool
	Sensitive           SensitivePolicy
}

func DefaultSecurityPolicy() SecurityPolicy {
	p := DefaultSensitivePolicy()
	return SecurityPolicy{RedactSecrets: true, AllowEnvExpansion: true, Sensitive: p}
}

type Option func(*Manager)

func WithReloadPolicy(p ReloadPolicy) Option { return func(m *Manager) { m.reloadPolicy = p } }
func WithSecurity(p SecurityPolicy) Option   { return func(m *Manager) { m.security = p } }
func WithProviders(p ...Provider) Option {
	return func(m *Manager) { m.providers = append(m.providers, p...) }
}
func WithParsers(p ...Parser) Option {
	return func(m *Manager) {
		for _, x := range p {
			m.RegisterParser(x)
		}
	}
}
func WithValidator(v Validator) Option {
	return func(m *Manager) { m.validators = append(m.validators, v) }
}
func WithValidateFunc(fn ValidateFunc) Option {
	return func(m *Manager) { m.validateFuncs = append(m.validateFuncs, fn) }
}
func WithChangeHandler(fn ChangeHandler) Option {
	return func(m *Manager) { m.handlers = append(m.handlers, fn) }
}

func WithSensitivePolicy(p SensitivePolicy) Option {
	return func(m *Manager) {
		m.security.Sensitive = p
		m.sensitive = NewSensitiveMatcher(p)
	}
}

func WithAuditSink(sinks ...AuditSink) Option {
	return func(m *Manager) { m.auditSinks = append(m.auditSinks, sinks...) }
}
func WithRuntimePolicies(policies ...RuntimePolicy) Option {
	return func(m *Manager) { m.SetRuntimePolicies(policies...) }
}
func WithLastKnownGood(path string, policy StartupPolicy) Option {
	return func(m *Manager) { m.lastGood = LastKnownGoodPolicy{Path: path, Startup: policy, FileMode: 0600} }
}
func WithHistory(max int) Option {
	return func(m *Manager) {
		if max < 0 {
			max = 0
		}
		m.historyLimit = max
	}
}
func WithMetrics(sinks ...MetricsSink) Option {
	return func(m *Manager) { m.metrics = append(m.metrics, sinks...) }
}
func WithIntegrity(p IntegrityPolicy) Option { return func(m *Manager) { m.integrity = p } }
func WithAdvancedReloadHandler(h AdvancedReloadHandler) Option {
	return func(m *Manager) { m.reloadHandlers = append(m.reloadHandlers, h) }
}
func WithCompliance(policies ...CompliancePolicy) Option {
	return func(m *Manager) { m.ApplyCompliance(policies...) }
}
