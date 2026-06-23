package config

import (
	"fmt"
	"sort"
	"strings"
)

func (m *Manager) EnvExample() string {
	var b strings.Builder
	for _, e := range m.Schema() {
		if e.Env == "" {
			continue
		}
		val := fmt.Sprint(e.Default)
		if e.Secret {
			val = ""
		}
		b.WriteString(e.Env)
		b.WriteByte('=')
		b.WriteString(val)
		b.WriteByte('\n')
	}
	return b.String()
}
func (m *Manager) Markdown() string {
	rows := m.Schema()
	sort.Slice(rows, func(i, j int) bool { return rows[i].Path < rows[j].Path })
	var b strings.Builder
	b.WriteString("| Path | Env | Type | Default | Secret | Description |\n|---|---|---|---|---|---|\n")
	for _, e := range rows {
		def := fmt.Sprint(e.Default)
		if e.Secret {
			def = "[REDACTED]"
		}
		b.WriteString("| ")
		b.WriteString(e.Path)
		b.WriteString(" | ")
		b.WriteString(e.Env)
		b.WriteString(" | ")
		b.WriteString(string(e.Kind))
		b.WriteString(" | ")
		b.WriteString(strings.ReplaceAll(def, "|", "\\|"))
		b.WriteString(" | ")
		b.WriteString(fmt.Sprint(e.Secret))
		b.WriteString(" | ")
		b.WriteString(strings.ReplaceAll(e.DescriptionText, "|", "\\|"))
		b.WriteString(" |\n")
	}
	return b.String()
}
