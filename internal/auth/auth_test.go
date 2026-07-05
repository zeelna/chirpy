package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// UNIT TESTS:
// Happy path: create → validate → compare IDs.
func TestMakeAndValidateJWT(t *testing.T) {
	userID := uuid.New()
	secret := "my-secret"

	token, err := MakeJWT(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotID, err := ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotID != userID {
		t.Errorf("expected %v, got %v", userID, gotID)
	}
}

// Expired: negative duration → expect error.
func TestExpiredJWTIsRejected(t *testing.T) {
	userID := uuid.New()
	secret := "my-secret"

	token, err := MakeJWT(userID, secret, -time.Hour)
	if err != nil {
		t.Fatalf("unexpected error making token: %v", err)
	}

	_, err = ValidateJWT(token, secret)
	if err == nil {
		t.Errorf("expected an error for expired token, got nil")
	}
}

// Wrong secret: sign with one, validate with another → expect error
func TestWrongSecretIsRejected(t *testing.T) {
	userID := uuid.New()

	token, err := MakeJWT(userID, "correct-secret", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error making token: %v", err)
	}

	_, err = ValidateJWT(token, "wrong-secret")
	if err == nil {
		t.Errorf("expected an error for wrong secret, got nil")
	}
}
