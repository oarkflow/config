package file

import (
	"context"
	"os"
	"path/filepath"
	"time"

	config "github.com/oarkflow/config"
)

type Provider struct {
	Path          string
	Required      bool
	Parsers       map[string]config.Parser
	PollInterval  time.Duration
	lastMod       time.Time
	lastSize      int64
	ChecksumPath  string
	SignaturePath string
	PublicKey     []byte
	RequirePerm   os.FileMode
}

func Required(path string, parsers ...config.Parser) *Provider {
	return newProvider(path, true, parsers...)
}
func Optional(path string, parsers ...config.Parser) *Provider {
	return newProvider(path, false, parsers...)
}
func newProvider(path string, required bool, parsers ...config.Parser) *Provider {
	p := &Provider{Path: path, Required: required, Parsers: map[string]config.Parser{}, PollInterval: 500 * time.Millisecond}
	for _, pa := range parsers {
		for _, ext := range pa.Extensions() {
			p.Parsers[trimDot(ext)] = pa
		}
	}
	return p
}
func trimDot(s string) string {
	if len(s) > 0 && s[0] == '.' {
		return s[1:]
	}
	return s
}
func (p *Provider) WithChecksum(path string) *Provider { p.ChecksumPath = path; return p }
func (p *Provider) WithSignature(path string, publicKey []byte) *Provider {
	p.SignaturePath = path
	p.PublicKey = append([]byte(nil), publicKey...)
	return p
}
func (p *Provider) RequireMode(mode os.FileMode) *Provider { p.RequirePerm = mode; return p }
func (p *Provider) Name() string                           { return "file:" + p.Path }
func (p *Provider) Load(ctx context.Context) (map[string]any, config.SourceMeta, error) {
	meta := config.SourceMeta{Name: p.Name(), Type: "file", Required: p.Required}
	if p.RequirePerm != 0 {
		if st, err := os.Stat(p.Path); err == nil && st.Mode().Perm() & ^p.RequirePerm != 0 {
			return nil, meta, &config.ConfigError{Kind: config.ErrSecurity, Source: p.Path, Message: "file permissions are broader than required"}
		}
	}
	if p.ChecksumPath != "" {
		if err := config.VerifyFileChecksum(p.Path, p.ChecksumPath); err != nil {
			return nil, meta, err
		}
	}
	if p.SignaturePath != "" && len(p.PublicKey) > 0 {
		if err := config.VerifyFileSignature(p.Path, p.SignaturePath, p.PublicKey); err != nil {
			return nil, meta, err
		}
	}
	b, err := os.ReadFile(p.Path)
	if err != nil {
		return nil, meta, err
	}
	ext := trimDot(filepath.Ext(p.Path))
	parser := p.Parsers[ext]
	if parser == nil {
		return nil, meta, &config.ConfigError{Kind: config.ErrParse, Source: p.Path, Message: "no parser registered for ." + ext}
	}
	vals, err := parser.Parse(b)
	if err != nil {
		if ce, ok := err.(*config.ConfigError); ok {
			ce.Source = p.Path
		}
		return nil, meta, err
	}
	st, _ := os.Stat(p.Path)
	if st != nil {
		p.lastMod = st.ModTime()
		p.lastSize = st.Size()
	}
	_ = ctx
	return vals, meta, nil
}
func (p *Provider) Watch(ctx context.Context, fn func(config.WatchEvent)) error {
	interval := p.PollInterval
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			st, err := os.Stat(p.Path)
			if err != nil {
				continue
			}
			if st.ModTime() != p.lastMod || st.Size() != p.lastSize {
				p.lastMod = st.ModTime()
				p.lastSize = st.Size()
				fn(config.WatchEvent{Provider: p.Name(), Path: p.Path, Op: "write"})
			}
		}
	}
}
