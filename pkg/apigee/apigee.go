package apigee

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sync"

	"bytes"
	"encoding/json"
	"net/http"
)

const (
	// DefaultApigeeHost is Apigee's default api endpoint host
	DefaultApigeeHost = "https://api.enterprise.apigee.com/"

	// EnvVarApigeeHost is the Env Var to set overide default apigee api host
	EnvVarApigeeHost = "AUTH_API_HOST"
)

// Client is the client struct for interacting with external Apigee APIs
type Client struct {
	// Authorization token used in the Authorization header on each request
	Token string

	// Apigee api host
	ApigeeAPIHost string

	// Shared HTTPClient for efficiency
	HTTPClient *http.Client
}

// GetKVMValue returns a string that corresponds to the value of a named KVM and Key
func (c *Client) GetKVMValue(org, env, kvmName, key string) (string, error) {
	c.initDefaults()

	kvmURL := fmt.Sprintf("%sv1/organizations/%s/environments/%s/keyvaluemaps/%s/entries/%s", c.ApigeeAPIHost, org, env, kvmName, key)

	resp, err := c.Get(kvmURL)
	if err != nil {
		errorMessage := fmt.Sprintf("Failed to make request for KVM: %v", err)
		return "", errors.New(errorMessage)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errorMessage := fmt.Sprintf("Invalid response status code when getting KVM: Code %d", resp.StatusCode)
		return "", errors.New(errorMessage)
	}
	kvmEntry := apigeeKVMEntry{}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&kvmEntry)
	if err != nil {
		errorMessage := fmt.Sprintf("Error decoding json response from KVM: %v", err)
		return "", errors.New(errorMessage)
	}

	return kvmEntry.Value, nil

}

// Hosts returns an array of host strings for the apigee environment
// - Must first gather VirtualHosts from env and then GET on each VirtualHost
func (c *Client) Hosts(org, env string) ([]string, error) {
	c.initDefaults()

	hosts := []string{}

	//construct URL
	virtualHostsURL := fmt.Sprintf("%sv1/organizations/%s/environments/%s/virtualhosts", c.ApigeeAPIHost, org, env)
	resp, err := c.Get(virtualHostsURL)
	if err != nil {
		errorMessage := fmt.Sprintf("Failed to make request for VirtualHosts: %v", err)
		return nil, errors.New(errorMessage)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errorMessage := fmt.Sprintf("Invalid response status code when getting VirtualHosts: Code %d", resp.StatusCode)
		return nil, errors.New(errorMessage)
	}

	// Response should look like
	// GET https://api.enterprise.apigee.com/v1/organizations/<org>/environments/test/virtualhosts
	// > [ "default", "secure" ]

	virtualHosts := []string{}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&virtualHosts)
	if err != nil {
		errorMessage := fmt.Sprintf("Error decoding json response from VirtualHosts: %v", err)
		return nil, errors.New(errorMessage)
	}

	// Create a map[string] to store hosts to filter duplicate hosts out
	tmpHosts := make(map[string]struct{})

	var wg sync.WaitGroup
	errChannel := make(chan error, 1)
	finishedChannel := make(chan bool, 1)

	for _, virtualHost := range virtualHosts {
		wg.Add(1)
		go func(virtualHost string) {
			defer wg.Done()
			aliases, err := c.hostAliases(org, env, virtualHost)
			// If error send err to the error channel
			if err != nil {
				errChannel <- err
			}
			for _, alias := range aliases {
				// Add to host map to filter out duplicate hosts
				tmpHosts[alias] = struct{}{}
			}
		}(virtualHost)
	}

	// Waiting forever is okay because of the blocking select below.
	// Once gorutine finishes close the finishedChannel
	go func() {
		wg.Wait()
		close(finishedChannel)
	}()

	// Blocks until either all gorutines are finished or a error occurs
	select {
	case <-finishedChannel:
	case err := <-errChannel:
		if err != nil {
			return nil, err
		}
	}

	// Convert map to array strings for return
	for alias := range tmpHosts {
		hosts = append(hosts, alias)
	}

	return hosts, nil
}

