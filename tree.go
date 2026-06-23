package config

import (
	"fmt"
	"sort"
	"strings"
)

type Tree map[string]any

func NewTree() Tree { return make(Tree) }

func CloneValue(v any) any {
	switch x := v.(type) {
	case Tree:
		m := make(Tree, len(x))
		for k, v := range x {
			m[k] = CloneValue(v)
		}
		return m
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, v := range x {
			m[k] = CloneValue(v)
		}
		return m
	case map[string]string:
		m := make(map[string]any, len(x))
		for k, v := range x {
			m[k] = v
		}
		return m
	case []any:
		a := make([]any, len(x))
		for i, v := range x {
			a[i] = CloneValue(v)
		}
		return a
	case []string:
		a := append([]string(nil), x...)
		return a
	case []int:
		a := append([]int(nil), x...)
		return a
	default:
		return v
	}
}
func (t Tree) Clone() Tree { return CloneValue(t).(Tree) }

func normalizeValue(v any) any {
	switch x := v.(type) {
	case Tree:
		return map[string]any(x)
	case map[string]string:
		m := map[string]any{}
		for k, v := range x {
			m[k] = v
		}
		return m
	case []string:
		a := make([]any, len(x))
		for i, v := range x {
			a[i] = v
		}
		return a
	case []int:
		a := make([]any, len(x))
		for i, v := range x {
			a[i] = v
		}
		return a
	default:
		return v
	}
}

func (t Tree) HasPath(path string) bool { _, ok := t.Get(path); return ok }
func (t Tree) Get(path string) (any, bool) {
	parts, err := parsePath(path)
	if err != nil {
		return nil, false
	}
	var cur any = map[string]any(t)
	for _, p := range parts {
		if p.index != nil {
			switch a := cur.(type) {
			case []any:
				if *p.index >= len(a) {
					return nil, false
				}
				cur = a[*p.index]
			case []string:
				if *p.index >= len(a) {
					return nil, false
				}
				cur = a[*p.index]
			default:
				return nil, false
			}
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[p.key]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}
func (t Tree) Set(path string, value any) error {
	parts, err := parsePath(path)
	if err != nil {
		return err
	}
	var cur any = map[string]any(t)
	for i, p := range parts {
		last := i == len(parts)-1
		if p.index != nil {
			a, ok := cur.([]any)
			if !ok {
				return &ConfigError{Kind: ErrPath, Path: path, Message: "index used on non-array"}
			}
			if *p.index >= len(a) {
				return &ConfigError{Kind: ErrPath, Path: path, Message: "index out of range"}
			}
			if last {
				a[*p.index] = normalizeValue(value)
				return nil
			}
			cur = a[*p.index]
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return &ConfigError{Kind: ErrPath, Path: path, Message: "key used on non-map"}
		}
		if last {
			m[p.key] = normalizeValue(value)
			return nil
		}
		nx, ok := m[p.key]
		if !ok {
			nx = map[string]any{}
			m[p.key] = nx
		}
		cur = nx
	}
	return nil
}
func (t Tree) Delete(path string) error {
	parts, err := parsePath(path)
	if err != nil {
		return err
	}
	var cur any = map[string]any(t)
	for i, p := range parts {
		if p.index != nil {
			return &ConfigError{Kind: ErrPath, Path: path, Message: "delete by index is not supported"}
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		if i == len(parts)-1 {
			delete(m, p.key)
			return nil
		}
		cur = m[p.key]
	}
	return nil
}
func (t Tree) Merge(prefix string, values map[string]any) error {
	if prefix == "" {
		DeepMerge(map[string]any(t), values)
		return nil
	}
	if ex, ok := t.Get(prefix); ok {
		if em, ok := ex.(map[string]any); ok {
			DeepMerge(em, values)
			return nil
		}
	}
	return t.Set(prefix, CloneValue(values))
}
func DeepMerge(dst map[string]any, src map[string]any) {
	for k, v := range src {
		v = normalizeValue(v)
		if sm, ok := v.(map[string]any); ok {
			if dm, ok := dst[k].(map[string]any); ok {
				DeepMerge(dm, sm)
				continue
			}
		}
		dst[k] = CloneValue(v)
	}
}
func (t Tree) Flatten() map[string]any {
	out := map[string]any{}
	var walk func(string, any)
	walk = func(prefix string, v any) {
		switch x := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				p := k
				if prefix != "" {
					p = prefix + "." + k
				}
				walk(p, x[k])
			}
		case Tree:
			walk(prefix, map[string]any(x))
		case []any:
			for i, vv := range x {
				p := fmt.Sprintf("%s.%d", prefix, i)
				if prefix == "" {
					p = fmt.Sprint(i)
				}
				walk(p, vv)
			}
		default:
			out[prefix] = v
		}
	}
	walk("", map[string]any(t))
	return out
}
func (t Tree) Keys(prefix string) []string {
	flat := t.Flatten()
	out := make([]string, 0, len(flat))
	for k := range flat {
		if prefix == "" || k == prefix || strings.HasPrefix(k, prefix+".") {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
