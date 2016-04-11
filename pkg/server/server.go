//TODO: Implement proper HTTP response codes
//TODO: Remove fmt.Fprintf calls

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/30x/enrober/pkg/enrober"
	"github.com/gorilla/mux"
	"k8s.io/kubernetes/pkg/client/restclient"
)

//Server struct
type Server struct {
	router *mux.Router
}

//Global Deployment Manager
var dm *enrober.DeploymentManager

//Init does stuff
func Init(clientConfig restclient.Config) error {
	var err error
	dm, err = enrober.CreateDeploymentManager(clientConfig)
	return err
}

//NewServer creates a new server
func NewServer() (server *Server) {
	router := mux.NewRouter()

	sub := router.PathPrefix("/beeswax/deploy/api/v1").Subrouter()

	//Refactor so that every combination gets it's own function

	sub.Path("/environmentGroups").Methods("GET").HandlerFunc(getEnvironmentGroups)

	sub.Path("/environmentGroups/{environmentGroupID}").Methods("GET").HandlerFunc(getEnvironmentGroup)

	sub.Path("/environmentGroups/{environmentGroupID}/environments").Methods("GET").HandlerFunc(getEnvironments)

	sub.Path("/environmentGroups/{environmentGroupID}/environments").Methods("POST").HandlerFunc(createEnvironment)

	sub.Path("/environmentGroups/{environmentGroupID}/environments/{environment}").Methods("GET").HandlerFunc(getEnvironment)
	sub.Path("/environmentGroups/{environmentGroupID}/environments/{environment}").Methods("DELETE").HandlerFunc(deleteEnvironment)

	sub.Path("/environmentGroups/{environmentGroupID}/environments/{environment}/deployments").Methods("GET").HandlerFunc(getDeployments)

	sub.Path("/environmentGroups/{environmentGroupID}/environments/{environment}/deployments").Methods("POST").HandlerFunc(createDeployment)

	sub.Path("/environmentGroups/{environmentGroupID}/environments/{environment}/deployments/{deployment}").Methods("GET").HandlerFunc(getDeployment)
	sub.Path("/environmentGroups/{environmentGroupID}/environments/{environment}/deployments/{deployment}").Methods("PATCH").HandlerFunc(updateDeployment)
	sub.Path("/environmentGroups/{environmentGroupID}/environments/{environment}/deployments/{deployment}").Methods("DELETE").HandlerFunc(deleteDeployment)

	//TODO: Remove when new route handlers work
	sub.HandleFunc("/{Namespace}", NamespaceHandler).Methods("GET") //TODO: Add support for POST

	sub.HandleFunc("/{Namespace}/{application}", ApplicationHandler).Methods("GET")

	sub.HandleFunc("/{Namespace}/{application}/{revision}", RevisionHandler).Methods("GET", "PUT", "POST")

	server = &Server{
		router: router,
	}
	return server
}

//Start the server
func (server *Server) Start() error {
	return http.ListenAndServe(":9000", server.router)
}

//Route handlers
func getEnvironmentGroups(w http.ResponseWriter, r *http.Request) {

}

func getEnvironmentGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("Got Group ID: %v\n", vars["environmentGroupID"])

}

func getEnvironments(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("Got Group ID: %v\n", vars["environmentGroupID"])

}

func createEnvironment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("Got Group ID: %v\n", vars["environmentGroupID"])

}

func getEnvironment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("Got Group ID: %v\n", vars["environmentGroupID"])
	fmt.Printf("Got Environment: %v\n", vars["environment"])

}

func deleteEnvironment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("Got Group ID: %v\n", vars["environmentGroupID"])
	fmt.Printf("Got Environment: %v\n", vars["environment"])

}

func getDeployments(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("Got Group ID: %v\n", vars["environmentGroupID"])
	fmt.Printf("Got Environment: %v\n", vars["environment"])

}

func createDeployment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("Got Group ID: %v\n", vars["environmentGroupID"])
	fmt.Printf("Got Environment: %v\n", vars["environment"])

}

func getDeployment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("Got Group ID: %v\n", vars["environmentGroupID"])
	fmt.Printf("Got Environment: %v\n", vars["environment"])
	fmt.Printf("Got Deployment: %v\n", vars["deployment"])

}

func updateDeployment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("Got Group ID: %v\n", vars["environmentGroupID"])
	fmt.Printf("Got Environment: %v\n", vars["environment"])
	fmt.Printf("Got Deployment: %v\n", vars["deployment"])

}

func deleteDeployment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("Got Group ID: %v\n", vars["environmentGroupID"])
	fmt.Printf("Got Environment: %v\n", vars["environment"])
	fmt.Printf("Got Deployment: %v\n", vars["deployment"])

}

//NamespaceHandler does stuff
func NamespaceHandler(w http.ResponseWriter, r *http.Request) {

	//get the variable path
	vars := mux.Vars(r)
	fmt.Fprintf(w, "Path: /%s\n", vars["Namespace"])

	//get the http verb
	verb := r.Method
	fmt.Fprintf(w, "HTTP Verb: %s\n", verb)

	imagedeployment := enrober.ImageDeployment{
		Namespace:    vars["Namespace"],
		Application:  "",
		Revision:     "",
		TrafficHosts: []string{},
		PublicPaths:  []string{},
		PathPort:     "",
		PodCount:     0,
	}

	//Case statement based on http verb
	switch verb {

	case "GET":

		ns, err := dm.GetNamespace(imagedeployment)
		if err != nil {
			fmt.Fprintf(w, "Broke at namespace: %v\n", err)
			fmt.Fprintf(w, "In function NamespaceHandler\n")
			return
		}
		fmt.Fprintf(w, "Got Namespace %s\n", ns.GetName())

		depList, err := dm.GetDeploymentList(imagedeployment)
		if err != nil {
			fmt.Fprintf(w, "Broke at deployment: %v\n", err)
			fmt.Fprintf(w, "In function ApplicationHandler\n")
			return
		}
		for _, dep := range depList.Items {
			fmt.Fprintf(w, "Got Deployment %v\n", dep.GetName())
		}
	}
}

