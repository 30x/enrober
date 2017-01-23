package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"k8s.io/client-go/pkg/api/v1"

	"github.com/30x/enrober/pkg/apigee"
	"github.com/30x/enrober/pkg/helper"
)

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

	// Retrieve hostnames from Apigee api
	apigeeClient := apigee.Client{Token: token}

	//Should attempt KVM creation before creating k8s objects
	if apigeeKVM {
		err := apigeeClient.CreateKVM(apigeeOrgName, apigeeEnvName, publicKey)
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
				"runtime":  "shipyard",
				"edge/org": apigeeOrgName,
				"edge/env": apigeeEnvName,
				"name":     environmentName,
			},
			Annotations: nsAnnotations,
		},
	}

	//Create Namespace
	createdNs, err := clientset.Core().Namespaces().Create(nsObject)
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

	tempSecret.Data["public-api-key"] = []byte(publicKey)
	tempSecret.Data["private-api-key"] = []byte(privateKey)

	//Create Secret
	_, err = clientset.Core().Secrets(environmentName).Create(&tempSecret)
	if err != nil {
		helper.LogError.Printf("Error creating secret: %s\n", err)

		err = clientset.Core().Namespaces().Delete(createdNs.GetName(), &v1.DeleteOptions{})
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

func composePathsJSON(paths []EdgePath) string {
	if paths == nil {
		return ""
	}
	b, err := json.MarshalIndent(paths, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(b)
}
