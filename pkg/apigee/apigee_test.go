package apigee

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"net/http"
	"net/http/httptest"

	"k8s.io/client-go/pkg/api/v1"
)

func TestApigeeEnvtoK8s(t *testing.T) {
	k8sEnvSlice := []v1.EnvVar{
		{
			Name:  "testKey",
			Value: "testValue",
		},
	}

	apigeeEnvSlice := []ApigeeEnvVar{
		{
			Name:  "testKey",
			Value: "testValue",
		},
	}

	resultK8sEnv, err := ApigeeEnvtoK8s(apigeeEnvSlice)
	if err != nil {
		t.Fatalf("Error when calling ApigeeEnvtoK8s: %v.", err)
	}
	if !reflect.DeepEqual(resultK8sEnv, k8sEnvSlice) {
		t.Fatalf("Expected %v, got %v", k8sEnvSlice, resultK8sEnv)
	}

}

func TestK8sEnvtoApigee(t *testing.T) {
	k8sEnvSlice := []v1.EnvVar{
		{
			Name:  "testKey",
			Value: "testValue",
		},
	}

	apigeeEnvSlice := []ApigeeEnvVar{
		{
			Name:  "testKey",
			Value: "testValue",
		},
	}

	resultK8sEnv, err := K8sEnvtoApigee(k8sEnvSlice)
	if err != nil {
		t.Fatalf("Error when calling ApigeeEnvtoK8s: %v.", err)
	}
	if !reflect.DeepEqual(resultK8sEnv, apigeeEnvSlice) {
		t.Fatalf("Expected %v, got %v", apigeeEnvSlice, resultK8sEnv[0])
	}

}

func TestCacheK8sEnvVars(t *testing.T) {

	envSlice1 := []v1.EnvVar{
		{
			Name:  "testKey1",
			Value: "testValue1",
		},
	}

	envSlice2 := []v1.EnvVar{
		{
			Name:  "testKey2",
			Value: "testValue2",
		},
	}

	envSliceCombined := []v1.EnvVar{
		{
			Name:  "testKey1",
			Value: "testValue1",
		},
		{
			Name:  "testKey2",
			Value: "testValue2",
		},
	}

	resultEnvSlice := CacheK8sEnvVars(envSlice1, envSlice2)
	if !reflect.DeepEqual(resultEnvSlice, envSliceCombined) {
		t.Fatalf("Expected %v, got %v", envSliceCombined, resultEnvSlice)
	}
}

func TestCacheApigeeEnvVars(t *testing.T) {
	envSlice1 := []ApigeeEnvVar{
		{
			Name:  "testKey1",
			Value: "testValue1",
		},
	}

	envSlice2 := []ApigeeEnvVar{
		{
			Name:  "testKey2",
			Value: "testValue2",
		},
	}

	envSliceCombined := []ApigeeEnvVar{
		{
			Name:  "testKey1",
			Value: "testValue1",
		},
		{
			Name:  "testKey2",
			Value: "testValue2",
		},
	}

	resultEnvSlice := CacheApigeeEnvVars(envSlice1, envSlice2)
	if !reflect.DeepEqual(resultEnvSlice, envSliceCombined) {
		t.Fatalf("Expected %v, got %v", envSliceCombined, resultEnvSlice)
	}
}

func TestEnvReftoEnv(t *testing.T) {
	ts := startMockServer()
	defer ts.Close()

	mockSource := &ApigeeEnvVarSource{
		KVMRef: &ApigeeKVMSelector{
			KvmName: "kvm",
			Key:     "key1",
		},
	}
	client := Client{Token: "<token>", ApigeeAPIHost: ts.URL + "/"}
	env, err := EnvReftoEnv(mockSource, client, "org", "env")
	if err != nil {
		t.Fatalf("Error when calling EnvReftoEnv: %v.", err)
	}
	if env.Value != "value1" {
		t.Fatalf("Expected %s, got %s", "value1", env.Value)
	}

}
func TestClientCreateKVM(t *testing.T) {
	ts := startMockServer()
	defer ts.Close()

	client := Client{Token: "<token>", ApigeeAPIHost: ts.URL + "/"}
	err := client.CreateKVM("org", "env", "key1")
	if err != nil {
		t.Fatalf("Error when calling CreateKVM: %v.", err)
	}

}

