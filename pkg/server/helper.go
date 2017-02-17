package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"k8s.io/client-go/pkg/api/v1"

	"github.com/30x/enrober/pkg/apigee"
	"github.com/30x/enrober/pkg/helper"
	"os"
	"regexp"
	"strconv"
)

var pathSegmentRegex *regexp.Regexp

func init() {
	pathSegmentRegex = regexp.MustCompile(`^([A-Za-z0-9\-._~!$&'()*+,;=:@]+|%[0-9A-Fa-f]{2})+$`)
}

func GeneratePTS(depBody deploymentPost, org, env string) (v1.PodTemplateSpec, error) {

	tempURI := os.Getenv("DOCKER_REGISTRY_URL")
	if tempURI == "" {
		return v1.PodTemplateSpec{}, errors.New("No URI set")
	}

	cidr := os.Getenv("POD_CIDR")
	if cidr == "" {
		cidr = "10.1.0.0/16"
	}

	var tempPaths string
	containerPorts := make([]v1.ContainerPort, len(depBody.Paths))
	if depBody.Paths == nil {
		//Make default paths
		defaultPath := []EdgePath{
			{
				BasePath:      "/" + depBody.DeploymentName,
				ContainerPort: "9000",
				TargetPath:    "/",
			},
		}
		containerPorts = []v1.ContainerPort{
			{
				ContainerPort: int32(9000),
			},
		}
		var err error
		tempPaths, err = composePathsJSON(defaultPath)
		if err != nil {
			return v1.PodTemplateSpec{}, err
		}
	} else {
		//We were given Paths
		var err error
		//Check to see if we were given multiple ports
		if multipleEdgePorts(depBody.Paths) {
			//Multiple Ports
			for i, val := range depBody.Paths {
				intPort, err := strconv.Atoi(val.ContainerPort)
				if err != nil {
					return v1.PodTemplateSpec{}, err
				}
				containerPorts[i] = v1.ContainerPort{
					ContainerPort: int32(intPort),
				}
			}

		} else {
			//Just the one port
			intPort, err := strconv.Atoi(depBody.Paths[0].ContainerPort)
			if err != nil {
				return v1.PodTemplateSpec{}, err
			}
			containerPorts = []v1.ContainerPort{
				{
					ContainerPort: int32(intPort),
				},
			}
		}
		tempPaths, err = composePathsJSON(depBody.Paths)
		if err != nil {
			return v1.PodTemplateSpec{}, err
		}
	}
	tempK8sEnv, err := apigee.ApigeeEnvtoK8s(depBody.EnvVars)
	if err != nil {
		return v1.PodTemplateSpec{}, err
	}
	apiKeyEnv := v1.EnvVar{
		Name: "API_KEY",
		ValueFrom: &v1.EnvVarSource{
			SecretKeyRef: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "routing",
				},

				Key: "api-key",
			},
		},
	}
	tempK8sEnv = append(tempK8sEnv, apiKeyEnv)

	//Default port env var
	portEnvVarSlice := make([]v1.EnvVar, len(depBody.Paths))

	//If no edgePaths are given set PORT to 9000
	if depBody.Paths == nil {
		portEnvVarSlice[0].Name = "PORT"
		portEnvVarSlice[0].Value = "9000"
	} else {
		//If we are given multiple ports we will number then in order in the form PORT{N} starting at 0
		if multipleEdgePorts(depBody.Paths) {
			uniquePorts := uniqueEdgePorts(depBody.Paths)
			for i, val := range uniquePorts {
				portEnvVarSlice[i].Name = fmt.Sprintf("PORT%d", i)
				portEnvVarSlice[i].Value = val
			}
		} else {
			//Else set PORT to the given containerPort in the only edgePath
			portEnvVarSlice[0].Name = "PORT"
			portEnvVarSlice[0].Value = depBody.Paths[0].ContainerPort
		}

	}
	tempK8sEnv = append(tempK8sEnv, portEnvVarSlice...)

	tempPTS := v1.PodTemplateSpec{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{
				"edge/paths":               tempPaths,
				"projectcalico.org/policy": fmt.Sprintf("allow tcp from cidr 192.168.0.0/16; allow tcp from cidr %s", cidr),
			},
			Labels: map[string]string{
				"component":     depBody.DeploymentName,
				"edge/app.name": depBody.DeploymentName,
				"edge/app.rev":  strconv.Itoa(int(depBody.Revision)),
				"edge/org":      org,
				"edge/env":      env,
				"edge/routable": "true",
				"runtime":       "shipyard",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            depBody.DeploymentName,
					Image:           tempURI + "/" + org + "/" + depBody.DeploymentName + ":" + strconv.Itoa(int(depBody.Revision)),
					ImagePullPolicy: v1.PullAlways,
					//Ensures that containers do not have privileged access
					//The weird lambda thing just gives a pointer to a bool set to false (because proto)
					SecurityContext: &v1.SecurityContext{
						Privileged: func() *bool { b := false; return &b }(),
					},
					Env:   tempK8sEnv,
					Ports: containerPorts,
				},
			},
		},
	}

	return tempPTS, nil
}

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
	apiKey, err := helper.GenerateRandomString(32)
	if err != nil {
		errorMessage := fmt.Sprintf("Error generating random string: %v\n", err)
		return errors.New(errorMessage)
	}

	// Retrieve hostnames from Apigee api
	apigeeClient := apigee.Client{Token: token}

	//Should attempt KVM creation before creating k8s objects
	if apigeeKVM {
		err := apigeeClient.CreateKVM(apigeeOrgName, apigeeEnvName, apiKey)
		if err != nil {
			errorMessage := fmt.Sprintf("Error creating KVM: %v", err)
			return errors.New(errorMessage)
		}
	}

	//Should create an annotation object and pass it into the object literal
	nsAnnotations := make(map[string]string)

	var hosts []string
	if apigeeKVM {
		hosts, err = apigeeClient.Hosts(apigeeOrgName, apigeeEnvName)
		if err != nil {
			errorMessage := fmt.Sprintf("Error retrieving hostnames from Apigee : %v", err)
			return errors.New(errorMessage)
		}
		nsAnnotations["edge/hosts"] = composeHostsJSON(hosts)
	}

	//Add network policy annotation if we are isolating namespaces
	if isolateNamespace {
		nsAnnotations["net.beta.kubernetes.io/network-policy"] = `{"ingress": {"isolation": "DefaultDeny"}}`
	}

	nsObject := &v1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: environmentName,
			Labels: map[string]string{
				"runtime":       "shipyard",
				"edge/routable": "true",
				"edge/org":      apigeeOrgName,
				"edge/env":      apigeeEnvName,
				"name":          environmentName,
			},
			Annotations: nsAnnotations,
		},
	}

	//Create Namespace
	createdNs, err := clientset.Namespaces().Create(nsObject)
	if err != nil {
		errorMessage := fmt.Sprintf("Error creating namespace: %v", err)
		return errors.New(errorMessage)
	}
	//Print to console for logging
	helper.LogInfo.Printf("Created Namespace: %s\n", createdNs.GetName())

	tempSecret := v1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name: "routing",
		},
		Data: map[string][]byte{},
		Type: "Opaque",
	}

	tempSecret.Data["api-key"] = []byte(apiKey)

	//Create Secret
	_, err = clientset.Secrets(environmentName).Create(&tempSecret)
	if err != nil {
		helper.LogError.Printf("Error creating secret: %s\n", err)

		err = clientset.Namespaces().Delete(createdNs.GetName(), &v1.DeleteOptions{})
		if err != nil {
			errorMessage := "Failed to cleanup namespace\n"
			return errors.New(errorMessage)
		}
		errorMessage := "Deleted namespace due to secret creation error\n"
		return errors.New(errorMessage)
	}
	return nil
}

