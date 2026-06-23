package config

import (
	"hash/fnv"
	"strings"
	"time"
)

type TenantConfig struct {
	base   *Manager
	tenant string
}

func (m *Manager) ForTenant(tenant string) TenantConfig { return TenantConfig{base: m, tenant: tenant} }
func (t TenantConfig) path(path string) string {
	return "tenants." + t.tenant + "." + strings.Trim(path, ".")
}
func (t TenantConfig) Get(path string, fallback ...any) any {
	if t.base.Has(t.path(path)) {
		return t.base.Get(t.path(path), fallback...)
	}
	return t.base.Get(path, fallback...)
}
func (t TenantConfig) String(path string, fallback ...string) string {
	if v, ok := ToString(t.Get(path)); ok {
		return v
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return ""
}
func (t TenantConfig) Bool(path string, fallback ...bool) bool {
	if v, ok := ToBool(t.Get(path)); ok {
		return v
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return false
}
func (t TenantConfig) Int(path string, fallback ...int) int {
	if v, ok := ToInt(t.Get(path)); ok {
		return v
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return 0
}
func (t TenantConfig) Set(path string, value any) error { return t.base.Set(t.path(path), value) }

type Feature struct {
	m      *Manager
	name   string
	tenant string
}

func (m *Manager) Feature(name string) Feature    { return Feature{m: m, name: name} }
func (f Feature) ForTenant(tenant string) Feature { f.tenant = tenant; return f }
func (f Feature) path(suffix string) string {
	base := "features." + f.name
	if f.tenant != "" && f.m.Has("tenants."+f.tenant+"."+base+"."+suffix) {
		return "tenants." + f.tenant + "." + base + "." + suffix
	}
	return base + "." + suffix
}
func (f Feature) Enabled(keys ...string) bool {
	if !f.m.Bool(f.path("enabled"), false) {
		return false
	}
	if exp := f.m.String(f.path("expires_at")); exp != "" {
		if t, err := time.Parse("2006-01-02", exp); err == nil && time.Now().After(t.Add(24*time.Hour)) {
			return false
		}
	}
	rollout := f.m.Int(f.path("rollout"), 100)
	if rollout <= 0 {
		return false
	}
	if rollout >= 100 || len(keys) == 0 {
		return true
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.Join(keys, "|")))
	return int(h.Sum32()%100) < rollout
}
