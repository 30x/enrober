package server

import (
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	//Top level clientset for server package
	clientset kubernetes.Clientset
)

//Init runs once
func Init(env State) error {

	//In Cluster Config
	//TODO: Use an enum here
	if env == "cluster" {
		tmpConfig, err := rest.InClusterConfig()
		if err != nil {
			return err
		}
		//Create the clientset
		tempClientset, err := kubernetes.NewForConfig(tmpConfig)
		if err != nil {
			return err
		}
		clientset = *tempClientset

		//Local Config
	} else {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		tmpConfig, err := config.ClientConfig()
		if err != nil {
			return err
		}
		//Create the clientset
		tempClientset, err := kubernetes.NewForConfig(tmpConfig)
		if err != nil {
			return err
		}
		clientset = *tempClientset

	}

	//Several features should be disabled for local testing
	if os.Getenv("DEPLOY_STATE") == "PROD" {

		if os.Getenv("ISOLATE_NAMESPACE") == "true" {
			isolateNamespace = true
		}

		//Set privileged container flag
		if os.Getenv("ALLOW_PRIV_CONTAINERS") == "true" {
			allowPrivilegedContainers = true
		}

		//Set apigeeKVM flag
		if os.Getenv("APIGEE_KVM") == "true" {
			apigeeKVM = true
		}
	}

	return nil
}
