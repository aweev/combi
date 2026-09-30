package security

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/combi/pkg/apperror"
)

var (
	// 500 Internal Server Error - Security failures should generally be opaque 500s to the user
	// to prevent oracle attacks, while logging the details internally.

	ErrEncryptionFailed = apperror.New(http.StatusInternalServerError, "ENCRYPTION_FAILED", "An internal security error occurred while protecting data.", nil)

	// Note: We explicitly do NOT define a 400 Bad Request for decryption.
	// If decryption fails, it means data corruption or tampering. That is a server-side data integrity issue.
	ErrDecryptionFailed = apperror.New(http.StatusInternalServerError, "DECRYPTION_FAILED", "Data integrity check failed.", nil)

	ErrInvalidHash  = apperror.New(http.StatusInternalServerError, "INVALID_HASH_FORMAT", "The stored password hash is in an unrecognized format.", nil)
	ErrHashMismatch = apperror.New(http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid password provided.", nil)
)

// CryptoError provides details about a cryptographic failure without leaking secrets.
type CryptoError struct {
	Operation string // "encrypt" or "decrypt"
	KeyID     string
	Err       error
}

func (e *CryptoError) Error() string {
	return fmt.Sprintf("crypto operation '%s' failed (key_id: %s): %v", e.Operation, e.KeyID, e.Err)
}

func (e *CryptoError) Unwrap() error { return e.Err }

// EncryptionContext holds metadata required to decrypt a specific piece of data.
// This is typically stored alongside the encrypted data in a JSONB column.
type EncryptionContext struct {
	// Nonce is the "Number used Once" required by AES-GCM.
	// Reusing a nonce with the same key destroys security.
	Nonce []byte `json:"nonce"`

	// KeyID identifies which key version was used to encrypt this data.
	// This supports key rotation. If empty, the current active key is assumed.
	KeyID string `json:"key_id,omitempty"`

	// AAD (Additional Authenticated Data) allows binding the encryption to a specific context
	// (e.g., a UserID) to prevent "Confused Deputy" attacks where valid ciphertext
	// is moved to another record.
	AAD []byte `json:"aad,omitempty"`
}

// RandomService defines the contract for a cryptographically secure random string generator.
// This abstracts sources like crypto/rand for testability and consistency.
type RandomService interface {
	// GenerateString returns a URL-safe random string of the specified byte length.
	GenerateString(byteLength int) (string, error)

	// GenerateBytes returns raw random bytes.
	GenerateBytes(length int) ([]byte, error)
}

// CryptoService is the contract for symmetric encryption (AES-GCM).
// It is used to protect sensitive data at rest (secrets, PII).
type CryptoService interface {
	// Encrypt secures plaintext. It returns the ciphertext and the context needed to decrypt it.
	Encrypt(ctx context.Context, plaintext []byte) (ciphertext []byte, context *EncryptionContext, err error)

	// Decrypt recovers plaintext using the ciphertext and the original context.
	Decrypt(ctx context.Context, ciphertext []byte, context *EncryptionContext) (plaintext []byte, err error)
}

// Hasher defines the contract for one-way hashing (Passwords).
type Hasher interface {
	// Hash creates a secure hash of the input (e.g., Argon2id).
	Hash(input string) (string, error)

	// Compare verifies if a plaintext input matches the hash.
	Compare(hash, input string) error
}

// TokenService defines the contract for signing and verifying data (HMAC/JWT).
type TokenService interface {
	Sign(data []byte, ttl time.Duration) (string, error)
	Verify(token string) ([]byte, error)
}

// TOTPService manages Time-based One-Time Passwords for multi-factor authentication.
type TOTPManager interface {
	GenerateSecretAndQRCode(issuer, accountEmail string) (rawSecret, encryptedSecret, qrCodeImageURL string, err error)
	VerifyCode(encryptedSecret, code string) (bool, error)
}

// PasswordService handles all password-related operations securely.
type PasswordService interface {
	Hash(password string) (string, error)
	Compare(hashedPassword, password string) error
	ValidateStrength(password string) error
}

// TokenGenerator creates secure, random tokens for single-use actions.
type TokenGenerator interface {
	Generate() (rawToken string, hashedToken string, err error)
	Hash(rawToken string) string
	Compare(hashedToken string, rawToken string) bool
}

// TokenSigner is responsible for creating and verifying JWTs (access tokens).
type TokenSigner interface {
	Sign(claims map[string]interface{}, ttl time.Duration) (string, error)
	Verify(tokenString string) (map[string]interface{}, error)
}
