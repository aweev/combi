package security

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"

	"github.com/pquerna/otp/totp"
)

// Statically verify that it implements the domain interface.
var _ TOTPManager = (*tOTPManager)(nil)

// TOTPManager handles the generation and verification of Time-based One-Time Passwords.
type tOTPManager struct {
	cryptoService CryptoService
}

// NewTOTPManager creates a new TOTP manager. It requires a crypto service
// to ensure that TOTP secrets are encrypted before being stored.
func NewTOTPManager(cryptoService CryptoService) *tOTPManager {
	return &tOTPManager{
		cryptoService: cryptoService,
	}
}

// GenerateSecretAndQRCode creates a new TOTP secret for a user and a corresponding QR code image.
// It returns the base32 secret (for manual entry), the encrypted secret (for storage),
// and a data URL for the QR code image.
func (m *tOTPManager) GenerateSecretAndQRCode(issuer, accountEmail string) (rawSecret, encryptedSecret, qrCodeImageURL string, err error) {
	// 1. Generate a new TOTP key.
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountEmail,
	})
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate TOTP key: %w", err)
	}

	// 2. Encrypt the secret for safe storage in the database.
	// We encrypt the key's secret, which is the raw shared secret.
	ciphertext, context, err := m.cryptoService.Encrypt(context.Background(), []byte(key.Secret()))
	if err != nil {
		return "", "", "", fmt.Errorf("failed to encrypt TOTP secret: %w", err)
	}

	// We need to store both the ciphertext and the context (nonce) to be able to decrypt it later.
	// We'll marshal them into a single JSON blob.
	encryptedPayload := map[string][]byte{
		"ciphertext": ciphertext,
		"nonce":      context.Nonce,
	}
	encryptedPayloadBytes, err := json.Marshal(encryptedPayload)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to marshal encrypted TOTP payload: %w", err)
	}
	encryptedSecret = base64.StdEncoding.EncodeToString(encryptedPayloadBytes)

	// 3. Generate the QR code image.
	var buf bytes.Buffer
	img, err := key.Image(256, 256)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate TOTP QR code image: %w", err)
	}
	if err := png.Encode(&buf, img); err != nil {
		return "", "", "", fmt.Errorf("failed to encode QR code image to PNG: %w", err)
	}
	qrCodeImageURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	// Return the raw secret for the user to see/enter manually, the encrypted version for storage,
	// and the QR code.
	return key.Secret(), encryptedSecret, qrCodeImageURL, nil
}

// VerifyCode validates a user-provided TOTP code against the encrypted secret from the database.
func (m *tOTPManager) VerifyCode(encryptedSecretFromDB, code string) (bool, error) {
	// 1. Unmarshal the encrypted payload from the database.
	encryptedPayloadBytes, err := base64.StdEncoding.DecodeString(encryptedSecretFromDB)
	if err != nil {
		return false, fmt.Errorf("invalid base64 for encrypted secret: %w", err)
	}

	var encryptedPayload struct {
		Ciphertext []byte `json:"ciphertext"`
		Nonce      []byte `json:"nonce"`
	}
	if err := json.Unmarshal(encryptedPayloadBytes, &encryptedPayload); err != nil {
		return false, fmt.Errorf("failed to unmarshal encrypted TOTP payload: %w", err)
	}

	// 2. Decrypt the secret.
	decryptedSecret, err := m.cryptoService.Decrypt(context.Background(), encryptedPayload.Ciphertext, &EncryptionContext{Nonce: encryptedPayload.Nonce})
	if err != nil {
		// This could indicate a problem with the stored data or the encryption key.
		return false, fmt.Errorf("failed to decrypt TOTP secret: %w", err)
	}

	// 3. Validate the code against the decrypted secret.
	valid := totp.Validate(code, string(decryptedSecret))
	return valid, nil
}