func TestClientGetKVM(t *testing.T) {
	ts := startMockServer()
	defer ts.Close()

	client := Client{Token: "<token>", ApigeeAPIHost: ts.URL + "/"}
	key, err := client.GetKVMValue("org", "env", "kvm", "key1")
	if err != nil {
		t.Fatalf("Error when calling GetKVM: %v.", err)
	}
	expectedValue := "value1"
	if key != expectedValue {
		t.Fatalf("Expected %s, got %s", expectedValue, key)
	}

}

// Test Client.Hosts() - It must return all three hosts from "org-env" on both virtual hosts "default" and "secure"
// Starts a mock http server as the api endpoint.
func TestClientHosts(t *testing.T) {
	ts := startMockServer()
	defer ts.Close()

	client := Client{Token: "<token>", ApigeeAPIHost: ts.URL + "/"}
	hosts, err := client.Hosts("org", "env")
	if err != nil {
		t.Fatalf("Error when calling Hosts: %v.", err)
	}

	if len(hosts) != 3 {
		t.Fatalf("Expected aliases length to be 2 is %d", len(hosts))
	}

	set := make(map[string]bool)
	for _, v := range hosts {
		set[v] = true
	}

	expectedHosts := []string{"org-env.apigee.net", "api.example.com", "secure.api.example.com"}
	for _, v := range expectedHosts {
		if set[v] == false {
			t.Fatalf("Expected %s in hosts array.", v)
		}
	}
}

// Test Client.hostAliases() - It must return both hosts for the default virtualhost on "org-env"
// Starts a mock http server as the api endpoint
func TestClienthostAliases(t *testing.T) {
	ts := startMockServer()
	defer ts.Close()

	client := Client{Token: "<token>", ApigeeAPIHost: ts.URL + "/"}
	aliases, err := client.hostAliases("org", "env", "default")
	if err != nil {
		t.Fatalf("Error when calling hostAliases: %v.", err)
	}

	if len(aliases) != 2 {
		t.Fatalf("Expected aliases length to be 2")
	}

	if aliases[0] != "org-env.apigee.net" {
		t.Fatalf("Expected first host to be org-env.apigee.net")
	}

	if aliases[1] != "api.example.com" {
		t.Fatalf("Expected first host to be api.example.com")
	}
}

// Ensure the apigee api host usees the ENV variable if set.
func TestClientEnvApiHost(t *testing.T) {
	resetEnv(t)
	os.Setenv(EnvVarApigeeHost, "http://some.api.host/")
	client := Client{}
	client.initDefaults()
	if client.ApigeeAPIHost != "http://some.api.host/" {
		t.Fatalf("client.ApigeeAPIHost did not match expected was %s", client.ApigeeAPIHost)
	}
}

// When ApigeeAPIHost is supplied when creating the client object it must override the Env variable
func TestClientParamApiHost(t *testing.T) {
	resetEnv(t)
	os.Setenv(EnvVarApigeeHost, "http://some.api.host/")
	client := Client{ApigeeAPIHost: "https://some.other.host/"}
	client.initDefaults()
	if client.ApigeeAPIHost != "https://some.other.host/" {
		t.Fatalf("client.ApigeeAPIHost did not match expected was %s", client.ApigeeAPIHost)
	}
}

// Test Client.Hosts() - When org does not exist Hosts() should return an error and no hosts.
func TestClientHostError(t *testing.T) {
	resetEnv(t)
	ts := startMockServer()
	defer ts.Close()

	client := Client{Token: "<token>", ApigeeAPIHost: ts.URL}
	hosts, err := client.Hosts("not-an-org", "env")
	if err == nil {
		t.Fatalf("Error should be returned when org does not exist.")
	}

	if len(hosts) != 0 {
		t.Fatalf("Expected aliases length to be 0 is %d", len(hosts))
	}
}

// When ApigeeAPIHost is not supplied and no Env var is set, ApigeeAPIHost should be default val
func TestClientDefaultApiHost(t *testing.T) {
	resetEnv(t)
	client := Client{}
	client.initDefaults()
	if client.ApigeeAPIHost != DefaultApigeeHost {
		t.Fatalf("client.ApigeeAPIHost did not match expected was %s", client.ApigeeAPIHost)
	}
}

