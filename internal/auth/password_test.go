package auth

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashAndCheck(t *testing.T) {
	const password = "correct horse battery staple"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == password {
		t.Fatalf("hash equals plaintext - this should be impossible")
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Fatalf("hash does not look like a bcrypt hash: %q", hash)
	}
	if err := CheckPassword(hash, password); err != nil {
		t.Fatalf("CheckPassword(hash, correct) = %v, want nil", err)
	}
}

func TestCheckWrongPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	err = CheckPassword(hash, "wrong password")
	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("CheckPassword with wrong password = %v, want ErrInvalidPassword", err)
	}
}

func TestHashDifferentHashesForSamePassword(t *testing.T) {
	hash1, err := HashPassword("same password")
	if err != nil {
		t.Fatalf("HashPassword #1: %v", err)
	}
	hash2, err := HashPassword("same password")
	if err != nil {
		t.Fatalf("HashPassword #2: %v", err)
	}

	if hash1 == hash2 {
		t.Fatal("two hashes of the same password are identical - salt not random?")
	}

	if err := CheckPassword(hash1, "same password"); err != nil {
		t.Fatalf("CheckPassword(hash1): %v", err)
	}
	if err := CheckPassword(hash2, "same password"); err != nil {
		t.Fatalf("CheckPassword(hash2): %v", err)
	}
}

func TestHashUsesConfiguredCost(t *testing.T) {
	hash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	gotCost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Fatalf("bcrypt.Cost: %v", err)
	}
	if gotCost != Cost {
		t.Fatalf("hash cost = %d, want %d", gotCost, Cost)
	}
}

func TestHashTooLongPassword(t *testing.T) {
	tooLong := strings.Repeat("a", 73)

	_, err := HashPassword(tooLong)
	if !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("HashPassword(73-byte password) = %v, want ErrPasswordTooLong", err)
	}

	exactly := strings.Repeat("a", 72)
	if _, err := HashPassword(exactly); err != nil {
		t.Fatalf("HashPassword(72-byte password) = %v, want nil", err)
	}
}

func TestCheckCorruptedHash(t *testing.T) {
	err := CheckPassword("this-is-not-a-bcrypt-hash", "anything")
	if err == nil {
		t.Fatal("CheckPassword with corrupted hash = nil, want an error")
	}
	if errors.Is(err, ErrInvalidPassword) {
		t.Fatal("CheckPassword with corrupted hash returned ErrInvalidPassword; should be a wrapped malformed-hash error")
	}
}
