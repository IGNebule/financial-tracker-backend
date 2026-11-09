package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

const SessionTokenBytes = 32

var ErrInvalidTokenFormat = errors.New("auth: invalid session token format")

func GenerateToken() (string, error) {
	buf := make([]byte, SessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func HashToken(token string) ([32]byte, error) {
	if len(token) != SessionTokenBytes*2 {
		return [32]byte{}, ErrInvalidTokenFormat
	}
	return sha256.Sum256([]byte(token)), nil
}
