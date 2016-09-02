package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/labels"

	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/30x/enrober/pkg/helper"
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

	router.Path("/environments").Methods("POST").HandlerFunc(createEnvironment)
	router.Path("/environments/{org}:{env}").Methods("GET").HandlerFunc(getEnvironment)
	router.Path("/environments/{org}:{env}").Methods("PATCH").HandlerFunc(updateEnvironment)
	router.Path("/environments/{org}:{env}").Methods("DELETE").HandlerFunc(deleteEnvironment)
	router.Path("/environments/{org}:{env}/deployments").Methods("POST").HandlerFunc(createDeployment)
	router.Path("/environments/{org}:{env}/deployments").Methods("GET").HandlerFunc(getDeployments)
	router.Path("/environments/{org}:{env}/deployments/{deployment}").Methods("GET").HandlerFunc(getDeployment)
	router.Path("/environments/{org}:{env}/deployments/{deployment}").Methods("PATCH").HandlerFunc(updateDeployment)
	router.Path("/environments/{org}:{env}/deployments/{deployment}").Methods("DELETE").HandlerFunc(deleteDeployment)
	router.Path("/environments/{org}:{env}/deployments/{deployment}/logs").Methods("GET").HandlerFunc(getDeploymentLogs)

	loggedRouter := handlers.CombinedLoggingHandler(os.Stdout, router)

	server = &Server{
		Router: loggedRouter,
	}
	return server
}

//Start the server
func (server *Server) Start() error {
	return http.ListenAndServe(":9000", server.Router)
}

