package env

import (
	"context"
	"os"
	"strings"

	config "github.com/oarkflow/config"
)

type Provider struct {
	Prefix    string
	Separator string
	Lower     bool
}

func Prefix(prefix string) Provider { return Provider{Prefix: prefix, Separator: "_", Lower: true} }
func New(prefix string) Provider    { return Prefix(prefix) }
func (p Provider) Name() string     { return "env:" + p.Prefix }
func (p Provider) Load(context.Context) (map[string]any, config.SourceMeta, error) {
	sep := p.Separator
	if sep == "" {
		sep = "_"
	}
	out := map[string]any{}
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		key := parts[0]
		val := ""
		if len(parts) > 1 {
			val = parts[1]
		}
		if p.Prefix != "" && !strings.HasPrefix(key, p.Prefix) {
			continue
		}
		name := strings.TrimPrefix(key, p.Prefix)
		if name == "" {
			continue
		}
		if p.Lower {
			name = strings.ToLower(name)
		}
		path := strings.ReplaceAll(name, sep, ".")
		_ = config.Tree(out).Set(path, val)
	}
	return out, config.SourceMeta{Name: p.Name(), Type: "env"}, nil
}
