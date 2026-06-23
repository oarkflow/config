package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	config "github.com/oarkflow/config"
	bclparser "github.com/oarkflow/config/parsers/bcl"
	"github.com/oarkflow/config/providers/file"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.New(config.WithReloadPolicy(config.ReloadPolicy{Debounce: 300 * time.Millisecond, MinInterval: time.Second, ValidateBeforeCommit: true, FailMode: config.KeepPrevious}))
	cfg.Providers(file.Optional("config.bcl", bclparser.New()))
	cfg.OnChange(func(ch config.Change) error {
		log.Printf("new version=%d changed=%+v", ch.NewVersion, ch.Redacted().Paths)
		return nil
	})
	if err := cfg.Load(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("watching config.bcl")
	if err := cfg.Watch(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
