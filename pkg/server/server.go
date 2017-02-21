package server

import (
	"fmt"
	"net/http"
	"os"
	"regexp"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/30x/enrober/pkg/helper"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

//Global Vars
var (

	//Env Name Regex
	envNameRegex = regexp.MustCompile(`\w+\:\w+`)

	//Privileged container flag
	allowPrivilegedContainers bool

	//Namespace Isolation
	isolateNamespace bool

	//Apigee KVM check
	apigeeKVM bool

	apigeeAPIHost string

	clientset kubernetes.Clientset
)

func init() {
	//Get environment variables
	authEnvVar := os.Getenv("AUTH_API_HOST")
	envState := os.Getenv("DEPLOY_STATE")

	// Initialize State
	state := StateLocal

	switch envState {
	case "PROD":
		fmt.Printf("DEPLOY_STATE set to PROD\n")
		state = StateCluster

		if os.Getenv("ISOLATE_NAMESPACE") == "true" {
			isolateNamespace = true
		}

		//Set privileged container flag
		if os.Getenv("ALLOW_PRIV_CONTAINERS") == "true" {
			allowPrivilegedContainers = true
		}

		//Set apigeeKVM flag
		if os.Getenv("APIGEE_KVM") == "true" {
			apigeeKVM = true
		}

	case "DEV_CONTAINER":
		fmt.Printf("DEPLOY_STATE set to DEV_CONTAINER\n")
		state = StateCluster
	case "DEV":
		fmt.Printf("DEPLOY_STATE set to DEV\n")
		state = StateLocal
	default:
		fmt.Printf("Defaulting to Local Dev Setup\n")

	}

	if authEnvVar == "" {
		apigeeAPIHost = "https://api.enterprise.apigee.com/"
	} else {
		apigeeAPIHost = authEnvVar
	}

	//In Cluster Config
	if state == StateCluster {
		tmpConfig, err := rest.InClusterConfig()
		if err != nil {
			panic(err)
		}
		//Create the clientset
		tempClientset, err := kubernetes.NewForConfig(tmpConfig)
		if err != nil {
			panic(err)
		}
		clientset = *tempClientset

	} else {
		//Local Config

		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		tmpConfig, err := config.ClientConfig()
		if err != nil {
			fmt.Printf("Error on init, creating Client Config: %v\n", err)
			panic(err)
		}
		//Create the clientset
		tempClientset, err := kubernetes.NewForConfig(tmpConfig)
		if err != nil {
			fmt.Printf("Error on init, creating Clientset: %v\n", err)
			panic(err)
		}
		clientset = *tempClientset

	}

}

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

	var adminRouter http.Handler
	var finalRouter http.Handler

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		adminRouter = helper.AdminMiddleware(router)
		finalRouter = handlers.CombinedLoggingHandler(os.Stdout, adminRouter)
	} else {
		finalRouter = handlers.CombinedLoggingHandler(os.Stdout, router)
	}

	server = &Server{
		Router: finalRouter,
	}
	return server
}

//Start the server
func (server *Server) Start() error {
	return http.ListenAndServe(":9000", server.Router)
}
