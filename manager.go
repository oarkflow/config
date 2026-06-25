package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	json "github.com/oarkflow/fastjson"
)

type Validator interface{ ValidateConfig(*Manager) error }
type ValidateFunc func(*Manager) error
type ChangeHandler func(Change) error

type Loader interface {
	Prefix() string
	Load(*Manager) error
}
type Module interface {
	Prefix() string
	Configure(*Section)
}

type Manager struct {
	cur             atomic.Pointer[Snapshot]
	mu              sync.Mutex
	providers       []Provider
	parsers         map[string]Parser
	validators      []Validator
	validateFuncs   []ValidateFunc
	handlers        []ChangeHandler
	reloadPolicy    ReloadPolicy
	security        SecurityPolicy
	secretPaths     map[string]bool
	sensitive       *SensitiveMatcher
	defaults        Tree
	meta            map[string]EntryMeta
	lastErr         atomic.Value
	auditSinks      []AuditSink
	runtimePolicies map[string]RuntimePolicy
	providerStatus  map[string]ProviderStatus
	lastGood        LastKnownGoodPolicy
	history         []*Snapshot
	historyLimit    int
	metrics         []MetricsSink
	integrity       IntegrityPolicy
	reloadHandlers  []AdvancedReloadHandler
	migrations      []Migration
	encryptor       Encryptor
	encryptedPaths  map[string]bool
}

func New(opts ...Option) *Manager {
	m := &Manager{parsers: map[string]Parser{}, reloadPolicy: DefaultReloadPolicy(), security: DefaultSecurityPolicy(), secretPaths: map[string]bool{}, sensitive: NewSensitiveMatcher(DefaultSensitivePolicy()), defaults: NewTree(), meta: map[string]EntryMeta{}, runtimePolicies: map[string]RuntimePolicy{}, providerStatus: map[string]ProviderStatus{}, historyLimit: 10, encryptedPaths: map[string]bool{}}
	m.cur.Store(newSnapshot(1, NewTree(), nil, nil))
	for _, o := range opts {
		o(m)
	}
	return m
}
func Load(ctx context.Context, opts ...Option) (*Manager, error) {
	m := New(opts...)
	return m, m.Load(ctx)
}
func MustLoad(ctx context.Context, opts ...Option) *Manager {
	m, err := Load(ctx, opts...)
	if err != nil {
		panic(err)
	}
	return m
}
func (m *Manager) MustLoad(ctx context.Context) {
	if err := m.Load(ctx); err != nil {
		panic(err)
	}
}

func (m *Manager) RegisterParser(p Parser) {
	for _, ext := range p.Extensions() {
		m.parsers[strings.TrimPrefix(strings.ToLower(ext), ".")] = p
	}
}
func (m *Manager) Parser(ext string) (Parser, bool) {
	p, ok := m.parsers[strings.TrimPrefix(strings.ToLower(ext), ".")]
	return p, ok
}
func (m *Manager) Providers(p ...Provider) *Manager {
	m.providers = append(m.providers, p...)
	return m
}
func (m *Manager) OnChange(fn ChangeHandler) { m.handlers = append(m.handlers, fn) }
func (m *Manager) Snapshot() Snapshot        { return *m.cur.Load() }
func (m *Manager) Version() uint64           { return m.cur.Load().Version }
func (m *Manager) Hash() string              { return m.cur.Load().Hash }
func (m *Manager) LastError() error {
	v := m.lastErr.Load()
	if v == nil {
		return nil
	}
	return v.(error)
}
func (m *Manager) All() map[string]any {
	if m.security.DisableRawDump || m.security.RequireRedactedDump {
		return m.Redacted()
	}
	return map[string]any(m.cur.Load().Tree.Clone())
}
func (m *Manager) Flatten() map[string]any { return m.cur.Load().Tree.Flatten() }
func (m *Manager) Keys(prefix ...string) []string {
	p := ""
	if len(prefix) > 0 {
		p = prefix[0]
	}
	return m.cur.Load().Tree.Keys(p)
}
func (m *Manager) Has(path string) bool { _, ok := m.cur.Load().Tree.Get(path); return ok }

