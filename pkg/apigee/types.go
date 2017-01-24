package apigee

import "k8s.io/kubernetes/pkg/api"

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
	KVMRef           *ApigeeKVMSelector `json:"kvmRef"`
	FieldRef         *api.ObjectFieldSelector
	ResourceFieldRef *api.ResourceFieldSelector
	ConfigMapKeyRef  *api.ConfigMapKeySelector
	SecretKeyRef     *api.SecretKeySelector
}

type ApigeeKVMSelector struct {
	KvmName string `json:"kvmName"`
	Key     string `json:"key"`
}
