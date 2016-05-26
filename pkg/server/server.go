package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/labels"

	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/30x/enrober/pkg/helper"
)

//Server struct
type Server struct {
	Router *mux.Router
}

//JSON object definitions

type environmentRequest struct {
	Name      string   `json:"name"`
	HostNames []string `json:"hostNames"`
}

type environmentResponse struct {
	Name          string   `json:"name"`
	HostNames     []string `json:"hostNames"`
	PublicSecret  string   `json:"publicSecret"`
	PrivateSecret string   `json:"privateSecret"`
}

type deploymentRequest struct {
	DeploymentName string               `json:"deploymentName"`
	PublicHosts    string               `json:"publicHosts"`
	PrivateHosts   string               `json:"privateHosts"`
	Replicas       int                  `json:"replicas"`
	PtsURL         string               `json:"ptsURL"`
	PTS            *api.PodTemplateSpec `json:"pts"`
}

//Global Kubernetes Client
var client k8sClient.Client

//Global Regex
var validIPAddressRegex = regexp.MustCompile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
var validHostnameRegex = regexp.MustCompile(`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`)

//Init runs once
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

	sub.Path("/environmentGroups/{environmentGroupID}/environments").Methods("GET").HandlerFunc(getEnvironments)
	sub.Path("/environmentGroups/{environmentGroupID}/environments").Methods("POST").HandlerFunc(createEnvironment)

	sub.Path("/environmentGroups/{environmentGroupID}/environments/{environment}").Methods("GET").HandlerFunc(getEnvironment)
	sub.Path("/environmentGroups/{environmentGroupID}/environments/{environment}").Methods("PATCH").HandlerFunc(updateEnvironment)
	sub.Path("/environmentGroups/{environmentGroupID}/environments/{environment}").Methods("DELETE").HandlerFunc(deleteEnvironment)

	sub.Path("/environmentGroups/{environmentGroupID}/environments/{environment}/deployments").Methods("GET").HandlerFunc(getDeployments)
	sub.Path("/environmentGroups/{environmentGroupID}/environments/{environment}/deployments").Methods("POST").HandlerFunc(createDeployment)

	sub.Path("/environmentGroups/{environmentGroupID}/environments/{environment}/deployments/{deployment}").Methods("GET").HandlerFunc(getDeployment)
	sub.Path("/environmentGroups/{environmentGroupID}/environments/{environment}/deployments/{deployment}").Methods("PATCH").HandlerFunc(updateDeployment)
	sub.Path("/environmentGroups/{environmentGroupID}/environments/{environment}/deployments/{deployment}").Methods("DELETE").HandlerFunc(deleteDeployment)

	server = &Server{
		Router: router,
	}
	return server
}

//Start the server
func (server *Server) Start() error {
	return http.ListenAndServe(":9000", server.Router)
}

//Route handlers

