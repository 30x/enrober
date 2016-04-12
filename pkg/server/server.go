//TODO: Implement proper HTTP response codes
//TODO: Remove fmt.Fprintf calls

package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/labels"

	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"
)

//Server struct
type Server struct {
	router *mux.Router
}

//Global Kubernetes Client
var client k8sClient.Client

//Init does stuff
func Init(clientConfig restclient.Config) error {
	var err error
	var tempClient *k8sClient.Client

	//In Cluster Config
	if clientConfig.Host == "" {
		tempConfig, err := restclient.InClusterConfig()
		if err != nil {
			return err
		}
		tempClient, err = k8sClient.New(tempConfig)

		client = *tempClient

		//Local Config
	} else {
		tempClient, err = k8sClient.New(&clientConfig)
		if err != nil {
			return err
		}
		client = *tempClient
	}

	return nil
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
	/*
		sub.HandleFunc("/{Namespace}", NamespaceHandler).Methods("GET") //TODO: Add support for POST

		sub.HandleFunc("/{Namespace}/{application}", ApplicationHandler).Methods("GET")

		sub.HandleFunc("/{Namespace}/{application}/{revision}", RevisionHandler).Methods("GET", "PUT", "POST")
	*/

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

//getEnvironmentGrouos returns a list of all Environment Groups
func getEnvironmentGroups(w http.ResponseWriter, r *http.Request) {

}

//getEnvironmentGroup returns an Environment Group matching the given environmentGroupID
func getEnvironmentGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("Got Group ID: %v\n", vars["environmentGroupID"])

}

//getEnvironments returns a list of all environments under a specific environmentGroupID
func getEnvironments(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)
	fmt.Printf("GET request on Group ID: %v\n", pathVars["environmentGroupID"])

	selector, err := labels.Parse("Group=" + pathVars["environmentGroupID"])
	nsList, err := client.Namespaces().List(api.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		fmt.Printf("Error in getEnvironments: %v\n", err)
		fmt.Fprintf(w, "%v\n", err)
		return
	}
	for _, value := range nsList.Items {
		fmt.Fprintf(w, "Got Namespace: %v\n", value.GetName())
	}
}

//createEnvironment creates a kubernetes namespace matching the given environmentGroupID and environmentName
func createEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)
	fmt.Printf("POST request on Group ID: %v\n", pathVars["environmentGroupID"])

	//Struct to put JSON into
	type environmentPost struct {
		EnvironmentName string `json:"environmentName"`
	}
	//Decode passed JSON body
	decoder := json.NewDecoder(r.Body)
	var tempJSON environmentPost
	err := decoder.Decode(&tempJSON)
	if err != nil {
		fmt.Printf("Error decoding JSON Body: %v\n", err)
		return
	}

	nsObject := &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name: pathVars["environmentGroupID"] + "-" + tempJSON.EnvironmentName,
			Labels: map[string]string{
				"Group": pathVars["environmentGroupID"],
			},
		},
	}

	createdNs, err := client.Namespaces().Create(nsObject)
	if err != nil {
		fmt.Printf("Error in createEnvironment: %v\n", err)
		fmt.Fprintf(w, "%v\n", err)
		return
	}
	fmt.Fprintf(w, "Created NS: %v\n", createdNs.GetName())
	fmt.Printf("Created Namespace: %v\n", createdNs.GetName())
}

//getEnvironment returns a kubernetes namespace matching the given environmentGroupID and environmentName
func getEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	labelSelector, err := labels.Parse("Group=" + pathVars["environmentGroupID"])
	if err != nil {
		fmt.Printf("Error in getEnvironment: %v\n", err)
	}
	nsList, err := client.Namespaces().List(api.ListOptions{
		LabelSelector: labelSelector,
	})
	for _, value := range nsList.Items {
		if value.GetName() == pathVars["environment"] {
			fmt.Fprintf(w, "Got Namespace: %v\n", value.GetName())
		}
	}

	fmt.Printf("GET request on Group ID: %v and Environment ID: %v\n", pathVars["environmentGroupID"], pathVars["environment"])

}

//deleteEnvironment deletes a kubernetes namespace matching the given environmentGroupID and environmentName
func deleteEnvironment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("DELETE request on Group ID: %v and Environment ID: %v\n", vars["environmentGroupID"], vars["environment"])

	//TODO: Filter based on environmentGroupID

	err := client.Namespaces().Delete(vars["environmentGroupID"] + "-" + vars["environment"])
	if err != nil {
		fmt.Printf("Error in deleteEnvironment: %v\n", err)
		return
	}
	fmt.Fprintf(w, "Deleted Namespace: %v\n", vars["environment"])

}

//getDeployments returns a list of all deployments matching the given environmentGroupID and environmentName
func getDeployments(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("Got Group ID: %v\n", vars["environmentGroupID"])
	fmt.Printf("Got Environment: %v\n", vars["environment"])

}

//createDeployment creates a deployment in the given environment(namespace) with the given environmentGroupID based on the given deploymentBody
func createDeployment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("Got Group ID: %v\n", vars["environmentGroupID"])
	fmt.Printf("Got Environment: %v\n", vars["environment"])

}

//getDeployment returns a deployment matching the given environmentGroupID, environmentName, and deploymentName
func getDeployment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("Got Group ID: %v\n", vars["environmentGroupID"])
	fmt.Printf("Got Environment: %v\n", vars["environment"])
	fmt.Printf("Got Deployment: %v\n", vars["deployment"])

}

//updateDeployment updates a deployment matching the given environmentGroupID, environmentName, and deploymentName
func updateDeployment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("Got Group ID: %v\n", vars["environmentGroupID"])
	fmt.Printf("Got Environment: %v\n", vars["environment"])
	fmt.Printf("Got Deployment: %v\n", vars["deployment"])

}

//deleteDeployment deletes a deployment matching the given environmentGroupID, environmentName, and deploymentName
func deleteDeployment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Printf("Got Group ID: %v\n", vars["environmentGroupID"])
	fmt.Printf("Got Environment: %v\n", vars["environment"])
	fmt.Printf("Got Deployment: %v\n", vars["deployment"])

}

//TODO: Everything below this should go away

/*
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
*/
