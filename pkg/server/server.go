package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
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

	k8sErrors "k8s.io/kubernetes/pkg/api/errors"

	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/30x/enrober/pkg/apigee"
	"github.com/30x/enrober/pkg/helper"
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

// Copied from https://github.com/30x/authsdk/blob/master/apigee.go#L19
//
// TODO: Turn this into some Go-based Apigee client/SDK to replace authsdk
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

// Lets start by just making a new function
func createEnvironment(environmentName, token string) error {

	//Make sure they passed a valid environment name of form {org}:{env}
	if !envNameRegex.MatchString(environmentName) {
		errorMessage := fmt.Sprintf("Not a valid environment name: %s\n", environmentName)
		return errors.New(errorMessage)
	}

	//Parse environment name into 2 parts
	nameSlice := strings.Split(environmentName, ":")
	apigeeOrgName := nameSlice[0]
	apigeeEnvName := nameSlice[1]

	// transform EnvironmentName into acceptable k8s namespace name
	environmentName = apigeeOrgName + "-" + apigeeEnvName

	//Generate both a public and private key
	privateKey, err := helper.GenerateRandomString(32)
	publicKey, err := helper.GenerateRandomString(32)
	if err != nil {
		errorMessage := fmt.Sprintf("Error generating random string: %v\n", err)
		return errors.New(errorMessage)
	}

	//Should attempt KVM creation before creating k8s objects
	if apigeeKVM {

		httpClient := &http.Client{}

		//construct URL
		apigeeKVMURL := fmt.Sprintf("%sv1/organizations/%s/environments/%s/keyvaluemaps", apigeeAPIHost, apigeeOrgName, apigeeEnvName)

		//create JSON body
		kvmBody := apigeeKVMBody{
			Name: apigeeKVMName,
			Entry: []apigeeKVMEntry{
				apigeeKVMEntry{
					Name:  apigeeKVMPKName,
					Value: base64.StdEncoding.EncodeToString([]byte(publicKey)),
				},
			},
		}

		b := new(bytes.Buffer)
		json.NewEncoder(b).Encode(kvmBody)

		req, err := http.NewRequest("POST", apigeeKVMURL, b)
		if err != nil {
			errorMessage := fmt.Sprintf("Unable to create request (Create KVM): %v", err)
			return errors.New(errorMessage)
		}

		//Must pass through the authz header
		req.Header.Add("Authorization", token)
		req.Header.Add("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			errorMessage := fmt.Sprintf("Error creating Apigee KVM: %v", err)
			return errors.New(errorMessage)
		}
		defer resp.Body.Close()

		//TODO: Probably want to generalize this logic

		// If the response was not a 201, we need to check if the response was a 409 because this means the KVM exists
		// already and we'll need to update the KVM value(s).
		if resp.StatusCode != 201 {
			var retryFlag bool

			// If the KVM already exists, we need to update its value(s).
			if resp.StatusCode == 409 {
				b2 := new(bytes.Buffer)
				updateKVMURL := fmt.Sprintf("%s/%s", apigeeKVMURL, apigeeKVMName) // Use non-CPS endpoint by default

				if isCPSEnabledForOrg(apigeeOrgName, token) {
					// When using CPS, the API endpoint is different and instead of sending the whole KVM body, we can only send
					// the KVM entry to update.  (This will work for now since we are only persisting one key but in the future
					// we might need to update this to make N calls, one per key.)
					updateKVMURL = fmt.Sprintf("%s/entries/%s", updateKVMURL, apigeeKVMPKName)

					json.NewEncoder(b2).Encode(kvmBody.Entry[0])
				} else {
					// When not using CPS, send the whole KVM body to update all keys in the KVM.
					json.NewEncoder(b2).Encode(kvmBody) // Non-CPS takes the whole payload
				}

				updateKVMReq, err := http.NewRequest("POST", updateKVMURL, b2)

				if err != nil {
					errorMessage := fmt.Sprintf("Unable to create request (Update KVM): %v", err)
					return errors.New(errorMessage)
				}

				fmt.Printf("The update KVM URL: %v\n", updateKVMReq.URL.String())

				updateKVMReq.Header.Add("Authorization", token)
				updateKVMReq.Header.Add("Content-Type", "application/json")

				resp2, err := httpClient.Do(updateKVMReq)
				if err != nil {
					errorMessage := fmt.Sprintf("Error creating entry in existing Apigee KVM: %v", err)
					return errors.New(errorMessage)
				}
				defer resp2.Body.Close()

				var updateKVMRes retryResponse

				//Decode response
				err = json.NewDecoder(resp2.Body).Decode(&updateKVMRes)

				if err != nil {
					errorMessage := fmt.Sprintf("Failed to decode response: %v\n", err)
					return errors.New(errorMessage)
				}

				// Updating a KVM returns a 200 on success so if it's not a 200, it's a failure
				if resp2.StatusCode != 200 {
					errorMessage := fmt.Sprintf("Couldn't create KVM entry (Status Code: %d): %v", resp2.StatusCode, updateKVMRes.Message)
					return errors.New(errorMessage)
				}

				retryFlag = true
			}

			if !retryFlag {
				errorMessage := fmt.Sprintf("Expected 201 or 409, got: %v", resp.StatusCode)
				return errors.New(errorMessage)
			}
		}

	}

	// Retrieve hostnames from Apigee api
	apigeeClient := apigee.Client{Token: token}
	hosts, err := apigeeClient.Hosts(apigeeOrgName, apigeeEnvName)
	if err != nil {
		errorMessage := fmt.Sprintf("Error retrieving hostnames from Apigee : %v", err)
		return errors.New(errorMessage)
	}

	//Should create an annotation object and pass it into the object literal
	nsAnnotations := make(map[string]string)
	nsAnnotations["hostNames"] = strings.Join(hosts, " ")

	//Add network policy annotation if we are isolating namespaces
	if isolateNamespace {
		nsAnnotations["net.beta.kubernetes.io/network-policy"] = `{"ingress": {"isolation": "DefaultDeny"}}`
	}

	//NOTE: Probably shouldn't create annotation if there are no hostNames
	nsObject := &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name: environmentName,
			Labels: map[string]string{
				"runtime":      "shipyard",
				"organization": apigeeOrgName,
				"environment":  apigeeEnvName,
				"name":         environmentName,
			},
			Annotations: nsAnnotations,
		},
	}

	//Create Namespace
	createdNs, err := client.Namespaces().Create(nsObject)
	if err != nil {
		errorMessage := fmt.Sprintf("Error creating namespace: %v", err)
		return errors.New(errorMessage)
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
	_, err = client.Secrets(environmentName).Create(&tempSecret)
	if err != nil {
		helper.LogError.Printf("Error creating secret: %s\n", err)

		err = client.Namespaces().Delete(createdNs.GetName())
		if err != nil {
			errorMessage := fmt.Sprintf("Failed to cleanup namespace\n")
			return errors.New(errorMessage)
		}
		errorMessage := fmt.Sprintf("Deleted namespace due to secret creation error\n")
		return errors.New(errorMessage)
	}
	return nil
}

