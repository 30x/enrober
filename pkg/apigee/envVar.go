package apigee

import (
	"errors"
	"fmt"
	"k8s.io/client-go/pkg/api/v1"
)

// EnvReftoEnv converts an ApigeeEnvVarSource to an ApigeeEnvVar
func EnvReftoEnv(source *ApigeeEnvVarSource, client Client, org, env string) (ApigeeEnvVar, error) {
	val, err := client.GetKVMValue(org, env, source.EdgeConfigRef.Name, source.EdgeConfigRef.Key)
	if err != nil {
		return ApigeeEnvVar{}, err
	}

	return ApigeeEnvVar{
		Name:  source.EdgeConfigRef.Key,
		Value: val,
	}, nil
}

// ApigeeEnvtoK8s converts a slice of apigee specific env vars to a k8s compatible env var slice
func ApigeeEnvtoK8s(apigeeEnv []ApigeeEnvVar) ([]v1.EnvVar, error) {
	k8sEnv := make([]v1.EnvVar, len(apigeeEnv))
	for i, val := range apigeeEnv {
		k8sEnv[i].Name = val.Name
		k8sEnv[i].Value = val.Value
		if val.ValueFrom != nil {
			if val.ValueFrom.ConfigMapKeyRef != nil {
				k8sEnv[i].ValueFrom.ConfigMapKeyRef = val.ValueFrom.ConfigMapKeyRef
			}
			if val.ValueFrom.FieldRef != nil {
				k8sEnv[i].ValueFrom.FieldRef = val.ValueFrom.FieldRef
			}
			if val.ValueFrom.ResourceFieldRef != nil {
				k8sEnv[i].ValueFrom.ResourceFieldRef = val.ValueFrom.ResourceFieldRef
			}
			if val.ValueFrom.SecretKeyRef != nil {
				k8sEnv[i].ValueFrom.SecretKeyRef = val.ValueFrom.SecretKeyRef
			}
		}

	}
	return k8sEnv, nil
}

//K8sEnvtoApigee converts a slice of k8s compatible env vars to an apigee specific env var slice
func K8sEnvtoApigee(k8sEnv []v1.EnvVar) ([]ApigeeEnvVar, error) {
	apigeeEnv := make([]ApigeeEnvVar, len(k8sEnv))
	for i, val := range k8sEnv {
		apigeeEnv[i].Name = val.Name
		apigeeEnv[i].Value = val.Value
		if val.ValueFrom != nil {
			if val.ValueFrom.ConfigMapKeyRef != nil {
				apigeeEnv[i].ValueFrom.ConfigMapKeyRef = val.ValueFrom.ConfigMapKeyRef
			}
			if val.ValueFrom.FieldRef != nil {
				apigeeEnv[i].ValueFrom.FieldRef = val.ValueFrom.FieldRef
			}
			if val.ValueFrom.ResourceFieldRef != nil {
				apigeeEnv[i].ValueFrom.ResourceFieldRef = val.ValueFrom.ResourceFieldRef
			}
			if val.ValueFrom.SecretKeyRef != nil {
				apigeeEnv[i].ValueFrom.SecretKeyRef = val.ValueFrom.SecretKeyRef
			}
		}
	}
	return apigeeEnv, nil
}

//CacheK8sEnvVars appends a list of k8s env vars to a given current list without duplication
func CacheK8sEnvVars(currentEnvVars, newEnvVars []v1.EnvVar) []v1.EnvVar {

	//Check for envVar conflicts and prioritize ones from passed JSON.
	finalEnvVar := currentEnvVars

	//Keep track of which jsonVars modified vs need to be added
	jsonEnvLength := len(newEnvVars)
	trackArray := make([]bool, jsonEnvLength)

	//Add on any additional envVars
	for i, jsonVar := range newEnvVars {
		for j, cacheVar := range currentEnvVars {
			if cacheVar.Name == jsonVar.Name {
				finalEnvVar[j] = jsonVar
				trackArray[i] = true
			}
		}
		if trackArray[i] == false {
			finalEnvVar = append(finalEnvVar, jsonVar)
		}
	}
	return finalEnvVar
}

//CacheApigeeEnvVars appends a list of apigee env vars to a given current list without duplication
func CacheApigeeEnvVars(currentEnvVars, newEnvVars []ApigeeEnvVar) []ApigeeEnvVar {

	//Check for envVar conflicts and prioritize ones from passed JSON.
	finalEnvVar := currentEnvVars

	//Keep track of which jsonVars modified vs need to be added
	jsonEnvLength := len(newEnvVars)
	trackArray := make([]bool, jsonEnvLength)

	//Add on any additional envVars
	for i, jsonVar := range newEnvVars {
		for j, cacheVar := range currentEnvVars {
			if cacheVar.Name == jsonVar.Name {
				finalEnvVar[j] = jsonVar
				trackArray[i] = true
			}
		}
		if trackArray[i] == false {
			finalEnvVar = append(finalEnvVar, jsonVar)
		}
	}
	return finalEnvVar
}

//TODO: Should get an integration test with this since it has to talk to live KVM
func GetKVMVars(vars []ApigeeEnvVar, kvmEnabled bool, client Client, org, env string) ([]ApigeeEnvVar, error) {
	for index, val := range vars {
		if val.ValueFrom != nil {
			if val.ValueFrom.EdgeConfigRef != (&ApigeeKVMSelector{}) {
				if kvmEnabled {
					// Gotta go retrieve the value from apigee KVM
					// In the future we may support other ref types
					var err error // I don't know why this is needed
					vars[index], err = EnvReftoEnv(val.ValueFrom, client, org, env)
					if err != nil {
						errorMessage := fmt.Sprintf("Failed at EnvReftoEnv: %v\n", err)
						return nil, errors.New(errorMessage)
					}
				} else {
					errorMessage := fmt.Sprint("Requested KVM resource when KVM isn't enabled\n")
					return nil, errors.New(errorMessage)
				}
			}
		}
	}
	return vars, nil
}
