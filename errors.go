package config

import (
	"errors"
	"fmt"
	"strings"
)

type ErrorKind string

const (
	ErrLoad           ErrorKind = "load"
	ErrParse          ErrorKind = "parse"
	ErrPath           ErrorKind = "path"
	ErrDecode         ErrorKind = "decode"
	ErrValidate       ErrorKind = "validate"
	ErrSecurity       ErrorKind = "security"
	ErrReloadRejected ErrorKind = "reload_rejected"
)

type ConfigError struct {
	Kind    ErrorKind
	Path    string
	Source  string
	Line    int
	Column  int
	Message string
	Cause   error
}

func (e *ConfigError) Error() string {
	var b strings.Builder
	if e.Kind != "" {
		b.WriteString(string(e.Kind))
		b.WriteString(" error")
	} else {
		b.WriteString("config error")
	}
	if e.Path != "" {
		b.WriteString(" at ")
		b.WriteString(e.Path)
	}
	if e.Source != "" {
		b.WriteString(" in ")
		b.WriteString(e.Source)
	}
	if e.Line > 0 {
		b.WriteString(fmt.Sprintf(":%d", e.Line))
		if e.Column > 0 {
			b.WriteString(fmt.Sprintf(":%d", e.Column))
		}
	}
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	if e.Cause != nil {
		b.WriteString(": ")
		b.WriteString(e.Cause.Error())
	}
	return b.String()
}
func (e *ConfigError) Unwrap() error { return e.Cause }

func PathError(path, msg string) error {
	return &ConfigError{Kind: ErrValidate, Path: path, Message: msg}
}
func RestartRequired(path string) error {
	return &ConfigError{Kind: ErrReloadRejected, Path: path, Message: "change requires restart"}
}

type MultiError struct{ Errors []error }

func (m MultiError) Error() string {
	parts := make([]string, 0, len(m.Errors))
	for _, e := range m.Errors {
		if e != nil {
			parts = append(parts, e.Error())
		}
	}
	return strings.Join(parts, "; ")
}
func (m MultiError) Unwrap() []error { return m.Errors }
func AppendError(base error, next error) error {
	if next == nil {
		return base
	}
	if base == nil {
		return next
	}
	var m MultiError
	if errors.As(base, &m) {
		m.Errors = append(m.Errors, next)
		return m
	}
	return MultiError{Errors: []error{base, next}}
}
