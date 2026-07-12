package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	// he RegisteredClaims struct doesn't store timestamps as plain time.Time values.
	//The library wraps them in its own type so it can handle JSON serialization correctly.
	//That type is *jwt.NumericDate, and the library gives you a helper to build one:
	nowTime := time.Now().UTC()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  jwt.NewNumericDate(nowTime),
		ExpiresAt: jwt.NewNumericDate(nowTime.Add(expiresIn)),
		Subject:   fmt.Sprintf("%v", userID),
	})

	jwtSigned, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}
	return jwtSigned, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	keyFunc := func(t *jwt.Token) (interface{}, error) {
		return []byte(tokenSecret), nil
	}

	// Pass empty 'Claims' struct that will be filled with fn-call 'jwt.ParseWithClaims()'
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, keyFunc)
	if err != nil {
		return uuid.Nil, err
	}

	id, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(id)
}

// When the user wants to make a request to the API, they send the token along with the request in the HTTP headers.
// The server can then verify that the token is valid, which means the user is who they say they are.
func GetBearerToken(headers http.Header) (string, error) {
	bearer := headers.Get("Authorization")
	isEmptyBearer := len(strings.Trim(bearer, "")) == 0
	if isEmptyBearer {
		return "", fmt.Errorf("Authorization incomplete : 'Bearer' token not found")
	}
	tokenString := strings.TrimPrefix(bearer, "Bearer ") // must include whitespace 'Bearer ' !
	return tokenString, nil
}

// Option 1: Same as GetBearerToken, but trimming 'ApiKey'
func GetAPIKey(headers http.Header) (string, error) {
	bearer := headers.Get("Authorization")
	isEmptyBearer := len(strings.Trim(bearer, "")) == 0
	if isEmptyBearer {
		return "", fmt.Errorf("Authorization incomplete : 'ApiKey' token not found")
	}
	tokenString := strings.TrimPrefix(bearer, "ApiKey ") // must include whitespace 'ApiKey ' !
	return tokenString, nil
}

// Option 2: Same as 'GetBearerToken' but with helper function _AuthorizationParser()
func GetAPIKey2(headers http.Header) (string, error) {
	bearer := headers.Get("Authorization")

	tokenString, err := _AuthorizationParser(bearer, authParse{
		TrimPrefix: "ApiKey ", // must include whitespace!
		ErrorMsg:   "Authorization incomplete : 'ApiKey' token not found",
	})
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

type authParse struct {
	TrimPrefix string
	ErrorMsg   string
}

func _AuthorizationParser(authHeader string, auth authParse) (string, error) {
	isEmpty := len(strings.Trim(authHeader, "")) == 0
	if isEmpty {
		return "", fmt.Errorf(auth.ErrorMsg)
	}
	tokenString := strings.TrimPrefix(authHeader, auth.TrimPrefix)
	return tokenString, nil
}

// Option 3:
func GetAPIKey3(headers http.Header) (string, error) {
	return parseAuthToken(
		headers.Get("Authorization"),
		"ApiKey",
		"Authorization incomplete : 'ApiKey' token not found")
}

func parseAuthToken(authHeader, prefix, errMsg string) (string, error) {
	if strings.TrimSpace(authHeader) == "" {
		return "", fmt.Errorf(errMsg)
	}
	return strings.TrimPrefix(authHeader, prefix), nil
}
