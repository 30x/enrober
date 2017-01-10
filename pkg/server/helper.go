package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/client-go/1.5/pkg/api"
	"k8s.io/client-go/1.5/pkg/api/v1"

	"github.com/30x/enrober/pkg/apigee"
	"github.com/30x/enrober/pkg/helper"
)

// Lets start by just making a new function
func createEnvironment(environmentName, token string) error {

	//Make sure they passed a valid environment name of form {org}:{env}
	if !envNameRegex.MatchString(environmentName) {
		errorMessage := fmt.Sprintf("Not a valid environment name: %s\n", environmentName)
		return errors.New(errorMessage)
	}

	//Parse environment name into 2 parts
	nameSlice := strings.Split(environmentName, ":")
	apigeeOrgName := nameSlice[0]
	apigeeEnvName := nameSlice[1]

	// transform EnvironmentName into acceptable k8s namespace name
	environmentName = apigeeOrgName + "-" + apigeeEnvName

	//Generate both a public and private key
	privateKey, err := helper.GenerateRandomString(32)
	publicKey, err := helper.GenerateRandomString(32)
	if err != nil {
		errorMessage := fmt.Sprintf("Error generating random string: %v\n", err)
		return errors.New(errorMessage)
	}

	//Should attempt KVM creation before creating k8s objects
	if apigeeKVM {

		httpClient := &http.Client{}

		//construct URL
		apigeeKVMURL := fmt.Sprintf("%sv1/organizations/%s/environments/%s/keyvaluemaps", apigeeAPIHost, apigeeOrgName, apigeeEnvName)

		//create JSON body
		kvmBody := apigeeKVMBody{
			Name: apigeeKVMName,
			Entry: []apigeeKVMEntry{
				apigeeKVMEntry{
					Name:  apigeeKVMPKName,
					Value: base64.StdEncoding.EncodeToString([]byte(publicKey)),
				},
			},
		}

		b := new(bytes.Buffer)
		json.NewEncoder(b).Encode(kvmBody)

		req, err := http.NewRequest("POST", apigeeKVMURL, b)
		if err != nil {
			errorMessage := fmt.Sprintf("Unable to create request (Create KVM): %v", err)
			return errors.New(errorMessage)
		}

		//Must pass through the authz header
		req.Header.Add("Authorization", token)
		req.Header.Add("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			errorMessage := fmt.Sprintf("Error creating Apigee KVM: %v", err)
			return errors.New(errorMessage)
		}
		defer resp.Body.Close()

		//TODO: Probably want to generalize this logic

		// If the response was not a 201, we need to check if the response was a 409 because this means the KVM exists
		// already and we'll need to update the KVM value(s).
		if resp.StatusCode != 201 {
			var retryFlag bool

			// If the KVM already exists, we need to update its value(s).
			if resp.StatusCode == 409 {
				b2 := new(bytes.Buffer)
				updateKVMURL := fmt.Sprintf("%s/%s", apigeeKVMURL, apigeeKVMName) // Use non-CPS endpoint by default

				if isCPSEnabledForOrg(apigeeOrgName, token) {
					// When using CPS, the API endpoint is different and instead of sending the whole KVM body, we can only send
					// the KVM entry to update.  (This will work for now since we are only persisting one key but in the future
					// we might need to update this to make N calls, one per key.)
					updateKVMURL = fmt.Sprintf("%s/entries/%s", updateKVMURL, apigeeKVMPKName)

					json.NewEncoder(b2).Encode(kvmBody.Entry[0])
				} else {
					// When not using CPS, send the whole KVM body to update all keys in the KVM.
					json.NewEncoder(b2).Encode(kvmBody) // Non-CPS takes the whole payload
				}

				updateKVMReq, err := http.NewRequest("POST", updateKVMURL, b2)

				if err != nil {
					errorMessage := fmt.Sprintf("Unable to create request (Update KVM): %v", err)
					return errors.New(errorMessage)
				}

				fmt.Printf("The update KVM URL: %v\n", updateKVMReq.URL.String())

				updateKVMReq.Header.Add("Authorization", token)
				updateKVMReq.Header.Add("Content-Type", "application/json")

				resp2, err := httpClient.Do(updateKVMReq)
				if err != nil {
					errorMessage := fmt.Sprintf("Error creating entry in existing Apigee KVM: %v", err)
					return errors.New(errorMessage)
				}
				defer resp2.Body.Close()

				var updateKVMRes retryResponse

				//Decode response
				err = json.NewDecoder(resp2.Body).Decode(&updateKVMRes)

				if err != nil {
					errorMessage := fmt.Sprintf("Failed to decode response: %v\n", err)
					return errors.New(errorMessage)
				}

				// Updating a KVM returns a 200 on success so if it's not a 200, it's a failure
				if resp2.StatusCode != 200 {
					errorMessage := fmt.Sprintf("Couldn't create KVM entry (Status Code: %d): %v", resp2.StatusCode, updateKVMRes.Message)
					return errors.New(errorMessage)
				}

				retryFlag = true
			}

			if !retryFlag {
				errorMessage := fmt.Sprintf("Expected 201 or 409, got: %v", resp.StatusCode)
				return errors.New(errorMessage)
			}
		}

	}

	// Retrieve hostnames from Apigee api
	apigeeClient := apigee.Client{Token: token}
	var hosts []string
	if apigeeKVM {
		hosts, err = apigeeClient.Hosts(apigeeOrgName, apigeeEnvName)
		if err != nil {
			errorMessage := fmt.Sprintf("Error retrieving hostnames from Apigee : %v", err)
			return errors.New(errorMessage)
		}

	}

	//Should create an annotation object and pass it into the object literal
	nsAnnotations := make(map[string]string)

	if apigeeKVM {
		nsAnnotations["hostNames"] = strings.Join(hosts, " ")
	}

	//Add network policy annotation if we are isolating namespaces
	if isolateNamespace {
		nsAnnotations["net.beta.kubernetes.io/network-policy"] = `{"ingress": {"isolation": "DefaultDeny"}}`
	}

	//NOTE: Probably shouldn't create annotation if there are no hostNames
	nsObject := &v1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: environmentName,
			Labels: map[string]string{
				"runtime":      "shipyard",
				"organization": apigeeOrgName,
				"environment":  apigeeEnvName,
				"name":         environmentName,
			},
			Annotations: nsAnnotations,
		},
	}

	//Create Namespace
	createdNs, err := clientset.Core().Namespaces().Create(nsObject)
	if err != nil {
		errorMessage := fmt.Sprintf("Error creating namespace: %v", err)
		return errors.New(errorMessage)
	}
	//Print to console for logging
	helper.LogInfo.Printf("Created Namespace: %s\n", createdNs.GetName())

	tempSecret := v1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name: "routing",
		},
		Data: map[string][]byte{},
		Type: "Opaque",
	}

	tempSecret.Data["public-api-key"] = []byte(publicKey)
	tempSecret.Data["private-api-key"] = []byte(privateKey)

	//Create Secret
	_, err = clientset.Core().Secrets(environmentName).Create(&tempSecret)
	if err != nil {
		helper.LogError.Printf("Error creating secret: %s\n", err)

		err = clientset.Core().Namespaces().Delete(createdNs.GetName(), &api.DeleteOptions{})
		if err != nil {
			errorMessage := fmt.Sprintf("Failed to cleanup namespace\n")
			return errors.New(errorMessage)
		}
		errorMessage := fmt.Sprintf("Deleted namespace due to secret creation error\n")
		return errors.New(errorMessage)
	}
	return nil
}

