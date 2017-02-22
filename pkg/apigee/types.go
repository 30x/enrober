package apigee

import "k8s.io/client-go/pkg/api/v1"

type apigeeKVMEntry struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type apigeeKVMBody struct {
	Name  string           `json:"name"`
	Entry []apigeeKVMEntry `json:"entry"`
}

type ApigeeEnvVar struct {
	Name      string              `json:"name"`
	Value     string              `json:"value"`
	ValueFrom *ApigeeEnvVarSource `json:"valueFrom"`
}

type ApigeeEnvVarSource struct {
	EdgeConfigRef    *ApigeeKVMSelector `json:"edgeConfigRef"`
	FieldRef         *v1.ObjectFieldSelector
	ResourceFieldRef *v1.ResourceFieldSelector
	ConfigMapKeyRef  *v1.ConfigMapKeySelector
	SecretKeyRef     *v1.SecretKeySelector
}

type ApigeeKVMSelector struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type retryResponse struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Contexts []string `json:"contexts"`
}
