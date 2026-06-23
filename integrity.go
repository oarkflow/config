package config

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"strings"
)

type IntegrityPolicy struct {
	Strict         bool              `json:"strict"`
	ChecksumFiles  map[string]string `json:"checksum_files,omitempty"`
	PublicKey      ed25519.PublicKey `json:"-"`
	SignatureFiles map[string]string `json:"signature_files,omitempty"`
}

func VerifyFileChecksum(path, checksumFile string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	cb, err := os.ReadFile(checksumFile)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	got := hex.EncodeToString(sum[:])
	fields := strings.Fields(string(cb))
	if len(fields) == 0 || !strings.EqualFold(got, fields[0]) {
		return &ConfigError{Kind: ErrSecurity, Source: path, Message: "checksum verification failed"}
	}
	return nil
}

func VerifyFileSignature(path, sigFile string, pub ed25519.PublicKey) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(sigFile)
	if err != nil {
		return err
	}
	sig = []byte(strings.TrimSpace(string(sig)))
	if decoded, err := hex.DecodeString(string(sig)); err == nil {
		sig = decoded
	} else if decoded, err := base64.StdEncoding.DecodeString(string(sig)); err == nil {
		sig = decoded
	}
	if !ed25519.Verify(pub, b, sig) {
		return &ConfigError{Kind: ErrSecurity, Source: path, Message: "signature verification failed"}
	}
	return nil
}
