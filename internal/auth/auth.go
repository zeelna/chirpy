package auth

import (
	"fmt"
	"strings"

	"github.com/alexedwards/argon2id"
)

// Must download library -> 'go get github.com/alexedwards/argon2id'

// HashPassword Hash the password using the argon2id.CreateHash function.
func HashPassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return "", fmt.Errorf("Password cannot be empty")
	}
	if len(password) < 5 {
		return "", fmt.Errorf("Password must be longer than 5 characters")
	}
	// happy path
	hashedPw, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return "", err
	}
	return hashedPw, nil
}

// CheckPasswordHash function to compare the password that the user entered in the HTTP request with the password that is stored in the database.
func CheckPasswordHash(password, hash string) (bool, error) {
	boolValue, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, err
	}
	return boolValue, nil
}
