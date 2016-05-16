//From Elithrar on StackOverflow

package helper

import (
	"crypto/rand"
	"encoding/base64"
)

// GenerateRandomBytes returns securely generated random bytes.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func GenerateRandomBytes(length int) ([]byte, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// GenerateRandomString returns a URL-safe, base64 encoded
// securely generated random string.
func GenerateRandomString(length int) (string, error) {
	b, err := GenerateRandomBytes(length)
	return base64.URLEncoding.EncodeToString(b), err
}
