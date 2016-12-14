package apigee

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"encoding/json"
	"net/http"
)

const (
	// DefaultApigeeHost is Apigees defai;t api endpoint host
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

// GetKVM returns a map that corresponds to the key:value of a named LVM
func (c *Client) GetKVM(org, env, kvmName string) (map[string]string, error) {
	c.initDefaults()

	kvmURL := fmt.Sprintf("%sv1/organizations/%s/environments/%s/keyvaluemaps/%s", c.ApigeeAPIHost, org, env, kvmName)
	resp, err := c.Get(kvmURL)
	if err != nil {
		errorMessage := fmt.Sprintf("Failed to make request for KVM: %v", err)
		return nil, errors.New(errorMessage)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errorMessage := fmt.Sprintf("Invalid response status code when getting KVM: Code %d", resp.StatusCode)
		return nil, errors.New(errorMessage)
	}
	kvmBody := apigeeKVMBody{}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&kvmBody)
	if err != nil {
		errorMessage := fmt.Sprintf("Error decoding json response from KVM: %v", err)
		return nil, errors.New(errorMessage)
	}

	kvmValues := make(map[string]string)
	for i := range kvmBody.Entry {
		kvmValues[kvmBody.Entry[i].Name] = kvmBody.Entry[i].Value
	}

	return kvmValues, nil

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

func (c *Client) initDefaults() {
	// Init HTTPClient used by all reqs for efficiency, can be used concurrently
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}

	// If apigee api host is not set configure to defult from env
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
