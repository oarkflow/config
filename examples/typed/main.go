package main

import (
	"context"
	"fmt"
	"log"

	config "github.com/oarkflow/config"
	"github.com/oarkflow/config/parsers/json"
	"github.com/oarkflow/config/providers/file"
)

type Config struct {
	App    AppConfig    `json:"app"`
	Server ServerConfig `json:"server"`
}
type AppConfig struct {
	Name  string `json:"name"`
	Debug bool   `json:"debug"`
}
type ServerConfig struct {
	Addr string `json:"addr"`
	Port int    `json:"port"`
}

func main() {
	cfg, err := config.Load(context.Background(), config.WithProviders(file.Required("config.json", jsonparser.New())))
	if err != nil {
		log.Fatal(err)
	}
	typed := config.MustDecode[Config](cfg, "")
	fmt.Printf("%+v\n", typed)
}