//getEnvironments returns a list of all environments under a specific environmentGroupID
func getEnvironments(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	selector, err := labels.Parse("group=" + pathVars["environmentGroupID"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error creating label selector: %v\n", err)
		return
	}
	nsList, err := client.Namespaces().List(api.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error in getEnvironments: %v\n", err)
		return
	}
	js, err := json.Marshal(nsList)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error marshalling namespace list: %v\n", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
	for _, value := range nsList.Items {
		//For debug/logging
		fmt.Printf("Got namespace: %v\n", value.GetName())
	}
}

//TODO: Create both secrets everytime
//TODO: Integrate permissions sdk
//createEnvironment creates a kubernetes namespace matching the given environmentGroupID and environmentName
func createEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	//Struct to put JSON into
	type environmentPost struct {
		EnvironmentName string   `json:"environmentName"`
		PrivateSecret   bool     `json:"privateSecret"`
		PublicSecret    bool     `json:"publicSecret"`
		HostNames       []string `json:"hostNames"`
	}

	//Decode passed JSON body
	decoder := json.NewDecoder(r.Body)
	var tempJSON environmentPost
	err := decoder.Decode(&tempJSON)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error decoding JSON Body: %v\n", err)
		return
	}

	//space delimited annotation of valid hostnames
	var hostsList bytes.Buffer

	//Loop through slice of HostNames
	for index, value := range tempJSON.HostNames {
		//Verify each Hostname matches regex
		validIP := validIPAddressRegex.MatchString(value)
		validHost := validHostnameRegex.MatchString(value)

		if !(validIP || validHost) {
			//Regex didn't match
			http.Error(w, "Invalid Hostname", http.StatusInternalServerError)
			fmt.Printf("Not a valid hostname: %v\n", value)
			return
		}
		if index == 0 {
			hostsList.WriteString(value)
		} else {
			hostsList.WriteString(" " + value)
		}

		//TODO: If this becomes a bottleneck at a high number of namespaces come back to this and optimize

		//Verify that hostname isn't on another namespace

		//Get list of all namespace and loop through each of their "validHosts" annotation looking for strings matching our value
		nsList, err := client.Namespaces().List(api.ListOptions{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			fmt.Printf("Error in getting nsList in createEnvironment: %v\n", err)
			fmt.Fprintf(w, "%v\n", err)
			return
		}

		for _, ns := range nsList.Items {
			//Make sure validHosts annotation exists
			if val, ok := ns.Annotations["hostNames"]; ok {
				//Get the hostsList annotation
				if strings.Contains(val, value) {
					//Duplicate HostNames
					http.Error(w, "Duplicate Hostname", http.StatusInternalServerError)
					fmt.Printf("Duplicate Hostname: %v\n", value)
					return
				}
			}
		}
	}

	nsObject := &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name: pathVars["environmentGroupID"] + "-" + tempJSON.EnvironmentName,
			Labels: map[string]string{
				"group": pathVars["environmentGroupID"],
			},
			Annotations: map[string]string{
				"hostNames": hostsList.String(),
			},
		},
	}

	//Create Namespace
	createdNs, err := client.Namespaces().Create(nsObject)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error in createEnvironment: %v\n", err)
		return
	}
	//Print to console for logging
	fmt.Printf("Created Namespace: %v\n", createdNs.GetName())

	tempSecret := api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name: "routing",
		},
		Data: map[string][]byte{},
		Type: "Opaque",
	}

	//Always generating both secrets
	privateKey, err := helper.GenerateRandomString(32)
	publicKey, err := helper.GenerateRandomString(32)
	if err != nil {
		fmt.Printf("Error generating random string: %v\n", err)
		http.Error(w, "", http.StatusInternalServerError)
	}
	tempSecret.Data["public-api-key"] = []byte(publicKey)
	tempSecret.Data["private-api-key"] = []byte(privateKey)

	//Create Secret
	secret, err := client.Secrets(pathVars["environmentGroupID"] + "-" + tempJSON.EnvironmentName).Create(&tempSecret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error creating secret: %v\n", err)
	}
	//Print to console for logging
	fmt.Printf("Created Secret: %v\n", secret.GetName())

	js, err := json.Marshal(secret.Data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error marshalling namespace: %v\n", err)
	}

	//TODO: Proper JSON response
	w.WriteHeader(201)
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

//getEnvironment returns a kubernetes namespace matching the given environmentGroupID and environmentName
func getEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	labelSelector, err := labels.Parse("group=" + pathVars["environmentGroupID"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error creating label selector in getEnvironment: %v\n", err)
		return
	}

	nsList, err := client.Namespaces().List(api.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error in getEnvironment: %v\n", err)
		return
	}
	//Flag indicating there is at least one value matching that name
	flag := false

	for _, value := range nsList.Items {
		if value.GetName() == pathVars["environmentGroupID"]+"-"+pathVars["environment"] {
			flag = true
			js, err := json.Marshal(value)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				fmt.Printf("Error marshalling namespace: %v\n", err)
			}
			//TODO: Proper JSON response
			w.Header().Set("Content-Type", "application/json")
			w.Write(js)
			fmt.Printf("Got Namespace: %v\n", value.GetName())
		}
	}
	if flag != true {
		w.WriteHeader(404)
		fmt.Fprintf(w, "Environment not found")
	}
}

func updateEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	//Need to get the existing environment
	getNs, err := client.Namespaces().Get(pathVars["environmentGroupID"] + "-" + pathVars["environment"])
	if err != nil {
		fmt.Printf("Error: Namespace doesn't exist\n")
		http.Error(w, "", http.StatusNotFound)
		return
	}

	//Struct to put JSON into
	type environmentPatch struct {
		HostNames []string `json:"hostNames"`
	}
	//Decode passed JSON body
	decoder := json.NewDecoder(r.Body)
	var tempJSON environmentPatch
	err = decoder.Decode(&tempJSON)
	if err != nil {
		fmt.Printf("Error decoding JSON Body: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//TODO: Do this split into two things for createEnvironment too

	//space delimited annotation of valid hostnames
	var hostsList bytes.Buffer

	//Take new json and put it into the space delimited string
	for index, value := range tempJSON.HostNames {
		//Verify each Hostname matches regex
		validIP := validIPAddressRegex.MatchString(value)
		validHost := validHostnameRegex.MatchString(value)

		if !(validIP || validHost) {
			//Regex didn't match
			http.Error(w, "Invalid Hostname", http.StatusInternalServerError)
			fmt.Printf("Error: Not a valid hostname: %v\n", value)
			return
		}
		if index == 0 {
			hostsList.WriteString(value)
		} else {
			hostsList.WriteString(" " + value)
		}
	}

	//Can do a quick optimization to just check if the new hostNames are the same as the old
	//if they are we can just give a 200 back without doing anything
	if bytes.Equal(hostsList.Bytes(), []byte(getNs.Annotations["hostNames"])) {
		fmt.Printf("New hostNames == Old hostNames so not modifying anything")
		return
	}

	//TODO: This should really be a separate function

	//Loop through slice of HostNames
	for _, value := range tempJSON.HostNames {
		//TODO: If this becomes a bottleneck at a high number of namespaces come back to this and optimize

		//Verify that hostname isn't on another namespace

		//Get list of all namespace and loop through each of their "validHosts" annotation looking for strings matching our value
		nsList, err := client.Namespaces().List(api.ListOptions{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			fmt.Printf("Error in getting nsList in createEnvironment: %v\n", err)
			fmt.Fprintf(w, "%v\n", err)
			return
		}

		for _, ns := range nsList.Items {
			//Make sure validHosts annotation exists
			if val, ok := ns.Annotations["hostNames"]; ok {
				//Get the hostsList annotation
				if strings.Contains(val, value) {
					//Duplicate HostNames
					http.Error(w, "Duplicate Hostname", http.StatusInternalServerError)
					fmt.Printf("Duplicate Hostname: %v\n", value)
					return
				}
			}
		}
	}

	//Modify the hostNames annotation if we are still good here
	getNs.Annotations["hostNames"] = hostsList.String()

	updateNS, err := client.Namespaces().Update(getNs)
	if err != nil {
		fmt.Printf("Error: Failed to update existing namespace\n")
		http.Error(w, "", http.StatusInternalServerError)
		return
		//500
	}
	fmt.Printf("Updated hostNames: %v\n", updateNS.Annotations["hostNames"])

	//TODO: Modify namespaces hostNames annotation
	//TODO: Proper JSON response
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")

}

//deleteEnvironment deletes a kubernetes namespace matching the given environmentGroupID and environmentName
func deleteEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	err := client.Namespaces().Delete(pathVars["environmentGroupID"] + "-" + pathVars["environment"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error in deleteEnvironment: %v\n", err)
		return
	}
	w.WriteHeader(200)
	//TODO: Proper JSON response
	fmt.Fprintf(w, "Deleted Namespace: %v\n", pathVars["environmentGroupID"]+"-"+pathVars["environment"])
	fmt.Printf("Deleted Namespace: %v\n", pathVars["environmentGroupID"]+"-"+pathVars["environment"])

}

//getDeployments returns a list of all deployments matching the given environmentGroupID and environmentName
func getDeployments(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	depList, err := client.Deployments(pathVars["environmentGroupID"] + "-" + pathVars["environment"]).List(api.ListOptions{
		LabelSelector: labels.Everything(),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error retrieving deployment list: %v\n", err)
		return
	}
	js, err := json.Marshal(depList)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error marshalling deployment list: %v\n", err)
	}
	//TODO: Proper JSON response
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
	for _, value := range depList.Items {
		fmt.Printf("Got Deployment: %v\n", value.GetName())
	}
}

//createDeployment creates a deployment in the given environment(namespace) with the given environmentGroupID based on the given deploymentBody
func createDeployment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	//Struct to put JSON into
	type deploymentPost struct {
		DeploymentName string               `json:"deploymentName"`
		PublicHosts    string               `json:"publicHosts"`
		PublicPaths    string               `json:"publicPaths"`
		PrivateHosts   string               `json:"privateHosts"`
		PrivatePaths   string               `json:"privatePaths"`
		Replicas       int                  `json:"replicas"`
		PtsURL         string               `json:"ptsURL"`
		PTS            *api.PodTemplateSpec `json:"pts"`
	}
	//Decode passed JSON body
	decoder := json.NewDecoder(r.Body)
	var tempJSON deploymentPost
	err := decoder.Decode(&tempJSON)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error decoding JSON Body: %v\n", err)
		return
	}

	if tempJSON.PublicHosts == "" && tempJSON.PrivateHosts == "" {
		http.Error(w, "", http.StatusInternalServerError)
		fmt.Printf("No privateHosts or publicHosts given\n")
		return
	}

	//Needs to be at higher scope than if statement
	tempPTS := &api.PodTemplateSpec{}
	//Check if we got a URL or a direct PTS
	if tempJSON.PTS == nil {
		//No PTS so check ptsURL
		fmt.Printf("No PTS\n")
		if tempJSON.PtsURL == "" {
			//No URL either so error
			http.Error(w, "", http.StatusInternalServerError)
			fmt.Printf("No ptsURL or PTS given\n")
			return
		}
		//Get from URL
		//TODO: Duplicated code, could be moved to helper function
		//Get JSON from url
		httpClient := &http.Client{}

		req, err := http.NewRequest("GET", tempJSON.PtsURL, nil)
		req.Header.Add("Content-Type", "application/json")

		//TODO: In the future if we require a secret to access the PTS store
		// then this call will need to pass in that key.
		urlJSON, err := httpClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			fmt.Printf("Error retrieving pod template spec: %v\n", err)
			return
		}
		defer urlJSON.Body.Close()

		if urlJSON.StatusCode != 200 {
			fmt.Printf("Expected 200 got: %v\n", urlJSON.StatusCode)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		err = json.NewDecoder(urlJSON.Body).Decode(tempPTS)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			fmt.Printf("Error decoding PTS JSON Body: %v\n", err)
			return
		}
	} else {
		//We got a direct PTS so just copy it
		tempPTS = tempJSON.PTS
	}

	//If map is empty then we need to make it
	if len(tempPTS.Annotations) == 0 {
		tempPTS.Annotations = make(map[string]string)
	}

	//Routing Annotations
	tempPTS.Annotations["publicHosts"] = tempJSON.PublicHosts
	tempPTS.Annotations["privateHosts"] = tempJSON.PrivateHosts

	//Add routable label
	tempPTS.Labels["routable"] = "true"

	template := extensions.Deployment{
		ObjectMeta: api.ObjectMeta{
			Name: tempJSON.DeploymentName,
		},
		Spec: extensions.DeploymentSpec{
			Replicas: tempJSON.Replicas,
			Selector: &unversioned.LabelSelector{
				MatchLabels: map[string]string{
					"app": tempPTS.Labels["app"],
				},
			},
			Template: *tempPTS,
		},
	}

	labelSelector, err := labels.Parse("app=" + tempPTS.Labels["app"])
	//Get list of all deployments in namespace with MatchLabels["app"] = tempPTS.Labels["app"]
	depList, err := client.Deployments(pathVars["environmentGroupID"] + "-" + pathVars["environment"]).List(api.ListOptions{
		LabelSelector: labelSelector,
	})
	if len(depList.Items) != 0 {
		fmt.Printf("LabelSelector " + labelSelector.String() + " already exists")
		http.Error(w, "LabelSelector "+labelSelector.String()+" already exists", http.StatusInternalServerError)
		return
	}

	//Create Deployment
	dep, err := client.Deployments(pathVars["environmentGroupID"] + "-" + pathVars["environment"]).Create(&template)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error creating deployment: %v\n", err)
		return
	}
	js, err := json.Marshal(dep)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error marshalling deployment: %v\n", err)
	}

	//TODO: Proper JSON response
	w.WriteHeader(201)
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)

	fmt.Printf("Created Deployment: %v\n", dep.GetName())
}