func TestCPSEnabledForOrgPass(t *testing.T) {
	ts := startMockServer()
	defer ts.Close()

	client := Client{Token: "<token>", ApigeeAPIHost: ts.URL + "/"}
	isCPS, err := client.CPSEnabledForOrg("cpsOn")
	if err != nil {
		t.Fatalf("Error should be returned when org does not exist.")
	}
	if !isCPS {
		t.Fatalf("client.CPSEnabledForOrg should return true, got %v", isCPS)
	}
}

func TestCPSEnabledForOrgFail(t *testing.T) {
	ts := startMockServer()
	defer ts.Close()

	client := Client{Token: "<token>", ApigeeAPIHost: ts.URL + "/"}
	isCPS, err := client.CPSEnabledForOrg("cpsOff")
	if err != nil {
		t.Fatalf("Error should be returned when org does not exist.")
	}
	if isCPS {
		t.Fatalf("client.CPSEnabledForOrg should return false, got %v", isCPS)
	}
}

// Starts mock httptest server that returns the used apigee resources, all other resources return 404
func startMockServer() *httptest.Server {

	var jsonOrganizationRespCPSOn = `{
		"properties": {
			"property": [
			{
				"name": "provisioningStatus",
				"value": "1"
			},
			{
				"name": "features.isCpsEnabled",
				"value": "true"
			}
			]
		}
	}`

	var jsonOrganizationRespCPSOff = `{
		"properties": {
			"property": [
			{
				"name": "provisioningStatus",
				"value": "1"
			},
			{
				"name": "features.isCpsEnabled",
				"value": "false"
			}
			]
		}
	}`

	var jsonKvmEntryResp = `{
		"name": "key1",
		"value": "value1"
	}`

	var jsonHostAliasesResp = `{
		"hostAliases" : [ "org-env.apigee.net", "api.example.com" ],
		"interfaces" : [ ],
		"listenOptions" : [ ],
		"name" : "default",
		"port" : "80"
	  }`

	var jsonSecureHostAliasesResp = `{
		"hostAliases" : [ "org-env.apigee.net", "secure.api.example.com" ],
		"interfaces" : [ ],
		"listenOptions" : [ ],
		"name" : "default",
		"port" : "80"
	  }`

	var jsonCreateKVMResp = `{
	  "encrypted": true,
	  "entry": [
		{
		  "name": "key1",
		  "value": "value1"
		}
	  ],
	  "name": "test"
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/organizations/org/environments/env/virtualhosts/default":
			fmt.Fprintln(w, jsonHostAliasesResp)
		case "/v1/organizations/org/environments/env/virtualhosts/secure":
			fmt.Fprintln(w, jsonSecureHostAliasesResp)
		case "/v1/organizations/org/environments/env/virtualhosts":
			fmt.Fprintln(w, "[\"default\",\"secure\"]")
		case "/v1/organizations/org/environments/env/keyvaluemaps/kvm/entries/key1":
			fmt.Fprintln(w, jsonKvmEntryResp)
		case "/v1/organizations/cpsOn", "/v1/organizations/org":
			fmt.Fprintln(w, jsonOrganizationRespCPSOn)
		case "/v1/organizations/cpsOff":
			fmt.Fprintln(w, jsonOrganizationRespCPSOff)
		case "/v1/organizations/org/environments/env/keyvaluemaps":
			if r.Method == "POST" {
				w.WriteHeader(409)
				fmt.Fprintln(w, jsonCreateKVMResp)
			} else {
				w.WriteHeader(404)
			}
		case "/v1/organizations/org/environments/env/keyvaluemaps/shipyard-routing/entries/x-routing-api-key":
			//w.WriteHeader(200)
			fmt.Fprintln(w, jsonKvmEntryResp)
		default:
			w.WriteHeader(404)
		}
	}))

	return ts
}

// Reset all used Env variables
func resetEnv(t *testing.T) {
	unsetEnv := func(name string) {
		err := os.Unsetenv(name)

		if err != nil {
			t.Fatalf("Unable to unset environment variable (%s): %v\n", name, err)
		}
	}

	unsetEnv(EnvVarApigeeHost)
}
