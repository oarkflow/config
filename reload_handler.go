package config

import "context"

type PreparedChange interface{ Commit(context.Context) error }
type PreparedFunc func(context.Context) error

func (f PreparedFunc) Commit(ctx context.Context) error { return f(ctx) }

type AdvancedReloadHandler struct {
	Prepare     func(context.Context, *Snapshot, *Snapshot, Change) (PreparedChange, error)
	AfterCommit func(context.Context, *Snapshot, *Snapshot, Change) error
}

func (m *Manager) OnReload(h AdvancedReloadHandler) { m.reloadHandlers = append(m.reloadHandlers, h) }
