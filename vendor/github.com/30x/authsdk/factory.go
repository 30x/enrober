package authsdk

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

//NewJWTToken creates a new JWT token based on the environment configuration
func NewJWTToken(token string) (JWTToken, error) {

	impl := os.Getenv("JWTTOKENIMPL")

	switch impl {
	case "test":
		return NewTestToken(token)
	default:
		return NewApigeeJWTToken(token)
	}

}

//NewJWTTokenFromRequest create and return our JWTToken impl from the http request
func NewJWTTokenFromRequest(r *http.Request) (JWTToken, error) {
	header := r.Header.Get("Authorization")

	if header == "" {
		return nil, fmt.Errorf("No 'Authorization' header was found in the request")
	}

	sections := strings.Fields(header)

	if len(sections) != 2 {
		return nil, fmt.Errorf("Expected the authorization header to have the format of 'Bearer JWTTOKEN'")
	}

	//if we get here, we have a Bearer token

	return NewJWTToken(sections[1])
}
