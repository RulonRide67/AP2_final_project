package repository

import "errors"

var (
	// ErrNotFound is returned when a requested user does not exist.
	ErrNotFound = errors.New("user not found")
	// ErrEmailExists is returned when email is already registered.
	ErrEmailExists = errors.New("email already exists")
	// ErrUsernameExists is returned when username is already taken.
	ErrUsernameExists = errors.New("username already exists")
)
