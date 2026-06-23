package main

import (
	"context"
	"fmt"
	"log"
	"time"

	config "github.com/oarkflow/config"
	bclparser "github.com/oarkflow/config/parsers/bcl"
	"github.com/oarkflow/config/providers/env"
	"github.com/oarkflow/config/providers/file"
)

type AppModule struct{}

func (AppModule) Prefix() string { return "app" }
func (AppModule) Configure(s *config.Section) {
	s.SecretString("key", "APP_KEY", "").Description("Application signing/encryption key").Required()
	s.String("name", "APP_NAME", "Workflow v1.0").Description("Application display name")
	s.Bool("debug", "APP_DEBUG", false).Description("Enable debug logging")
	s.String("host", "APP_HOST", "0.0.0.0")
	s.Int("port", "APP_PORT", 3003)
	s.Bool("reload", "APP_RELOAD", true).Description("Enable hot reload")
	s.Duration("shutdown_timeout", "APP_SHUTDOWN_TIMEOUT", 30*time.Second)
}

type DatabaseModule struct{}

func (DatabaseModule) Prefix() string { return "database" }
func (DatabaseModule) Configure(s *config.Section) {
	s.String("driver", "DB_DRIVER", "postgres")
	s.String("host", "DB_HOST", "localhost")
	s.Int("port", "DB_PORT", 5432)
	s.String("user", "DB_USER", "postgres")
	s.SecretString("password", "DB_PASSWORD", "")
	s.String("name", "DB_NAME", "app")
	s.String("sslmode", "DB_SSLMODE", "require")
	s.Int("max_open", "DB_MAX_OPEN", 25)
}

type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

func main() {
	ctx := context.Background()
	cfg := config.New(
		config.WithValidateFunc(func(c *config.Manager) error {
			if c.String("app.name") == "" {
				return config.PathError("app.name", "is required")
			}
			if c.String("database.sslmode") == "disable" && !c.Bool("app.debug") {
				return config.PathError("database.sslmode", "cannot be disable outside debug mode")
			}
			return nil
		}),
	)

	if err := cfg.Register(AppModule{}, DatabaseModule{}); err != nil {
		log.Fatal(err)
	}

	cfg.Providers(
		file.Optional("config.bcl", bclparser.New()),
		env.Prefix("APP_"),
	)

	cfg.OnChange(func(ch config.Change) error {
		if ch.Changed("database.driver") {
			return config.RestartRequired("database.driver")
		}
		log.Printf("config changed: %+v", ch.Redacted().Paths)
		return nil
	})

	if err := cfg.Load(ctx); err != nil {
		log.Fatal(err)
	}

	fmt.Println("app:", cfg.String("app.name"))
	fmt.Println("port:", cfg.Int("app.port"))
	fmt.Println("db password redacted:", cfg.Secret("database.password"))

	var server ServerConfig
	if err := cfg.Decode("app", &server); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("typed server config: %+v\n", server)

	if err := cfg.Set("app.debug", true); err != nil {
		log.Fatal(err)
	}
	fmt.Println("runtime debug:", cfg.Bool("app.debug"))
}
