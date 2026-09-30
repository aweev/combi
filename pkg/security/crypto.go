package security

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/combi/pkg/apperror"
	"github.com/alexedwards/argon2id"
)

// --- SYMMETRIC ENCRYPTION (AES-GCM) ---

// AESCryptoService implements CryptoService using AES-GCM.
type AESCryptoService struct {
	key   []byte // 32 bytes for AES-256
	keyID string // Identifier for key rotation
}

// NewAESCryptoService creates a service with a specific key version.
func NewAESCryptoService(keyBase64 string, keyID string) (*AESCryptoService, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes (AES-256), got %d", len(key))
	}
	if keyID == "" {
		keyID = "v1" // Default if not provided
	}
	return &AESCryptoService{key: key, keyID: keyID}, nil
}

// Encrypt secures plaintext. It accepts optional AAD (Additional Authenticated Data)
// via the context if needed, but usually, AAD is passed explicitly.
// Since the interface signature is fixed: Encrypt(ctx, plaintext), we will generate
// the AAD if it exists in the context, or leave it nil.
func (s *AESCryptoService) Encrypt(ctx context.Context, plaintext []byte) ([]byte, *EncryptionContext, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, nil, apperror.New(http.StatusInternalServerError, apperror.Internal, "encryption init failed", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, apperror.New(http.StatusInternalServerError, apperror.Internal, "gcm init failed", err)
	}

	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, apperror.New(http.StatusInternalServerError, apperror.Internal, "nonce generation failed", err)
	}

	// Check if AAD is provided in context (advanced usage)
	var aad []byte
	if val, ok := ctx.Value("aad").([]byte); ok {
		aad = val
	}

	// Encrypt. The 'aad' here binds this data to the context.
	ciphertext := aesgcm.Seal(nil, nonce, plaintext, aad)

	return ciphertext, &EncryptionContext{
		Nonce: nonce,
		KeyID: s.keyID,
		AAD:   aad,
	}, nil
}

func (s *AESCryptoService) Decrypt(ctx context.Context, ciphertext []byte, ec *EncryptionContext) ([]byte, error) {
	// In a real key rotation scenario, you would check ec.KeyID here
	// and select the correct key from a map of keys.
	// For now, we assume single key.

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, apperror.Internal, "decryption init failed", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, apperror.New(http.StatusInternalServerError, apperror.Internal, "gcm init failed", err)
	}

	// Open checks the signature (MAC) automatically using the AAD.
	plaintext, err := aesgcm.Open(nil, ec.Nonce, ciphertext, ec.AAD)
	if err != nil {
		// Security Best Practice: Do not distinguish between "bad key" and "tampered data".
		return nil, apperror.New(http.StatusInternalServerError, apperror.Internal, "decryption failed: integrity check", ErrDecryptionFailed)
	}

	return plaintext, nil
}

// --- PASSWORD HASHING (Argon2id) ---

// ArgonHasher implements Hasher using the alexedwards/argon2id library.
// It follows OWASP recommendations for parameter tuning.
type ArgonHasher struct {
	params *argon2id.Params
}

func NewArgonHasher() *ArgonHasher {
	return &ArgonHasher{
		params: argon2id.DefaultParams,
	}
}

// Hash creates an Argon2id hash of the password.
func (h *ArgonHasher) Hash(password string) (string, error) {
	hash, err := argon2id.CreateHash(password, h.params)
	if err != nil {
		return "", apperror.New(http.StatusInternalServerError, apperror.Internal, "failed to hash password", err)
	}
	return hash, nil
}

// Compare verifies a password against a hash.
// Returns nil on success, or ErrHashMismatch on failure.
func (h *ArgonHasher) Compare(hash, password string) error {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		// This usually means the hash format is invalid (e.g. not argon2id)
		return apperror.New(http.StatusInternalServerError, apperror.Internal, "password comparison error", err)
	}
	if !match {
		return ErrHashMismatch
	}
	return nil
}
