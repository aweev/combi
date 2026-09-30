package security

import (
	"strings"
	"testing"
	"time"

	"github.com/combi/pkg/testutil"
	"github.com/pquerna/otp/totp"
)

func TestTOTPManager(t *testing.T) {
	_, teardown := testutil.NewTestRedis(t)
	defer teardown()

	cryptoSvc, _ := NewAesGcmCryptoService(make([]byte, 32), NewRandomService())
	mgr := NewTOTPManager(cryptoSvc)

	issuer, email := "test-app", "user@example.com"

	t.Run("GenerateSecretAndQRCode", func(t *testing.T) {
		raw, enc, qr, err := mgr.GenerateSecretAndQRCode(issuer, email)
		if err != nil {
			t.Fatal(err)
		}
		if raw == "" || enc == "" || qr == "" {
			t.Fatal("empty return values")
		}
		if !strings.HasPrefix(qr, "data:image/png;base64,") {
			t.Fatal("qr code is not a data url")
		}

		// ensure encrypted secret is different from raw
		if enc == raw {
			t.Fatal("encrypted equals raw")
		}
	})

	t.Run("VerifyCode success", func(t *testing.T) {
		raw, enc, _, _ := mgr.GenerateSecretAndQRCode(issuer, email)
		code, err := totp.GenerateCode(raw, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		ok, err := mgr.VerifyCode(enc, code)
		if err != nil || !ok {
			t.Fatalf("valid code rejected: ok=%v err=%v", ok, err)
		}
	})

	t.Run("VerifyCode wrong code", func(t *testing.T) {
		_, enc, _, _ := mgr.GenerateSecretAndQRCode(issuer, email)
		ok, err := mgr.VerifyCode(enc, "000000")
		if err != nil || ok {
			t.Fatalf("invalid code accepted: ok=%v err=%v", ok, err)
		}
	})
}
