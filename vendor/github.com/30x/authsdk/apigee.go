package authsdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

const (
	// DefaultApigeeHost is Apigee's default api endpoint host
	DefaultApigeeHost = "https://api.enterprise.apigee.com/"

	// EnvVarApigeeHost is the Env Var to set overide default apigee api host
	EnvVarApigeeHost = "AUTH_API_HOST"
)

//ApigeeJWTToken the apigee impelmentation of the JWT token auth sdk
type ApigeeJWTToken struct {
	tokenPayload
	//the cached roles, can be nill if not set
	roles *roles
}

var apiBase string

func init() {
	envVar := os.Getenv(EnvVarApigeeHost)
	if envVar == "" {
		apiBase = DefaultApigeeHost
	} else {
		apiBase = envVar
	}
}

//NewApigeeJWTToken create a new Apigee JWT token.  Return the instance or an error if one cannot be created
func NewApigeeJWTToken(token string) (JWTToken, error) {

	parsedToken, err := parseToken(token)

	if err != nil {
		return nil, err
	}

	return &ApigeeJWTToken{tokenPayload: *parsedToken}, nil
}

//IsOrgAdmin is the current JWTToken subject an organization admin.  If so return true, if not, return false.  An error is returned if the check cannot be performed
func (token *ApigeeJWTToken) IsOrgAdmin(orgName string) (bool, error) {

	//we haven't pulled the roles and cache them, go get them
	if token.roles == nil {

		url := fmt.Sprintf("%sv1/users/%s/userroles", apiBase, token.GetUsername())

		req, err := http.NewRequest("GET", url, nil)
		req.Header.Add("Accept", "application/json")
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.EncodedToken))
		client := &http.Client{}
		response, err := client.Do(req)

		if err != nil {
			return false, err
		}

		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			return false, errors.New("You did not provide a valid apigee JWT token")
		}

		body, err := ioutil.ReadAll(response.Body)

		if err != nil {
			return false, err
		}

		/**
			 {
		  "role": [
		    {
		      "name": "readonlyadmin",
		      "organization": "woolworths"
		    },
		    {
		      "name": "orgadmin",
		      "organization": "michaelarusso"
		    }
		  ]
		}
			  **/

		roles := &roles{}

		err = json.Unmarshal(body, &roles)

		if err != nil {
			return false, err
		}

		token.roles = roles

	}

	return hasOrganizationAdmin(token.roles, orgName), nil
}

//hasOrganization returns true if the organization is found
func hasOrganizationAdmin(roles *roles, organizatioName string) bool {
	for _, role := range roles.Role {
		if role.Name == "orgadmin" && role.Organization == organizatioName {
			return true
		}
	}

	return false
}

type roles struct {
	Role []struct {
		Name         string `json:"name"`         // readonlyadmin
		Organization string `json:"organization"` // woolworths
	} `json:"role"`
}
