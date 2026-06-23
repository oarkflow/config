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
