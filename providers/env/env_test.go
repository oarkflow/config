package env_test

import (
	"context"
	config "github.com/oarkflow/config"
	"github.com/oarkflow/config/providers/env"
	"testing"
)

func TestEnvProvider(t *testing.T) {
	t.Setenv("APP_SERVER_PORT", "8081")
	cfg := config.New(config.WithProviders(env.Prefix("APP_")))
	if err := cfg.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cfg.Int("server.port") != 8081 {
		t.Fatalf("got %v", cfg.Get("server.port"))
	}
}
