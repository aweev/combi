package security

import (
	"testing"

	"github.com/combi/pkg/config"
)

func TestPasswordService(t *testing.T) {
	policy := config.PasswordPolicyConfig{
		MinLength:           8,
		RequireUppercase:    true,
		RequireLowercase:    true,
		RequireNumbers:      true,
		RequireSpecialChars: true,
	}
	svc := NewPasswordService(policy)

	t.Run("hash and compare success", func(t *testing.T) {
		hash, err := svc.Hash("Valid123!")
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.Compare(hash, "Valid123!"); err != nil {
			t.Fatal("compare failed", err)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		hash, _ := svc.Hash("Valid123!")
		err := svc.Compare(hash, "Wrong123!")
		if err == nil {
			t.Fatal("expected password mismatch")
		}
	})

	t.Run("validate strength", func(t *testing.T) {
		tests := []struct {
			pass string
			ok   bool
		}{
			{"short", false},
			{"noupper123!", false},
			{"NOLOWER123!", false},
			{"NoNumber!", false},
			{"NoSpecial123", false},
			{"Valid123!", true},
		}
		for _, tc := range tests {
			err := svc.ValidateStrength(tc.pass)
			if (err == nil) != tc.ok {
				t.Errorf("pass=%q, expected ok=%v, got err=%v", tc.pass, tc.ok, err)
			}
		}
	})
}