func (m *Manager) Load(ctx context.Context) error { return m.Reload(ctx) }
func (m *Manager) Reload(ctx context.Context) error {
	startAll := time.Now()
	m.emitAudit(ctx, AuditEvent{Event: EventReloadStarted})
	merged := m.defaults.Clone()
	sources := make([]SourceMeta, 0, len(m.providers))
	for _, p := range m.providers {
		st := time.Now()
		vals, meta, err := p.Load(ctx)
		meta.LoadedAt = time.Now()
		m.updateProviderStatus(meta, time.Since(st), vals, err)
		if err != nil {
			meta.Error = err.Error()
			sources = append(sources, meta)
			m.lastErr.Store(err)
			m.emitAudit(ctx, AuditEvent{Event: EventProviderFailed, Source: meta.Name, Error: err.Error()})
			if meta.Required {
				if m.lastGood.Path != "" {
					if s, e := m.LoadLastKnownGood(); e == nil {
						m.cur.Store(s)
						m.emitAudit(ctx, AuditEvent{Event: EventLoadSucceeded, Reason: "loaded last-known-good cache after provider failure"})
						return nil
					}
				}
				m.emitAudit(ctx, AuditEvent{Event: EventLoadFailed, Error: err.Error()})
				return err
			}
			continue
		}
		sources = append(sources, meta)
		DeepMerge(map[string]any(merged), vals)
	}
	err := m.commitTreeWithContext(ctx, merged, sources, nil, SystemActor(), EventReloadCommitted, "")
	for _, ms := range m.metrics {
		ms.ObserveConfigLoad(time.Since(startAll), err == nil)
		ms.IncReload(err == nil)
		if err == nil {
			ms.SetVersion(m.Version())
		}
	}
	return err
}
func (m *Manager) Add(prefix string, values map[string]any) error {
	old := m.cur.Load()
	if err := m.defaults.Merge(prefix, values); err != nil {
		return err
	}
	nt := old.Tree.Clone()
	if err := nt.Merge(prefix, values); err != nil {
		return err
	}
	return m.commitTree(nt, old.Sources, old.Warnings)
}
func (m *Manager) Set(path string, value any) error {
	return m.SetWithActor(context.Background(), SystemActor(), path, value)
}
func (m *Manager) SetDefault(path string, value any) error {
	if !m.Has(path) {
		return m.Set(path, value)
	}
	return nil
}
func (m *Manager) Delete(path string) error {
	return m.DeleteWithActor(context.Background(), SystemActor(), path)
}

func (m *Manager) commitTree(tree Tree, sources []SourceMeta, warnings []Warning) error {
	return m.commitTreeWithContext(context.Background(), tree, sources, warnings, SystemActor(), EventReloadCommitted, "")
}
func (m *Manager) commitTreeWithContext(ctx context.Context, tree Tree, sources []SourceMeta, warnings []Warning, actor Actor, event, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old := m.cur.Load()
	tree = m.expandTree(tree)
	warnings = append(warnings, m.applyMigrations(tree)...)
	next := newSnapshot(old.Version+1, tree, sources, warnings)
	if old.Hash == next.Hash {
		return nil
	}
	tmp := m.cloneForValidation(next)
	if err := tmp.validate(); err != nil {
		m.lastErr.Store(err)
		m.emitAudit(ctx, AuditEvent{Event: EventValidationFailed, Actor: actor, Path: path, Error: err.Error()})
		return err
	}
	change := diffSnapshots(old, next, m.secretPaths, m.sensitive)
	prepared := make([]PreparedChange, 0, len(m.reloadHandlers))
	for _, h := range m.reloadHandlers {
		if h.Prepare != nil {
			pc, err := h.Prepare(ctx, old, next, change.Redacted())
			if err != nil {
				m.lastErr.Store(err)
				m.emitAudit(ctx, AuditEvent{Event: EventReloadRejected, Actor: actor, Path: path, Changed: change.Paths, Error: err.Error()})
				return err
			}
			if pc != nil {
				prepared = append(prepared, pc)
			}
		}
	}
	for _, h := range m.handlers {
		if err := h(change.Redacted()); err != nil {
			m.lastErr.Store(err)
			m.emitAudit(ctx, AuditEvent{Event: EventReloadRejected, Actor: actor, Path: path, Changed: change.Paths, Error: err.Error()})
			return err
		}
	}
	m.rememberSnapshot(old)
	m.cur.Store(next)
	for _, pc := range prepared {
		if err := pc.Commit(ctx); err != nil {
			m.cur.Store(old)
			m.lastErr.Store(err)
			m.emitAudit(ctx, AuditEvent{Event: EventReloadRejected, Actor: actor, Path: path, Changed: change.Paths, Error: err.Error()})
			return err
		}
	}
	for _, h := range m.reloadHandlers {
		if h.AfterCommit != nil {
			_ = h.AfterCommit(ctx, old, next, change.Redacted())
		}
	}
	_ = m.saveLastKnownGood(next)
	m.emitAudit(ctx, AuditEvent{Event: event, Actor: actor, Path: path, Changed: change.Paths})
	for _, ms := range m.metrics {
		ms.SetVersion(next.Version)
		if event == EventRuntimeSet || event == EventRuntimeDelete {
			ms.IncRuntimeMutation(true)
		}
	}
	return nil
}
func (m *Manager) cloneForValidation(next *Snapshot) *Manager {
	tmp := *m
	tmp.cur.Store(next)
	return &tmp
}
func (m *Manager) validate() error {
	var err error
	err = AppendError(err, m.validateMetaRules())
	for _, v := range m.validators {
		err = AppendError(err, v.ValidateConfig(m))
	}
	for _, fn := range m.validateFuncs {
		err = AppendError(err, fn(m))
	}
	return err
}

