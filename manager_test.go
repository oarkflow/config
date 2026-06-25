package config_test

import (
	"context"
	"strings"
	"testing"
	"time"

	config "github.com/oarkflow/config"
	"github.com/oarkflow/config/providers/memory"
)

type AppModule struct{}

func (AppModule) Prefix() string { return "app" }
func (AppModule) Configure(s *config.Section) {
	s.String("name", "APP_NAME", "demo")
	s.Bool("reload", "APP_RELOAD", true)
	s.Int("port", "APP_PORT", 8080)
	s.SecretString("key", "APP_KEY", "secret")
}

func TestDotSetGetAndRedaction(t *testing.T) {
	cfg := config.New()
	if err := cfg.Register(AppModule{}); err != nil {
		t.Fatal(err)
	}
	if !cfg.Bool("app.reload") {
		t.Fatal("expected reload true")
	}
	if cfg.Int("app.port") != 8080 {
		t.Fatal("bad port")
	}
	if err := cfg.Set("app.reload", false); err != nil {
		t.Fatal(err)
	}
	if cfg.Bool("app.reload") {
		t.Fatal("runtime set failed")
	}
	red := cfg.Redacted()
	app := red["app"].(map[string]any)
	if app["key"] != "[REDACTED]" {
		t.Fatalf("secret not redacted: %#v", app["key"])
	}
}

func TestLoadProviderMergeAndDecode(t *testing.T) {
	cfg := config.New(config.WithProviders(memory.New("m", map[string]any{"server": map[string]any{"addr": ":9000", "timeout": "5s"}})))
	if err := cfg.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	type Server struct {
		Addr    string `json:"addr"`
		Timeout string `json:"timeout"`
	}
	var srv Server
	if err := cfg.Decode("server", &srv); err != nil {
		t.Fatal(err)
	}
	if srv.Addr != ":9000" {
		t.Fatal(srv.Addr)
	}
	if cfg.Duration("server.timeout") != 5*time.Second {
		t.Fatal("duration conversion failed")
	}
}

func TestChangeHandlerRejects(t *testing.T) {
	cfg := config.New()
	if err := cfg.Set("a.b", 1); err != nil {
		t.Fatal(err)
	}
	cfg.OnChange(func(c config.Change) error {
		if c.Changed("a.b") {
			return config.RestartRequired("a.b")
		}
		return nil
	})
	if err := cfg.Set("a.b", 2); err == nil {
		t.Fatal("expected rejection")
	}
	if cfg.Int("a.b") != 1 {
		t.Fatal("commit should not apply on rejection")
	}
}

