package config

type Classification string

const (
	ClassPublic       Classification = "public"
	ClassInternal     Classification = "internal"
	ClassConfidential Classification = "confidential"
	ClassRestricted   Classification = "restricted"
	ClassSecret       Classification = "secret"
)

type PrivacyPolicy struct {
	RedactPersonalData bool
	PersonalWords      []string
	DisableRawDump     bool
}
type CompliancePolicy struct {
	Name            string
	SensitiveWords  []string
	SensitivePaths  []string
	DisableRawDump  bool
	DenyPaymentData bool
}

func PCIMode() CompliancePolicy {
	return CompliancePolicy{Name: "pci", SensitiveWords: []string{"pan", "card", "card_number", "credit_card", "cvv", "cvc", "track", "pin", "payment_token"}, DenyPaymentData: true, DisableRawDump: true}
}
func GDPRMode() CompliancePolicy {
	return CompliancePolicy{Name: "gdpr", SensitiveWords: []string{"email", "phone", "address", "full_name", "national_id", "passport", "date_of_birth"}, DisableRawDump: true}
}
func SOC2Mode() CompliancePolicy     { return CompliancePolicy{Name: "soc2", DisableRawDump: true} }
func ISO27001Mode() CompliancePolicy { return CompliancePolicy{Name: "iso27001", DisableRawDump: true} }

func (m *Manager) ApplyCompliance(policies ...CompliancePolicy) {
	for _, p := range policies {
		if len(p.SensitiveWords) > 0 {
			m.AddSensitiveWords(p.SensitiveWords...)
		}
		if len(p.SensitivePaths) > 0 {
			m.AddSensitivePaths(p.SensitivePaths...)
		}
		if p.DisableRawDump {
			m.security.DisableRawDump = true
			m.security.RequireRedactedDump = true
		}
		if p.DenyPaymentData {
			m.SetRuntimePolicies(RuntimePolicy{Path: "*.card_number", Mutable: false, Sensitive: true}, RuntimePolicy{Path: "*.cvv", Mutable: false, Sensitive: true})
		}
	}
}
