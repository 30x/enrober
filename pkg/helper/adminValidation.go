package helper

import (
	"fmt"
	"net/http"

	"github.com/30x/authsdk"
	"strings"
)

//ValidAdmin checks if the user is an org admin for the organization they are making a request in
func ValidAdmin(organization string, w http.ResponseWriter, r *http.Request) bool {
	token, err := authsdk.NewJWTTokenFromRequest(r)
	if err != nil {
		fmt.Printf("Error getting JWT Token: %v\n", err)
		http.Error(w, "Invalid Token", http.StatusUnauthorized) //401
		return false
	}
	isAdmin, err := token.IsOrgAdmin(organization)
	if err != nil {
		fmt.Printf("Error checking caller is an Org Admin: %v\n", err) //401
		http.Error(w, "", http.StatusUnauthorized)
		return false
	}
	if !isAdmin {
		//Throwing a 403
		fmt.Printf("Caller isn't an Org Admin\n")
		http.Error(w, "You aren't an Org Admin", http.StatusForbidden) //403
		return false
	}
	return true
}

//AdminMiddleware is a middleware wrapper for the ValidAdmin function
func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//Can't get pathvars from inside middleware with mux.vars

		tempString := strings.Split(r.URL.String(), "/")
		secondSplit := strings.Split(tempString[2], ":")

		if !ValidAdmin(secondSplit[0], w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}
