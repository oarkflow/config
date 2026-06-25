package main

import (
	"context"
	"fmt"
	"log"

	config "github.com/oarkflow/config"
)

type DatabaseModule struct{}

func (DatabaseModule) Prefix() string { return "database" }
func (DatabaseModule) Configure(s *config.Section) {
	s.String("host", "DB_HOST", "localhost")
	s.Int("port", "DB_PORT", 5432)
	s.String("user", "DB_USER", "admin")
	s.EncryptedString("password", "DB_PASSWORD", "default-pass")
}

func main() {
	enc := config.NewAESGCMEncryptorFromPassphrase("my-strong-passphrase")
	cfg := config.New(config.WithEncryptor(enc))
	if err := cfg.Register(DatabaseModule{}); err != nil {
		log.Fatal(err)
	}
	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}
	for _, path := range []string{"database.password"} {
		if err := cfg.EncryptPath(path); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("host:", cfg.String("database.host"))
	fmt.Println("port:", cfg.Int("database.port"))
	fmt.Println("user:", cfg.String("database.user"))
	fmt.Println("password:", cfg.String("database.password"))
	raw, _ := cfg.Snapshot().Tree.Get("database.password")
	fmt.Printf("raw in tree: %s\n", raw)
}
