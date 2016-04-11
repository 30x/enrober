//TODO: Decide on better naming scheme
//TODO: Make sure all functions have proper description
//TODO: Make sure all functions have proper error handling

package enrober

import (
	"fmt"
	"strconv"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/restclient"
	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
)

//DeploymentManager is a wrapper type around kubernetes client
type DeploymentManager struct {
	client *k8sClient.Client
}

//ImageDeployment is a collection of necesarry resources for Replication Controller Deployments
//TODO: May have to add a secret name here?
type ImageDeployment struct {
	Namespace       string
	Application     string
	Revision        string
	TrafficHosts    []string
	PublicPaths     []string
	PathPort        string
	PodCount        int
	Image           string
	ImagePullSecret string
	EnvVars         map[string]string
}

//CreateDeploymentManager creates an instance of the DeploymentManager from the config passed in, and returns the instance
//TODO: Refactor this so we don't have to  pass around a restclient.Config
func CreateDeploymentManager(config restclient.Config) (*DeploymentManager, error) {
	//Function scoping client
	kubeclient := k8sClient.Client{}

	//No given config so use InClusterConfig
	if config.Host == "" {
		c, err := restclient.InClusterConfig()

		if err != nil {
			return nil, err
		}
		client, err := k8sClient.New(c)

		kubeclient = *client

	} else {
		//Creates client based on passed in config
		client, err := k8sClient.New(&config)

		if err != nil {
			return nil, err
		}

		kubeclient = *client
	}

	//Create the DeploymentManager
	DeploymentManager := &DeploymentManager{
		client: &kubeclient,
	}
	return DeploymentManager, nil
}

//CreateNamespace <description goes here>
//Returns a Namespace and an error
func (deploymentManager *DeploymentManager) CreateNamespace(imageDeployment ImageDeployment) (api.Namespace, error) {
	opt := &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name: imageDeployment.Namespace,
		},
	}
	ns, err := deploymentManager.client.Namespaces().Create(opt)
	if err != nil {
		return *ns, err //TODO: Better error handling
	}
	return *ns, nil
}

//TODO: Test if we can rename Namespaces

//GetNamespace <description goes here>
//Returns a Namespace and an error
func (deploymentManager *DeploymentManager) GetNamespace(imageDeployment ImageDeployment) (api.Namespace, error) {
	ns, err := deploymentManager.client.Namespaces().Get(imageDeployment.Namespace)
	if err != nil {
		return *ns, err //TODO: Better error handling
	}
	return *ns, nil
}

//DeleteNamespace <description goes here>
//Returns an error
func (deploymentManager *DeploymentManager) DeleteNamespace(imageDeployment ImageDeployment) error {
	ns := imageDeployment.Namespace
	err := deploymentManager.client.Namespaces().Delete(ns)
	if err != nil {
		return err //TODO: Better error handling
	}
	return nil
}

//constructDeployment creates a deployment object from the passed arguments and a default deployment template
func constructDeployment(imageDeployment ImageDeployment) extensions.Deployment {
	//Concatenate Annotations
	//TODO: This should be reviewed
	trafficHosts := ""
	for index, value := range imageDeployment.TrafficHosts {
		if index != 0 {
			trafficHosts += " " + value
		} else {
			trafficHosts += value
		}
	}

	publicPaths := ""
	for index, value := range imageDeployment.PublicPaths {
		if index != 0 {
			publicPaths += " " + value
		} else {
			publicPaths += value
		}
	}

	//TODO: Handle this error down the line
	intPathPort, err := strconv.Atoi(imageDeployment.PathPort)
	if err != nil {
		return extensions.Deployment{}
	}

	//Need to make sure the EnvVars map[string]string into a []api.EnvVar
	//TODO: This may be really stupid, review
	var keys []string
	var values []string
	for k, v := range imageDeployment.EnvVars {
		keys = append(keys, k)
		values = append(values, v)
	}

	envVarTemp := make([]api.EnvVar, len(keys))

	for index, value := range keys {
		envVarTemp[index].Name = value
	}
	for index, value := range values {
		envVarTemp[index].Value = value
	}

	portEnvVar := api.EnvVar{
		Name:  "PORT",
		Value: imageDeployment.PathPort,
	}
	envVarFinal := append(envVarTemp, portEnvVar)

	calicoPolicy := "allow tcp from label Application=" + imageDeployment.Application + " to ports " + imageDeployment.PathPort + "; allow tcp from app=nginx-ingress"

	depTemplate := extensions.Deployment{
		ObjectMeta: api.ObjectMeta{
			Name: imageDeployment.Application + "-" + imageDeployment.Revision, //May take variable
		},
		Spec: extensions.DeploymentSpec{
			Replicas: imageDeployment.PodCount,
			Selector: &unversioned.LabelSelector{ //Deployment Labels go here
				MatchLabels: map[string]string{
					"Namespace":   imageDeployment.Namespace,
					"Application": imageDeployment.Application,
					"Revision":    imageDeployment.Revision,
				},
			},
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: map[string]string{
						"Namespace":    imageDeployment.Namespace,
						"Application":  imageDeployment.Application,
						"Revision":     imageDeployment.Revision,
						"microservice": "true",
					},
					Annotations: map[string]string{
						//TODO: Should we make this optional?
						"projectcalico.org/policy": calicoPolicy,
						"trafficHosts":             trafficHosts,
						"publicPaths":              publicPaths,
						"pathPort":                 imageDeployment.PathPort,
					},
				},
				Spec: api.PodSpec{
					//TODO: Ensure that this works
					ImagePullSecrets: []api.LocalObjectReference{
						api.LocalObjectReference{
							Name: imageDeployment.ImagePullSecret,
						},
					},
					Containers: []api.Container{
						api.Container{
							Name:  imageDeployment.Application + "-" + imageDeployment.Revision,
							Image: imageDeployment.Image,
							Env:   envVarFinal,
							Ports: []api.ContainerPort{
								api.ContainerPort{
									ContainerPort: intPathPort,
								},
							},
							//ReadinessProbe goes here
							//TODO: Should we be implementing this? If so how?
							/*
								ReadinessProbe: &api.Probe{
									Handler: api.Handler{
										HTTPGet: &api.HTTPGetAction{
											Path: "/ready", //TODO: This should be determined based on an annotation
											Port: intstr.FromInt(8080),
										},
									},
								},*/
						},
					},
				},
			},
		},
		Status: extensions.DeploymentStatus{},
	}
	//TODO: Do we want this in final implementation?
	for key, val := range depTemplate.Spec.Template.Annotations {
		fmt.Printf(key + ": " + val + "\n") //Print annotations to stdout
	}
	return depTemplate
}

