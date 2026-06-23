package config

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"time"

	json "github.com/oarkflow/fastjson"
)

type StartupPolicy struct {
	AllowStale bool
	MaxAge     time.Duration
}
type LastKnownGoodPolicy struct {
	Path     string
	Startup  StartupPolicy
	FileMode os.FileMode
}

func (m *Manager) saveLastKnownGood(s *Snapshot) error {
	if m.lastGood.Path == "" {
		return nil
	}
	mode := m.lastGood.FileMode
	if mode == 0 {
		mode = 0600
	}
	if err := os.MkdirAll(filepath.Dir(m.lastGood.Path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.lastGood.Path + ".tmp"
	if err := os.WriteFile(tmp, b, mode); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		return err
	}
	return os.Rename(tmp, m.lastGood.Path)
}

func (m *Manager) LoadLastKnownGood() (*Snapshot, error) {
	if m.lastGood.Path == "" {
		return nil, &ConfigError{Kind: ErrLoad, Message: "last-known-good cache is not configured"}
	}
	b, err := os.ReadFile(m.lastGood.Path)
	if err != nil {
		return nil, err
	}
	var s Snapshot
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	if m.lastGood.Startup.MaxAge > 0 && time.Since(s.LoadedAt) > m.lastGood.Startup.MaxAge && !m.lastGood.Startup.AllowStale {
		return nil, &ConfigError{Kind: ErrSecurity, Message: "last-known-good cache is stale"}
	}
	if s.Tree == nil {
		s.Tree = NewTree()
	}
	return &s, nil
}

func (m *Manager) StartWithLastKnownGood(ctx context.Context) error {
	if err := m.Load(ctx); err == nil {
		return nil
	}
	s, e := m.LoadLastKnownGood()
	if e != nil {
		return e
	}
	tmp := m.cloneForValidation(s)
	if err := tmp.validate(); err != nil {
		return err
	}
	m.cur.Store(s)
	m.emitAudit(ctx, AuditEvent{Event: EventLoadSucceeded, Reason: "loaded last-known-good cache"})
	return nil
}