func composeHostsJSON(hosts []string) string {
	//Return empty string on empty slice
	if hosts == nil {
		return ""
	}
	obj := make(map[string]HostsConfig)
	for _, host := range hosts {
		obj[host] = HostsConfig{}
	}

	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	return string(b)
}

func parseHoststoMap(hostString string) (map[string]HostsConfig, error) {
	tempMap := make(map[string]HostsConfig)
	if hostString == "" {
		return tempMap, nil
	}
	err := json.NewDecoder(strings.NewReader(hostString)).Decode(&tempMap)
	if err != nil {
		return nil, err
	}

	return tempMap, nil
}

func composePathsJSON(paths []EdgePath) (string, error) {
	if paths == nil {
		return "", errors.New("No paths given")
	}
	//Validate that the paths are valid
	for _, path := range paths {
		if !validatePath(path.BasePath) {
			return "", errors.New(fmt.Sprintf("Invalid Path: %v", path.BasePath))

		} else if !validatePath(path.TargetPath) {
			return "", errors.New(fmt.Sprintf("Invalid Path: %v", path.TargetPath))
		}
	}

	b, err := json.MarshalIndent(paths, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func validatePath(path string) bool {
	if !strings.HasPrefix(path, "/") {
		return false
	}
	pathSegments := strings.Split(path, "/")
	for i, pathSegment := range pathSegments {
		if (i == 0 || i == len(pathSegments)-1) && pathSegment == "" {
			continue
		} else if !pathSegmentRegex.MatchString(pathSegment) {
			return false
		}
	}
	return true
}

func multipleEdgePorts(paths []EdgePath) bool {
	if len(paths) <= 1 {
		return false
	}
	m := map[string]bool{}
	for i, val := range paths {
		if i != 0 {
			_, seen := m[val.ContainerPort]
			if !seen {
				//Got a new port so return true
				return true
			}
		}
		//Add the value to map only after we've checked it doesn't have new port
		m[val.ContainerPort] = true
	}
	return false
}

//TODO: Unit test this
func uniqueEdgePorts(paths []EdgePath) []string {
	// Use map to record duplicates as we find them.
	encountered := map[string]bool{}
	result := []string{}

	for _, v := range paths {
		_, seen := encountered[v.ContainerPort]
		if !seen {
			encountered[v.ContainerPort] = true
			result = append(result, v.ContainerPort)
		}
	}
	// Return the new slice.
	return result
}
