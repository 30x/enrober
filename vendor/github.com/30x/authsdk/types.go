package authsdk

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

//tokenPayload The struct representing the token
type tokenPayload struct {
	//the original encoded JWT token
	EncodedToken string
	Subject      string `json:"sub"`
	Username     string `json:"user_name"`
	Email        string `json:"email"`
}

//parseToken Parse the token and return the struct
func parseToken(token string) (*tokenPayload, error) {
	parts := strings.Split(token, ".")

	length := len(parts)

	if length != 3 {
		return nil, fmt.Errorf("Expected JWT token to contain a header, a payload, and a signature.  Only received %d parts", length)
	}

	partToDecode := parts[1]

	// fmt.Printf("Decoding base64 value of %s\n", partToDecode)

	decodedPayload, err := base64.RawURLEncoding.DecodeString(partToDecode)

	if err != nil {
		return nil, fmt.Errorf("Unable to decode payload.  Decode error: %s", err)
	}

	// fmt.Printf("Decoded payload is %s\n", decodedPayload)

	var tokenPayload tokenPayload

	err = json.Unmarshal([]byte(decodedPayload), &tokenPayload)

	if err != nil {
		return nil, err
	}

	tokenPayload.EncodedToken = token

	return &tokenPayload, nil
}

//basic setters/getters for composition

//GetSubject get the subject claim from the token
func (token *tokenPayload) GetSubject() string {
	return token.Subject
}

//GetUsername return the username if possible for the subject
func (token *tokenPayload) GetUsername() string {
	return token.Username
}

//GetEmail return the username if possible for the subject
func (token *tokenPayload) GetEmail() string {
	return token.Email
}
