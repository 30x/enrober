package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	k8sErrors "k8s.io/client-go/pkg/api/errors"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"

	"github.com/30x/enrober/pkg/apigee"
	"github.com/30x/enrober/pkg/helper"
	"os"
	"reflect"
	"strings"
)

//getEnvironment returns a kubernetes namespace matching the given environmentGroupID and environmentName
func getEnvironment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	getNs, err := clientset.Namespaces().Get(pathVars["org"] + "-" + pathVars["env"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error getting existing Environment: %v\n", err)
		return
	}

	getSecret, err := clientset.Secrets(pathVars["org"] + "-" + pathVars["env"]).Get("routing")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error getting existing Secret: %v\n", err)
		return
	}

	var jsResponse environmentResponse
	jsResponse.Name = getNs.Name
	jsResponse.ApiSecret = getSecret.Data["api-key"]
	tempHosts, err := parseHoststoMap(getNs.Annotations["edge/hosts"])
	if err != nil {
		errorMessage := fmt.Sprintf("Error Parsing Hosts: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
	}
	jsResponse.EdgeHosts = tempHosts

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

	ns, err := clientset.Namespaces().Get(pathVars["org"] + "-" + pathVars["env"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error getting existing Environment: %v\n", err)
		return
	}

	routingSecret, err := clientset.Secrets(pathVars["org"] + "-" + pathVars["env"]).Get("routing")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		helper.LogError.Printf("Error getting existing Secret: %v\n", err)
		return
	}

	if os.Getenv("DEPLOY_STATE") == "PROD" {
		apigeeClient := apigee.Client{Token: r.Header.Get("Authorization")}
		hosts, err := apigeeClient.Hosts(pathVars["org"], pathVars["env"])
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			helper.LogError.Printf("Error getting Hosts from Apigee: %v\n", err)
			return
		}

		ns.ObjectMeta.Annotations["edge/hosts"] = composeHostsJSON(hosts)
		ns, err = clientset.Namespaces().Update(ns)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			helper.LogError.Printf("Error updating Environment: %v\n", err)
			return
		}
	}

	var jsResponse environmentResponse
	jsResponse.Name = ns.Name
	jsResponse.ApiSecret = routingSecret.Data["api-key"]

	tempHosts, err := parseHoststoMap(ns.Annotations["edge/hosts"])
	if err != nil {
		errorMessage := fmt.Sprintf("Error Parsing Hosts: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}
	jsResponse.EdgeHosts = tempHosts

	js, err := json.Marshal(jsResponse)
	if err != nil {
		errorMessage := fmt.Sprintf("Error marshalling Environment: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(js)
	helper.LogInfo.Printf("Patched environment: %s\n", pathVars["org"]+"-"+pathVars["env"])
}

//getDeployments returns a list of all deployments matching the given org and env name
func getDeployments(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	depList, err := clientset.Deployments(pathVars["org"] + "-" + pathVars["env"]).List(v1.ListOptions{})
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
	w.WriteHeader(http.StatusOK)
	w.Write(js)
	for _, value := range depList.Items {
		helper.LogInfo.Printf("Got Deployment: %s\n", value.GetName())
	}
}

//createDeployment creates a deployment in the given environment(namespace) with the given environmentGroupID based on the given deploymentBody
func createDeployment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	// Check if the environment already exists
	_, err := clientset.Namespaces().Get(pathVars["org"] + "-" + pathVars["env"])
	if err != nil {
		if k8sErrors.IsNotFound(err) {
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

	//Check if deployment with given name already exists
	_, err = clientset.Deployments(pathVars["org"] + "-" + pathVars["env"]).Get(tempJSON.DeploymentName)
	if err == nil {
		//Fail because it means the deployment must exist
		errorMessage := fmt.Sprintf("Deployment %s already exists\n", tempJSON.DeploymentName)
		http.Error(w, errorMessage, http.StatusConflict)
		helper.LogError.Printf(errorMessage)
		return
	}

	apigeeClient := apigee.Client{Token: r.Header.Get("Authorization")}
	tempJSON.EnvVars, err = apigee.GetKVMVars(tempJSON.EnvVars, apigeeKVM, apigeeClient, pathVars["org"], pathVars["env"])
	if err != nil {
		errorMessage := fmt.Sprintf("Failed at GetKVMVars: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	tempPTS, err := GeneratePTS(tempJSON, pathVars["org"], pathVars["env"])
	if err != nil {
		errorMessage := fmt.Sprintf("Failed to generate PTS: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

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
	depList, err := clientset.Deployments(pathVars["org"] + "-" + pathVars["env"]).List(v1.ListOptions{
		LabelSelector: labelSelector,
	})
	if len(depList.Items) != 0 {
		errorMessage := fmt.Sprintf("LabelSelector " + labelSelector + " already exists")
		helper.LogError.Printf(errorMessage)
		http.Error(w, errorMessage, http.StatusConflict)
		return
	}

	//Create Deployment
	dep, err := clientset.Deployments(pathVars["org"] + "-" + pathVars["env"]).Create(&template)
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
	url := "/environments/" + pathVars["org"] + ":" + pathVars["env"] + "/deployments/" + tempJSON.DeploymentName
	w.Header().Add("Location", url)
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(js)

	helper.LogInfo.Printf("Created Deployment: %s\n", dep.GetName())
}

//getDeployment returns a deployment matching the given environmentGroupID, environmentName, and deploymentName
func getDeployment(w http.ResponseWriter, r *http.Request) {
	pathVars := mux.Vars(r)

	getDep, err := clientset.Deployments(pathVars["org"] + "-" + pathVars["env"]).Get(pathVars["deployment"])
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
	getDep, err := clientset.Deployments(pathVars["org"] + "-" + pathVars["env"]).Get(pathVars["deployment"])
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
	//If the JSON body is empty then we bounce the pods
	if reflect.DeepEqual(tempJSON, deploymentPatch{}) {
		podList, err := clientset.Pods(pathVars["org"] + "-" + pathVars["env"]).List(v1.ListOptions{
			LabelSelector: "component=" + getDep.Labels["component"],
		})
		for _, value := range podList.Items {
			err = clientset.Pods(pathVars["org"]+"-"+pathVars["env"]).Delete(value.GetName(), &v1.DeleteOptions{})
			if err != nil {
				errorMessage := fmt.Sprintf("Error deleting pod: %v\n", err)
				http.Error(w, errorMessage, http.StatusInternalServerError)
				helper.LogError.Printf(errorMessage)
				return
			}
			helper.LogInfo.Printf("Deleted Pod: %v\n", value.GetName())
		}
	}

	//Only set the replica count if the user passed the variable
	if tempJSON.Replicas != nil {
		getDep.Spec.Replicas = tempJSON.Replicas
	}
	//Only modify paths if user passed the variable
	if tempJSON.Paths != nil {
		intPort, err := strconv.Atoi(tempJSON.Paths[0].ContainerPort)
		if err != nil {
			errorMessage := fmt.Sprintf("Invalid Container Port: %v\n", err)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			helper.LogError.Printf(errorMessage)
			return
		}
		tempPaths, err := composePathsJSON(tempJSON.Paths)
		if err != nil {
			errorMessage := fmt.Sprintf("Error Composing Paths: %v\n", err)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			helper.LogError.Printf(errorMessage)
			return
		}
		getDep.Spec.Template.Annotations["edge/hosts"] = tempPaths
		getDep.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = int32(intPort)
	}

	if tempJSON.EnvVars != nil {
		apigeeClient := apigee.Client{Token: r.Header.Get("Authorization")}
		tempJSON.EnvVars, err = apigee.GetKVMVars(tempJSON.EnvVars, apigeeKVM, apigeeClient, pathVars["org"], pathVars["env"])
		if err != nil {
			errorMessage := fmt.Sprintf("Failed at GetKVMVars: %v\n", err)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			helper.LogError.Printf(errorMessage)
			return
		}
		tempK8sEnv, err := apigee.ApigeeEnvtoK8s(tempJSON.EnvVars)
		if err != nil {
			errorMessage := fmt.Sprintf("Failed at ApigeeEnvtoK8s: %v\n", err)
			http.Error(w, errorMessage, http.StatusInternalServerError)
			helper.LogError.Printf(errorMessage)
			return
		}
		getDep.Spec.Template.Spec.Containers[0].Env = apigee.CacheK8sEnvVars(getDep.Spec.Template.Spec.Containers[0].Env, tempK8sEnv)
	}

	//Revision was given
	if tempJSON.Revision != nil {
		newStrRevision := strconv.Itoa(int(*tempJSON.Revision))
		//It's a new revision
		if newStrRevision != getDep.Spec.Template.Labels["edge/app.rev"] {
			//Split old image URI into the main part and the revision
			oldImageURISlice := strings.Split(getDep.Spec.Template.Spec.Containers[0].Image, ":")

			//Update the deployment to have new revision label
			getDep.Spec.Template.Labels["edge/app.rev"] = newStrRevision

			//Update the deployment to have new image name
			getDep.Spec.Template.Spec.Containers[0].Image = oldImageURISlice[0] + ":" + newStrRevision
		}
	}

	dep, err := clientset.Deployments(pathVars["org"] + "-" + pathVars["env"]).Update(getDep)
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
	dep, err := clientset.Deployments(pathVars["org"] + "-" + pathVars["env"]).Get(pathVars["deployment"])
	if err != nil {
		errorMessage := fmt.Sprintf("Error getting old deployment: %s\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	//Get the match label
	selector := "component=" + dep.Labels["component"]

	//Get the replica sets with the corresponding label
	rsList, err := clientset.ReplicaSets(pathVars["org"] + "-" + pathVars["env"]).List(v1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		errorMessage := fmt.Sprintf("Error getting replica set list: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	//Get the pods with the corresponding label
	podList, err := clientset.Pods(pathVars["org"] + "-" + pathVars["env"]).List(v1.ListOptions{
		LabelSelector: selector,
	})

	//Delete Deployment
	err = clientset.Deployments(pathVars["org"]+"-"+pathVars["env"]).Delete(pathVars["deployment"], &v1.DeleteOptions{})
	if err != nil {
		errorMessage := fmt.Sprintf("Error deleting deployment: %v\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}
	helper.LogInfo.Printf("Deleted Deployment: %v\n", pathVars["deployment"])

	//Delete all Replica Sets that came up in the list
	for _, value := range rsList.Items {
		err = clientset.ReplicaSets(pathVars["org"]+"-"+pathVars["env"]).Delete(value.GetName(), &v1.DeleteOptions{})
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
		err = clientset.Pods(pathVars["org"]+"-"+pathVars["env"]).Delete(value.GetName(), &v1.DeleteOptions{})
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
	dep, err := clientset.Deployments(pathVars["org"] + "-" + pathVars["env"]).Get(pathVars["deployment"])
	if err != nil {
		errorMessage := fmt.Sprintf("Error retrieving deployment: %s\n", err)
		http.Error(w, errorMessage, http.StatusInternalServerError)
		helper.LogError.Printf(errorMessage)
		return
	}

	selector := dep.Spec.Selector
	label := "component=" + selector.MatchLabels["component"]

	podInterface := clientset.Pods(pathVars["org"] + "-" + pathVars["env"])

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
