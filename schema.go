package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type SchemaValidator struct {
	schema *jsonschema.Schema
}

func NewSchemaValidator(schema any) (*SchemaValidator, error) {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("config.json", schema); err != nil {
		return nil, fmt.Errorf("config: invalid schema: %w", err)
	}
	sch, err := compiler.Compile("config.json")
	if err != nil {
		return nil, fmt.Errorf("config: schema compilation failed: %w", err)
	}
	return &SchemaValidator{schema: sch}, nil
}

func MustSchemaValidator(schema any) *SchemaValidator {
	v, err := NewSchemaValidator(schema)
	if err != nil {
		panic(err)
	}
	return v
}

func (sv *SchemaValidator) ValidateConfig(m *Manager) error {
	tree := m.All()
	err := sv.schema.Validate(tree)
	if err == nil {
		return nil
	}
	var verr *jsonschema.ValidationError
	if !errors.As(err, &verr) {
		return &ConfigError{Kind: ErrValidate, Message: err.Error(), Cause: err}
	}
	paths := collectSchemaErrors(verr)
	if len(paths) == 0 {
		return &ConfigError{Kind: ErrValidate, Message: err.Error(), Cause: err}
	}
	var out error
	for _, pe := range paths {
		msg := pe.message
		if msg == "" {
			msg = "schema validation failed"
		}
		out = AppendError(out, &ConfigError{
			Kind:    ErrValidate,
			Path:    pe.path,
			Message: msg,
		})
	}
	return out
}

type schemaPathError struct {
	path    string
	message string
}

var defaultPrinter = message.NewPrinter(language.English)

func collectSchemaErrors(err *jsonschema.ValidationError) []schemaPathError {
	var out []schemaPathError
	var walk func(*jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		path := strings.Join(e.InstanceLocation, ".")
		msg := e.ErrorKind.LocalizedString(defaultPrinter)
		out = append(out, schemaPathError{path: path, message: msg})
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(err)
	return out
}
