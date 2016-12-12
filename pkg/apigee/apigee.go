package apigee

import (
	"os"
	"fmt"
	"errors"
	"sync"

	"net/http"
	"encoding/json"
)

const (
	// Default Apigee's api endpoint host
	DefaultApigeeHost = "api.enterprise.apigee.com"

	// Env Var to set overide default apigee api host
	EnvVarApigeeHost = "AUTH_API_HOST"
)

type Client struct {
	// Authorization token used in the Authorization header on each request
	token   string
	
	// Apigee api host
	apigeeApiHost  string
	
	// Shared httpClient for efficiency
	httpClient *http.Client
}


// Return an array of host strings for the apigee environment
// - Must first gather VirtualHosts from env and then GET on each VirtualHost
func (c *Client) Hosts(org, env string) ([]string, error) {
	c.initDefaults()

	hosts := []string{}

	//construct URL
	virtualHostsUrl := fmt.Sprintf("https://%s/v1/organizations/%s/environments/%s/virtualhosts", c.apigeeApiHost, org, env)
	resp, err := c.Get(virtualHostsUrl)
	if err != nil {
		errorMessage := fmt.Sprintf("Failed to make request for VirtualHosts: %v", err)
		return nil, errors.New(errorMessage)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errorMessage := fmt.Sprintf("Invalid response status code when getting VirtualHosts: Code %d", resp.StatusCode)
		return nil, errors.New(errorMessage)
	}

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
	for alias, _ := range tmpHosts {
		hosts = append(hosts, alias)
	}
	
	return hosts, nil
}

// Get all hosts from a VirtualHost of an apigee environment
func (c *Client) hostAliases(org, env, virtualHost string) ([]string, error) {
	c.initDefaults()

	hosts := []string{}

	//construct URL
	virtualHostsUrl := fmt.Sprintf("https://%s/v1/organizations/%s/environments/%s/virtualhosts/%s", c.apigeeApiHost, org, env, virtualHost)
	resp, err := c.Get(virtualHostsUrl)
	if err != nil {
		errorMessage := fmt.Sprintf("Failed to make request for VirtualHost: %v", err)
		return nil, errors.New(errorMessage)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errorMessage := fmt.Sprintf("Invalid response status code when getting VirtualHost: Code %d", resp.StatusCode)
		return nil, errors.New(errorMessage)
	}

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
	// Init httpClient used by all reqs for efficiency, can be used concurrently
	if c.httpClient == nil {
		c.httpClient = &http.Client{}
	}
	
	// If apigee api host is not set configure to defult from env
	if c.apigeeApiHost == "" {
		envVar := os.Getenv(EnvVarApigeeHost)
		if envVar == "" {
			c.apigeeApiHost = DefaultApigeeHost
		} else {
			c.apigeeApiHost = envVar
		}
	}
}

// Make HTTP GET to api server with supplied url, return http.Response
func (c *Client) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	//Must pass through the authz header
	req.Header.Add("Authorization", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	
	return resp, nil
}


//func (c Client) getVirtualHost(org, env, virtualhost string) ([]string, error) {
//}


/*


 client =: apigee.Client{}
 client.GetHosts(org, env)

GET https://api.enterprise.apigee.com/v1/organizations/adammagaluk1/environments/test/virtualhosts
> [ "default", "secure" ]

GET https://api.enterprise.apigee.com/v1/organizations/adammagaluk1/environments/test/virtualhosts/default
> {
    "hostAliases" : [ "adammagaluk1-test.apigee.net" ],
    "interfaces" : [ ],
    "listenOptions" : [ ],
    "name" : "default",
    "port" : "80"
  }
*/
