package auth

import (
	"errors"
	"testing"
)

func TestGenerateTokenUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		tok, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}
		if len(tok) != SessionTokenBytes*2 {
			t.Fatalf("token length = %d, want %d", len(tok), SessionTokenBytes*2)
		}
		if seen[tok] {
			t.Fatalf("duplicate token on iteration %d — randomness is broken", i)
		}
		seen[tok] = true
	}
}

func TestHashToken(t *testing.T) {
	tok := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	h1, err := HashToken(tok)
	if err != nil {
		t.Fatalf("HashToken: %v", err)
	}
	h2, err := HashToken(tok)
	if err != nil {
		t.Fatalf("HashToken: %v", err)
	}
	if h1 != h2 {
		t.Fatal("HashToken is not deterministic — lookup by equality will not work")
	}
}

func TestHashTokenRejectsWrongLength(t *testing.T) {
	cases := []string{
		"",
		"short",
		"0123456789abcdef0123456789abcdef", // 32 chars (16 bytes)
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef00", // 66 chars, too long
	}
	for _, c := range cases {
		if _, err := HashToken(c); !errors.Is(err, ErrInvalidTokenFormat) {
			t.Fatalf("HashToken(%q) = %v, want ErrInvalidTokenFormat", c, err)
		}
	}
}
