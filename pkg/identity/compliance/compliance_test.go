package compliance_test

import (
	"testing"
	"time"

	"github.com/combi/pkg/identity/compliance"
)

func TestRedact(t *testing.T) {
	if got := compliance.Redact("1234567890"); got != "******7890" {
		t.Fatalf("unexpected redact output: %s", got)
	}
}

func TestImmutableConsentHash(t *testing.T) {
	record := compliance.ConsentRecord{Subject: "user1", Purpose: "auth", AcceptedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Version: "1.0"}
	hash := compliance.ImmutableConsentHash(record)
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
}

func TestValidateConsent(t *testing.T) {
	record := compliance.ConsentRecord{Subject: "user1", Purpose: "auth", AcceptedAt: time.Now(), Version: "1.0"}
	if err := compliance.ValidateConsent(record); err != nil {
		t.Fatal(err)
	}
}
