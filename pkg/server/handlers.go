package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	k8sErrors "k8s.io/client-go/pkg/api/errors"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"

	"github.com/30x/enrober/pkg/apigee"
	"github.com/30x/enrober/pkg/helper"
)

//getEnvironment returns a kubernetes namespace matching the given environmentGroupID and environmentName
func getEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	getNs, err := clientset.Core().Namespaces().Get(pathVars["org"] + "-" + pathVars["env"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error getting existing Environment: %v\n", err)
		return
	}

	getSecret, err := clientset.Core().Secrets(pathVars["org"] + "-" + pathVars["env"]).Get("routing")
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

	depList, err := clientset.Extensions().Deployments(pathVars["org"] + "-" + pathVars["env"]).List(v1.ListOptions{})
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

	// Check if the environment already exists
	_, err := clientset.Core().Namespaces().Get(pathVars["org"] + "-" + pathVars["env"])
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

	tempPTS := v1.PodTemplateSpec{}

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

	if apigeeKVM {
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

	template := v1beta1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name: tempJSON.DeploymentName,
		},
		Spec: v1beta1.DeploymentSpec{
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

	labelSelector := "component=" + tempPTS.Labels["component"]
	//Get list of all deployments in namespace with MatchLabels["app"] = tempPTS.Labels["app"]
	depList, err := clientset.Extensions().Deployments(pathVars["org"] + "-" + pathVars["env"]).List(v1.ListOptions{
		LabelSelector: labelSelector,
	})
	if len(depList.Items) != 0 {
		errorMessage := fmt.Sprintf("LabelSelector " + labelSelector + " already exists")
		helper.LogError.Printf(errorMessage)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		return
	}

	//Create Deployment
	dep, err := clientset.Extensions().Deployments(pathVars["org"] + "-" + pathVars["env"]).Create(&template)
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

	getDep, err := clientset.Extensions().Deployments(pathVars["org"] + "-" + pathVars["env"]).Get(pathVars["deployment"])
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

	//Get the old namespace first so we can fail quickly if it's not there
	getDep, err := clientset.Extensions().Deployments(pathVars["org"] + "-" + pathVars["env"]).Get(pathVars["deployment"])
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

	tempPTS := v1.PodTemplateSpec{}

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
		getDep.Spec.Replicas = tempJSON.Replicas
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

	if apigeeKVM {
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

	dep, err := clientset.Extensions().Deployments(pathVars["org"] + "-" + pathVars["env"]).Update(getDep)
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

	//Get the deployment object
	dep, err := clientset.Extensions().Deployments(pathVars["org"] + "-" + pathVars["env"]).Get(pathVars["deployment"])
	if err != nil {
		errorMessage := fmt.Sprintf("Error getting old deployment: %s\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	//Get the match label
	selector := "component=" + dep.Labels["component"]

	//Get the replica sets with the corresponding label
	rsList, err := clientset.Extensions().ReplicaSets(pathVars["org"] + "-" + pathVars["env"]).List(v1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		errorMessage := fmt.Sprintf("Error getting replica set list: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	//Get the pods with the corresponding label
	podList, err := clientset.Core().Pods(pathVars["org"] + "-" + pathVars["env"]).List(v1.ListOptions{
		LabelSelector: selector,
	})

	//Delete Deployment
	err = clientset.Extensions().Deployments(pathVars["org"]+"-"+pathVars["env"]).Delete(pathVars["deployment"], &v1.DeleteOptions{})
	if err != nil {
		errorMessage := fmt.Sprintf("Error deleting deployment: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}
	helper.LogInfo.Printf("Deleted Deployment: %v\n", pathVars["deployment"])

	//Delete all Replica Sets that came up in the list
	for _, value := range rsList.Items {
		err = clientset.Extensions().ReplicaSets(pathVars["org"]+"-"+pathVars["env"]).Delete(value.GetName(), &v1.DeleteOptions{})
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
		err = clientset.Core().Pods(pathVars["org"]+"-"+pathVars["env"]).Delete(value.GetName(), &v1.DeleteOptions{})
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
	dep, err := clientset.Extensions().Deployments(pathVars["org"] + "-" + pathVars["env"]).Get(pathVars["deployment"])
	if err != nil {
		errorMessage := fmt.Sprintf("Error retrieving deployment: %s\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	selector := dep.Spec.Selector
	label := "component=" + selector.MatchLabels["component"]

	podInterface := clientset.Core().Pods(pathVars["org"] + "-" + pathVars["env"])

	pods, err := podInterface.List(v1.ListOptions{
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
		podLogOpts := &v1.PodLogOptions{}

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
