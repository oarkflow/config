package memory

import (
	"context"
	config "github.com/oarkflow/config"
)

type Provider struct {
	name     string
	values   map[string]any
	required bool
}

func New(name string, values map[string]any) Provider {
	return Provider{name: name, values: values, required: true}
}
func Optional(name string, values map[string]any) Provider {
	return Provider{name: name, values: values}
}
func (p Provider) Name() string {
	if p.name == "" {
		return "memory"
	}
	return p.name
}
func (p Provider) Load(context.Context) (map[string]any, config.SourceMeta, error) {
	return config.CloneValue(p.values).(map[string]any), config.SourceMeta{Name: p.Name(), Type: "memory", Required: p.required}, nil
}
