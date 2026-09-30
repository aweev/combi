package security

import (
	"errors"
	"testing"
	"time"

	"github.com/combi/pkg/domain/auth"
)

func TestJWTSigner(t *testing.T) {
	secret := "this-is-a-very-secure-secret-that-is-32-bytes-long-now!"
	issuer := "test-issuer"
	signer, err := NewJWTSigner(secret, issuer)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	claims := map[string]interface{}{"sub": "user-123", "role": "admin"}

	t.Run("sign and verify happy path", func(t *testing.T) {
		token, err := signer.Sign(claims, 5*time.Minute)
		if err != nil {
			t.Fatal(err)
		}

		got, err := signer.Verify(token)
		if err != nil {
			t.Fatal(err)
		}
		if got["sub"] != "user-123" || got["role"] != "admin" || got["iss"] != issuer {
			t.Fatalf("unexpected claims: %v", got)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		token, _ := signer.Sign(claims, -2*time.Minute) // older than leeway
		_, err := signer.Verify(token)
		if !errors.Is(err, auth.ErrTokenExpired) {
			t.Fatalf("expected ErrTokenExpired, got %v", err)
		}
	})
	t.Run("invalid signature", func(t *testing.T) {
		token, _ := signer.Sign(claims, 5*time.Minute)
		token = token[:len(token)-10] + "wrongbits"
		_, err := signer.Verify(token)
		if !errors.Is(err, auth.ErrTokenSignature) {
			t.Fatalf("expected ErrTokenSignature, got %v", err)
		}
	})

	t.Run("malformed token", func(t *testing.T) {
		_, err := signer.Verify("not.jwt")
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Fatalf("expected ErrTokenInvalid, got %v", err)
		}
	})
}