// Get all hosts from a VirtualHost of an apigee environment
func (c *Client) hostAliases(org, env, virtualHost string) ([]string, error) {
	c.initDefaults()

	hosts := []string{}

	//construct URL
	virtualHostsURL := fmt.Sprintf("%sv1/organizations/%s/environments/%s/virtualhosts/%s", c.ApigeeAPIHost, org, env, virtualHost)
	resp, err := c.Get(virtualHostsURL)
	if err != nil {
		errorMessage := fmt.Sprintf("Failed to make request for VirtualHost: %v", err)
		return nil, errors.New(errorMessage)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errorMessage := fmt.Sprintf("Invalid response status code when getting VirtualHost: Code %d", resp.StatusCode)
		return nil, errors.New(errorMessage)
	}

	// Response should look like:
	// GET https://api.enterprise.apigee.com/v1/organizations/<org>/environments/test/virtualhosts/default
	// > {
	//     "hostAliases" : [ "<org>-test.apigee.net" ],
	//     "interfaces" : [ ],
	//     "listenOptions" : [ ],
	//     "name" : "default",
	//     "port" : "80"
	//   }

	data := map[string]interface{}{}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&data)
	if err != nil {
		errorMessage := fmt.Sprintf("Error decoding json response from VirtualHost: %v", err)
		return nil, errors.New(errorMessage)
	}

	if data["hostAliases"] == nil {
		return nil, errors.New("Missing hostAliases property in the response of VirtualHost")
	}

	for _, v := range data["hostAliases"].([]interface{}) {
		hosts = append(hosts, v.(string))
	}

	return hosts, nil
}

// CPSEnabledForOrg returns a bool indicating whether the requested Org has CPS enabled or not
func (c *Client) CPSEnabledForOrg(orgName string) (bool, error) {
	c.initDefaults()

	orgURL := fmt.Sprintf("%sv1/organizations/%s", c.ApigeeAPIHost, orgName)
	resp, err := c.Get(orgURL)
	if err != nil {
		return false, err
	}

	defer resp.Body.Close()

	var rawOrg interface{}

	err = json.NewDecoder(resp.Body).Decode(&rawOrg)

	if err != nil {
		fmt.Printf("Error unmarshalling response: %v\n", err)
		return false, err
	}

	org := rawOrg.(map[string]interface{})
	orgProps := org["properties"].(map[string]interface{})
	orgProp := orgProps["property"].([]interface{})

	for _, rawProp := range orgProp {
		prop := rawProp.(map[string]interface{})
		if prop["name"] == "features.isCpsEnabled" {
			if prop["value"] == "true" {
				return true, nil
			}
			break
		}
	}
	return false, nil
}

func (c *Client) CreateKVM(orgName, envName, publicKey string) error {
	const apigeeKVMName = "shipyard-routing"
	const apigeeKVMPKName = "x-routing-api-key"

	httpClient := &http.Client{}

	//construct URL
	kvmURL := fmt.Sprintf("%sv1/organizations/%s/environments/%s/keyvaluemaps", c.ApigeeAPIHost, orgName, envName)

	//create JSON body
	kvmBody := apigeeKVMBody{
		Name: apigeeKVMName,
		Entry: []apigeeKVMEntry{
			{
				Name:  apigeeKVMPKName,
				Value: base64.StdEncoding.EncodeToString([]byte(publicKey)),
			},
		},
	}

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(kvmBody)

	req, err := http.NewRequest("POST", kvmURL, b)
	if err != nil {
		errorMessage := fmt.Sprintf("Unable to create request (Create KVM): %v", err)
		return errors.New(errorMessage)
	}

	//Must pass through the authz header
	req.Header.Add("Authorization", c.Token)
	req.Header.Add("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		errorMessage := fmt.Sprintf("Error creating Apigee KVM: %v", err)
		return errors.New(errorMessage)
	}
	defer resp.Body.Close()

	// If the response was not a 201, we need to check if the response was a 409 because this means the KVM exists
	// already and we'll need to update the KVM value(s).
	if resp.StatusCode != 201 {
		var retryFlag bool

		// If the KVM already exists, we need to update its value(s).
		if resp.StatusCode == 409 {
			b2 := new(bytes.Buffer)
			updateKVMURL := fmt.Sprintf("%s/%s", kvmURL, apigeeKVMName) // Use non-CPS endpoint by default

			cpsEnabled, err := c.CPSEnabledForOrg(orgName)
			if err != nil {
				return err
			}
			if cpsEnabled {
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

			updateKVMReq.Header.Add("Authorization", c.Token)
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
	return nil
}

func (c *Client) initDefaults() {
	// Init HTTPClient used by all reqs for efficiency, can be used concurrently
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}

	// If apigee api host is not set configure to default from env
	if c.ApigeeAPIHost == "" {
		envVar := os.Getenv(EnvVarApigeeHost)
		if envVar == "" {
			c.ApigeeAPIHost = DefaultApigeeHost
		} else {
			c.ApigeeAPIHost = envVar
		}
	}
}

// Get makes HTTP GET to api server with supplied url, return http.Response
func (c *Client) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	//Must pass through the authz header
	req.Header.Add("Authorization", c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
