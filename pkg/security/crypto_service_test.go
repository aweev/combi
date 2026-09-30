package security

import (
	"bytes"
	"context"
	"testing"

	"github.com/combi/pkg/testutil"
)

func TestAesGcmCryptoService(t *testing.T) {
	_, teardown := testutil.NewTestRedis(t) 
	defer teardown()

	randomSvc := NewRandomService()
	key := make([]byte, 32) // AES-256
	if _, err := randomSvc.GenerateBytes(32); err != nil {
		t.Fatal(err)
	}

	svc, err := NewAesGcmCryptoService(key, randomSvc)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	ctx := context.Background()

	t.Run("round-trip encrypt/decrypt", func(t *testing.T) {
		plain := []byte("hello world")

		cipher, encCtx, err := svc.Encrypt(ctx, plain)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(cipher, plain) {
			t.Fatal("ciphertext equals plaintext")
		}

		got, err := svc.Decrypt(ctx, cipher, encCtx)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, plain) {
			t.Fatalf("got %q, want %q", got, plain)
		}
	})

	t.Run("decrypt with wrong nonce fails", func(t *testing.T) {
		plain := []byte("secret")
		cipher, encCtx, _ := svc.Encrypt(ctx, plain)
		encCtx.Nonce[0] ^= 0xFF // corrupt nonce

		_, err := svc.Decrypt(ctx, cipher, encCtx)
		if err == nil {
			t.Fatal("expected decrypt error")
		}
	})

	t.Run("invalid key length rejected", func(t *testing.T) {
		_, err := NewAesGcmCryptoService([]byte("short"), randomSvc)
		if err == nil {
			t.Fatal("expected error for short key")
		}
	})
}
