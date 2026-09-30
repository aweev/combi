package compliance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type PIIKey string

const (
	KeyName     PIIKey = "name"
	KeyAddress  PIIKey = "address"
	KeyPhone    PIIKey = "phone"
	KeyEmail    PIIKey = "email"
	KeyIDNumber PIIKey = "id_number"
)

type ConsentRecord struct {
	Subject    string    `json:"subject"`
	Purpose    string    `json:"purpose"`
	AcceptedAt time.Time `json:"accepted_at"`
	Version    string    `json:"version"`
}

type Residency struct {
	Country   string
	Region    string
	GrantedAt time.Time
}

func Redact(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= 4 {
		return strings.Repeat("*", len(trimmed))
	}
	visible := trimmed[len(trimmed)-4:]
	return strings.Repeat("*", len(trimmed)-4) + visible
}

func ImmutableConsentHash(record ConsentRecord) string {
	builder := strings.Builder{}
	builder.WriteString(strings.TrimSpace(record.Subject))
	builder.WriteString("|")
	builder.WriteString(strings.TrimSpace(record.Purpose))
	builder.WriteString("|")
	builder.WriteString(record.AcceptedAt.UTC().Format(time.RFC3339Nano))
	builder.WriteString("|")
	builder.WriteString(strings.TrimSpace(record.Version))

	h := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(h[:])
}

func MustEncryptLocal(res Residency) bool {
	return strings.TrimSpace(res.Country) != "" && strings.TrimSpace(res.Region) != ""
}

func ValidateConsent(record ConsentRecord) error {
	if strings.TrimSpace(record.Subject) == "" {
		return fmt.Errorf("compliance: subject required")
	}
	if strings.TrimSpace(record.Purpose) == "" {
		return fmt.Errorf("compliance: purpose required")
	}
	if record.AcceptedAt.IsZero() {
		return fmt.Errorf("compliance: accepted timestamp required")
	}
	return nil
}