//createEnvironment creates a kubernetes namespace and secret
func createEnvironment(w http.ResponseWriter, r *http.Request) {

	//Decode passed JSON body
	var tempJSON environmentPost
	err := json.NewDecoder(r.Body).Decode(&tempJSON)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error decoding JSON Body: %s\n", err)
		return
	}

	//Make sure they passed a valid environment name of form {org}:{env}
	if !envNameRegex.MatchString(tempJSON.EnvironmentName) {
		http.Error(w, "Invalid environment name", http.StatusInternalServerError)
		helper.LogError.Printf("Not a valid environment name: %s\n", tempJSON.EnvironmentName)
		return
	}

	//Parse environment name into 2 parts
	nameSlice := strings.Split(tempJSON.EnvironmentName, ":")
	apigeeOrgName := nameSlice[0]
	apigeeEnvName := nameSlice[1]

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		if !helper.ValidAdmin(apigeeOrgName, w, r) {
			return
		}
	}

	// transform EnvironmentName into acceptable k8s namespace name
	tempJSON.EnvironmentName = apigeeOrgName + "-" + apigeeEnvName

	//space delimited annotation of valid hostnames
	var hostsList bytes.Buffer

	for index, value := range tempJSON.HostNames {
		//Verify each Hostname matches regex
		validIP := validIPAddressRegex.MatchString(value)
		validHost := validHostnameRegex.MatchString(value)

		if !(validIP || validHost) {
			//Regex didn't match
			http.Error(w, "Invalid Hostname", http.StatusInternalServerError)
			helper.LogError.Printf("Not a valid hostname: %s\n", value)
			return
		}
		if index == 0 {
			hostsList.WriteString(value)
		} else {
			hostsList.WriteString(" " + value)
		}
	}

	uniqueHosts, err := helper.UniqueHostNames(tempJSON.HostNames, client)
	if err != nil {
		errorMessage := fmt.Sprintf("Error in UniqueHostNames: %v", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage + "\n")
		return
	}
	if !uniqueHosts {
		errorMessage := "Duplicate HostNames"
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	//Generate both a public and private key
	privateKey, err := helper.GenerateRandomString(32)
	publicKey, err := helper.GenerateRandomString(32)
	if err != nil {
		helper.LogError.Printf("Error generating random string: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	//Should attempt KVM creation before creating k8s objects
	if apigeeKVM {

		httpClient := &http.Client{}

		//construct URL
		apigeeURL := fmt.Sprintf("https://api.enterprise.apigee.com/v1/organizations/%s/environments/%s/keyvaluemaps",
			apigeeOrgName, apigeeEnvName)

		//create JSON body
		kvmBody := apigeeKVMBody{
			Name: "routing",
			Entry: []apigeeKVMEntry{
				apigeeKVMEntry{
					Name:  "public-key",
					Value: publicKey,
				},
			},
		}

		b := new(bytes.Buffer)
		json.NewEncoder(b).Encode(kvmBody)

		req, err := http.NewRequest("POST", apigeeURL, b)
		if err != nil {

		}
		//Must pass through the authz header
		req.Header.Add("Authorization", r.Header.Get("Authorization"))
		req.Header.Add("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			errorMessage := fmt.Sprintf("Error creating Apigee KVM: %v", err)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			helper.LogError.Printf(errorMessage + "\n")
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 201 {
			//Attempt to just create the entry not the KVM map
			var retryFLag bool
			if resp.StatusCode == 409 {
				b2 := new(bytes.Buffer)

				retryReq, err := http.NewRequest("POST", apigeeURL+"/routing/entries", b2)
				fmt.Printf("retry URL: %v\n", retryReq.URL.String())

				retryReq.Header.Add("Authorization", r.Header.Get("Authorization"))
				retryReq.Header.Add("Content-Type", "application/json")

				json.NewEncoder(b2).Encode(kvmBody.Entry[0])

				resp2, err := httpClient.Do(retryReq)
				if err != nil {
					errorMessage := fmt.Sprintf("Error creating entry in existing Apigee KVM: %v", err)
					http.Error(w, errorMessage, http.StatusInternalServerError)
					helper.LogError.Printf(errorMessage + "\n")
					return
				}
				defer resp2.Body.Close()

				var retryResponse retryResponse
				//Decode response
				err = json.NewDecoder(resp2.Body).Decode(&retryResponse)
				if err != nil {
					fmt.Printf("Failed to decode response: %v\n", err)
					http.Error(w, "Failed to decode response", http.StatusInternalServerError)
					return
				}

				if resp2.StatusCode != 201 && resp2.StatusCode != 409 {
					errorMessage := fmt.Sprintf("Couldn't create KVM entry, Status Code: %d", resp2.StatusCode)
					http.Error(w, errorMessage, http.StatusInternalServerError)
					helper.LogError.Printf(errorMessage + "\n")
					return
				}
				retryFLag = true
			}
			if !retryFLag {
				errorMessage := fmt.Sprintf("Expected 201 or 409, got: %v", resp.StatusCode)
				http.Error(w, errorMessage, http.StatusInternalServerError)
				helper.LogError.Printf(errorMessage + "\n")
				return
			}
		}

	}

	//Should create an annotation object and pass it into the object literal
	nsAnnotations := make(map[string]string)
	nsAnnotations["hostNames"] = hostsList.String()

	//Add network policy annotation if we are isolating namespaces
	if isolateNamespace {
		nsAnnotations["net.beta.kubernetes.io/network-policy"] = `{"ingress": {"isolation": "DefaultDeny"}}`
	}

	//NOTE: Probably shouldn't create annotation if there are no hostNames
	nsObject := &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name: tempJSON.EnvironmentName,
			Labels: map[string]string{
				"Runtime":      "shipyard",
				"Organziation": apigeeOrgName,
				"Environment":  apigeeEnvName,
				"Name":         tempJSON.EnvironmentName,
			},
			Annotations: nsAnnotations,
		},
	}

	//Create Namespace
	createdNs, err := client.Namespaces().Create(nsObject)
	if err != nil {
		errorMessage := fmt.Sprintf("Error creating namespace: %v", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage + "\n")
		return
	}
	//Print to console for logging
	helper.LogInfo.Printf("Created Namespace: %s\n", createdNs.GetName())

	tempSecret := api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name: "routing",
		},
		Data: map[string][]byte{},
		Type: "Opaque",
	}

	tempSecret.Data["public-api-key"] = []byte(publicKey)
	tempSecret.Data["private-api-key"] = []byte(privateKey)

	//Create Secret
	secret, err := client.Secrets(tempJSON.EnvironmentName).Create(&tempSecret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error creating secret: %s\n", err)

		err = client.Namespaces().Delete(createdNs.GetName())
		if err != nil {
			helper.LogError.Printf("Failed to cleanup namespace\n")
			return
		}
		helper.LogError.Printf("Deleted namespace due to secret creation error\n")
		return
	}
	//Print to console for logging
	helper.LogInfo.Printf("Created Secret: %s\n", secret.GetName())

	var jsResponse environmentResponse
	jsResponse.Name = tempJSON.EnvironmentName
	jsResponse.PrivateSecret = secret.Data["private-api-key"]
	jsResponse.PublicSecret = secret.Data["public-api-key"]
	jsResponse.HostNames = tempJSON.HostNames

	js, err := json.Marshal(jsResponse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error marshalling response JSON: %s\n", err)
		return
	}

	//Create absolute path for Location header
	locationURL := "/environments/" + apigeeOrgName + ":" + apigeeEnvName
	w.Header().Add("Location", locationURL)
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(js)
}