func updateEnvironmentHosts(org, env, token string) error {
	ns, err := clientset.Core().Namespaces().Get(org + "-" + env)
	if err != nil {
		return err
	}

	apigeeClient := apigee.Client{Token: token}
	hosts, err := apigeeClient.Hosts(org, env)
	if err != nil {
		return err
	}

	ns.ObjectMeta.Annotations["hostNames"] = strings.Join(hosts, " ")
	_, err = clientset.Core().Namespaces().Update(ns)
	if err != nil {
		return err
	}

	return nil
}

func isCPSEnabledForOrg(orgName, authzHeader string) bool {
	cpsEnabled := false
	httpClient := &http.Client{}

	req, err := http.NewRequest("GET", fmt.Sprintf("%sv1/organizations/%s", apigeeAPIHost, orgName), nil)

	if err != nil {
		fmt.Printf("Error checking for CPS: %v", err)

		return cpsEnabled
	}

	fmt.Printf("Checking if %s has CPS enabled using URL: %v\n", orgName, req.URL.String())

	req.Header.Add("Authorization", authzHeader)

	res, err := httpClient.Do(req)

	defer res.Body.Close()

	if err != nil {
		fmt.Printf("Error checking for CPS: %v", err)
	} else {
		var rawOrg interface{}

		err := json.NewDecoder(res.Body).Decode(&rawOrg)

		if err != nil {
			fmt.Printf("Error unmarshalling response: %v\n", err)
		} else {
			org := rawOrg.(map[string]interface{})
			orgProps := org["properties"].(map[string]interface{})
			orgProp := orgProps["property"].([]interface{})

			for _, rawProp := range orgProp {
				prop := rawProp.(map[string]interface{})

				if prop["name"] == "features.isCpsEnabled" {
					if prop["value"] == "true" {
						cpsEnabled = true
					}

					break
				}
			}
		}
	}

	fmt.Printf("  CPS Enabled: %v", cpsEnabled)

	return cpsEnabled
}
