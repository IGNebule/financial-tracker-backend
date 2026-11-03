package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const Cost = bcrypt.DefaultCost

var ErrInvalidPassword = errors.New("auth: invalid password")
var ErrPasswordTooLong = errors.New("auth: password exceeds bcrypt's limit")

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), Cost)
	if err != nil {
		if errors.Is(err, bcrypt.ErrPasswordTooLong) {
			return "", ErrPasswordTooLong
		}
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err == nil {
		return nil
	}
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return ErrInvalidPassword
	}
	return fmt.Errorf("check password: %w", err)
}
