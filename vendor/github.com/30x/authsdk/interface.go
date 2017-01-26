package authsdk

//JWTToken the interface for permissions client
type JWTToken interface {

	//GetSubject get the subject claim from the token
	GetSubject() string

	//GetUsername return the username if possible for the subject
	GetUsername() string

	//GetEmail return the username if possible for the subject
	GetEmail() string

	//IsOrgAdmin is the current JWTToken subject an organization admin.  If so return true, if not, return false.  An error is returned if the check cannot be performed
	IsOrgAdmin(orgName string) (bool, error)
}
