package apigee

import (
	"fmt"
	"os"
	"testing"
	
	"net/http"
	"net/http/httptest"
)

// Test Client.Hosts() - It must return all three hosts from "org-env" on both virtual hosts "default" and "secure"
// Starts a mock http server as the api endpoint.
func TestClientHosts(t *testing.T) {
	ts := startMockServer()
	defer ts.Close()

	client := Client{ token: "<token>", apigeeApiHost: ts.URL + "/" }
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

	client := Client{ token: "<token>", apigeeApiHost: ts.URL + "/" }
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
	if client.apigeeApiHost != "http://some.api.host/" {
		t.Fatalf("client.apigeeApiHost did not match expected was %s", client.apigeeApiHost)
	}
}

// When apigeeApiHost is supplied when creating the client object it must override the Env variable
func TestClientParamApiHost(t *testing.T) {
	resetEnv(t)
	os.Setenv(EnvVarApigeeHost, "http://some.api.host/")
	client := Client{ apigeeApiHost: "https://some.other.host/"}
	client.initDefaults()
	if client.apigeeApiHost != "https://some.other.host/" {
		t.Fatalf("client.apigeeApiHost did not match expected was %s", client.apigeeApiHost)
	}
}

// Test Client.Hosts() - When org does not exist Hosts() should return an error and no hosts.
func TestClientHostError(t *testing.T) {
	ts := startMockServer()
	defer ts.Close()

	client := Client{ token: "<token>", apigeeApiHost: ts.URL }
	hosts, err := client.Hosts("not-an-org", "env")
	if err == nil {
		t.Fatalf("Error should be returned when org does not exist.")
	}

	if len(hosts) != 0 {
		t.Fatalf("Expected aliases length to be 0 is %d", len(hosts))
	}
}

// When apigeeApiHost is not supplied and no Env var is set, apigeeApiHost should be default val
func TestClientDefaultApiHost(t *testing.T) {
	resetEnv(t)
	client := Client{}
	client.initDefaults()
	if client.apigeeApiHost != DefaultApigeeHost {
		t.Fatalf("client.apigeeApiHost did not match expected was %s", client.apigeeApiHost)
	}
}

// Starts mock httptest server thar returns the used apigee resources, all other resources return 404
func startMockServer() *httptest.Server {
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
	
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/organizations/org/environments/env/virtualhosts/default" {
			fmt.Fprintln(w, jsonHostAliasesResp)
		} else if r.URL.Path == "/v1/organizations/org/environments/env/virtualhosts/secure" {
			fmt.Fprintln(w, jsonSecureHostAliasesResp)
		} else if r.URL.Path == "/v1/organizations/org/environments/env/virtualhosts" {
			fmt.Fprintln(w, "[\"default\",\"secure\"]")
		} else {
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

