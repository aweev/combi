package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/auth"
	"github.com/golang-jwt/jwt/v5"
)

var _ TokenSigner = (*JWTSigner)(nil)

// JWTSigner is a JWT-based implementation of the TokenSigner interface.
type JWTSigner struct {
	accessSecret []byte
	issuer       string
}

// NewJWTSigner creates a new signer for creating and verifying JWTs.
func NewJWTSigner(accessSecret, issuer string) (*JWTSigner, error) {
	if accessSecret == "" {
		return nil, fmt.Errorf("JWT access secret cannot be empty")
	}

	const minSecretLength = 32
	if len(accessSecret) < minSecretLength {
		return nil, fmt.Errorf("JWT secret must be at least %d bytes long", minSecretLength)
	}
	return &JWTSigner{
		accessSecret: []byte(accessSecret),
		issuer:       issuer,
	}, nil
}

// Sign creates a new JWT with the provided claims and signs it.
func (s *JWTSigner) Sign(claims map[string]interface{}, ttl time.Duration) (string, error) {
	now := time.Now()
	fullClaims := jwt.MapClaims(claims)
	fullClaims["iss"] = s.issuer
	fullClaims["iat"] = now.Unix()
	fullClaims["exp"] = now.Add(ttl).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, fullClaims)

	signedString, err := token.SignedString(s.accessSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}
	return signedString, nil
}

// Verify parses a token string, validates its signature, expiry, and issuer.
func (s *JWTSigner) Verify(tokenString string) (map[string]interface{}, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.accessSecret, nil
	}, jwt.WithIssuer(s.issuer),
		jwt.WithLeeway(1*time.Minute),
		jwt.WithValidMethods([]string{"HS256"}))

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, auth.ErrTokenExpired
		}
		if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, auth.ErrTokenSignature
		}
		return nil, auth.ErrInvalidToken
	}

	if !token.Valid {
		return nil, auth.ErrTokenExpired
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, auth.ErrInvalidToken
	}
	return claims, nil
}
