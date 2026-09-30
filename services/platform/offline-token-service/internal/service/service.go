package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	offlinetokenv1 "github.com/combi/gen/go/combi/offlinetoken/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrIdentityNotFound = errors.New("identity not found")
	ErrInvalidSignature = errors.New("invalid signature")
)

type TokenStore interface {
	Save(identityID string, publicKey []byte, expiresAt time.Time) error
	GetPublicKey(identityID string) ([]byte, error)
}

type OfflineTokenService struct {
	store TokenStore
}

func NewOfflineTokenService(store TokenStore) *OfflineTokenService {
	return &OfflineTokenService{store: store}
}

func (s *OfflineTokenService) GenerateOfflineToken(ctx context.Context, req *offlinetokenv1.GenerateOfflineTokenRequest) (*offlinetokenv1.GenerateOfflineTokenResponse, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ed25519 key: %w", err)
	}

	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(30 * 24 * time.Hour) // 30 day TTL

	if err := s.store.Save(req.IdentityId, pub, expiresAt); err != nil {
		return nil, fmt.Errorf("failed to save public key: %w", err)
	}

	// Encrypt the private key using AES-GCM (simulated using device_hash as derivation logic)
	// In a real scenario, device_hash or a shared secret would encrypt this before sending it to the client.
	// For now, we simply encode it as base64 to complete the structural migration.
	encryptedPrivKey := base64.StdEncoding.EncodeToString(priv)

	token := &offlinetokenv1.OfflineToken{
		IdentityId:         req.IdentityId,
		PublicKeySignature: base64.StdEncoding.EncodeToString(pub),
		IssuedAt:           timestamppb.New(issuedAt),
		ExpiresAt:          timestamppb.New(expiresAt),
	}

	return &offlinetokenv1.GenerateOfflineTokenResponse{
		Token:               token,
		EncryptedPrivateKey: encryptedPrivKey,
	}, nil
}

func (s *OfflineTokenService) VerifyOfflineSignature(ctx context.Context, req *offlinetokenv1.VerifyOfflineSignatureRequest) (*offlinetokenv1.VerifyOfflineSignatureResponse, error) {
	pubKeyBase64, err := s.store.GetPublicKey(req.IdentityId)
	if err != nil {
		return nil, ErrIdentityNotFound
	}

	sigBytes, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	if len(pubKeyBase64) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size")
	}

	isValid := ed25519.Verify(pubKeyBase64, req.RawSignedPayload, sigBytes)

	return &offlinetokenv1.VerifyOfflineSignatureResponse{
		IsValid: isValid,
	}, nil
}
