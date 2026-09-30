package security

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

// aesGcmCryptoService implements the CryptoService interface using AES-GCM.
// AES-GCM is a modern, authenticated encryption mode that provides both
// confidentiality and integrity.
type aesGcmCryptoService struct {
	key          []byte
	randomSvc    RandomService
	gcmNonceSize int
}

// Statically verify that it implements the interface.
var _ CryptoService = (*aesGcmCryptoService)(nil)

// NewAesGcmCryptoService creates a new encryption service.
// It requires a pre-configured encryption key and a RandomService for generating nonces.
// The key MUST be either 16, 24, or 32 bytes to select AES-128, AES-192, or AES-256.
func NewAesGcmCryptoService(encryptionKey []byte, randomSvc RandomService) (CryptoService, error) {
	keyLen := len(encryptionKey)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, fmt.Errorf("invalid encryption key length: must be 16, 24, or 32 bytes, but got %d", keyLen)
	}

	// Create a temporary block cipher to determine the standard nonce size for GCM.
	// This avoids having to recalculate it on every encryption call.
	tempBlock, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cipher block: %w", err)
	}
	tempGcm, err := cipher.NewGCM(tempBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCM mode: %w", err)
	}

	return &aesGcmCryptoService{
		key:          encryptionKey,
		randomSvc:    randomSvc,
		gcmNonceSize: tempGcm.NonceSize(),
	}, nil
}

// Encrypt transforms plaintext into ciphertext using AES-GCM.
func (s *aesGcmCryptoService) Encrypt(ctx context.Context, plaintext []byte) ([]byte, *EncryptionContext, error) {
	// 1. Create a new AES cipher block from the pre-configured key.
	block, err := aes.NewCipher(s.key)
	if err != nil {
		// This should not happen if the key length was validated at creation.
		return nil, nil, fmt.Errorf("failed to create AES cipher block: %w", err)
	}

	// 2. Create a new GCM instance.
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM cipher mode: %w", err)
	}

	// 3. Generate a cryptographically secure, random nonce.
	// The nonce is critical for security and MUST NOT be reused with the same key.
	nonce, err := s.randomSvc.GenerateBytes(s.gcmNonceSize)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate secure nonce: %w", err)
	}

	// 4. Encrypt the data. GCM's `Seal` method handles the encryption and
	// generates an authentication tag, which it appends to the ciphertext.
	// The first argument to Seal is a destination buffer; passing `nil` tells it
	// to allocate a new buffer. The second argument is the nonce. The third is
	// the plaintext. The fourth is optional additional authenticated data (AAD),
	// which we are not using here.
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// 5. Create the context for decryption.
	context := &EncryptionContext{
		Nonce: nonce,
	}

	return ciphertext, context, nil
}

// Decrypt transforms ciphertext back into plaintext using the provided context.
func (s *aesGcmCryptoService) Decrypt(ctx context.Context, ciphertext []byte, context *EncryptionContext) ([]byte, error) {
	// 1. Validate the context.
	if context == nil || len(context.Nonce) != s.gcmNonceSize {
		return nil, fmt.Errorf("invalid encryption context: nonce is missing or has incorrect size")
	}

	// 2. Create the AES cipher block and GCM instance.
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher block for decryption: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM cipher mode for decryption: %w", err)
	}

	// 3. Decrypt the data. GCM's `Open` method verifies the authentication tag
	// (which is part of the ciphertext) before decrypting. If the ciphertext
	// has been tampered with, this operation will fail, which is a key security feature.
	plaintext, err := gcm.Open(nil, context.Nonce, ciphertext, nil)
	if err != nil {
		// This is a critical error. It means the data is corrupt or has been tampered with.
		// We return a generic error to avoid leaking information.
		return nil, fmt.Errorf("failed to decrypt or authenticate data")
	}

	return plaintext, nil
}
