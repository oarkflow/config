package file_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	config "github.com/oarkflow/config"
	"github.com/oarkflow/config/parsers/json"
	"github.com/oarkflow/config/providers/file"
)

func TestFileJSONProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"app":{"name":"x"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := config.New(config.WithProviders(file.Required(path, jsonparser.New())))
	if err := cfg.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cfg.String("app.name") != "x" {
		t.Fatal(cfg.String("app.name"))
	}
}
