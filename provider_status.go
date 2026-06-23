package config

import "time"

type ProviderStatus struct {
	Name         string        `json:"name"`
	Type         string        `json:"type"`
	Required     bool          `json:"required"`
	LastLoadedAt time.Time     `json:"last_loaded_at,omitempty"`
	LastDuration time.Duration `json:"last_duration,omitempty"`
	LastHash     string        `json:"last_hash,omitempty"`
	LastError    string        `json:"last_error,omitempty"`
	ReloadCount  uint64        `json:"reload_count"`
	FailCount    uint64        `json:"fail_count"`
}

func (m *Manager) ProviderStatus() []ProviderStatus {
	out := make([]ProviderStatus, 0, len(m.providerStatus))
	for _, s := range m.providerStatus {
		out = append(out, s)
	}
	return out
}
func (m *Manager) updateProviderStatus(meta SourceMeta, dur time.Duration, vals map[string]any, err error) {
	if m.providerStatus == nil {
		m.providerStatus = map[string]ProviderStatus{}
	}
	st := m.providerStatus[meta.Name]
	st.Name, st.Type, st.Required = meta.Name, meta.Type, meta.Required
	st.ReloadCount++
	st.LastDuration = dur
	if err != nil {
		st.LastError = err.Error()
		st.FailCount++
	} else {
		st.LastError = ""
		st.LastLoadedAt = time.Now()
		st.LastHash = HashTree(Tree(vals))
	}
	m.providerStatus[meta.Name] = st
}
