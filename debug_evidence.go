package config

import "time"

type DebugOptions struct {
	IncludeConfig          bool
	Redacted               bool
	IncludeSchema          bool
	IncludeProviders       bool
	IncludeSensitivePolicy bool
}
type DebugInfo struct {
	Version         uint64            `json:"version"`
	Hash            string            `json:"hash"`
	LoadedAt        time.Time         `json:"loaded_at"`
	LastError       string            `json:"last_error,omitempty"`
	Providers       []ProviderStatus  `json:"providers,omitempty"`
	SensitivePolicy SensitiveSnapshot `json:"sensitive_policy,omitempty"`
	Schema          []EntryMeta       `json:"schema,omitempty"`
	Config          map[string]any    `json:"config,omitempty"`
}

func (m *Manager) DebugInfo(opts DebugOptions) DebugInfo {
	s := m.Snapshot()
	di := DebugInfo{Version: s.Version, Hash: s.Hash, LoadedAt: s.LoadedAt}
	if err := m.LastError(); err != nil {
		di.LastError = err.Error()
	}
	if opts.IncludeProviders {
		di.Providers = m.ProviderStatus()
	}
	if opts.IncludeSensitivePolicy {
		di.SensitivePolicy = m.Sensitive()
	}
	if opts.IncludeSchema {
		di.Schema = m.Schema()
	}
	if opts.IncludeConfig {
		if opts.Redacted {
			di.Config = m.Redacted()
		} else {
			di.Config = m.All()
		}
	}
	return di
}

type EvidenceOptions struct {
	Redacted      bool
	IncludeConfig bool
}
type EvidenceBundle struct {
	GeneratedAt     time.Time         `json:"generated_at"`
	Version         uint64            `json:"version"`
	Hash            string            `json:"hash"`
	LoadedAt        time.Time         `json:"loaded_at"`
	Sources         []SourceMeta      `json:"sources"`
	Providers       []ProviderStatus  `json:"providers"`
	Schema          []EntryMeta       `json:"schema"`
	SensitivePolicy SensitiveSnapshot `json:"sensitive_policy"`
	RuntimePolicies []RuntimePolicy   `json:"runtime_policies"`
	LastError       string            `json:"last_error,omitempty"`
	Integrity       IntegrityPolicy   `json:"integrity"`
	Config          map[string]any    `json:"config,omitempty"`
}

func (m *Manager) EvidenceBundle(opts EvidenceOptions) EvidenceBundle {
	s := m.Snapshot()
	eb := EvidenceBundle{GeneratedAt: time.Now(), Version: s.Version, Hash: s.Hash, LoadedAt: s.LoadedAt, Sources: s.Sources, Providers: m.ProviderStatus(), Schema: m.Schema(), SensitivePolicy: m.Sensitive(), RuntimePolicies: m.RuntimePolicies(), Integrity: m.integrity}
	if err := m.LastError(); err != nil {
		eb.LastError = err.Error()
	}
	if opts.IncludeConfig {
		if opts.Redacted {
			eb.Config = m.Redacted()
		} else {
			eb.Config = m.All()
		}
	}
	return eb
}

func (m *Manager) ControlMappingMarkdown() string {
	return `# Configuration Control Mapping

| Control Area | Feature | Evidence |
|---|---|---|
| Change Management | Audit events for load, reload, set, delete, rollback | AuditSink events |
| Logical Access | Runtime mutation policies and actor roles | RuntimePolicies |
| Confidentiality | Redaction, sensitive policy, classification | SensitivePolicy + Schema |
| Availability | Last-known-good cache and rollback history | LastKnownGood + History |
| Integrity | Checksums and Ed25519 signature helpers | IntegrityPolicy |
| Monitoring | Provider status, debug info, metrics sink | ProviderStatus + MetricsSink |
| Validation | Required fields and validation rules | Schema + validation errors |
`
}