//getEnvironment returns a kubernetes namespace matching the given environmentGroupID and environmentName
func getEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		if !helper.ValidAdmin(pathVars["org"], w, r) {
			return
		}
	}

	getNs, err := client.Namespaces().Get(pathVars["org"] + "-" + pathVars["env"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error getting existing Environment: %v\n", err)
		return
	}

	getSecret, err := client.Secrets(pathVars["org"] + "-" + pathVars["env"]).Get("routing")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error getting existing Secret: %v\n", err)
		return
	}

	var jsResponse environmentResponse
	jsResponse.Name = getNs.Name
	jsResponse.PrivateSecret = getSecret.Data["private-api-key"]
	jsResponse.PublicSecret = getSecret.Data["public-api-key"]
	jsResponse.HostNames = strings.Split(getNs.Annotations["hostNames"], " ")

	js, err := json.Marshal(jsResponse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error marshalling response JSON: %v\n", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(js)

	helper.LogInfo.Printf("Got Namespace: %s\n", getNs.GetName())
}

//updateEnvironment modifies the hostNames array on an existing environment
func updateEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		if !helper.ValidAdmin(pathVars["org"], w, r) {
			return
		}
	}

	//Get the existing namespace
	getNs, err := client.Namespaces().Get(pathVars["org"] + "-" + pathVars["env"])
	if err != nil {
		errorMessage := fmt.Sprintf("Namespace %s doesn't exist\n", pathVars["org"]+"-"+pathVars["env"])
		helper.LogError.Printf(errorMessage)
		http.Error(w, errorMessage, http.StatusNotFound)
		return
	}

	//Get the existing routing secret
	getSecret, err := client.Secrets(pathVars["org"] + "-" + pathVars["env"]).Get("routing")
	if err != nil {
		errorMessage := fmt.Sprintf("Failed to get existing routing secret on %s namespace\n", pathVars["org"]+"-"+pathVars["env"])
		helper.LogError.Printf(errorMessage)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		return
	}

	//Decode passed JSON body
	var tempJSON environmentPatch
	err = json.NewDecoder(r.Body).Decode(&tempJSON)
	if err != nil {
		errorMessage := fmt.Sprintf("Error decoding JSON Body: %s\n", err)
		helper.LogError.Printf(errorMessage)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		return
	}

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
			helper.LogError.Printf("Not a valid hostname: %s\n", value)
			return
		}
		if index == 0 {
			hostsList.WriteString(value)
		} else {
			hostsList.WriteString(" " + value)
		}
	}

	//If hostNames are same as old then just give 200 back
	if bytes.Equal(hostsList.Bytes(), []byte(getNs.Annotations["hostNames"])) {
		helper.LogInfo.Printf("Nothing to be updated\n")
		return
	}

	uniqueHosts, err := helper.UniqueHostNames(tempJSON.HostNames, client)
	if err != nil {
		errorMessage := fmt.Sprintf("Error in UniqueHostNames: %v", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}
	if !uniqueHosts {
		errorMessage := "Duplicate HostNames"
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	getNs.Annotations["hostNames"] = hostsList.String()

	updateNS, err := client.Namespaces().Update(getNs)
	if err != nil {
		errorMessage := fmt.Sprintf("Failed to update existing namespace '%s'\n", getNs)
		helper.LogError.Printf(errorMessage)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		return
	}
	helper.LogInfo.Printf("Updated hostNames: %s\n", updateNS.Annotations["hostNames"])

	var jsResponse environmentResponse
	jsResponse.Name = pathVars["environment"]
	jsResponse.PrivateSecret = getSecret.Data["private-api-key"]
	jsResponse.PublicSecret = getSecret.Data["public-api-key"]
	jsResponse.HostNames = tempJSON.HostNames

	js, err := json.Marshal(jsResponse)
	if err != nil {
		errorMessage := fmt.Sprintf("Couldn't marshall namespace: %s\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(js)

}

//deleteEnvironment deletes a kubernetes namespace matching the given org and env name
func deleteEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		if !helper.ValidAdmin(pathVars["org"], w, r) {
			return
		}
	}

	err := client.Namespaces().Delete(pathVars["org"] + "-" + pathVars["env"])
	if err != nil {
		errorMessage := fmt.Sprintf("Error in deleteEnvironment: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}
	w.WriteHeader(204)

	helper.LogInfo.Printf("Deleted Namespace: %s\n", pathVars["org"]+"-"+pathVars["env"])
}

//getDeployments returns a list of all deployments matching the given org and env name
func getDeployments(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		if !helper.ValidAdmin(pathVars["org"], w, r) {
			return
		}
	}

	depList, err := client.Deployments(pathVars["org"] + "-" + pathVars["env"]).List(api.ListOptions{
		LabelSelector: labels.Everything(),
	})
	if err != nil {
		errorMessage := fmt.Sprintf("Error retrieving deployment list: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}
	js, err := json.Marshal(depList)
	if err != nil {
		errorMessage := fmt.Sprintf("Error marshalling deployment list: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(js)
	for _, value := range depList.Items {
		helper.LogInfo.Printf("Got Deployment: %s\n", value.GetName())
	}
}

//createDeployment creates a deployment in the given environment(namespace) with the given environmentGroupID based on the given deploymentBody
func createDeployment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		if !helper.ValidAdmin(pathVars["org"], w, r) {
			return
		}
	}

	//Decode passed JSON body
	var tempJSON deploymentPost
	err := json.NewDecoder(r.Body).Decode(&tempJSON)
	if err != nil {
		errorMessage := fmt.Sprintf("Error decoding JSON Body: %s\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	if tempJSON.PublicHosts == nil && tempJSON.PrivateHosts == nil {
		errorMessage := fmt.Sprintf("No privateHosts or publicHosts given\n")
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	tempPTS := api.PodTemplateSpec{}

	//Check if we got a URL or a direct PTS
	if tempJSON.PTS == nil {
		//No PTS so check ptsURL
		if tempJSON.PtsURL == "" {
			//No URL either so error
			errorMessage := fmt.Sprintf("No ptsURL or PTS given\n")
			http.Error(w, errorMessage, http.StatusInternalServerError)
			helper.LogError.Printf(errorMessage)
			return
		}

		tempPTS, err = helper.GetPTSFromURL(tempJSON.PtsURL, r)
		if err != nil {
			helper.LogError.Printf(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	} else {
		//We got a direct PTS so just copy it
		tempPTS = *tempJSON.PTS
	}

	if allowPrivilegedContainers == false {
		for _, val := range tempPTS.Spec.Containers {
			if val.SecurityContext != nil {
				val.SecurityContext.Privileged = func() *bool { b := false; return &b }()
			}
		}
	}

	tempPTS.Spec.Containers[0].Env = helper.CacheEnvVars(tempPTS.Spec.Containers[0].Env, tempJSON.EnvVars)

	//If map is empty then we need to make it
	if len(tempPTS.Annotations) == 0 {
		tempPTS.Annotations = make(map[string]string)
	}

	if tempJSON.PrivateHosts != nil {
		tempPTS.Annotations["privateHosts"] = *tempJSON.PrivateHosts
	}

	if tempJSON.PublicHosts != nil {
		tempPTS.Annotations["publicHosts"] = *tempJSON.PublicHosts
	}

	//If map is empty then we need to make it
	if len(tempPTS.Labels) == 0 {
		tempPTS.Labels = make(map[string]string)
	}

	//Add routable label
	tempPTS.Labels["routable"] = "true"

	//Could also use proto package
	tempInt := int32(5)

	template := extensions.Deployment{
		ObjectMeta: api.ObjectMeta{
			Name: tempJSON.DeploymentName,
		},
		Spec: extensions.DeploymentSpec{
			RevisionHistoryLimit: &tempInt,
			Replicas:             tempJSON.Replicas,
			Selector: &unversioned.LabelSelector{
				MatchLabels: map[string]string{
					"component": tempPTS.Labels["component"],
				},
			},
			Template: tempPTS,
		},
	}

	labelSelector, err := labels.Parse("app=" + tempPTS.Labels["app"])
	//Get list of all deployments in namespace with MatchLabels["app"] = tempPTS.Labels["app"]
	depList, err := client.Deployments(pathVars["org"] + "-" + pathVars["env"]).List(api.ListOptions{
		LabelSelector: labelSelector,
	})
	if len(depList.Items) != 0 {
		errorMessage := fmt.Sprintf("LabelSelector " + labelSelector.String() + " already exists")
		helper.LogError.Printf(errorMessage)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		return
	}

	//Create Deployment
	dep, err := client.Deployments(pathVars["org"] + "-" + pathVars["env"]).Create(&template)
	if err != nil {
		errorMessage := fmt.Sprintf("Error creating deployment: %s\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}
	js, err := json.Marshal(dep)
	if err != nil {
		errorMessage := fmt.Sprintf("Error marshalling deployment: %s\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
	}

	//Create absolute path for Location header
	url := "/environments/" + pathVars["org"] + "-" + pathVars["env"] + "/deployments/" + tempJSON.DeploymentName
	w.Header().Add("Location", url)
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(js)

	helper.LogInfo.Printf("Created Deployment: %s\n", dep.GetName())
}

//getDeployment returns a deployment matching the given environmentGroupID, environmentName, and deploymentName
func getDeployment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		if !helper.ValidAdmin(pathVars["org"], w, r) {
			//Errors should be returned from function
			return
		}
	}

	getDep, err := client.Deployments(pathVars["org"] + "-" + pathVars["env"]).Get(pathVars["deployment"])
	if err != nil {
		errorMessage := fmt.Sprintf("Error retrieving deployment: %s\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}
	js, err := json.Marshal(getDep)
	if err != nil {
		errorMessage := fmt.Sprintf("Error marshalling deployment: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(js)

	helper.LogInfo.Printf("Got Deployment: %v\n", getDep.GetName())
}

//updateDeployment updates a deployment matching the given environmentGroupID, environmentName, and deploymentName
func updateDeployment(w http.ResponseWriter, r *http.Request) {

	pathVars := mux.Vars(r)

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		if !helper.ValidAdmin(pathVars["org"], w, r) {
			return
		}
	}

	//Get the old namespace first so we can fail quickly if it's not there
	getDep, err := client.Deployments(pathVars["org"] + "-" + pathVars["env"]).Get(pathVars["deployment"])
	if err != nil {
		errorMessage := fmt.Sprintf("Error getting existing deployment: %s\n", err)
		http.Error(w, errorMessage, http.StatusNotFound)
		helper.LogError.Printf(errorMessage)
		return
	}
	//Decode passed JSON body
	var tempJSON deploymentPatch
	err = json.NewDecoder(r.Body).Decode(&tempJSON)
	if err != nil {
		errorMessage := fmt.Sprintf("Error decoding JSON Body: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	tempPTS := api.PodTemplateSpec{}

	//Check if we got a URL or a direct PTS
	if tempJSON.PTS == nil {
		//No PTS so check ptsURL
		if tempJSON.PtsURL == "" {
			//No URL either
			prevDep, err := client.Deployments(pathVars["org"] + "-" + pathVars["env"]).Get(pathVars["deployment"])
			if err != nil {
				errorMessage := fmt.Sprintf("No ptsURL or PTS given and failed to retrieve previous PTS: %v\n", err)
				http.Error(w, errorMessage, http.StatusInternalServerError)
				helper.LogError.Printf(errorMessage)
				return
			}
			tempPTS = prevDep.Spec.Template
		} else {

			tempPTS, err = helper.GetPTSFromURL(tempJSON.PtsURL, r)
			if err != nil {
				helper.LogError.Printf(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	} else {
		//We got a direct PTS so just copy it
		tempPTS = *tempJSON.PTS
	}

	//If annotations map is empty then we need to make it
	if len(tempPTS.Annotations) == 0 {
		tempPTS.Annotations = make(map[string]string)
	}

	//If labels map is empty then we need to make it
	if len(tempPTS.Labels) == 0 {
		tempPTS.Labels = make(map[string]string)
	}

	//Need to cache the previous annotations
	cacheAnnotations := getDep.Spec.Template.Annotations

	getDep.Spec.Replicas = tempJSON.Replicas
	getDep.Spec.Template = tempPTS

	//Replace the privateHosts and publicHosts annotations with cached ones
	getDep.Spec.Template.Annotations["publicHosts"] = cacheAnnotations["publicHosts"]
	getDep.Spec.Template.Annotations["privateHosts"] = cacheAnnotations["privateHosts"]

	if tempJSON.PrivateHosts != nil {
		getDep.Spec.Template.Annotations["privateHosts"] = *tempJSON.PrivateHosts
	}

	if tempJSON.PublicHosts != nil {
		getDep.Spec.Template.Annotations["publicHosts"] = *tempJSON.PublicHosts
	}

	getDep.Spec.Template.Spec.Containers[0].Env = helper.CacheEnvVars(getDep.Spec.Template.Spec.Containers[0].Env, tempJSON.EnvVars)

	//Add routable label
	getDep.Spec.Template.Labels["routable"] = "true"

	dep, err := client.Deployments(pathVars["org"] + "-" + pathVars["env"]).Update(getDep)
	if err != nil {
		errorMessage := fmt.Sprintf("Error updating deployment: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	js, err := json.Marshal(dep)
	if err != nil {
		errorMessage := fmt.Sprintf("Error marshalling deployment: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(js)
	helper.LogInfo.Printf("Updated Deployment: %s\n", dep.GetName())
}

//deleteDeployment deletes a deployment matching the given environmentGroupID, environmentName, and deploymentName
func deleteDeployment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		if !helper.ValidAdmin(pathVars["org"], w, r) {
			return
		}
	}

	//Get the deployment object
	dep, err := client.Deployments(pathVars["org"] + "-" + pathVars["env"]).Get(pathVars["deployment"])
	if err != nil {
		errorMessage := fmt.Sprintf("Error getting old deployment: %s\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	//Get the match label
	selector, err := labels.Parse("component=" + dep.Labels["component"])
	if err != nil {
		errorMessage := fmt.Sprintf("Error creating label selector: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	//Get the replica sets with the corresponding label
	rsList, err := client.ReplicaSets(pathVars["org"] + "-" + pathVars["env"]).List(api.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		errorMessage := fmt.Sprintf("Error getting replica set list: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	//Get the pods with the corresponding label
	podList, err := client.Pods(pathVars["org"] + "-" + pathVars["env"]).List(api.ListOptions{
		LabelSelector: selector,
	})

	//Delete Deployment
	err = client.Deployments(pathVars["org"]+"-"+pathVars["env"]).Delete(pathVars["deployment"], &api.DeleteOptions{})
	if err != nil {
		errorMessage := fmt.Sprintf("Error deleting deployment: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}
	helper.LogInfo.Printf("Deleted Deployment: %v\n", pathVars["deployment"])

	//Delete all Replica Sets that came up in the list
	for _, value := range rsList.Items {
		err = client.ReplicaSets(pathVars["org"]+"-"+pathVars["env"]).Delete(value.GetName(), &api.DeleteOptions{})
		if err != nil {
			errorMessage := fmt.Sprintf("Error deleting replica set: %v\n", err)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			helper.LogError.Printf(errorMessage)
			return
		}
		helper.LogInfo.Printf("Deleted Replica Set: %v\n", value.GetName())
	}

	//Delete all Pods that came up in the list
	for _, value := range podList.Items {
		err = client.Pods(pathVars["org"]+"-"+pathVars["env"]).Delete(value.GetName(), &api.DeleteOptions{})
		if err != nil {
			errorMessage := fmt.Sprintf("Error deleting pod: %v\n", err)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			helper.LogError.Printf(errorMessage)
			return
		}
		helper.LogInfo.Printf("Deleted Pod: %v\n", value.GetName())
	}
	w.WriteHeader(204)
}

func getDeploymentLogs(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		if !helper.ValidAdmin(pathVars["org"], w, r) {
			return
		}
	}

	//Get query strings
	queries := r.URL.Query()

	tailString := queries.Get("tail")
	var tail int64 = -1
	if tailString != "" {
		tailInt, err := strconv.Atoi(tailString)
		if err != nil {
			errorMessage := fmt.Sprintf("Invalid tail value: %s\n", err)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			helper.LogError.Printf(errorMessage)
			return
		}
		tail = int64(tailInt)
	}

	//Get the deployment
	dep, err := client.Deployments(pathVars["org"] + "-" + pathVars["env"]).Get(pathVars["deployment"])
	if err != nil {
		errorMessage := fmt.Sprintf("Error retrieving deployment: %s\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	selector := dep.Spec.Selector
	label, err := labels.Parse("component=" + selector.MatchLabels["component"])
	if err != nil {
		errorMessage := fmt.Sprintf("Error parsing label selector: %s\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	podInterface := client.Pods(pathVars["org"] + "-" + pathVars["env"])

	pods, err := podInterface.List(api.ListOptions{
		LabelSelector: label,
	})

	if err != nil {
		errorMessage := fmt.Sprintf("Error retrieving pods: %s\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	logBuffer := bytes.NewBuffer(nil)

	for _, pod := range pods.Items {
		podLogOpts := &api.PodLogOptions{}

		if tail != -1 {
			podLogOpts.TailLines = &tail
		}

		req := podInterface.GetLogs(pod.Name, podLogOpts)
		stream, err := req.Stream()
		if err != nil {
			errorMessage := fmt.Sprintf("Error getting log stream: %s\n", err)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			helper.LogError.Printf(errorMessage)
			return
		}

		defer stream.Close()
		podLogLine := fmt.Sprintf("Logs for pod: %v\n", pod.Name)
		_, err = logBuffer.WriteString(podLogLine)
		_, err = io.Copy(logBuffer, stream)
		if err != nil {
			errorMessage := fmt.Sprintf("Error copying log stream to var: %s\n", err)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			helper.LogError.Printf(errorMessage)
			return
		}
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(200)
	w.Write(logBuffer.Bytes())

	helper.LogInfo.Printf("Got Logs for Deployment: %v\n", dep.GetName())
}
