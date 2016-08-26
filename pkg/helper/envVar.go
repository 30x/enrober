package helper

import (
	"k8s.io/kubernetes/pkg/api"
)

//CacheEnvVars appends a list of new env vars to a given current list without duplication
func CacheEnvVars(currentEnvVars []api.EnvVar, newEnvVars []api.EnvVar) []api.EnvVar {

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
