package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/gorilla/handlers"
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
	Router http.Handler
}

//Global Vars
var (
	//Kubernetes Client
	client k8sClient.Client

	//Global Regex
	validIPAddressRegex = regexp.MustCompile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
	validHostnameRegex  = regexp.MustCompile(`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`)

	//Env Name Regex
	envNameRegex = regexp.MustCompile(`\w+\-\w+`)

	//ECR Pull Secrets
	lookForECRSecret bool
	ecrSecretName    string

	//Privileged container flag
	allowPrivilegedContainers bool

	//Namespace Isolation
	isolateNamespace bool
)

//Init runs once
func Init(clientConfig restclient.Config) error {
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
		tempClient, err := k8sClient.New(&clientConfig)
		if err != nil {
			return err
		}
		client = *tempClient
	}

	if os.Getenv("ISOLATE_NAMESPACE") == "false" {
		isolateNamespace = false
	} else {
		isolateNamespace = true
	}

	//Several features should be disabled for local testing
	if os.Getenv("DEPLOY_STATE") == "PROD" {
		//Set global ECR secret flag
		if os.Getenv("ECR_SECRET") == "true" {
			lookForECRSecret = true

			//Set name of ECR Secret to look for
			if os.Getenv("ECR_SECRET_NAME") != "" {
				ecrSecretName = os.Getenv("ECR_SECRET_NAME")
			} else {
				ecrSecretName = "shipyard-pull-secret"
			}
		} else {
			lookForECRSecret = false
		}

		//Set privileged container flag
		if os.Getenv("ALLOW_PRIV_CONTAINERS") == "true" {
			allowPrivilegedContainers = true
		} else {
			allowPrivilegedContainers = false
		}

	} else {
		lookForECRSecret = false
		allowPrivilegedContainers = false
	}

	return nil
}

//NewServer creates a new server
func NewServer() (server *Server) {
	router := mux.NewRouter()

	router.Path("/environments").Methods("POST").HandlerFunc(createEnvironment)
	router.Path("/environments").Methods("GET").HandlerFunc(getEnvironments)
	router.Path("/environments/{org}-{env}").Methods("GET").HandlerFunc(getEnvironment)
	router.Path("/environments/{org}-{env}").Methods("PATCH").HandlerFunc(updateEnvironment)
	router.Path("/environments/{org}-{env}").Methods("DELETE").HandlerFunc(deleteEnvironment)
	router.Path("/environments/{org}-{env}/deployments").Methods("POST").HandlerFunc(createDeployment)
	router.Path("/environments/{org}-{env}/deployments").Methods("GET").HandlerFunc(getDeployments)
	router.Path("/environments/{org}-{env}/deployments/{deployment}").Methods("GET").HandlerFunc(getDeployment)
	router.Path("/environments/{org}-{env}/deployments/{deployment}").Methods("PATCH").HandlerFunc(updateDeployment)
	router.Path("/environments/{org}-{env}/deployments/{deployment}").Methods("DELETE").HandlerFunc(deleteDeployment)

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

//getEnvironments returns a list of all environments
func getEnvironments(w http.ResponseWriter, r *http.Request) {

	nsList, err := client.Namespaces().List(api.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error in getEnvironments: %v\n", err)
		return
	}

	var envList []environmentResponse

	//Loops through all namespaces and returns those that have a "routing" secret present
	for _, value := range nsList.Items {
		//Construct a temp object
		var tempEnv environmentResponse

		//Get []string from the space delimited annotation
		hostNamesArray := strings.Split(value.Annotations["hostNames"], " ")

		//Need to initialize the tempEnv.HostNames slice
		tempEnv.HostNames = hostNamesArray
		tempEnv.Name = value.Name

		//For each namespace we have to do a get on the secrets in it
		getSecret, err := client.Secrets(value.Name).Get("routing")
		if err == nil {
			//Only return namespaces with the relevant secrets present
			tempEnv.PrivateSecret = getSecret.Data["private-api-key"]
			tempEnv.PublicSecret = getSecret.Data["public-api-key"]

			//Append the temp object to the slice
			envList = append(envList, tempEnv)
		}

	}
	//If there are no environments then return a blank json
	if len(envList) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		return
	}

	js, err := json.Marshal(envList)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error marshalling environment array: %s\n", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(js)

	//TODO: What do we want logging response to be?
	for _, value := range nsList.Items {
		helper.LogInfo.Printf("Got namespace: %s\n", value.GetName())
	}
}

