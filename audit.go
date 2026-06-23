package config

import (
	"context"
	"sync"
	"time"
)

const (
	EventLoadStarted            = "config.load.started"
	EventLoadSucceeded          = "config.load.succeeded"
	EventLoadFailed             = "config.load.failed"
	EventReloadStarted          = "config.reload.started"
	EventReloadCommitted        = "config.reload.committed"
	EventReloadRejected         = "config.reload.rejected"
	EventRuntimeSet             = "config.runtime.set"
	EventRuntimeDelete          = "config.runtime.delete"
	EventRuntimeRollback        = "config.runtime.rollback"
	EventSensitivePolicyChanged = "config.sensitive_policy.changed"
	EventProviderFailed         = "config.provider.failed"
	EventIntegrityFailed        = "config.integrity.failed"
	EventValidationFailed       = "config.validation.failed"
)

type Actor struct {
	ID       string   `json:"id,omitempty"`
	Name     string   `json:"name,omitempty"`
	Roles    []string `json:"roles,omitempty"`
	TenantID string   `json:"tenant_id,omitempty"`
	Source   string   `json:"source,omitempty"`
}

func SystemActor() Actor { return Actor{ID: "system", Source: "config"} }

type AuditEvent struct {
	Event     string       `json:"event"`
	Version   uint64       `json:"version,omitempty"`
	Hash      string       `json:"hash,omitempty"`
	Actor     Actor        `json:"actor,omitempty"`
	Source    string       `json:"source,omitempty"`
	Path      string       `json:"path,omitempty"`
	Changed   []PathChange `json:"changed,omitempty"`
	Reason    string       `json:"reason,omitempty"`
	Error     string       `json:"error,omitempty"`
	RequestID string       `json:"request_id,omitempty"`
	TenantID  string       `json:"tenant_id,omitempty"`
	Time      time.Time    `json:"time"`
}

type AuditSink interface {
	EmitConfigEvent(context.Context, AuditEvent) error
}

type AuditFunc func(context.Context, AuditEvent) error

func (f AuditFunc) EmitConfigEvent(ctx context.Context, ev AuditEvent) error { return f(ctx, ev) }

type MemoryAuditSink struct {
	mu     sync.Mutex
	events []AuditEvent
	max    int
}

func NewMemoryAuditSink(max int) *MemoryAuditSink {
	if max <= 0 {
		max = 1000
	}
	return &MemoryAuditSink{max: max}
}
func (s *MemoryAuditSink) EmitConfigEvent(_ context.Context, ev AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, ev)
	if len(s.events) > s.max {
		copy(s.events, s.events[len(s.events)-s.max:])
		s.events = s.events[:s.max]
	}
	return nil
}
func (s *MemoryAuditSink) Events() []AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]AuditEvent(nil), s.events...)
}

func (m *Manager) emitAudit(ctx context.Context, ev AuditEvent) {
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	if ev.Version == 0 {
		ev.Version = m.Version()
	}
	if ev.Hash == "" {
		ev.Hash = m.Hash()
	}
	if ev.Actor.ID == "" && ev.Actor.Source == "" {
		ev.Actor = SystemActor()
	}
	ev.Changed = redactPathChanges(ev.Changed, m.sensitive)
	for _, sink := range m.auditSinks {
		_ = sink.EmitConfigEvent(ctx, ev)
	}
}

func redactPathChanges(in []PathChange, matcher *SensitiveMatcher) []PathChange {
	out := append([]PathChange(nil), in...)
	text := DefaultRedactionText
	if matcher != nil {
		text = matcher.RedactionText()
	}
	for i := range out {
		if out[i].Sensitive {
			out[i].OldValue = text
			out[i].NewValue = text
		}
	}
	return out
}
