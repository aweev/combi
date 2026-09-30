// Package id generates and validates the platform's prefixed ULID
// identifiers.
//
// Per the foundation services spec, every entity id is a ULID — 26
// Crockford-base32 characters (48-bit millisecond timestamp + 80 random
// bits, lexicographically sortable) — behind a stable lowercase prefix:
//
//	idn_01ARZ3NDEKTSV4RRFFQ69G5FAV   identity      (PrefixIdentity)
//	dev_01HABC…                      device       (PrefixDevice)
//	ten_…                            tenant       (PrefixTenant)
//	ses_…                            session      (PrefixSession)
//	ntf_…                            notification (PrefixNotification)
//	pay_…                            payment      (PrefixPayment)
//
// Prefixes are 2-8 characters of [a-z0-9]. Any prefix may be used with New;
// the constants above are the spec-reserved ones shared across services.
// ULIDs are generated with crypto/rand and need no coordination between
// nodes.
package id

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Spec-reserved prefixes (foundation services spec).
const (
	PrefixIdentity     = "idn"
	PrefixDevice       = "dev"
	PrefixTenant       = "ten"
	PrefixSession      = "ses"
	PrefixNotification = "ntf"
	PrefixPayment      = "pay"
	PrefixEvent        = "evt"
)

// ulidLen is the canonical ULID length: 10 timestamp + 16 randomness chars.
const ulidLen = 26

// crockford is the ULID base32 alphabet (excludes I, L, O, U).
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// Errors returned by Parse and friends; wrap with %w when re-raising.
var (
	ErrInvalidID     = errors.New("id: invalid identifier")
	ErrInvalidPrefix = errors.New("id: invalid prefix")
)

// New returns a fresh prefixed ULID: New(PrefixIdentity) →
// "idn_01ARZ3NDEKTSV4RRFFQ69G5FAV".
func New(prefix string) (string, error) {
	p, err := NormalizePrefix(prefix)
	if err != nil {
		return "", err
	}
	u, err := NewULID()
	if err != nil {
		return "", err
	}
	return p + "_" + u, nil
}

// MustNew is New for wiring-time generation; it panics only on an invalid
// prefix or exhausted entropy, i.e. programmer/system error.
func MustNew(prefix string) string {
	id, err := New(prefix)
	if err != nil {
		panic(err)
	}
	return id
}

// NewULID returns a bare 26-char ULID (no prefix).
func NewULID() (string, error) {
	var rnd [10]byte // 80 random bits
	if _, err := rand.Read(rnd[:]); err != nil {
		return "", fmt.Errorf("id: reading randomness: %w", err)
	}

	var out [ulidLen]byte
	ts := encodeTimestamp(uint64(time.Now().UnixMilli()))
	copy(out[:10], ts[:])
	rndOut := encodeRandom(rnd)
	copy(out[10:], rndOut[:])
	return string(out[:]), nil
}

// Parse splits a prefixed id: Parse("idn_01ARZ…") → ("idn", "01ARZ…", nil).
// Prefix case is normalized to lowercase and the ULID to canonical
// uppercase, so values round-tripping through URLs still validate.
func Parse(id string) (prefix string, ulid string, err error) {
	raw := strings.TrimSpace(id)
	prefix, rest, found := strings.Cut(raw, "_")
	if !found {
		return "", "", fmt.Errorf("%w: %q has no prefix separator", ErrInvalidID, id)
	}
	prefix = strings.ToLower(prefix)
	if _, err := NormalizePrefix(prefix); err != nil {
		return "", "", fmt.Errorf("%w: bad prefix %q", ErrInvalidID, prefix)
	}
	rest = strings.ToUpper(rest)
	if !validULID(rest) {
		return "", "", fmt.Errorf("%w: %q is not a ULID", ErrInvalidID, rest)
	}
	return prefix, rest, nil
}

// Validate reports whether id is a well-formed prefixed ULID.
func Validate(id string) bool {
	_, _, err := Parse(id)
	return err == nil
}

// NormalizePrefix validates a prefix (2-8 chars of [a-z0-9],
// case-insensitive) and returns its canonical lowercase form.
func NormalizePrefix(prefix string) (string, error) {
	p := strings.ToLower(strings.TrimSpace(prefix))
	if n := len(p); n < 2 || n > 8 {
		return "", fmt.Errorf("%w: %q must be 2-8 [a-z0-9] characters", ErrInvalidPrefix, prefix)
	}
	for _, r := range p {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return "", fmt.Errorf("%w: %q must be 2-8 [a-z0-9] characters", ErrInvalidPrefix, prefix)
		}
	}
	return p, nil
}

// ValidatePrefix reports whether prefix would be accepted by New.
func ValidatePrefix(prefix string) bool {
	_, err := NormalizePrefix(prefix)
	return err == nil
}

// HasPrefix reports whether id carries the given prefix.
func HasPrefix(id, prefix string) bool {
	p, err := NormalizePrefix(prefix)
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(id)), p+"_")
}

// ValidateULID checks a bare (unprefixed) 26-char ULID, accepting
// non-canonical casing.
func ValidateULID(u string) bool {
	return validULID(strings.ToUpper(strings.TrimSpace(u)))
}

// Mask shortens an id for logs and UIs where the full value is unnecessary:
// Mask("idn_01ARZ3NDEKTSV4RRFFQ69G5FAV") → "idn_01ARZ3…5FAV".
// Returns "" for invalid input.
func Mask(id string) string {
	prefix, u, err := Parse(id)
	if err != nil {
		return ""
	}
	return prefix + "_" + u[:6] + "…" + u[len(u)-4:]
}

// DID returns the spec's decentralized identifier for a valid entity id:
// DID("idn_01HXYZ…") → "did:afros:idn_01HXYZ…".
func DID(id string) (string, error) {
	if !Validate(id) {
		return "", fmt.Errorf("%w: cannot derive DID from %q", ErrInvalidID, id)
	}
	return "did:afros:" + id, nil
}

// validULID checks length and the Crockford alphabet (strict: the excluded
// homoglyphs I, L, O, U are rejected).
func validULID(u string) bool {
	if len(u) != ulidLen {
		return false
	}
	for i := 0; i < len(u); i++ {
		if !strings.ContainsRune(crockford, rune(u[i])) {
			return false
		}
	}
	return true
}

// encodeTimestamp renders the 48-bit millisecond timestamp as the ULID's
// first 10 chars (MSB-first; the first char only carries 3 bits).
func encodeTimestamp(ms uint64) [10]byte {
	var out [10]byte
	for i := 0; i < 10; i++ {
		shift := uint(45 - 5*i)
		out[i] = crockford[(ms>>shift)&0x1f]
	}
	return out
}

// encodeRandom renders 80 random bits (10 bytes) as the ULID's last 16
// chars, MSB-first.
func encodeRandom(b [10]byte) [16]byte {
	var out [16]byte
	var buf uint32
	bits := uint(0)
	idx := 0
	for _, x := range b {
		buf = buf<<8 | uint32(x)
		bits += 8
		for bits >= 5 && idx < 16 {
			bits -= 5
			out[idx] = crockford[(buf>>bits)&0x1f]
			idx++
		}
	}
	return out
}