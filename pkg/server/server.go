package server

import (
	"net/http"
	"os"
	"regexp"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"
)

// TODO:
// Need to rename these vars to highlight that they are for
// routing and not for environment variable storage

const (
	apigeeKVMName   = "shipyard-routing"
	apigeeKVMPKName = "x-routing-api-key"
)

//Global Vars
var (
	//Kubernetes Client
	client k8sClient.Client

	//Global Regex
	validIPAddressRegex = regexp.MustCompile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
	validHostnameRegex  = regexp.MustCompile(`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`)

	//Env Name Regex
	envNameRegex = regexp.MustCompile(`\w+\:\w+`)

	//Privileged container flag
	allowPrivilegedContainers bool

	//Namespace Isolation
	isolateNamespace bool

	//Apigee KVM check
	apigeeKVM bool
)

//NOTE: routing secret should probably be a configurable name

//NewServer creates a new server
func NewServer() (server *Server) {
	router := mux.NewRouter()

	router.Path("/environments/{org}:{env}").Methods("GET").HandlerFunc(getEnvironment)
	router.Path("/environments/{org}:{env}").Methods("PATCH").HandlerFunc(patchEnvironment)
	router.Path("/environments/{org}:{env}/deployments").Methods("POST").HandlerFunc(createDeployment)
	router.Path("/environments/{org}:{env}/deployments").Methods("GET").HandlerFunc(getDeployments)
	router.Path("/environments/{org}:{env}/deployments/{deployment}").Methods("GET").HandlerFunc(getDeployment)
	router.Path("/environments/{org}:{env}/deployments/{deployment}").Methods("PATCH").HandlerFunc(updateDeployment)
	router.Path("/environments/{org}:{env}/deployments/{deployment}").Methods("DELETE").HandlerFunc(deleteDeployment)
	router.Path("/environments/{org}:{env}/deployments/{deployment}/logs").Methods("GET").HandlerFunc(getDeploymentLogs)

	// Health Check
	router.Path("/environments/status/").Methods("GET").HandlerFunc(getStatus)
	router.Path("/environments/status").Methods("GET").HandlerFunc(getStatus)

	loggedRouter := handlers.CombinedLoggingHandler(os.Stdout, router)

	server = &Server{
		Router: loggedRouter,
	}
	return server
}

var apigeeAPIHost string

func init() {
	envVar := os.Getenv("AUTH_API_HOST")

	if envVar == "" {
		apigeeAPIHost = "https://api.enterprise.apigee.com/"
	} else {
		apigeeAPIHost = envVar
	}
}

//Start the server
func (server *Server) Start() error {
	return http.ListenAndServe(":9000", server.Router)
}
