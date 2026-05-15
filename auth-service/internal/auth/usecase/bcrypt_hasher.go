package usecase

import "github.com/madiyar/final-project/auth-service/pkg/hash"

// BcryptHasher implements PasswordHasher using bcrypt.
type BcryptHasher struct{}

func (BcryptHasher) Hash(password string) (string, error) {
	return hash.HashPassword(password)
}

func (BcryptHasher) Compare(password, hashed string) error {
	return hash.CheckPassword(password, hashed)
}
