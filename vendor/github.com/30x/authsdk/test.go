package authsdk

//TestJWTToken the apigee impelmentation of the JWT token auth sdk
type TestJWTToken struct {
	tokenPayload
}

//NewTestToken create a new Apigee JWT token.  Return the instance or an error if one cannot be created
func NewTestToken(token string) (JWTToken, error) {
	parsedToken, err := parseToken(token)

	if err != nil {
		return nil, err
	}

	return &TestJWTToken{tokenPayload: *parsedToken}, nil
}

//IsOrgAdmin is the current JWTToken subject an organization admin.  If so return true, if not, return false.  An error is returned if the check cannot be performed
func (token *TestJWTToken) IsOrgAdmin(orgName string) (bool, error) {
	//always return true
	return true, nil
}
