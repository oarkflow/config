# oarkflow/config

Enterprise-grade configuration management for Go applications.

This package is built around a safe runtime configuration tree with dot-notation access, typed decoding, atomic snapshots, hot reload, provider/parser separation, secret redaction, validation, and module-based configuration registration.

## Design goals

- Enterprise approach, no global object required.
- Fast reads through atomic immutable snapshots.
- Safe runtime mutation using clone, validate, diff, hook, commit.
- Dot notation for dynamic access: `app.reload`, `database.host`, `server.ports.0`.
- Typed decoding for production-safe config structs.
- Provider-based loading: file, env, dotenv, flags, memory.
- Parser-based decoding: JSON and BCL.
- BCL parser uses `github.com/oarkflow/bcl` directly as a first-class parser.
- Hot reload via stdlib polling watcher.
- Secret-aware values, configurable sensitive words/paths/env names, and redacted dumps/diffs.
- Validation hooks and restart-required rejection.
- Config schema, markdown docs, and `.env.example` generation.

## Install

```bash
go get github.com/oarkflow/config
```

BCL is a normal first-class dependency through `github.com/oarkflow/bcl`; no build tags are required.

## Basic usage

```go
cfg := config.New()

cfg.Set("app.name", "Workflow")
cfg.Set("app.reload", true)

fmt.Println(cfg.String("app.name"))
fmt.Println(cfg.Bool("app.reload"))
```

Runtime `Set` does not mutate the active map directly. It clones the current tree, applies the change, validates, computes a diff, executes change handlers, and atomically swaps the snapshot only on success.

## Enterprise module approach

```go
type AppModule struct{}

func (AppModule) Prefix() string { return "app" }

func (AppModule) Configure(s *config.Section) {
    s.SecretString("key", "APP_KEY", "").Required()
    s.String("name", "APP_NAME", "Workflow v1.0")
    s.Bool("debug", "APP_DEBUG", false)
    s.Int("port", "APP_PORT", 3003)
    s.Bool("reload", "APP_RELOAD", true)
}
```

Register modules:

```go
cfg := config.New()
err := cfg.Register(AppModule{})
```

Read values:

```go
name := cfg.String("app.name")
port := cfg.Int("app.port")
reload := cfg.Bool("app.reload")
```

## BCL file loading

`parsers/bcl` uses the real upstream package `github.com/oarkflow/bcl`.

```go
cfg := config.New()
cfg.Providers(
    file.Optional("config.bcl", bclparser.New()),
    env.Prefix("APP_"),
)
err := cfg.Load(context.Background())
```

Example `config.bcl`:

```hcl
app {
  name = "Workflow Enterprise"
  debug = false
  host = "0.0.0.0"
  port = 3003
  reload = true
}

database {
  driver = "postgres"
  host = "${DB_HOST}"
  port = 5432
  password = "${DB_PASSWORD}"
}
```

## Providers

Built in:

- `providers/file`
- `providers/env`
- `providers/dotenv`
- `providers/flag`
- `providers/memory`

Provider order matters. Later providers override earlier values.

```go
cfg.Providers(
    file.Optional("config.bcl", bclparser.New()),
    dotenv.Optional(".env"),
    env.Prefix("APP_"),
    flag.Args(os.Args[1:]),
)
```

## Typed decoding

```go
type AppConfig struct {
    Name  string `json:"name"`
    Debug bool   `json:"debug"`
    Port  int    `json:"port"`
}

var app AppConfig
err := cfg.Decode("app", &app)
```

Generic helper:

```go
app := config.MustDecode[AppConfig](cfg, "app")
```

## Hot reload

```go
cfg := config.New(config.WithReloadPolicy(config.ReloadPolicy{
    Debounce: 300 * time.Millisecond,
    MinInterval: time.Second,
    ValidateBeforeCommit: true,
    FailMode: config.KeepPrevious,
}))

cfg.Providers(file.Optional("config.bcl", bclparser.New()))

cfg.OnChange(func(ch config.Change) error {
    if ch.Changed("database.driver") {
        return config.RestartRequired("database.driver")
    }
    return nil
})

err := cfg.Watch(ctx)
```

If validation or a change hook fails, the old snapshot remains active.

## Sensitive variable and secret handling

Use module fields:

```go
s.SecretString("password", "DB_PASSWORD", "")
```

Or mark runtime paths:

```go
cfg.MarkSecret("database.password", "auth.jwt.secret")
```

Configure sensitive matching at startup:

```go
cfg := config.New(config.WithSensitivePolicy(config.SensitivePolicy{
    Words:         []string{"password", "secret", "token", "card", "cvv", "license"},
    Paths:         []string{"tenant.private_note"},
    EnvVars:       []string{"CUSTOM_VENDOR_SECRET"},
    Redaction:     "<hidden>",
    MatchContains: true,
}))
```

Update sensitive policy at runtime:

```go
cfg.AddSensitiveWords("pin", "license")
cfg.RemoveSensitiveWords("license")
cfg.SetSensitiveWords("password", "secret", "token")

cfg.AddSensitivePaths("payment.card_number")
cfg.RemoveSensitivePaths("payment.card_number")
cfg.SetSensitivePaths("database.password", "auth.jwt.private_key")

cfg.AddSensitiveEnvVars("APP_KEY", "DB_PASSWORD")
cfg.RemoveSensitiveEnvVars("APP_KEY")
cfg.SetSensitiveEnvVars("DB_PASSWORD", "JWT_SECRET")

cfg.SetRedactionText("***")
```

