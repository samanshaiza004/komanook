package auth

import (
	"errors"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const Cost = 12

func Email(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	a, err := mail.ParseAddress(value)
	if err != nil || a.Address != value || len(value) > 254 {
		return "", errors.New("enter a valid email address")
	}
	return value, nil
}

func Hash(password []byte) ([]byte, error) {
	if len(password) < 12 || len(password) > 72 {
		return nil, errors.New("password must contain 12–72 bytes")
	}
	return bcrypt.GenerateFromPassword(password, Cost)
}

// Unknown accounts use a cost-matched hash to avoid a fast account-existence path.
// This is a bcrypt fixture, never an account password or authentication shortcut.
var dummyHash = []byte("$2a$12$R9h/cIPz0gi.URNNX3kh2OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW")

func Verify(hash, password []byte) bool {
	if len(hash) == 0 {
		hash = dummyHash
	}
	return bcrypt.CompareHashAndPassword(hash, password) == nil
}
