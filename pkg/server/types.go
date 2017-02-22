package server

import (
	"net/http"

	"github.com/30x/enrober/pkg/apigee"

	"k8s.io/client-go/pkg/api"
)

// State is an enum to select between local and in cluster state
type State string

const (
	// StateLocal is for local dev/testing
	StateLocal State = "local"

	// StateCluster is for when app is deployed in a cluster
	StateCluster State = "cluster"
)

//Server struct
type Server struct {
	Router http.Handler
}

//Temp struct for future host configuration
type HostsConfig struct {
}

type EdgePath struct {
	BasePath      string `json:"basePath"`
	ContainerPort string `json:"containerPort"`
	TargetPath    string `json:"targetPath,omitempty"`
}

type environmentResponse struct {
	Name      string                 `json:"name"`
	EdgeHosts map[string]HostsConfig `json:"edgeHosts,omitempty"`
	ApiSecret []byte                 `json:"apiSecret"`
}

type deploymentPost struct {
	DeploymentName string                `json:"deploymentName"`
	Paths          []EdgePath            `json:"edgePaths,omitempty"`
	Replicas       *int32                `json:"replicas"`
	Revision       int32                 `json:"revision"`
	EnvVars        []apigee.ApigeeEnvVar `json:"envVars,omitempty"`
}

type deploymentPatch struct {
	Paths    []EdgePath            `json:"edgePaths,omitempty"`
	Replicas *int32                `json:"replicas,omitempty"`
	Revision *int32                `json:"revision,omitempty"`
	EnvVars  []apigee.ApigeeEnvVar `json:"envVars,omitempty"`
}

type deploymentResponse struct {
	DeploymentName  string               `json:"deploymentName"`
	Replicas        int32                `json:"replicas"`
	PodTemplateSpec *api.PodTemplateSpec `json:"podTemplateSpec"`
}

type apigeeKVMEntry struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type apigeeKVMBody struct {
	Name  string           `json:"name"`
	Entry []apigeeKVMEntry `json:"entry"`
}

type retryResponse struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Contexts []string `json:"contexts"`
}
