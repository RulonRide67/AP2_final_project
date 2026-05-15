package hash_test

import (
	"testing"

	"github.com/madiyar/final-project/auth-service/pkg/hash"
)

func TestHashPasswordAndCheckPassword(t *testing.T) {
	t.Parallel()

	hashed, err := hash.HashPassword("secret-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if err := hash.CheckPassword("secret-password", hashed); err != nil {
		t.Fatalf("CheckPassword valid: %v", err)
	}

	if err := hash.CheckPassword("wrong-password", hashed); err == nil {
		t.Fatal("expected mismatch error")
	}
}
