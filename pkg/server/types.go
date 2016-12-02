package server

import (
	"net/http"

	"k8s.io/kubernetes/pkg/api"
)

//Server struct
type Server struct {
	Router http.Handler
}

type environmentPost struct {
	EnvironmentName string   `json:"environmentName"`
	HostNames       []string `json:"hostNames,omitempty"`
}

type environmentPatch struct {
	HostNames []string `json:"hostNames"`
}

type environmentRequest struct {
	Name      string   `json:"name"`
	HostNames []string `json:"hostNames"`
}

type environmentResponse struct {
	Name          string   `json:"name"`
	HostNames     []string `json:"hostNames,omitempty"`
	PublicSecret  []byte   `json:"publicSecret"`
	PrivateSecret []byte   `json:"privateSecret"`
}

type deploymentPost struct {
	DeploymentName string       `json:"deploymentName"`
	PublicHosts    *string      `json:"publicHosts,omitempty"`
	PrivateHosts   *string      `json:"privateHosts,omitempty"`
	Replicas       *int32       `json:"replicas"`
	PtsURL         string       `json:"ptsURL,omitempty"`
	EnvVars        []api.EnvVar `json:"envVars,omitempty"`
}

type deploymentPatch struct {
	PublicHosts  *string      `json:"publicHosts,omitempty"`
	PrivateHosts *string      `json:"privateHosts,omitempty"`
	Replicas     *int32       `json:"replicas,omitempty"`
	PtsURL       string       `json:"ptsURL"`
	EnvVars      []api.EnvVar `json:"envVars,omitempty"`
}

type deploymentResponse struct {
	DeploymentName  string               `json:"deploymentName"`
	PublicHosts     string               `json:"publicHosts,omitempty"`
	PublicPaths     string               `json:"publicPaths,omitempty"`
	PrivateHosts    string               `json:"privateHosts,omitempty"`
	PrivatePaths    string               `json:"privatePaths,omitempty"`
	Replicas        int32                `json:"replicas"`
	Environment     string               `json:"environment"`
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