func updateEnvironmentHosts(org, env, token string) error {
	ns, err := client.Namespaces().Get(org + "-" + env)
	if err != nil {
		return err
	}

	apigeeClient := apigee.Client{Token: token}
	hosts, err := apigeeClient.Hosts(org, env)
	if err != nil {
		return err
	}

	ns.ObjectMeta.Annotations["hostNames"] = strings.Join(hosts, " ")
	_, err = client.Namespaces().Update(ns)
	if err != nil {
		return err
	}

	return nil
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

//patchEnvironment - Patches environment if supplied with nothing hosts are synced from apigee to kubernetes
func patchEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		if !helper.ValidAdmin(pathVars["org"], w, r) {
			return
		}
	}

	err := updateEnvironmentHosts(pathVars["org"], pathVars["env"], r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error syncing Hosts to the Environment: %v\n", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	helper.LogInfo.Printf("Patched environment: %s\n", pathVars["org"]+"-"+pathVars["env"])
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

	// Check if the environment already exists
	_, err := client.Namespaces().Get(pathVars["org"] + "-" + pathVars["env"])
	if err != nil {
		if k8sErrors.IsAlreadyExists(err) == false {

			// Create environment if it doesn't exist
			err := createEnvironment(pathVars["org"]+":"+pathVars["env"], r.Header.Get("Authorization"))
			if err != nil {
				errorMessage := fmt.Sprintf("Broke at createEnvironment: %v", err)
				helper.LogError.Printf(errorMessage)
				http.Error(w, errorMessage, http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			helper.LogError.Printf("Error getting existing Environment: %v\n", err)
			return
		}
	}

	//Decode passed JSON body
	var tempJSON deploymentPost
	err = json.NewDecoder(r.Body).Decode(&tempJSON)
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

	//Check if we got a URL
	if tempJSON.PtsURL == "" {
		//No URL so error
		errorMessage := fmt.Sprintf("No ptsURL given\n")
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	tempPTS, err = helper.GetPTSFromURL(tempJSON.PtsURL, r)
	if err != nil {
		helper.LogError.Printf(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if allowPrivilegedContainers == false {
		for _, val := range tempPTS.Spec.Containers {
			if val.SecurityContext != nil {
				val.SecurityContext.Privileged = func() *bool { b := false; return &b }()
			}
		}
	}

	for index, val := range tempJSON.EnvVars {
		if val.ValueFrom != (&apigee.ApigeeEnvVarSource{}) {
			// Gotta go retrieve the value from apigee KVM
			// In the future we may support other ref types
			apigeeClient := apigee.Client{Token: r.Header.Get("Authorization")}
			tempJSON.EnvVars[index], err = apigee.EnvReftoEnv(val.ValueFrom, apigeeClient, pathVars["org"], pathVars["env"])
			if err != nil {
				errorMessage := fmt.Sprintf("Failed at EnvReftoEnv: %v\n", err)
				http.Error(w, errorMessage, http.StatusInternalServerError)
				helper.LogError.Printf(errorMessage)
				return
			}
		}
	}
	tempK8sEnv, err := apigee.ApigeeEnvtoK8s(tempJSON.EnvVars)
	if err != nil {
		errorMessage := fmt.Sprintf("Failed at ApigeeEnvtoK8s: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}
	tempPTS.Spec.Containers[0].Env = apigee.CacheK8sEnvVars(tempPTS.Spec.Containers[0].Env, tempK8sEnv)

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
	tempPTS.Labels["runtime"] = "shipyard"

	//Could also use proto package
	tempInt := int32(5)

	template := extensions.Deployment{
		ObjectMeta: api.ObjectMeta{
			Name: tempJSON.DeploymentName,
		},
		Spec: extensions.DeploymentSpec{
			RevisionHistoryLimit: &tempInt,
			Replicas:             *tempJSON.Replicas,
			Selector: &unversioned.LabelSelector{
				MatchLabels: map[string]string{
					"component": tempPTS.Labels["component"],
				},
			},
			Template: tempPTS,
		},
	}

	labelSelector, err := labels.Parse("component=" + tempPTS.Labels["component"])
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

	//Check if we got a URL
	if tempJSON.PtsURL == "" {
		//No URL so error
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

	//Only set the replica count if the passed variable
	if tempJSON.Replicas != nil {
		getDep.Spec.Replicas = *tempJSON.Replicas
	}
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

	for index, val := range tempJSON.EnvVars {
		if val.ValueFrom != (&apigee.ApigeeEnvVarSource{}) {
			// Gotta go retrieve the value from apigee KVM
			// In the future we may support other ref types
			apigeeClient := apigee.Client{Token: r.Header.Get("Authorization")}
			tempJSON.EnvVars[index], err = apigee.EnvReftoEnv(val.ValueFrom, apigeeClient, pathVars["org"], pathVars["env"])
			if err != nil {
				errorMessage := fmt.Sprintf("Failed at EnvReftoEnv: %v\n", err)
				http.Error(w, errorMessage, http.StatusInternalServerError)
				helper.LogError.Printf(errorMessage)
				return
			}
		}
	}
	tempK8sEnv, err := apigee.ApigeeEnvtoK8s(tempJSON.EnvVars)
	if err != nil {
		errorMessage := fmt.Sprintf("Failed at ApigeeEnvtoK8s: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}
	getDep.Spec.Template.Spec.Containers[0].Env = apigee.CacheK8sEnvVars(getDep.Spec.Template.Spec.Containers[0].Env, tempK8sEnv)

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

	previousString := queries.Get("previous")
	var previous bool
	if previousString != "" {
		var err error
		previous, err = strconv.ParseBool(previousString)
		if err != nil {
			errorMessage := fmt.Sprintf("Invalid previous value: %s\n", err)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			helper.LogError.Printf(errorMessage)
			return
		}
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

		if previous {
			podLogOpts.Previous = previous
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

func getStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("OK"))
}

func isCPSEnabledForOrg(orgName, authzHeader string) bool {
	cpsEnabled := false
	httpClient := &http.Client{}

	req, err := http.NewRequest("GET", fmt.Sprintf("%sv1/organizations/%s", apigeeAPIHost, orgName), nil)

	if err != nil {
		fmt.Printf("Error checking for CPS: %v", err)

		return cpsEnabled
	}

	fmt.Printf("Checking if %s has CPS enabled using URL: %v\n", orgName, req.URL.String())

	req.Header.Add("Authorization", authzHeader)

	res, err := httpClient.Do(req)

	defer res.Body.Close()

	if err != nil {
		fmt.Printf("Error checking for CPS: %v", err)
	} else {
		var rawOrg interface{}

		err := json.NewDecoder(res.Body).Decode(&rawOrg)

		if err != nil {
			fmt.Printf("Error unmarshalling response: %v\n", err)
		} else {
			org := rawOrg.(map[string]interface{})
			orgProps := org["properties"].(map[string]interface{})
			orgProp := orgProps["property"].([]interface{})

			for _, rawProp := range orgProp {
				prop := rawProp.(map[string]interface{})

				if prop["name"] == "features.isCpsEnabled" {
					if prop["value"] == "true" {
						cpsEnabled = true
					}

					break
				}
			}
		}
	}

	fmt.Printf("  CPS Enabled: %v", cpsEnabled)

	return cpsEnabled
}