func (m *Manager) Watch(ctx context.Context) error {
	events := make(chan WatchEvent, 16)
	var wg sync.WaitGroup
	watched := 0
	for _, p := range m.providers {
		wp, ok := p.(WatchProvider)
		if !ok {
			continue
		}
		watched++
		wg.Add(1)
		go func(w WatchProvider) {
			defer wg.Done()
			_ = w.Watch(ctx, func(e WatchEvent) {
				select {
				case events <- e:
				case <-ctx.Done():
				}
			})
		}(wp)
	}
	if watched == 0 {
		<-ctx.Done()
		return ctx.Err()
	}
	deb := m.reloadPolicy.Debounce
	if deb <= 0 {
		deb = 250 * time.Millisecond
	}
	min := m.reloadPolicy.MinInterval
	var last time.Time
	var timer *time.Timer
	var timerC <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			wg.Wait()
			return ctx.Err()
		case <-events:
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(deb)
			timerC = timer.C
		case <-timerC:
			if min > 0 && time.Since(last) < min {
				timer = time.NewTimer(min - time.Since(last))
				timerC = timer.C
				continue
			}
			last = time.Now()
			if err := m.Reload(ctx); err != nil && m.reloadPolicy.FailMode == FailClosed {
				return err
			}
			timerC = nil
		}
	}
}