//GetDeployment <description goes here>
func (deploymentManager *DeploymentManager) GetDeployment(imageDeployment ImageDeployment) (extensions.Deployment, error) {
	dep, err := deploymentManager.client.Deployments(imageDeployment.Namespace).Get(imageDeployment.Application + "-" + imageDeployment.Revision)
	if err != nil {
		return *dep, err //TODO: Better error handling
	}
	return *dep, nil
}

//GetDeploymentList <description goes here>
func (deploymentManager *DeploymentManager) GetDeploymentList(imageDeployment ImageDeployment) (extensions.DeploymentList, error) {
	depList := &extensions.DeploymentList{}
	selector, err := labels.Parse("Application=" + imageDeployment.Application)

	//No application is passed
	if imageDeployment.Application == "" {
		depList, err = deploymentManager.client.Deployments(imageDeployment.Namespace).List(api.ListOptions{
			LabelSelector: labels.Everything(),
		})
		if err != nil {
			return *depList, err //TODO: Better error handling
		}
	} else {
		depList, err = deploymentManager.client.Deployments(imageDeployment.Namespace).List(api.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			return *depList, err //TODO: Better error handling
		}
	}
	return *depList, nil
}

//CreateDeployment <description goes here>
func (deploymentManager *DeploymentManager) CreateDeployment(imageDeployment ImageDeployment) (extensions.Deployment, error) {
	template := constructDeployment(imageDeployment)
	dep, err := deploymentManager.client.Deployments(imageDeployment.Namespace).Create(&template)
	if err != nil {
		return *dep, err //TODO: Better error handling
	}
	return *dep, nil
}

//UpdateDeployment <description goes here>
func (deploymentManager *DeploymentManager) UpdateDeployment(imageDeployment ImageDeployment) (extensions.Deployment, error) {
	template := constructDeployment(imageDeployment)
	dep, err := deploymentManager.client.Deployments(imageDeployment.Namespace).Update(&template)
	if err != nil {
		return *dep, err //TODO: Better error handling
	}
	return *dep, nil
}

// //ConstructSecret <description goes here>
// func (deploymentManager *DeploymentManager) ConstructSecret(name string) (api.Secret, error) {
// 	secretTemplate := api.Secret{
// 		ObjectMeta: api.ObjectMeta{
// 			Name: name,
// 		},
// 		Data: map[string][]byte{},
// 		Type: "Opaque",
// 	}
// 	return secretTemplate, nil
// }

// //CreateSecret <description goes here>
// func (deploymentManager *DeploymentManager) CreateSecret(imageDeployment ImageDeployment) (api.Secret, error) {
// 	template := ConstructSecret() //TODO
// 	secret, err := deploymentManager.client.Secrets(imageDeployment.Namespace).Create(&template)
// 	if err != nil {
// 		return *dep, err //TODO: Better error handling
// 	}
// 	return *dep, nil
// }
