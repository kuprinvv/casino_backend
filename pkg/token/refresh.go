package token

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
)

func GenerateRefreshToken() (string, error) {
	b := make([]byte, 32) // 256 бит
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func HashRefreshToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func VerifyRefreshToken(token string, hash string) bool {
	h := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(
		[]byte(hex.EncodeToString(h[:])),
		[]byte(hash),
	) == 1
}