func (m *Manager) Get(path string, fallback ...any) any {
	if v, ok := m.cur.Load().Tree.Get(path); ok {
		return m.maybeDecrypt(v)
	}
	if len(fallback) > 0 {
		return m.maybeDecrypt(fallback[0])
	}
	return nil
}
func (m *Manager) maybeDecrypt(v any) any {
	if m.encryptor == nil {
		return v
	}
	s, ok := v.(string)
	if !ok || !IsEncryptedValue(s) {
		return v
	}
	algo, data, err := DecodeEncrypted(s)
	if err != nil || algo != m.encryptor.Name() {
		return v
	}
	plaintext, err := m.encryptor.Decrypt(data)
	if err != nil {
		return v
	}
	return string(plaintext)
}
func (m *Manager) String(path string, fallback ...string) string {
	if s, ok := ToString(m.Get(path)); ok {
		return s
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return ""
}
func (m *Manager) Bool(path string, fallback ...bool) bool {
	if b, ok := ToBool(m.Get(path)); ok {
		return b
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return false
}
func (m *Manager) Int(path string, fallback ...int) int {
	if i, ok := ToInt(m.Get(path)); ok {
		return i
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return 0
}
func (m *Manager) Int64(path string, fallback ...int64) int64 {
	if i, ok := ToInt64(m.Get(path)); ok {
		return i
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return 0
}
func (m *Manager) Float64(path string, fallback ...float64) float64 {
	if f, ok := ToFloat64(m.Get(path)); ok {
		return f
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return 0
}
func (m *Manager) Duration(path string, fallback ...time.Duration) time.Duration {
	if d, ok := ToDuration(m.Get(path)); ok {
		return d
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return 0
}
func (m *Manager) Size(path string, fallback ...Size) Size {
	if s, ok := ToSize(m.Get(path)); ok {
		return s
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return 0
}
func (m *Manager) StringSlice(path string, fallback ...[]string) []string {
	if s, ok := ToStringSlice(m.Get(path)); ok {
		return s
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return nil
}
func (m *Manager) IntSlice(path string, fallback ...[]int) []int {
	if s, ok := ToIntSlice(m.Get(path)); ok {
		return s
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return nil
}
func (m *Manager) Int64Slice(path string, fallback ...[]int64) []int64 {
	if s, ok := ToInt64Slice(m.Get(path)); ok {
		return s
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return nil
}
func (m *Manager) Float64Slice(path string, fallback ...[]float64) []float64 {
	if s, ok := ToFloat64Slice(m.Get(path)); ok {
		return s
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return nil
}
func (m *Manager) Secret(path string) SecretString {
	switch x := m.Get(path).(type) {
	case SecretString:
		return x
	case string:
		return NewSecretString(x)
	default:
		return SecretString{}
	}
}
func (m *Manager) Map(path string) map[string]any {
	v := m.Get(path)
	if mp, ok := v.(map[string]any); ok {
		return CloneValue(mp).(map[string]any)
	}
	return nil
}
func (m *Manager) Decode(prefix string, out any) error {
	v := any(map[string]any(m.cur.Load().Tree))
	if prefix != "" {
		x, ok := m.cur.Load().Tree.Get(prefix)
		if !ok {
			return &ConfigError{Kind: ErrDecode, Path: prefix, Message: "path not found"}
		}
		v = x
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(out); err != nil {
		return &ConfigError{Kind: ErrDecode, Path: prefix, Cause: err}
	}
	return nil
}
func Decode[T any](m *Manager, prefix string) (T, error) {
	var out T
	err := m.Decode(prefix, &out)
	return out, err
}
func MustDecode[T any](m *Manager, prefix string) T {
	v, err := Decode[T](m, prefix)
	if err != nil {
		panic(err)
	}
	return v
}

func (m *Manager) SetEncryptor(enc Encryptor) { m.encryptor = enc }

func (m *Manager) MarkEncrypted(paths ...string) {
	for _, p := range paths {
		m.encryptedPaths[p] = true
	}
}
func (m *Manager) UnmarkEncrypted(paths ...string) {
	for _, p := range paths {
		delete(m.encryptedPaths, p)
	}
}
func (m *Manager) IsEncrypted(path string) bool { return m.encryptedPaths[path] }

func (m *Manager) EncryptPath(path string) error {
	if m.encryptor == nil {
		return errors.New("config: no encryptor configured")
	}
	v, ok := m.cur.Load().Tree.Get(path)
	if !ok {
		return &ConfigError{Kind: ErrPath, Path: path, Message: "path not found"}
	}
	if IsEncryptedValue(v) {
		return nil
	}
	plaintext := []byte(fmt.Sprint(v))
	ciphertext, err := m.encryptor.Encrypt(plaintext)
	if err != nil {
		return err
	}
	encoded := EncodeEncrypted(ciphertext, m.encryptor.Name())
	return m.Set(path, encoded)
}

func (m *Manager) DecryptPath(path string) error {
	if m.encryptor == nil {
		return errors.New("config: no encryptor configured")
	}
	raw, ok := m.cur.Load().Tree.Get(path)
	if !ok {
		return &ConfigError{Kind: ErrPath, Path: path, Message: "path not found"}
	}
	s, ok := raw.(string)
	if !ok || !IsEncryptedValue(s) {
		return fmt.Errorf("config: value at %s is not encrypted", path)
	}
	algo, data, err := DecodeEncrypted(s)
	if err != nil {
		return err
	}
	if algo != m.encryptor.Name() {
		return fmt.Errorf("config: encrypted value at %s uses algorithm %q, but encryptor is %q", path, algo, m.encryptor.Name())
	}
	plaintext, err := m.encryptor.Decrypt(data)
	if err != nil {
		return err
	}
	return m.Set(path, string(plaintext))
}

func (m *Manager) Register(items ...any) error {
	for _, it := range items {
		switch x := it.(type) {
		case Loader:
			if err := x.Load(m); err != nil {
				return err
			}
		case Module:
			sec := m.Section(x.Prefix())
			x.Configure(sec)
			if err := sec.Commit(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported config registration %T", it)
		}
	}
	return nil
}
func (m *Manager) Section(prefix string) *Section {
	return &Section{m: m, prefix: prefix, values: map[string]any{}}
}
func (m *Manager) Schema() []EntryMeta {
	out := make([]EntryMeta, 0, len(m.meta))
	for _, v := range m.meta {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