The sensitive policy applies to redacted dumps and redacted change diffs before handlers receive them.

Redacted output:

```go
fmt.Println(string(cfg.RedactedJSON()))
```

## Validation

```go
cfg := config.New(config.WithValidateFunc(func(c *config.Manager) error {
    if c.String("app.name") == "" {
        return config.PathError("app.name", "is required")
    }
    return nil
}))
```

## Generate docs

```go
fmt.Println(cfg.Markdown())
fmt.Println(cfg.EnvExample())
```

## Examples

Run enterprise BCL example:

```bash
go get github.com/oarkflow/bcl@v0.0.28
DB_HOST=localhost DB_PASSWORD=secret go run ./examples/enterprise
```

Run typed JSON example:

```bash
go run ./examples/typed
```

Run configurable sensitive policy example:

```bash
go run ./examples/sensitive
```

Run hot reload watcher with BCL:

```bash
go run ./examples/hotreload
```

## Tests

```bash
go test ./...
go test -race ./...
```

## Enterprise readiness

See `ENTERPRISE_READY.md` for the implementation review, completed production features, and recommended optional extensions such as last-known-good cache, signed configs, audit sinks, provider diagnostics, remote providers, and restart-policy enforcement.

## Notes

The configuration platform uses `github.com/oarkflow/bcl` as a first-class parser. BCL examples run normally without build tags.

## Enterprise controls

The manager includes enterprise controls for secure runtime mutation, auditability, compliance, resilience, and operational diagnostics.

### Actor-aware runtime mutation

```go
sink := config.NewMemoryAuditSink(1000)

cfg := config.New(
    config.WithAuditSink(sink),
    config.WithRuntimePolicies(
        config.RuntimePolicy{Path: "app.debug", Mutable: true, Roles: []string{"platform"}, DenyEnvironments: []string{"production"}},
        config.RuntimePolicy{Path: "database.driver", Mutable: false},
    ),
)

err := cfg.SetWithActor(ctx, config.Actor{ID: "admin-1", Roles: []string{"platform"}}, "app.debug", true)
```

All runtime mutations go through policy checks, validation, change handlers, audit events, and atomic snapshot commit.

### Configurable sensitive variables

```go
cfg.AddSensitiveWords("license", "private_note")
cfg.AddSensitivePaths("payment.card_number", "payment.cvv")
cfg.AddSensitiveEnvVars("APP_LICENSE_KEY")
cfg.SetRedactionText("<hidden>")

fmt.Println(string(cfg.RedactedJSON()))
```

The JSON output keeps `<hidden>` readable and does not HTML-escape it.

### Compliance presets

```go
cfg.ApplyCompliance(
    config.PCIMode(),
    config.GDPRMode(),
    config.SOC2Mode(),
    config.ISO27001Mode(),
)
```

These presets add sensitive-key defaults and safe dump behavior. They do not make an application certified or compliant by themselves, but they provide technical controls and evidence needed by business/compliance programs.

### Last-known-good cache

```go
cfg := config.New(
    config.WithLastKnownGood("var/cache/config.snapshot", config.StartupPolicy{
        AllowStale: true,
        MaxAge: 24 * time.Hour,
    }),
)

err := cfg.StartWithLastKnownGood(ctx)
```

If live providers fail on startup, the manager can start from the last valid cached snapshot.

### File integrity and permissions

```go
p := file.Required("config.bcl", bclparser.New()).
    WithChecksum("config.bcl.sha256").
    RequireMode(0600)
```

The file provider can verify checksums, verify Ed25519 signatures, and reject files with overly broad permissions.

### Validation rules

```go
s.Int("port", "APP_PORT", 8080).
    Validate("required,port").
    Description("HTTP listen port")

s.String("env", "APP_ENV", "development").
    Validate("oneof=development staging production")
```

Built-in rules include required, min, max, len, oneof, not_oneof, port, host, hostport, url, ip, cidr, duration_min, duration_max, file_exists, dir_exists, absolute_path, and non_empty_slice.

### Debug info and audit evidence

```go
debug := cfg.DebugInfo(config.DebugOptions{
    IncludeConfig: true,
    Redacted: true,
    IncludeSchema: true,
    IncludeProviders: true,
    IncludeSensitivePolicy: true,
})

evidence := cfg.EvidenceBundle(config.EvidenceOptions{
    Redacted: true,
    IncludeConfig: true,
})
```

These models are framework-independent and can be exposed through `net/http`, `fh`, Fiber, CLI, admin panels, or support bundles.

### Snapshot history and rollback

```go
history := cfg.History()
err := cfg.Rollback(ctx, history[0].Version)
```

Rollback validates the target snapshot and emits an audit event.

### Tenant-aware config and feature flags

```go
cfg.Set("features.new_checkout.enabled", true)
cfg.Set("features.new_checkout.rollout", 25)

if cfg.Feature("new_checkout").ForTenant("acme").Enabled("user-123") {
    // enabled for this tenant/user bucket
}

cfg.ForTenant("acme").Set("features.new_checkout.enabled", true)
```

Tenant overrides inherit global config while keeping tenant-specific changes under `tenants.<id>`.
