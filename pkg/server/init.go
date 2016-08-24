package server

import (
	"os"

	"k8s.io/kubernetes/pkg/client/restclient"

	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"
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

		//Set privileged container flag
		if os.Getenv("ALLOW_PRIV_CONTAINERS") == "true" {
			allowPrivilegedContainers = true
		} else {
			allowPrivilegedContainers = false
		}

	} else {
		allowPrivilegedContainers = false
	}

	return nil
}
