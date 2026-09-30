package security

import (
	"crypto/rand"
	"encoding/base64"
)

// cryptoRandomService implements the RandomService interface using Go's crypto/rand package.
type cryptoRandomService struct{}

// Statically verify that it implements the interface.
var _ RandomService = (*cryptoRandomService)(nil)

// NewRandomService creates a new instance of the random service.
func NewRandomService() RandomService {
	return &cryptoRandomService{}
}

func (s *cryptoRandomService) GenerateBytes(length int) ([]byte, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err // rand.Read should not fail on modern OSes
	}
	return b, nil
}

func (s *cryptoRandomService) GenerateString(byteLength int) (string, error) {
	randomBytes, err := s.GenerateBytes(byteLength)
	if err != nil {
		return "", err
	}
	// Use URL-safe encoding to avoid characters like '+' or '/'
	return base64.URLEncoding.EncodeToString(randomBytes), nil
}
