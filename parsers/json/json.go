package jsonparser

import (
	"bytes"

	json "github.com/oarkflow/fastjson"

	config "github.com/oarkflow/config"
)

type Parser struct{}

func New() Parser                   { return Parser{} }
func (Parser) Name() string         { return "json" }
func (Parser) Extensions() []string { return []string{"json"} }
func (Parser) Parse(data []byte) (map[string]any, error) {
	var out map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, &config.ConfigError{Kind: config.ErrParse, Message: "invalid json", Cause: err}
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}
