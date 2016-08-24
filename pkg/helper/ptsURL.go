package helper

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"k8s.io/kubernetes/pkg/api"
)

var (
	shipyardHost          string
	internalRouterHost    string
	shipyardPrivateSecret string
	apiRoutingKeyHeader   string
)

//GetPTSFromURL gets a pod template spec from a given URL
func GetPTSFromURL(ptsURLString string, request *http.Request) (api.PodTemplateSpec, error) {

	httpClient := &http.Client{}

	ptsURL, err := url.Parse(ptsURLString)
	if err != nil {
		errorMessage := fmt.Sprintf("Error parsing ptsURL\n")
		return api.PodTemplateSpec{}, errors.New(errorMessage)
	}

	//This could be moved up
	if os.Getenv("DEPLOY_STATE") == "PROD" {
		u, err := url.Parse(ptsURLString)
		if err != nil {
			errorMessage := fmt.Sprintf("Error parsing ptsURL: %s\n", err)
			return api.PodTemplateSpec{}, errors.New(errorMessage)
		}
		if u.Host != request.Host {
			errorMessage := fmt.Sprintf("Attempting to use PTS from unauthorized host: %v, expected: %v\n", u.Host, request.Host)
			return api.PodTemplateSpec{}, errors.New(errorMessage)
		}
	}

	//Get the necesarry environment variables
	shipyardHost = os.Getenv("SHIPYARD_HOST")
	internalRouterHost = os.Getenv("INTERNAL_ROUTER_HOST")
	shipyardPrivateSecret = os.Getenv("SHIPYARD_PRIVATE_SECRET")
	apiRoutingKeyHeader = os.Getenv("API_ROUTING_KEY_HEADER")
	if apiRoutingKeyHeader == "" {
		apiRoutingKeyHeader = "X-ROUTING-API-KEY"
	}

	internalRouterFlag := false

	if ptsURL.Host == shipyardHost {
		ptsURL.Host = internalRouterHost
		ptsURL.Scheme = "http"
		internalRouterFlag = true
	}

	req, err := http.NewRequest("GET", ptsURL.String(), nil)

	if internalRouterFlag {
		req.Host = shipyardHost
		req.Header.Add("Host", shipyardHost)
		req.Header.Add(apiRoutingKeyHeader, base64.StdEncoding.EncodeToString([]byte(shipyardPrivateSecret)))
	}
	req.Header.Add("Authorization", request.Header.Get("Authorization"))
	req.Header.Add("Content-Type", "application/json")

	urlJSON, err := httpClient.Do(req)
	if err != nil {
		errorMessage := fmt.Sprintf("Error retrieving pod template spec: %s\n", err)
		return api.PodTemplateSpec{}, errors.New(errorMessage)
	}
	defer urlJSON.Body.Close()

	if urlJSON.StatusCode != 200 {
		errorMessage := fmt.Sprintf("Expected 200 from ptsURL got: %v\n", urlJSON.StatusCode)
		return api.PodTemplateSpec{}, errors.New(errorMessage)
	}

	tempPTS := &api.PodTemplateSpec{}

	err = json.NewDecoder(urlJSON.Body).Decode(tempPTS)
	if err != nil {
		errorMessage := fmt.Sprintf("Error decoding PTS JSON Body: %s\n", err)
		return api.PodTemplateSpec{}, errors.New(errorMessage)
	}
	return *tempPTS, nil
}
