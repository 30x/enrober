package helper

import (
	"strings"

	"k8s.io/kubernetes/pkg/api"
	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"
)

//UniqueHostNames checks if the desired hostNames are unique among existing namespaces
func UniqueHostNames(hostNames []string, client k8sClient.Client) (bool, error) {
	for _, value := range hostNames {
		//Get list of all namespace and loop through each of their "validHosts" annotation looking for strings matching our value
		nsList, err := client.Namespaces().List(api.ListOptions{})
		if err != nil {
			return false, err
		}
		for _, ns := range nsList.Items {
			//Make sure validHosts annotation exists
			if val, ok := ns.Annotations["hostNames"]; ok {
				//Get the hostsList annotation
				if strings.Contains(val, value) {
					return false, nil
				}
			}
		}
	}
	return true, nil
}
