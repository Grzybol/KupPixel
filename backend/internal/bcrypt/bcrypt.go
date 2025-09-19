package bcrypt

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
)

const (
	// DefaultCost mirrors the upstream bcrypt package constant and is kept for API compatibility.
	DefaultCost = 10

	saltSize = 16
)

var (
	// ErrMismatchedHashAndPassword is returned when the provided password does not match the hash.
	ErrMismatchedHashAndPassword = errors.New("crypto/bcrypt: hashedPassword is not the hash of the given password")
	// ErrHashTooShort is returned when the stored hash is shorter than expected.
	ErrHashTooShort = errors.New("crypto/bcrypt: hashedSecret too short to be a bcrypted password")
)

// GenerateFromPassword returns a salted hash of the given password. The implementation is a lightweight
// stand-in that keeps the same API surface as the upstream package while avoiding the need for external
// dependencies in this exercise environment. It is not a drop-in replacement for production usage.
func GenerateFromPassword(password []byte, cost int) ([]byte, error) {
	if password == nil {
		return nil, errors.New("crypto/bcrypt: password must not be nil")
	}
	if cost <= 0 {
		return nil, fmt.Errorf("crypto/bcrypt: invalid cost %d", cost)
	}

	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("crypto/bcrypt: generate salt: %w", err)
	}

	digest := sha256.Sum256(append(salt, password...))
	combined := append(salt, digest[:]...)
	encoded := base64.StdEncoding.EncodeToString(combined)
	return []byte(encoded), nil
}

// CompareHashAndPassword reports whether the given password matches the hashed value.
func CompareHashAndPassword(hashedPassword, password []byte) error {
	if len(hashedPassword) == 0 {
		return ErrHashTooShort
	}
	decoded, err := base64.StdEncoding.DecodeString(string(hashedPassword))
	if err != nil {
		return fmt.Errorf("crypto/bcrypt: decode hash: %w", err)
	}
	if len(decoded) < saltSize {
		return ErrHashTooShort
	}

	salt := decoded[:saltSize]
	storedDigest := decoded[saltSize:]
	digest := sha256.Sum256(append(salt, password...))
	if subtle.ConstantTimeCompare(storedDigest, digest[:]) != 1 {
		return ErrMismatchedHashAndPassword
	}
	return nil
}
