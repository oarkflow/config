package config

import (
	"context"
)

type Provider interface {
	Name() string
	Load(context.Context) (map[string]any, SourceMeta, error)
}
type WatchEvent struct {
	Provider string
	Path     string
	Op       string
}
type WatchProvider interface {
	Provider
	Watch(context.Context, func(WatchEvent)) error
}
type Parser interface {
	Name() string
	Extensions() []string
	Parse([]byte) (map[string]any, error)
}