//ApplicationHandler does stuff
func ApplicationHandler(w http.ResponseWriter, r *http.Request) {

	//get the variable path
	vars := mux.Vars(r)
	fmt.Fprintf(w, "Path: /%s\n", vars["Namespace"])

	//get the http verb
	verb := r.Method
	fmt.Fprintf(w, "HTTP Verb: %s\n", verb)

	//get namespace matching vars["Namespace"]
	imagedeployment := enrober.ImageDeployment{
		Namespace:    vars["Namespace"],
		Application:  vars["application"],
		Revision:     "",
		TrafficHosts: []string{},
		PublicPaths:  []string{},
		PathPort:     "",
		PodCount:     0,
	}

	//Case statement based on http verb
	switch verb {

	case "GET":
		depList, err := dm.GetDeploymentList(imagedeployment)
		if err != nil {
			fmt.Fprintf(w, "Broke at deployment: %v\n", err)
			fmt.Fprintf(w, "In function ApplicationHandler\n")
			return
		}
		for _, dep := range depList.Items {
			fmt.Fprintf(w, "Got Deployment %v\n", dep.GetName())
		}
	}
}

//RevisionHandler does stuff
func RevisionHandler(w http.ResponseWriter, r *http.Request) {
	//get the variable path
	vars := mux.Vars(r)
	fmt.Fprintf(w, "Path: /%s\n", vars["Namespace"])

	//get the http verb
	verb := r.Method
	fmt.Fprintf(w, "HTTP Verb: %s\n", verb)

	imagedeployment := enrober.ImageDeployment{
		Namespace:    vars["Namespace"],
		Application:  vars["application"],
		Revision:     vars["revision"],
		TrafficHosts: []string{},
		PublicPaths:  []string{},
		PathPort:     "",
		PodCount:     1,
		EnvVars:      map[string]string{},
	}

	//Case statement based on http verb
	switch verb {

	case "GET":
		dep, err := dm.GetDeployment(imagedeployment)
		if err != nil {
			fmt.Fprintf(w, "Broke at deployment: %v\n", err)
			fmt.Fprintf(w, "In function RevisionHandler\n")
			return
		}
		fmt.Fprintf(w, "Got Deployment %v\n", dep.GetName())

	case "PUT", "POST":
		//Check if namespace already exists
		getNs, err := dm.GetNamespace(imagedeployment)

		//TODO: No longer creating namespace here
		//Namespace wasn't found so create it
		if err != nil && getNs.GetName() == "" {
			ns, err := dm.CreateNamespace(imagedeployment)
			if err != nil {
				fmt.Fprintf(w, "Broke at namespace: %v\n", err)
				fmt.Fprintf(w, "In function RevisionHandler\n")
				return
			}
			fmt.Fprintf(w, "Put Namespace %s\n", ns.GetName())
		}

		//TODO: Possibly put types somewhere else
		//Where I'm putting the JSON body
		type deploymentVariables struct {
			PodCount        int               `json:"podCount"`
			Image           string            `json:"image"`
			ImagePullSecret string            `json:"imagePullSecret"`
			TrafficHosts    []string          `json:"trafficHosts"`
			PublicPaths     []string          `json:"publicPaths"`
			PathPort        int               `json:"pathPort"`
			EnvVars         map[string]string `json:"envVars"`
		}

		//TODO: Probably a horrifying amount of input validation
		decoder := json.NewDecoder(r.Body)
		var t deploymentVariables
		err = decoder.Decode(&t)
		if err != nil {
			fmt.Printf("Error decoding JSON")
		}

		//TrafficHosts can't be empty so fail if it is
		//TODO: Should do this checking before we create the namespace
		if t.TrafficHosts[0] == "" {
			fmt.Fprintf(w, "Traffic Hosts cannot be empty")
			return
		}

		//TODO: This feels repetitive
		imagedeployment.PodCount = t.PodCount
		imagedeployment.Image = t.Image
		imagedeployment.ImagePullSecret = t.ImagePullSecret
		imagedeployment.TrafficHosts = t.TrafficHosts
		imagedeployment.PublicPaths = t.PublicPaths
		imagedeployment.PathPort = strconv.Itoa(t.PathPort)
		imagedeployment.EnvVars = t.EnvVars

		//Check if deployment already exists
		getDep, err := dm.GetDeployment(imagedeployment)

		//Deployment wasn't found so create it
		if err != nil && getDep.GetName() == "" {

			//TODO: Should create a secret here?

			dep, err := dm.CreateDeployment(imagedeployment)
			if err != nil {
				fmt.Fprintf(w, "Broke at deployment: %v\n", err)
				fmt.Fprintf(w, "In function RevisionHandler\n")
				return
			}
			fmt.Fprintf(w, "New Deployment %s\n", dep.GetName())
		} else {
			//Deployment was found so modify it
			dep, err := dm.UpdateDeployment(imagedeployment)
			if err != nil {
				fmt.Fprintf(w, "Broke at deployment: %v\n", err)
				fmt.Fprintf(w, "In function RevisionHandler\n")
				return
			}
			fmt.Fprintf(w, "Modified Deployment %s\n", dep.GetName())
		}
	}
}
