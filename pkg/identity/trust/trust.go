package trust

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

type ClaimType string

type TrustScore int

const (
	ClaimID      ClaimType = "identity"
	ClaimAddress ClaimType = "address"
	ClaimDevice  ClaimType = "device"
)

var DefaultVouchTimeout = 24 * time.Hour

type Vouch struct {
	Issuer    string    `json:"issuer"`
	Subject   string    `json:"subject"`
	Claim     ClaimType `json:"claim"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Signature string    `json:"signature"`
}

func NewVouch(issuer, subject string, claim ClaimType, privateKey ed25519.PrivateKey, duration time.Duration) (*Vouch, error) {
	if duration <= 0 {
		duration = DefaultVouchTimeout
	}
	v := &Vouch{
		Issuer:    issuer,
		Subject:   subject,
		Claim:     claim,
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(duration),
	}
	payload, err := json.Marshal(v.payload())
	if err != nil {
		return nil, err
	}
	v.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
	return v, nil
}

func (v *Vouch) payload() interface{} {
	return map[string]interface{}{
		"issuer":     v.Issuer,
		"subject":    v.Subject,
		"claim":      v.Claim,
		"issued_at":  v.IssuedAt.Unix(),
		"expires_at": v.ExpiresAt.Unix(),
	}
}

func (v *Vouch) Verify(publicKey ed25519.PublicKey) error {
	if time.Now().UTC().After(v.ExpiresAt) {
		return fmt.Errorf("trust: vouch expired")
	}
	payload, err := json.Marshal(v.payload())
	if err != nil {
		return err
	}
	sig, err := base64.RawStdEncoding.DecodeString(v.Signature)
	if err != nil {
		return fmt.Errorf("trust: invalid signature encoding")
	}
	if !ed25519.Verify(publicKey, payload, sig) {
		return fmt.Errorf("trust: signature mismatch")
	}
	return nil
}

func NewKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	return pub, priv, err
}

type TrustNode struct {
	NodeID string
	Score  TrustScore
}

func (n TrustNode) AddWeight(weight TrustScore) TrustScore {
	combined := int(n.Score) + int(weight)
	if combined > 100 {
		return 100
	}
	return TrustScore(combined)
}

func (n TrustNode) IsTrusted(min TrustScore) bool {
	return n.Score >= min
}

func EncodeToQRCode(data interface{}) (string, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(raw), nil
}

func DecodeFromQRCode(encoded string, dst interface{}) error {
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}
