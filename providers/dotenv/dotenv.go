package dotenv

import (
	"bufio"
	"context"
	"os"
	"strings"

	config "github.com/oarkflow/config"
)

type Provider struct {
	Path     string
	Prefix   string
	Required bool
}

func Required(path string) Provider { return Provider{Path: path, Required: true} }
func Optional(path string) Provider { return Provider{Path: path} }
func (p Provider) Name() string     { return "dotenv:" + p.Path }
func (p Provider) Load(context.Context) (map[string]any, config.SourceMeta, error) {
	meta := config.SourceMeta{Name: p.Name(), Type: "dotenv", Required: p.Required}
	f, err := os.Open(p.Path)
	if err != nil {
		return nil, meta, err
	}
	defer f.Close()
	out := map[string]any{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		val = strings.Trim(val, "\"'")
		if p.Prefix != "" && !strings.HasPrefix(key, p.Prefix) {
			continue
		}
		key = strings.TrimPrefix(key, p.Prefix)
		path := strings.ToLower(strings.ReplaceAll(key, "_", "."))
		_ = config.Tree(out).Set(path, val)
	}
	if err := s.Err(); err != nil {
		return nil, meta, err
	}
	return out, meta, nil
}
