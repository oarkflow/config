package config

type Migration struct {
	From      string `json:"from"`
	To        string `json:"to"`
	DeleteOld bool   `json:"delete_old"`
}

func (m *Manager) AddMigration(from, to string, deleteOld ...bool) {
	m.migrations = append(m.migrations, Migration{From: from, To: to, DeleteOld: len(deleteOld) > 0 && deleteOld[0]})
}
func (m *Manager) applyMigrations(t Tree) []Warning {
	var warnings []Warning
	for _, mg := range m.migrations {
		if v, ok := t.Get(mg.From); ok {
			if !t.HasPath(mg.To) {
				_ = t.Set(mg.To, v)
			}
			if mg.DeleteOld {
				_ = t.Delete(mg.From)
			}
			warnings = append(warnings, Warning{Path: mg.From, Message: "deprecated; use " + mg.To})
		}
	}
	for _, meta := range m.meta {
		if meta.Deprecated != "" {
			if _, ok := t.Get(meta.Path); ok {
				msg := meta.Deprecated
				if meta.Replacement != "" {
					msg += "; use " + meta.Replacement
				}
				warnings = append(warnings, Warning{Path: meta.Path, Message: msg})
			}
		}
	}
	return warnings
}
