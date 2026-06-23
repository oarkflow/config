package bclparser

import (
	"github.com/oarkflow/bcl"
	config "github.com/oarkflow/config"
)

type Parser struct{}

func New() Parser                   { return Parser{} }
func (Parser) Name() string         { return "bcl" }
func (Parser) Extensions() []string { return []string{"bcl", "hcl"} }
func (Parser) Parse(data []byte) (map[string]any, error) {
	var out map[string]any
	err := bcl.Unmarshal(data, &out)
	if err != nil {
		return nil, &config.ConfigError{Kind: config.ErrParse, Message: "invalid bcl", Cause: err}
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}
func Marshal(v any) ([]byte, error) { return bcl.Marshal(v) }
