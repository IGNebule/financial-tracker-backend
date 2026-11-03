package auth

import "testing"

func BenchmarkHashPassword(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := HashPassword("benchmark-password"); err != nil {
			b.Fatalf("HashPassword: %v", err)
		}
	}
}
