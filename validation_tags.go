package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func ValidateValue(v any) error { return validateReflect(reflect.ValueOf(v), "") }

func validateReflect(v reflect.Value, prefix string) error {
	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		return validateReflect(v.Elem(), prefix)
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	t := v.Type()
	var err error
	for i := 0; i < v.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		fv := v.Field(i)
		name := f.Tag.Get("config")
		if name == "" {
			name = f.Tag.Get("conf")
		}
		if name == "" {
			name = strings.ToLower(f.Name)
		}
		name = strings.Split(name, ",")[0]
		if name == "-" {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		if tag := f.Tag.Get("validate"); tag != "" {
			err = AppendError(err, validateRules(path, fv, tag))
		}
		err = AppendError(err, validateReflect(fv, path))
	}
	return err
}

func (m *Manager) validateMetaRules() error {
	var err error
	for _, meta := range m.meta {
		if meta.Validation != "" {
			v, _ := m.cur.Load().Tree.Get(meta.Path)
			err = AppendError(err, validateRules(meta.Path, reflect.ValueOf(v), meta.Validation))
		}
		if meta.Required {
			if v, ok := m.cur.Load().Tree.Get(meta.Path); !ok || isZeroAny(v) {
				err = AppendError(err, PathError(meta.Path, "is required"))
			}
		}
	}
	return err
}

func validateRules(path string, v reflect.Value, tag string) error {
	var err error
	for _, raw := range strings.Split(tag, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		key, val, _ := strings.Cut(raw, "=")
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if e := validateRule(path, v, key, val); e != nil {
			err = AppendError(err, e)
		}
	}
	return err
}

func validateRule(path string, v reflect.Value, key, arg string) error {
	av := any(nil)
	if v.IsValid() {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				av = nil
			} else {
				av = v.Elem().Interface()
			}
		} else if v.CanInterface() {
			av = v.Interface()
		}
	}
	switch key {
	case "required":
		if isZeroAny(av) {
			return PathError(path, "is required")
		}
	case "min":
		f, ok := ToFloat64(av)
		if !ok {
			return PathError(path, "must be numeric")
		}
		n, _ := strconv.ParseFloat(arg, 64)
		if f < n {
			return PathError(path, "must be >= "+arg)
		}
	case "max":
		f, ok := ToFloat64(av)
		if !ok {
			return PathError(path, "must be numeric")
		}
		n, _ := strconv.ParseFloat(arg, 64)
		if f > n {
			return PathError(path, "must be <= "+arg)
		}
	case "len":
		s, _ := ToString(av)
		n, _ := strconv.Atoi(arg)
		if len(s) != n {
			return PathError(path, fmt.Sprintf("length must be %d", n))
		}
	case "oneof":
		s, _ := ToString(av)
		for _, p := range strings.Fields(arg) {
			if s == p {
				return nil
			}
		}
		return PathError(path, "must be one of "+arg)
	case "not_oneof":
		s, _ := ToString(av)
		for _, p := range strings.Fields(arg) {
			if s == p {
				return PathError(path, "must not be one of "+arg)
			}
		}
	case "port":
		i, ok := ToInt(av)
		if !ok || i < 1 || i > 65535 {
			return PathError(path, "must be a valid port")
		}
	case "url":
		s, _ := ToString(av)
		u, e := url.Parse(s)
		if e != nil || u.Scheme == "" {
			return PathError(path, "must be a valid URL")
		}
	case "host":
		s, _ := ToString(av)
		if s == "" || strings.ContainsAny(s, "/ ") {
			return PathError(path, "must be a valid host")
		}
	case "hostport":
		s, _ := ToString(av)
		if _, _, e := net.SplitHostPort(s); e != nil {
			return PathError(path, "must be host:port")
		}
	case "ip":
		s, _ := ToString(av)
		if net.ParseIP(s) == nil {
			return PathError(path, "must be an IP address")
		}
	case "cidr":
		s, _ := ToString(av)
		if _, _, e := net.ParseCIDR(s); e != nil {
			return PathError(path, "must be CIDR")
		}
	case "duration_min":
		d, ok := ToDuration(av)
		min, e := time.ParseDuration(arg)
		if !ok || e != nil || d < min {
			return PathError(path, "duration must be >= "+arg)
		}
	case "duration_max":
		d, ok := ToDuration(av)
		max, e := time.ParseDuration(arg)
		if !ok || e != nil || d > max {
			return PathError(path, "duration must be <= "+arg)
		}
	case "file_exists":
		s, _ := ToString(av)
		if st, e := os.Stat(s); e != nil || st.IsDir() {
			return PathError(path, "file does not exist")
		}
	case "dir_exists":
		s, _ := ToString(av)
		if st, e := os.Stat(s); e != nil || !st.IsDir() {
			return PathError(path, "directory does not exist")
		}
	case "absolute_path":
		s, _ := ToString(av)
		if !strings.HasPrefix(s, "/") {
			return PathError(path, "must be absolute path")
		}
	case "non_empty_slice":
		if v.Kind() != reflect.Slice || v.Len() == 0 {
			return PathError(path, "must be a non-empty slice")
		}
	}
	return nil
}

func isZeroAny(v any) bool {
	if v == nil {
		return true
	}
	switch x := v.(type) {
	case string:
		return x == ""
	case SecretString:
		return x.IsZero()
	case []any:
		return len(x) == 0
	case []string:
		return len(x) == 0
	}
	rv := reflect.ValueOf(v)
	return rv.IsValid() && rv.IsZero()
}