//getDeployment returns a deployment matching the given environmentGroupID, environmentName, and deploymentName
func getDeployment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	depList, err := client.Deployments(pathVars["environmentGroupID"] + "-" + pathVars["environment"]).List(api.ListOptions{
		LabelSelector: labels.Everything(),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error retrieving deployment list: %v\n", err)
		return
	}
	for _, value := range depList.Items {
		if value.GetName() == pathVars["deployment"] {
			js, err := json.Marshal(value)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				fmt.Printf("Error marshalling deployment: %v\n", err)
			}

			//TODO: Proper JSON response
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			w.Write(js)
			fmt.Printf("Got Deployment: %v\n", value.GetName())

			break
		}
	}

}

//updateDeployment updates a deployment matching the given environmentGroupID, environmentName, and deploymentName
func updateDeployment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	//Struct to put JSON into
	type deploymentPatch struct {
		PublicHosts  string               `json:"publicHosts"`
		PublicPaths  string               `json:"publicPaths"`
		PrivateHosts string               `json:"privateHosts"`
		PrivatePaths string               `json:"privatePaths"`
		Replicas     int                  `json:"Replicas"`
		PtsURL       string               `json:"ptsURL"`
		PTS          *api.PodTemplateSpec `json:"pts"`
	}
	//Decode passed JSON body
	decoder := json.NewDecoder(r.Body)
	var tempJSON deploymentPatch
	err := decoder.Decode(&tempJSON)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error decoding JSON Body: %v\n", err)
		return
	}

	//Needs to be at higher scope than if statement
	tempPTS := &api.PodTemplateSpec{}
	//Check if we got a URL or a direct PTS
	if tempJSON.PTS == nil {
		//No PTS so check ptsURL
		fmt.Printf("No PTS\n")
		if tempJSON.PtsURL == "" {
			fmt.Printf("No ptsURL\n")
			//No URL either so error
			prevDep, err := client.Deployments(pathVars["environmentGroupID"] + "-" + pathVars["environment"]).Get(pathVars["deployment"])
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				fmt.Printf("No ptsURL or PTS given and failed to retrieve previous PTS: %v\n", err)
				return
			}
			tempPTS = &prevDep.Spec.Template
		} else {
			//Get from URL
			//TODO: Duplicated code, could be moved to helper function
			//Get JSON from url
			httpClient := &http.Client{}

			req, err := http.NewRequest("GET", tempJSON.PtsURL, nil)
			req.Header.Add("Content-Type", "application/json")

			//TODO: In the future if we require a secret to access the PTS store
			// then this call will need to pass in that key.
			urlJSON, err := httpClient.Do(req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				fmt.Printf("Error retrieving pod template spec: %v\n", err)
				return
			}
			defer urlJSON.Body.Close()

			if urlJSON.StatusCode != 200 {
				fmt.Printf("Expected 200 got: %v\n", urlJSON.StatusCode)
				http.Error(w, "", http.StatusInternalServerError)
				return
			}

			err = json.NewDecoder(urlJSON.Body).Decode(tempPTS)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				fmt.Printf("Error decoding PTS JSON Body: %v\n", err)
				return
			}
		}
	} else {
		//We got a direct PTS so just copy it
		tempPTS = tempJSON.PTS
	}

	//If map is empty then we need to make it
	if len(tempPTS.Annotations) == 0 {
		tempPTS.Annotations = make(map[string]string)
	}

	//Don't overwrite routing annotations unless they're non empty
	if tempJSON.PublicHosts != "" {
		tempPTS.Annotations["publicHosts"] = tempJSON.PublicHosts
	}
	if tempJSON.PrivateHosts != "" {
		tempPTS.Annotations["privateHosts"] = tempJSON.PrivateHosts
	}

	//Add routable label
	//TODO: routable label should already exist, do we need this?
	tempPTS.Labels["routable"] = "true"

	getDep, err := client.Deployments(pathVars["environmentGroupID"] + "-" + pathVars["environment"]).Get(pathVars["deployment"])
	if err != nil {
		//TODO: Should this be a 404?
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error getting old deployment: %v\n", err)
		return
	}
	getDep.Spec.Replicas = tempJSON.Replicas
	getDep.Spec.Template = *tempPTS

	dep, err := client.Deployments(pathVars["environmentGroupID"] + "-" + pathVars["environment"]).Update(getDep)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error updating deployment: %v\n", err)
		return
	}
	js, err := json.Marshal(dep)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error marshalling deployment: %v\n", err)
	}

	//TODO: Proper JSON response
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
	fmt.Printf("Updated Deployment: %v\n", dep.GetName())
}