//createEnvironment creates a kubernetes namespace matching the given environmentGroupID and environmentName
func createEnvironment(w http.ResponseWriter, r *http.Request) {

	//Decode passed JSON body
	decoder := json.NewDecoder(r.Body)
	var tempJSON environmentPost
	err := decoder.Decode(&tempJSON)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error decoding JSON Body: %s\n", err)
		return
	}

	//Make sure they passed a valid environment name of form {org}-{env}
	if !envNameRegex.MatchString(tempJSON.EnvironmentName) {
		http.Error(w, "Invalid environment name", http.StatusInternalServerError)
		helper.LogError.Printf("Not a valid environment name: %s\n", tempJSON.EnvironmentName)
		return
	}

	//Parse environment name into 2 parts
	nameSlice := strings.Split(tempJSON.EnvironmentName, "-")
	apigeeOrgName := nameSlice[0]
	apigeeEnvName := nameSlice[1]

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		if !helper.ValidAdmin(apigeeOrgName, w, r) {
			//Errors should be returned from function
			return
		}
	}

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

	//Verify that hostname isn't on another namespace
	for _, value := range tempJSON.HostNames {

		//Get list of all namespace and loop through each of their "validHosts" annotation looking for strings matching our value
		nsList, err := client.Namespaces().List(api.ListOptions{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			helper.LogError.Printf("Error in getting nsList in createEnvironment: %s\n", err)
			return
		}

		for _, ns := range nsList.Items {
			//Make sure validHosts annotation exists
			if val, ok := ns.Annotations["hostNames"]; ok {
				//Get the hostsList annotation
				if strings.Contains(val, value) {
					//Duplicate HostNames
					http.Error(w, "Duplicate Hostname", http.StatusInternalServerError)
					helper.LogError.Printf("Duplicate Hostname: %s\n", value)
					return
				}
			}
		}
	}

	//Should create an annotation object and pass it into the object literal
	nsAnnotations := make(map[string]string)
	nsAnnotations["hostNames"] = hostsList.String()

	if isolateNamespace {
		nsAnnotations["net.beta.kubernetes.io/network-policy"] = `{"ingress": {"isolation": "DefaultDeny"}}`
	}

	//TODO: Probably shouldn't create annotation if there are no hostNames
	nsObject := &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name: tempJSON.EnvironmentName,
			Labels: map[string]string{
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error in createEnvironment: %s\n", err)
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

	//Always generating both secrets
	privateKey, err := helper.GenerateRandomString(32)
	publicKey, err := helper.GenerateRandomString(32)
	if err != nil {
		helper.LogError.Printf("Error generating random string: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	tempSecret.Data["public-api-key"] = []byte(publicKey)
	tempSecret.Data["private-api-key"] = []byte(privateKey)

	//Create Secret
	secret, err := client.Secrets(tempJSON.EnvironmentName).Create(&tempSecret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error creating secret: %s\n", err)
	}
	//Print to console for logging
	helper.LogInfo.Printf("Created Secret: %s\n", secret.GetName())

	//TODO: Should be configurable
	if lookForECRSecret {
		getPullSecret, err := client.Secrets("apigee").Get(ecrSecretName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			helper.LogError.Printf("Error getting Image Pull Secret: %s\n", err)
		}
		//Blank out all the metadata
		getPullSecret.ObjectMeta = api.ObjectMeta{}
		//Have to set the name
		getPullSecret.SetName(ecrSecretName)
		newPullSecret, err := client.Secrets(tempJSON.EnvironmentName).Create(getPullSecret)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			helper.LogError.Printf("Error creating new Image Pull Secret: %s\n", err)
		}
		helper.LogInfo.Printf("New Pull Secret: %v\n", newPullSecret.GetName())
	}

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
	url := "/environments/" + tempJSON.EnvironmentName
	w.Header().Add("Location", url)
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(js)
}

//getEnvironment returns a kubernetes namespace matching the given environmentGroupID and environmentName
func getEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		if !helper.ValidAdmin(pathVars["org"], w, r) {
			//Errors should be returned from function
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

	//TODO: What do we want the logging response to be
	helper.LogInfo.Printf("Got Namespace: %s\n", getNs.GetName())
	helper.LogInfo.Printf("Got Secret: %s\n", getSecret.GetName())
}

func updateEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		if !helper.ValidAdmin(pathVars["org"], w, r) {
			//Errors should be returned from function
			return
		}
	}

	//Need to get the existing environment
	getNs, err := client.Namespaces().Get(pathVars["org"] + "-" + pathVars["env"])
	if err != nil {
		errorMessage := fmt.Sprintf("Namespace %s doesn't exist\n", pathVars["org"]+"-"+pathVars["env"])
		helper.LogError.Printf(errorMessage)
		http.Error(w, errorMessage, http.StatusNotFound)
		return
	}

	//Looks like we have to do a get on the existing secrets in the namespace to print them out
	getSecret, err := client.Secrets(pathVars["org"] + "-" + pathVars["env"]).Get("routing")
	if err != nil {
		errorMessage := fmt.Sprintf("Failed to get existing routing secret on %s namespace\n", pathVars["org"]+"-"+pathVars["env"])
		helper.LogError.Printf(errorMessage)
		http.Error(w, errorMessage, http.StatusInternalServerError)
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

	//Can do a quick optimization to just check if the new hostNames are the same as the old
	//if they are we can just give a 200 back without doing anything
	if bytes.Equal(hostsList.Bytes(), []byte(getNs.Annotations["hostNames"])) {
		helper.LogInfo.Printf("Nothing to be updated\n")
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
			helper.LogError.Printf("Error in getting nsList in updateEnvironment: %s\n", err)
			return
		}

		for _, ns := range nsList.Items {
			//Make sure validHosts annotation exists
			if val, ok := ns.Annotations["hostNames"]; ok {
				//Get the hostsList annotation
				if strings.Contains(val, value) {
					// Ignore duplicate hostNames on the namespace we are updating
					if !(ns.GetName() == getNs.GetName()) {
						//Duplicate HostNames
						errorMessage := fmt.Sprintf("Duplicate Hostname: %s\n", value)
						http.Error(w, errorMessage, http.StatusInternalServerError)
						helper.LogError.Printf(errorMessage)
						return
					}

				}
			}
		}
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

//deleteEnvironment deletes a kubernetes namespace matching the given environmentGroupID and environmentName
func deleteEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		if !helper.ValidAdmin(pathVars["org"], w, r) {
			//Errors should be returned from function
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

	//Print to stdout for logging
	helper.LogInfo.Printf("Deleted Namespace: %s\n", pathVars["org"]+"-"+pathVars["env"])
}

//getDeployments returns a list of all deployments matching the given environmentGroupID and environmentName
func getDeployments(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	if !helper.ValidAdmin(pathVars["org"], w, r) {
		//Errors should be returned from function
		return
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
			//Errors should be returned from function
			return
		}
	}

	//Decode passed JSON body
	decoder := json.NewDecoder(r.Body)
	var tempJSON deploymentPost
	err := decoder.Decode(&tempJSON)
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

	//Needs to be at higher scope than if statement
	tempPTS := &api.PodTemplateSpec{}
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
			errorMessage := fmt.Sprintf("Error retrieving pod template spec: %s\n", err)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			helper.LogError.Printf(errorMessage)
			return
		}
		defer urlJSON.Body.Close()

		if urlJSON.StatusCode != 200 {
			errorMessage := fmt.Sprintf("Expected 200 from ptsURL got: %v\n", urlJSON.StatusCode)
			helper.LogError.Printf(errorMessage)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			return
		}

		err = json.NewDecoder(urlJSON.Body).Decode(tempPTS)
		if err != nil {
			errorMessage := fmt.Sprintf("Error decoding PTS JSON Body: %s\n", err)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			helper.LogError.Printf(errorMessage)
			return
		}

		if os.Getenv("DEPLOY_STATE") == "PROD" {
			u, err := url.Parse(tempJSON.PtsURL)
			if err != nil {
				errorMessage := fmt.Sprintf("Error parsing ptsURL: %s\n", err)
				http.Error(w, errorMessage, http.StatusInternalServerError)
				helper.LogError.Printf(errorMessage)
				return
			}
			if u.Host != r.Host {
				errorMessage := fmt.Sprintf("Attempting to use PTS from unauthorized host: %v, expected: %v\n", u.Host, r.Host)
				helper.LogError.Printf(errorMessage)
				http.Error(w, errorMessage, http.StatusInternalServerError)
				return
			}
		}

	} else {
		//We got a direct PTS so just copy it
		tempPTS = tempJSON.PTS
	}

	if allowPrivilegedContainers == false {
		for _, val := range tempPTS.Spec.Containers {
			if val.SecurityContext != nil {
				val.SecurityContext.Privileged = func() *bool { b := false; return &b }()
			}
		}
	}

	//TODO: In the future we may want to have a check to ensure that publicPaths and/or privatePaths exists

	//TODO: Break this out into function later

	//Need to cache the previous envVars
	cacheEnvVars := tempPTS.Spec.Containers[0].Env

	//Check for envVar conflicts and prioritize ones from passed JSON.
	finalEnvVar := cacheEnvVars

	//Keep track of which jsonVars modified vs need to be added
	jsonEnvLength := len(tempJSON.EnvVars)
	trackArray := make([]bool, jsonEnvLength)

	//Add on any additional envVars
	for i, jsonVar := range tempJSON.EnvVars {
		for j, cacheVar := range cacheEnvVars {
			if cacheVar.Name == jsonVar.Name {
				finalEnvVar[j] = jsonVar
				trackArray[i] = true
			}
		}
		if trackArray[i] == false {
			finalEnvVar = append(finalEnvVar, jsonVar)
		}
	}

	tempPTS.Spec.Containers[0].Env = finalEnvVar

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

	template := extensions.Deployment{
		ObjectMeta: api.ObjectMeta{
			Name: tempJSON.DeploymentName,
		},
		Spec: extensions.DeploymentSpec{
			Replicas: tempJSON.Replicas,
			Selector: &unversioned.LabelSelector{
				MatchLabels: map[string]string{
					"component": tempPTS.Labels["component"],
				},
			},
			Template: *tempPTS,
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

	//TODO: What do we want logging message to be?
	helper.LogInfo.Printf("Got Deployment: %v\n", getDep.GetName())
}

//updateDeployment updates a deployment matching the given environmentGroupID, environmentName, and deploymentName
func updateDeployment(w http.ResponseWriter, r *http.Request) {

	pathVars := mux.Vars(r)

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		if !helper.ValidAdmin(pathVars["org"], w, r) {
			//Errors should be returned from function
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
	decoder := json.NewDecoder(r.Body)
	var tempJSON deploymentPatch
	err = decoder.Decode(&tempJSON)
	if err != nil {
		errorMessage := fmt.Sprintf("Error decoding JSON Body: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	tempPTS := &api.PodTemplateSpec{}
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
				errorMessage := fmt.Sprintf("Error retrieving pod template spec: %s\n", err)
				http.Error(w, errorMessage, http.StatusInternalServerError)
				helper.LogError.Printf(errorMessage)
				return
			}
			defer urlJSON.Body.Close()

			if urlJSON.StatusCode != 200 {
				errorMessage := fmt.Sprintf("Expected 200 got: %v\n", urlJSON.StatusCode)
				helper.LogError.Printf(errorMessage)
				http.Error(w, errorMessage, http.StatusInternalServerError)
				return
			}

			err = json.NewDecoder(urlJSON.Body).Decode(tempPTS)
			if err != nil {
				errorMessage := fmt.Sprintf("Error decoding PTS JSON Body: %s\n", err)
				http.Error(w, errorMessage, http.StatusInternalServerError)
				helper.LogError.Printf(errorMessage)
				return
			}

			if os.Getenv("DEPLOY_STATE") == "PROD" {
				u, err := url.Parse(tempJSON.PtsURL)
				if err != nil {
					errorMessage := fmt.Sprintf("Error parsing ptsURL: %v\n", err)
					http.Error(w, errorMessage, http.StatusInternalServerError)
					helper.LogError.Printf(errorMessage)
					return
				}
				if u.Host != r.Host {
					errorMessage := fmt.Sprintf("Attempting to use PTS from unauthorized host: %v, expected: %v\n", u.Host, r.Host)
					helper.LogError.Printf(errorMessage)
					http.Error(w, errorMessage, http.StatusInternalServerError)
					return
				}
			}
		}
	} else {
		//We got a direct PTS so just copy it
		tempPTS = tempJSON.PTS
	}

	//If annotations map is empty then we need to make it
	if len(tempPTS.Annotations) == 0 {
		tempPTS.Annotations = make(map[string]string)
	}

	//If labels map is empty then we need to make it
	if len(tempPTS.Labels) == 0 {
		tempPTS.Labels = make(map[string]string)
	}

	//Need to cache the previous envVars
	cacheEnvVars := getDep.Spec.Template.Spec.Containers[0].Env

	//Need to cache the previous annotations
	cacheAnnotations := getDep.Spec.Template.Annotations

	getDep.Spec.Replicas = tempJSON.Replicas
	getDep.Spec.Template = *tempPTS

	//Replace the privateHosts and publicHosts annotations with cached ones
	getDep.Spec.Template.Annotations["publicHosts"] = cacheAnnotations["publicHosts"]
	getDep.Spec.Template.Annotations["privateHosts"] = cacheAnnotations["privateHosts"]

	if tempJSON.PrivateHosts != nil {
		getDep.Spec.Template.Annotations["privateHosts"] = *tempJSON.PrivateHosts
	}

	if tempJSON.PublicHosts != nil {
		getDep.Spec.Template.Annotations["publicHosts"] = *tempJSON.PublicHosts
	}

	//Check for envVar conflicts and prioritize ones from passed JSON.
	finalEnvVar := cacheEnvVars

	//Keep track of which jsonVars modified vs need to be added
	jsonEnvLength := len(tempJSON.EnvVars)
	trackArray := make([]bool, jsonEnvLength)

	//Add on any additional envVars
	for i, jsonVar := range tempJSON.EnvVars {
		for j, cacheVar := range cacheEnvVars {
			if cacheVar.Name == jsonVar.Name {
				finalEnvVar[j] = jsonVar
				trackArray[i] = true
			}
		}
		if trackArray[i] == false {
			finalEnvVar = append(finalEnvVar, jsonVar)
		}
	}

	getDep.Spec.Template.Spec.Containers[0].Env = finalEnvVar

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
			//Errors should be returned from function
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
