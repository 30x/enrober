package server

import "k8s.io/kubernetes/pkg/api"

type environmentPost struct {
	EnvironmentName string   `json:"environmentName"`
	HostNames       []string `json:"hostNames,omitempty"`
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
	DeploymentName string               `json:"deploymentName"`
	PublicHosts    *string              `json:"publicHosts,omitempty"`
	PrivateHosts   *string              `json:"privateHosts,omitempty"`
	Replicas       int                  `json:"replicas"`
	PtsURL         string               `json:"ptsURL,omitempty"`
	PTS            *api.PodTemplateSpec `json:"pts,omitempty"`
	EnvVars        []api.EnvVar         `json:"envVars,omitempty"`
}

type deploymentPatch struct {
	PublicHosts  *string              `json:"publicHosts,omitempty"`
	PrivateHosts *string              `json:"privateHosts,omitempty"`
	Replicas     int                  `json:"Replicas"`
	PtsURL       string               `json:"ptsURL"`
	PTS          *api.PodTemplateSpec `json:"pts"`
	EnvVars      []api.EnvVar         `json:"envVars,omitempty"`
}

type deploymentResponse struct {
	DeploymentName  string               `json:"deploymentName"`
	PublicHosts     string               `json:"publicHosts,omitempty"`
	PublicPaths     string               `json:"publicPaths,omitempty"`
	PrivateHosts    string               `json:"privateHosts,omitempty"`
	PrivatePaths    string               `json:"privatePaths,omitempty"`
	Replicas        int                  `json:"replicas"`
	Environment     string               `json:"environment"`
	PodTemplateSpec *api.PodTemplateSpec `json:"podTemplateSpec"`
}
