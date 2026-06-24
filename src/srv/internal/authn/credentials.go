package authn

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	passwordAlgorithm  = "pbkdf2_sha256"
	passwordIterations = 210000
	passwordSaltBytes  = 16
	passwordKeyBytes   = 32
)

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password is required")
	}
	salt, err := randomBytes(passwordSaltBytes)
	if err != nil {
		return "", err
	}
	key := pbkdf2.Key([]byte(password), salt, passwordIterations, passwordKeyBytes, sha256.New)
	return fmt.Sprintf("%s$%d$%s$%s",
		passwordAlgorithm,
		passwordIterations,
		base64.RawURLEncoding.EncodeToString(salt),
		base64.RawURLEncoding.EncodeToString(key),
	), nil
}

func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != passwordAlgorithm {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}
	salt, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil || len(expected) == 0 {
		return false
	}
	actual := pbkdf2.Key([]byte(password), salt, iterations, len(expected), sha256.New)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func NewSessionToken() (string, error) {
	raw, err := randomBytes(32)
	if err != nil {
		return "", err
	}
	return "ps_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomBytes(size int) ([]byte, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}
