package main

import (
	"fmt"

	config "github.com/oarkflow/config"
)

func main() {
	cfg := config.New(config.WithSensitivePolicy(config.SensitivePolicy{
		Words:         []string{"password", "secret", "card", "cvv", "license"},
		Paths:         []string{"tenant.private_note"},
		EnvVars:       []string{"CUSTOM_VENDOR_SECRET"},
		Redaction:     "<hidden>",
		MatchContains: true,
	}))

	_ = cfg.Set("payment.card_number", "4111111111111111")
	_ = cfg.Set("payment.cvv", "123")
	_ = cfg.Set("app.license_key", "enterprise-license")
	_ = cfg.Set("tenant.private_note", "internal-only")
	_ = cfg.Set("app.name", "Workflow Enterprise")

	fmt.Println(string(cfg.RedactedJSON()))

	cfg.RemoveSensitiveWords("license")
	fmt.Println("license visible after policy update:", cfg.String("app.license_key"))
}
