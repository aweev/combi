package security

import (
	"testing"
)

func TestCryptoRandomService(t *testing.T) {
	svc := NewRandomService()

	t.Run("GenerateBytes", func(t *testing.T) {
		b1, err := svc.GenerateBytes(32)
		if err != nil {
			t.Fatal(err)
		}
		b2, err := svc.GenerateBytes(32)
		if err != nil {
			t.Fatal(err)
		}
		if len(b1) != 32 || len(b2) != 32 {
			t.Fatal("wrong length")
		}
		// extremely unlikely to collide
		if string(b1) == string(b2) {
			t.Fatal("duplicate random bytes")
		}
	})

	t.Run("GenerateString", func(t *testing.T) {
		s1, err := svc.GenerateString(32)
		if err != nil {
			t.Fatal(err)
		}
		s2, err := svc.GenerateString(32)
		if err != nil {
			t.Fatal(err)
		}
		if s1 == s2 {
			t.Fatal("duplicate random strings")
		}
		// base64 url encoding → no '+' or '/'
		if len(s1) == 0 || len(s2) == 0 {
			t.Fatal("empty string")
		}
	})
}
