package server

import (
	"os"

	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/rest"
	"k8s.io/client-go/1.5/tools/clientcmd"
)

var (
	//Top level clientset for server package
	clientset kubernetes.Clientset
)

//SetState determines whether the server is running locally or in a cluster
func SetState(env State) error {

	//In Cluster Config
	if env == StateCluster {
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
