package flag

import (
	"context"
	"strings"

	config "github.com/oarkflow/config"
)

type Provider struct {
	Args   []string
	Prefix string
}

func Args(args []string) Provider { return Provider{Args: args} }
func (p Provider) Name() string   { return "flags" }
func (p Provider) Load(context.Context) (map[string]any, config.SourceMeta, error) {
	out := map[string]any{}
	for i := 0; i < len(p.Args); i++ {
		a := p.Args[i]
		if !strings.HasPrefix(a, "--") {
			continue
		}
		a = strings.TrimPrefix(a, "--")
		key := a
		val := "true"
		if idx := strings.IndexByte(a, '='); idx >= 0 {
			key = a[:idx]
			val = a[idx+1:]
		} else if i+1 < len(p.Args) && !strings.HasPrefix(p.Args[i+1], "--") {
			i++
			val = p.Args[i]
		}
		if p.Prefix != "" {
			key = strings.TrimPrefix(key, p.Prefix)
		}
		key = strings.ReplaceAll(key, "-", "_")
		path := strings.ReplaceAll(key, "_", ".")
		_ = config.Tree(out).Set(path, val)
	}
	return out, config.SourceMeta{Name: p.Name(), Type: "flag"}, nil
}
