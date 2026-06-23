package config

import "context"

type HistoryPolicy struct{ Max int }

func (m *Manager) History() []Snapshot {
	out := make([]Snapshot, 0, len(m.history))
	for _, s := range m.history {
		out = append(out, *s)
	}
	return out
}
func (m *Manager) Rollback(ctx context.Context, version uint64) error {
	for i := len(m.history) - 1; i >= 0; i-- {
		if m.history[i].Version == version {
			s := m.history[i]
			tmp := m.cloneForValidation(s)
			if err := tmp.validate(); err != nil {
				return err
			}
			old := m.cur.Load()
			change := diffSnapshots(old, s, m.secretPaths, m.sensitive)
			m.cur.Store(s)
			m.emitAudit(ctx, AuditEvent{Event: EventRuntimeRollback, Changed: change.Paths})
			return nil
		}
	}
	return &ConfigError{Kind: ErrLoad, Message: "snapshot version not found"}
}
func (m *Manager) rememberSnapshot(s *Snapshot) {
	if m.historyLimit <= 0 {
		return
	}
	m.history = append(m.history, s)
	if len(m.history) > m.historyLimit {
		copy(m.history, m.history[len(m.history)-m.historyLimit:])
		m.history = m.history[:m.historyLimit]
	}
}
