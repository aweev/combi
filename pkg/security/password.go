package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/auth"
	"golang.org/x/crypto/argon2"
)

// Statically verify that it implements the domain interface.
var _ PasswordService = (*passwordService)(nil)

// PasswordService provides secure password hashing and verification using Argon2id.
type passwordService struct {
	policy config.PasswordPolicyConfig
	// Argon2id parameters - these are sensible defaults for a web application.
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

// NewPasswordService creates a new password service with the given policy.
func NewPasswordService(policy config.PasswordPolicyConfig) PasswordService {
	return &passwordService{
		policy:      policy,
		memory:      64 * 1024, // 64 MB
		iterations:  3,
		parallelism: 2,
		saltLength:  16,
		keyLength:   32,
	}
}

// Hash creates an Argon2id hash of a password.
func (s *passwordService) Hash(password string) (string, error) {
	salt := make([]byte, s.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, s.iterations, s.memory, s.parallelism, s.keyLength)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encodedHash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, s.memory, s.iterations, s.parallelism, b64Salt, b64Hash)

	return encodedHash, nil
}

// Compare verifies if a plaintext password matches a stored Argon2id hash.
func (s *passwordService) Compare(hashedPassword, password string) error {
	p, salt, hash, err := s.decodeHash(hashedPassword)
	if err != nil {
		return err
	}

	otherHash := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, p.keyLength)

	if subtle.ConstantTimeCompare(hash, otherHash) == 1 {
		return nil // Match
	}

	return auth.ErrIncorrectPassword
}

// ValidateStrength checks a password against the configured security policy.
func (s *passwordService) ValidateStrength(password string) error {
	if len(password) < s.policy.MinLength {
		return fmt.Errorf("%w: must be at least %d characters long", auth.ErrPasswordTooWeak, s.policy.MinLength)
	}
	var (
		hasUpper, hasLower, hasNumber, hasSpecial bool
	)
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if s.policy.RequireUppercase && !hasUpper {
		return fmt.Errorf("%w: must contain at least one uppercase letter", auth.ErrPasswordTooWeak)
	}
	if s.policy.RequireLowercase && !hasLower {
		return fmt.Errorf("%w: must contain at least one lowercase letter", auth.ErrPasswordTooWeak)
	}
	if s.policy.RequireNumbers && !hasNumber {
		return fmt.Errorf("%w: must contain at least one number", auth.ErrPasswordTooWeak)
	}
	if s.policy.RequireSpecialChars && !hasSpecial {
		return fmt.Errorf("%w: must contain at least one special character", auth.ErrPasswordTooWeak)
	}

	return nil
}

// params holds the decoded Argon2id parameters from a hash string.
type params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

// decodeHash parses the Argon2id hash string format.
func (s *passwordService) decodeHash(encodedHash string) (*params, []byte, []byte, error) {
	vals := strings.Split(encodedHash, "$")
	if len(vals) != 6 {
		return nil, nil, nil, auth.ErrInvalidHashFormat
	}

	var version int
	_, err := fmt.Sscanf(vals[2], "v=%d", &version)
	if err != nil || version != argon2.Version {
		return nil, nil, nil, auth.ErrInvalidHashFormat
	}

	p := &params{}
	_, err = fmt.Sscanf(vals[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism)
	if err != nil {
		return nil, nil, nil, auth.ErrInvalidHashFormat
	}

	salt, err := base64.RawStdEncoding.DecodeString(vals[4])
	if err != nil {
		return nil, nil, nil, auth.ErrInvalidHashFormat
	}
	p.saltLength = uint32(len(salt))

	hash, err := base64.RawStdEncoding.DecodeString(vals[5])
	if err != nil {
		return nil, nil, nil, auth.ErrInvalidHashFormat
	}
	p.keyLength = uint32(len(hash))

	return p, salt, hash, nil
}
