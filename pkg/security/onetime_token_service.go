// internal/platform/security/one_time_token_service.go
package security

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

type secureTokenGenerator struct {
	randomSvc RandomService
}

var _ TokenGenerator = (*secureTokenGenerator)(nil)

func NewTokenGenerator(randomSvc RandomService) TokenGenerator {
	return &secureTokenGenerator{
		randomSvc: randomSvc,
	}
}

func (s *secureTokenGenerator) Generate() (rawToken string, hashedToken string, err error) {
	rawToken, err = s.randomSvc.GenerateString(32)
	if err != nil {
		return "", "", err
	}

	hashedToken = s.Hash(rawToken)

	return rawToken, hashedToken, nil
}

func (s *secureTokenGenerator) Hash(rawToken string) string {
	hasher := sha256.New()
	hasher.Write([]byte(rawToken))
	hashBytes := hasher.Sum(nil)
	return hex.EncodeToString(hashBytes)
}

func (s *secureTokenGenerator) Compare(hashedToken string, rawToken string) bool {
	newHash := s.Hash(rawToken)
	// Use constant time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(hashedToken), []byte(newHash)) == 1
}
