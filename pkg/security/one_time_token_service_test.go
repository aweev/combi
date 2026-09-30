package security

import (
	"testing"

	"github.com/combi/pkg/testutil"
)

func TestSecureTokenGenerator(t *testing.T) {
	// needs only random service; redis container reused for convenience
	_, teardown := testutil.NewTestRedis(t)
	defer teardown()

	svc := NewTokenGenerator(NewRandomService())

	t.Run("Generate unique tokens", func(t *testing.T) {
		raw1, hash1, err := svc.Generate()
		if err != nil {
			t.Fatal(err)
		}
		raw2, hash2, err := svc.Generate()
		if err != nil {
			t.Fatal(err)
		}
		if raw1 == raw2 || hash1 == hash2 {
			t.Fatal("tokens not unique")
		}
	})

	t.Run("Compare success", func(t *testing.T) {
		raw, hash, _ := svc.Generate()
		if !svc.Compare(hash, raw) {
			t.Fatal("compare should succeed")
		}
	})

	t.Run("Compare failure", func(t *testing.T) {
		_, hash, _ := svc.Generate()
		if svc.Compare(hash, "wrong") {
			t.Fatal("compare should fail")
		}
	})

	t.Run("Hash is deterministic", func(t *testing.T) {
		raw := "test-raw"
		h1 := svc.Hash(raw)
		h2 := svc.Hash(raw)
		if h1 != h2 {
			t.Fatal("hash not deterministic")
		}
	})
}