//deleteDeployment deletes a deployment matching the given environmentGroupID, environmentName, and deploymentName
func deleteDeployment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	//Get the deployment object
	dep, err := client.Deployments(pathVars["environmentGroupID"] + "-" + pathVars["environment"]).Get(pathVars["deployment"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error getting old deployment: %v\n", err)
		return
	}

	//Get the match label
	selector, err := labels.Parse("app=" + dep.Labels["app"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error creating label selector: %v\n", err)
		return
	}

	//Get the replica sets with the corresponding label
	rsList, err := client.ReplicaSets(pathVars["environmentGroupID"] + "-" + pathVars["environment"]).List(api.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error getting replica set list: %v\n", err)
		return
	}

	//Get the pods with the corresponding label
	podList, err := client.Pods(pathVars["environmentGroupID"] + "-" + pathVars["environment"]).List(api.ListOptions{
		LabelSelector: selector,
	})

	//Delete Deployment
	err = client.Deployments(pathVars["environmentGroupID"]+"-"+pathVars["environment"]).Delete(pathVars["deployment"], &api.DeleteOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error deleting deployment: %v\n", err)
		return
	}
	fmt.Printf("Deleted Deployment: %v\n", pathVars["deployment"])

	//Delete all Replica Sets that came up in the list
	for _, value := range rsList.Items {
		err = client.ReplicaSets(pathVars["environmentGroupID"]+"-"+pathVars["environment"]).Delete(value.GetName(), &api.DeleteOptions{})
		if err != nil {
			fmt.Printf("Error deleting replica set: %v\n", err)
			return
		}
		fmt.Printf("Deleted Replica Set: %v\n", value.GetName())

	}

	//Delete all Pods that came up in the list
	for _, value := range podList.Items {
		err = client.Pods(pathVars["environmentGroupID"]+"-"+pathVars["environment"]).Delete(value.GetName(), &api.DeleteOptions{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			fmt.Printf("Error deleting pod: %v\n", err)
			return
		}
		fmt.Printf("Deleted Pod: %v\n", value.GetName())

	}

	//TODO: Proper JSON response
	w.WriteHeader(200)
	fmt.Fprintf(w, "Deleted Deployment: %v\n", pathVars["deployment"])
}
