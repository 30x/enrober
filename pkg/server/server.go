package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/labels"

	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"
)

//Server struct
type Server struct {
	Router *mux.Router
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

	sub.Path("/environmentGroups").Methods("GET").HandlerFunc(getEnvironmentGroups)

	sub.Path("/environmentGroups/{environmentGroupID}").Methods("GET").HandlerFunc(getEnvironmentGroup)

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

//getEnvironmentGroups returns a list of all Environment Groups
//What is an environmentGroup?
func getEnvironmentGroups(w http.ResponseWriter, r *http.Request) {
	//TODO: What is this supposed to do?
	//For now I'll just return 405 I guess...
	http.Error(w, "405 Method not allowed", 405)
}

//getEnvironmentGroup returns an Environment Group matching the given environmentGroupID
//What is an environmentGroup?
func getEnvironmentGroup(w http.ResponseWriter, r *http.Request) {
	// vars := mux.Vars(r)

	//TODO: What is this supposed to do?
	//For now I'll just return 405 I guess...
	http.Error(w, "405 Method not allowed", 405)
}

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
		fmt.Fprintf(w, "%v\n", err)
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

//createEnvironment creates a kubernetes namespace matching the given environmentGroupID and environmentName
func createEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	//Struct to put JSON into
	type environmentPost struct {
		EnvironmentName string `json:"environmentName"`
		Secret          string `json:"secret"`
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

	nsObject := &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name: pathVars["environmentGroupID"] + "-" + tempJSON.EnvironmentName,
			Labels: map[string]string{
				"group": pathVars["environmentGroupID"],
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
			Name: "ingress",
		},
		Data: map[string][]byte{
			"api-key": []byte(tempJSON.Secret),
		},
		Type: "Opaque",
	}

	//Create Secret
	secret, err := client.Secrets(pathVars["environmentGroupID"] + "-" + tempJSON.EnvironmentName).Create(&tempSecret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error creating secret: %v\n", err)
	}
	//Print to console for logging
	fmt.Printf("Created Secret: %v\n", secret.GetName())

	js, err := json.Marshal(createdNs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error marshalling namespace: %v\n", err)
	}
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

	//Struct to put JSON into
	type environmentPost struct {
		Secret string `json:"secret"`
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

	//Check if there is a secret named ingress in the given environment
	getSecret, err := client.Secrets(pathVars["environmentGroupID"] + "-" + pathVars["environment"]).Get("ingress")
	if err != nil {
		test := errors.New("secrets \"ingress\" not found")
		if err.Error() == test.Error() {
			//Create secret
			tempSecret := api.Secret{
				ObjectMeta: api.ObjectMeta{
					Name: "ingress",
				},
				Data: map[string][]byte{
					"api-key": []byte(tempJSON.Secret),
				},
				Type: "Opaque",
			}
			secret, err := client.Secrets(pathVars["environmentGroupID"] + "-" + pathVars["environment"]).Create(&tempSecret)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				fmt.Printf("Error creating secret: %v\n", err)
				return
			}

			w.WriteHeader(200)
			fmt.Fprintf(w, "Created Secret: %v\n", secret.GetName())
			fmt.Printf("Created Secret: %v\n", secret.GetName())

		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			fmt.Printf("Error getting secret: %v\n", err)
			return
		}
	} else {
		getSecret.Data["api-key"] = []byte(tempJSON.Secret)
		secret, err := client.Secrets(pathVars["environmentGroupID"] + "-" + pathVars["environment"]).Update(getSecret)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			fmt.Printf("Error updating secret: %v\n", err)
		}

		w.WriteHeader(200)
		fmt.Printf("Updated Secret: %v\n", secret.GetName())
		fmt.Fprintf(w, "Updated Secret: %v\n", secret.GetName())

	}
}

//deleteEnvironment deletes a kubernetes namespace matching the given environmentGroupID and environmentName
func deleteEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	//TODO: Can only delete a namespace based on its name not it's annotations.
	//Ensure that this is thorough enough for our uses.

	err := client.Namespaces().Delete(pathVars["environmentGroupID"] + "-" + pathVars["environment"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("Error in deleteEnvironment: %v\n", err)
		return
	}
	w.WriteHeader(200)
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

	//TODO: Could be moved to types file
	//Struct to put JSON into
	type deploymentPost struct {
		DeploymentName string               `json:"deploymentName"`
		TrafficHosts   string               `json:"trafficHosts"`
		Replicas       int                  `json:"Replicas"`
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

	//If the passed pod template spec doesn't have prior annotations
	//then we have to call the below line. Determine if we need to check this
	//or if we are assuming all pod template specs have prior annotations.
	// tempPTS.Annotations = make(map[string]string)
	tempPTS.Annotations["trafficHosts"] = tempJSON.TrafficHosts

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

			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			w.Write(js)
			fmt.Printf("Got Deployment: %v\n", value.GetName())

			break
		}
	}

}

//TODO: Allow modifying the PTS
//updateDeployment updates a deployment matching the given environmentGroupID, environmentName, and deploymentName
func updateDeployment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	//TODO: Could be moved to types file
	//Struct to put JSON into
	type deploymentPatch struct {
		TrafficHosts string               `json:"trafficHosts"`
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

	//If the old pod template spec doesn't have prior annotations
	//then we have to call the below line. Determine if we need to check this
	//or if we are assuming all pod template specs have prior annotations.
	// tempPTS.Annotations = make(map[string]string)
	tempPTS.Annotations["trafficHosts"] = tempJSON.TrafficHosts

	getDep, err := client.Deployments(pathVars["environmentGroupID"] + "-" + pathVars["environment"]).Get(pathVars["deployment"])
	if err != nil {
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

	w.WriteHeader(200)
	fmt.Fprintf(w, "Deleted Deployment: %v\n", pathVars["deployment"])
}