func TestConfigurableSensitiveWordsPathsAndRedaction(t *testing.T) {
	cfg := config.New()
	if err := cfg.Set("payment.card_number", "4111111111111111"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set("payment.cvv", "123"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set("payment.visible", "ok"); err != nil {
		t.Fatal(err)
	}

	cfg.SetRedactionText("***")
	cfg.AddSensitiveWords("card", "cvv")
	red := cfg.Redacted()["payment"].(map[string]any)
	if red["card_number"] != "***" || red["cvv"] != "***" {
		t.Fatalf("expected configurable sensitive redaction: %#v", red)
	}
	if red["visible"] != "ok" {
		t.Fatalf("non-sensitive value should remain visible: %#v", red)
	}

	cfg.RemoveSensitiveWords("card")
	red = cfg.Redacted()["payment"].(map[string]any)
	if red["card_number"] == "***" {
		t.Fatalf("removed sensitive word should stop redacting path: %#v", red)
	}
	if red["cvv"] != "***" {
		t.Fatalf("cvv should still redact: %#v", red)
	}

	cfg.AddSensitivePaths("payment.visible")
	red = cfg.Redacted()["payment"].(map[string]any)
	if red["visible"] != "***" {
		t.Fatalf("explicit sensitive path should redact: %#v", red)
	}
	cfg.RemoveSensitivePaths("payment.visible")
	red = cfg.Redacted()["payment"].(map[string]any)
	if red["visible"] == "***" {
		t.Fatalf("removed sensitive path should stop redacting: %#v", red)
	}
}

func TestSensitiveChangesAreRedactedBeforeHandlers(t *testing.T) {
	cfg := config.New()
	cfg.AddSensitiveWords("pin")
	var captured config.Change
	cfg.OnChange(func(ch config.Change) error {
		captured = ch
		return nil
	})
	if err := cfg.Set("user.pin", "1234"); err != nil {
		t.Fatal(err)
	}
	if len(captured.Paths) != 1 {
		t.Fatalf("expected one change, got %#v", captured.Paths)
	}
	if captured.Paths[0].NewValue != "[REDACTED]" || !captured.Paths[0].Sensitive {
		t.Fatalf("sensitive diff was not redacted before handler: %#v", captured.Paths[0])
	}
}

func TestSensitivePolicyOption(t *testing.T) {
	cfg := config.New(config.WithSensitivePolicy(config.SensitivePolicy{
		Words:         []string{"license"},
		Paths:         []string{"vendor.private"},
		EnvVars:       []string{"VENDOR_SECRET"},
		Redaction:     "<hidden>",
		MatchContains: true,
	}))
	if !cfg.IsSensitive("app.license_key") {
		t.Fatal("license_key should be sensitive from policy word")
	}
	if !cfg.IsSensitive("vendor.private") {
		t.Fatal("vendor.private should be sensitive from policy path")
	}
	if got := cfg.Sensitive().Redaction; got != "<hidden>" {
		t.Fatalf("bad redaction text: %s", got)
	}
	if err := cfg.Set("app.license_key", "abc"); err != nil {
		t.Fatal(err)
	}
	red := cfg.Redacted()["app"].(map[string]any)
	if red["license_key"] != "<hidden>" {
		t.Fatalf("custom policy redaction failed: %#v", red)
	}
}

func TestRedactedJSONDoesNotHTMLEscapeRedactionText(t *testing.T) {
	cfg := config.New(config.WithSensitivePolicy(config.SensitivePolicy{
		Words:         []string{"license"},
		Redaction:     "<hidden>",
		MatchContains: true,
	}))
	if err := cfg.Set("app.license_key", "enterprise-license"); err != nil {
		t.Fatal(err)
	}
	out := string(cfg.RedactedJSON())
	if !strings.Contains(out, `"license_key": "<hidden>"`) {
		t.Fatalf("redaction text should remain human-readable, got: %s", out)
	}
	if strings.Contains(out, `\u003c`) || strings.Contains(out, `\u003e`) {
		t.Fatalf("redacted json should not html-escape angle brackets: %s", out)
	}
}

func TestAESGCMEncryptorRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef") // 16 bytes
	enc, err := config.NewAESGCMEncryptor(key)
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("hello-world")
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if string(decrypted) != string(plaintext) {
		t.Fatalf("round-trip failed: got %q, want %q", decrypted, plaintext)
	}
	if string(ciphertext) == string(plaintext) {
		t.Fatal("ciphertext matches plaintext – no encryption")
	}
}

func TestAESGCMEncryptorFromPassphrase(t *testing.T) {
	enc := config.NewAESGCMEncryptorFromPassphrase("my-secret-passphrase")
	if enc.Name() != "aes256-gcm" {
		t.Fatalf("bad name: %s", enc.Name())
	}
	ciphertext, err := enc.Encrypt([]byte("test"))
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if string(decrypted) != "test" {
		t.Fatalf("bad decryption: %s", decrypted)
	}
}

func TestEncodedFormat(t *testing.T) {
	data := []byte{1, 2, 3, 4}
	encoded := config.EncodeEncrypted(data, "aes256-gcm")
	if !config.IsEncryptedValue(encoded) {
		t.Fatal("should detect encrypted value")
	}
	algo, decoded, err := config.DecodeEncrypted(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if algo != "aes256-gcm" {
		t.Fatalf("bad algo: %s", algo)
	}
	if string(decoded) != string(data) {
		t.Fatalf("bad decode: %x vs %x", decoded, data)
	}
	if config.IsEncryptedValue("plain string") {
		t.Fatal("should not detect plain string as encrypted")
	}
	if config.IsEncryptedValue(42) {
		t.Fatal("should not detect non-string as encrypted")
	}
}

func TestEncryptAndDecryptPath(t *testing.T) {
	enc := config.NewAESGCMEncryptorFromPassphrase("test-key")
	cfg := config.New(config.WithEncryptor(enc))
	if err := cfg.Set("db.password", "s3cret!"); err != nil {
		t.Fatal(err)
	}
	if cfg.IsEncrypted("db.password") {
		t.Fatal("should not be encrypted yet")
	}
	cfg.MarkEncrypted("db.password")
	if !cfg.IsEncrypted("db.password") {
		t.Fatal("should be marked encrypted")
	}
	if err := cfg.EncryptPath("db.password"); err != nil {
		t.Fatal(err)
	}
	if cfg.Get("db.password") != "s3cret!" {
		t.Fatalf("Get should auto-decrypt: got %q", cfg.Get("db.password"))
	}
	if err := cfg.DecryptPath("db.password"); err != nil {
		t.Fatal(err)
	}
	if cfg.Get("db.password") != "s3cret!" {
		t.Fatalf("after decrypt path: got %q", cfg.Get("db.password"))
	}
}

func TestEncryptedValueTypedGetters(t *testing.T) {
	enc := config.NewAESGCMEncryptorFromPassphrase("test-key")
	cfg := config.New(config.WithEncryptor(enc))
	if err := cfg.Set("db.port", "5432"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set("db.host", "localhost"); err != nil {
		t.Fatal(err)
	}
	cfg.MarkEncrypted("db.port", "db.host")
	if err := cfg.EncryptPath("db.port"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.EncryptPath("db.host"); err != nil {
		t.Fatal(err)
	}
	if cfg.String("db.port") != "5432" {
		t.Fatalf("String getter: got %q", cfg.String("db.port"))
	}
	if cfg.Int("db.port") != 5432 {
		t.Fatalf("Int getter: got %d", cfg.Int("db.port"))
	}
	if cfg.String("db.host") != "localhost" {
		t.Fatalf("String getter for host: got %q", cfg.String("db.host"))
	}
}

func TestEncryptedValueWithFallback(t *testing.T) {
	enc := config.NewAESGCMEncryptorFromPassphrase("test-key")
	cfg := config.New(config.WithEncryptor(enc))
	if v := cfg.String("nonexistent", "fallback"); v != "fallback" {
		t.Fatalf("fallback returned %q", v)
	}
}

func TestEncryptPathRequiresEncryptor(t *testing.T) {
	cfg := config.New()
	if err := cfg.EncryptPath("x.y"); err == nil {
		t.Fatal("expected error without encryptor")
	}
}

func TestEncryptPathNotFound(t *testing.T) {
	cfg := config.New(config.WithEncryptor(config.NewAESGCMEncryptorFromPassphrase("key")))
	if err := cfg.EncryptPath("no.such.path"); err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestDecryptPathNotEncrypted(t *testing.T) {
	cfg := config.New(config.WithEncryptor(config.NewAESGCMEncryptorFromPassphrase("key")))
	if err := cfg.Set("x.y", "plain"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.DecryptPath("x.y"); err == nil {
		t.Fatal("expected error for non-encrypted path")
	}
}

func TestUnmarkEncrypted(t *testing.T) {
	cfg := config.New()
	cfg.MarkEncrypted("a.b", "c.d")
	if !cfg.IsEncrypted("a.b") {
		t.Fatal("should be encrypted")
	}
	if !cfg.IsEncrypted("c.d") {
		t.Fatal("should be encrypted")
	}
	cfg.UnmarkEncrypted("a.b")
	if cfg.IsEncrypted("a.b") {
		t.Fatal("should no longer be encrypted")
	}
	if !cfg.IsEncrypted("c.d") {
		t.Fatal("c.d should still be encrypted")
	}
}

type encryptedModule struct{}

func (encryptedModule) Prefix() string { return "db" }
func (encryptedModule) Configure(s *config.Section) {
	s.EncryptedString("password", "DB_PASSWORD", "default-pass")
}

func TestEncryptedViaSectionBuilder(t *testing.T) {
	enc := config.NewAESGCMEncryptorFromPassphrase("test-key")
	cfg := config.New(config.WithEncryptor(enc))
	if err := cfg.Register(encryptedModule{}); err != nil {
		t.Fatal(err)
	}
	if !cfg.IsEncrypted("db.password") {
		t.Fatal("should be marked encrypted by section builder")
	}
	if err := cfg.EncryptPath("db.password"); err != nil {
		t.Fatal(err)
	}
	if cfg.String("db.password") != "default-pass" {
		t.Fatalf("got %q", cfg.String("db.password"))
	}
}

func TestWithEncryptedPathsOption(t *testing.T) {
	enc := config.NewAESGCMEncryptorFromPassphrase("key")
	cfg := config.New(config.WithEncryptor(enc), config.WithEncryptedPaths("app.api_key"))
	if !cfg.IsEncrypted("app.api_key") {
		t.Fatal("WithEncryptedPaths should mark path")
	}
}

func TestGetReturnsEncryptedValueWhenNoEncryptor(t *testing.T) {
	cfg := config.New()
	enc := config.NewAESGCMEncryptorFromPassphrase("external")
	ciphertext, _ := enc.Encrypt([]byte("mysecret"))
	encoded := config.EncodeEncrypted(ciphertext, enc.Name())
	if err := cfg.Set("x.y", encoded); err != nil {
		t.Fatal(err)
	}
	if cfg.Get("x.y") != encoded {
		t.Fatalf("should return raw encrypted value without encryptor: got %v", cfg.Get("x.y"))
	}
}

func TestEncryptPathAlreadyEncryptedIsNoop(t *testing.T) {
	enc := config.NewAESGCMEncryptorFromPassphrase("key")
	cfg := config.New(config.WithEncryptor(enc))
	if err := cfg.Set("x.y", "value"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.EncryptPath("x.y"); err != nil {
		t.Fatal(err)
	}
	encrypted, _ := cfg.Snapshot().Tree.Get("x.y")
	if err := cfg.EncryptPath("x.y"); err != nil {
		t.Fatal(err)
	}
	after, _ := cfg.Snapshot().Tree.Get("x.y")
	if encrypted != after {
		t.Fatal("re-encrypt should be noop")
	}
}

func TestIntSliceGetter(t *testing.T) {
	cfg := config.New()
	if err := cfg.Set("nums", []int{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	got := cfg.IntSlice("nums")
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("got %v", got)
	}
}

func TestIntSliceFromAnySlice(t *testing.T) {
	cfg := config.New()
	if err := cfg.Set("nums", []any{10, 20, 30}); err != nil {
		t.Fatal(err)
	}
	got := cfg.IntSlice("nums")
	if len(got) != 3 || got[0] != 10 || got[1] != 20 || got[2] != 30 {
		t.Fatalf("got %v", got)
	}
}

func TestIntSliceFallback(t *testing.T) {
	cfg := config.New()
	got := cfg.IntSlice("nonexistent", []int{99, 100})
	if len(got) != 2 || got[0] != 99 || got[1] != 100 {
		t.Fatalf("got %v", got)
	}
}

func TestInt64SliceGetter(t *testing.T) {
	cfg := config.New()
	if err := cfg.Set("nums", []int64{1e12, 2e12}); err != nil {
		t.Fatal(err)
	}
	got := cfg.Int64Slice("nums")
	if len(got) != 2 || got[0] != 1e12 || got[1] != 2e12 {
		t.Fatalf("got %v", got)
	}
}

func TestFloat64SliceGetter(t *testing.T) {
	cfg := config.New()
	if err := cfg.Set("vals", []float64{1.5, 2.5, 3.5}); err != nil {
		t.Fatal(err)
	}
	got := cfg.Float64Slice("vals")
	if len(got) != 3 || got[0] != 1.5 || got[1] != 2.5 || got[2] != 3.5 {
		t.Fatalf("got %v", got)
	}
}

func TestFloat64SliceFromAnySlice(t *testing.T) {
	cfg := config.New()
	if err := cfg.Set("vals", []any{1.5, 2, 3.5}); err != nil {
		t.Fatal(err)
	}
	got := cfg.Float64Slice("vals")
	if len(got) != 3 || got[0] != 1.5 || got[1] != 2.0 || got[2] != 3.5 {
		t.Fatalf("got %v", got)
	}
}

func TestIntSliceFromStringSliceViaParse(t *testing.T) {
	cfg := config.New()
	if err := cfg.Set("nums", []string{"5", "10", "15"}); err != nil {
		t.Fatal(err)
	}
	got := cfg.IntSlice("nums")
	if len(got) != 3 || got[0] != 5 || got[1] != 10 || got[2] != 15 {
		t.Fatalf("got %v", got)
	}
}

func TestConversionToIntSlice(t *testing.T) {
	if _, ok := config.ToIntSlice("not-a-slice"); ok {
		t.Fatal("string should not convert to int slice")
	}
	v, ok := config.ToIntSlice([]any{1, 2, 3})
	if !ok || len(v) != 3 || v[0] != 1 {
		t.Fatal("failed to convert []any{1,2,3}")
	}
}

func TestConversionToInt64Slice(t *testing.T) {
	v, ok := config.ToInt64Slice([]any{1, 2, 3})
	if !ok || len(v) != 3 || v[0] != 1 {
		t.Fatal("failed to convert []any{1,2,3} to int64")
	}
}

func TestConversionToFloat64Slice(t *testing.T) {
	v, ok := config.ToFloat64Slice([]any{1.5, 2.5})
	if !ok || len(v) != 2 || v[0] != 1.5 {
		t.Fatal("failed to convert []any{1.5,2.5}")
	}
}

func TestIntSliceNonexistentReturnsNil(t *testing.T) {
	cfg := config.New()
	got := cfg.IntSlice("no.such.path")
	if got != nil {
		t.Fatal("expected nil")
	}
}

func TestFloat64SliceNonexistentReturnsNil(t *testing.T) {
	cfg := config.New()
	got := cfg.Float64Slice("no.such.path")
	if got != nil {
		t.Fatal("expected nil")
	}
}

func TestAESGCMInvalidKeySize(t *testing.T) {
	_, err := config.NewAESGCMEncryptor([]byte("short"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestSchemaValidatorValid(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"app": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":  map[string]any{"type": "string"},
					"debug": map[string]any{"type": "boolean"},
				},
				"required": []any{"name"},
			},
		},
	}
	cfg := config.New(config.WithSchema(schema))
	if err := cfg.Set("app.name", "test"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set("app.debug", true); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaValidatorInvalid(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"app": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"port": map[string]any{"type": "integer", "minimum": 1024},
				},
			},
		},
	}
	cfg := config.New(config.WithSchema(schema))
	err := cfg.Set("app.port", 80)
	if err == nil {
		t.Fatal("expected validation error for port < 1024")
	}
}

func TestSchemaValidatorTypeMismatch(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"app": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"debug": map[string]any{"type": "boolean"},
				},
			},
		},
	}
	cfg := config.New(config.WithSchema(schema))
	err := cfg.Set("app.debug", "not-a-bool")
	if err == nil {
		t.Fatal("expected validation error for type mismatch")
	}
}

func TestSchemaValidatorRequiredMissing(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"database": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"host":     map[string]any{"type": "string"},
					"password": map[string]any{"type": "string"},
				},
				"required": []any{"host", "password"},
			},
		},
	}
	cfg := config.New(config.WithSchema(schema))
	err := cfg.Set("database.host", "localhost")
	if err == nil {
		t.Fatal("expected validation error for missing password")
	}
}

func TestMustSchemaValidatorPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	config.MustSchemaValidator("not-a-valid-schema")
}

func TestNewSchemaValidatorInvalidSchema(t *testing.T) {
	_, err := config.NewSchemaValidator("not-a-valid-schema")
	if err == nil {
		t.Fatal("expected error for invalid schema")
	}
}

func TestSchemaValidatorAcceptsNestedPathErrors(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"server": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"port": map[string]any{"type": "integer", "minimum": 1, "maximum": 65535},
				},
			},
		},
	}
	cfg := config.New(config.WithSchema(schema))
	err := cfg.Set("server.port", 70000)
	if err == nil {
		t.Fatal("expected validation error for port > 65535")
	}
}

func TestSchemaValidatorValidComplexObject(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"app": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":    map[string]any{"type": "string"},
					"version": map[string]any{"type": "string"},
				},
				"required": []any{"name", "version"},
			},
			"server": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"host": map[string]any{"type": "string"},
					"port": map[string]any{"type": "integer", "minimum": 1, "maximum": 65535},
				},
				"required": []any{"host", "port"},
			},
		},
	}
	cfg := config.New(
		config.WithSchema(schema),
		config.WithProviders(memory.New("test", map[string]any{
			"app":    map[string]any{"name": "myapp", "version": "1.0.0"},
			"server": map[string]any{"host": "0.0.0.0", "port": 8080},
		})),
	)
	if err := cfg.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
}
