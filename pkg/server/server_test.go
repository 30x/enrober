package server_test

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/30x/enrober/pkg/server"

	"k8s.io/kubernetes/pkg/client/restclient"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Server Test", func() {
	ServerTests := func(testServer *server.Server, hostBase string) {

		client := &http.Client{}

		It("Create Environment", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments", hostBase)

			jsonStr := []byte(`{"environmentName": "testenv1","secret": "12345"}`)
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on POST. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(201), "Respone should be 201 Created")
		})

		It("Update Environment", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1", hostBase)

			jsonStr := []byte(`{"environmentName": "testenv","secret": "54321"}`)
			req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on PATCH. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")
		})

		It("Create Deployment", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1/deployments", hostBase)

			jsonStr := []byte(`{
				"deploymentName": "testdep1",
    			"trafficHosts": "deploy.k8s.local",
    			"trafficWeights": "weight1",
    			"replicas": 1,
    			"ptsURL": "https://api.myjson.com/bins/4nja0"}`)

			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on POST. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(201), "Response should be 200 OK")

		})

		It("Update Deployment", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1/deployments/testdep1", hostBase)

			jsonStr := []byte(`{
				    "trafficHosts": "deploy.k8s.local",
   					"trafficWeights": "weight1",
    				"replicas": 3,
					"ptsURL": "https://api.myjson.com/bins/4nja0"}`)

			req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on PATCH. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")

		})

		It("Get Deployment", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1/deployments/testdep1", hostBase)

			req, err := http.NewRequest("GET", url, nil)

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on GET. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")

		})

		It("Get Environment", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1", hostBase)
			req, err := http.NewRequest("GET", url, nil)

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on GET. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")
		})

		It("Delete Deployment", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1/deployments/testdep1", hostBase)
			req, err := http.NewRequest("DELETE", url, nil)

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on DELETE. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")

		})

		It("Delete Environment", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1", hostBase)

			req, err := http.NewRequest("DELETE", url, nil)

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on DELETE. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")
		})
	}

	Context("Local Testing", func() {
		server, hostBase, err := setup()
		if err != nil {
			Fail(fmt.Sprintf("Failed to start server %s", err))
		}

		ServerTests(server, hostBase)
	})
})

//Initialize a server for testing
func setup() (*server.Server, string, error) {
	testServer := server.NewServer()
	clientConfig := restclient.Config{
		Host: "127.0.0.1:8080",
	}
	err := server.Init(clientConfig)
	if err != nil {
		fmt.Printf("Error on init: %v\n", err)
	}

	//Start in background
	go func() {
		err := testServer.Start()

		if err != nil {
			fmt.Printf("Could not start server %s", err)
		}
	}()

	hostBase := "http://localhost:9000/beeswax/deploy/api/v1"

	return testServer, hostBase, nil
}

/*

//Path: /environmentGroups
//Method: GET
func TestGetEnvironmentGroups(t *testing.T) {
	//This should 404

}

//Path: /environmentGroups/{environmentGroupID
//Method: GET
func TestGetEnvironmentGroup(t *testing.T) {
	//This should 404

}

//Path: /environmentGroups/{environmentGroupID}/environments
//Method: GET
func TestGetEnvironments(t *testing.T) {

}

//Path: /environmentGroups/{environmentGroupID}/environments
//Method: POST
func TestCreateEnvironment(t *testing.T) {
	server, hostBase, err := setup()
	assert.Nil(t, err)
	t.Logf("Got %v, %v, %v\n", server.Router, hostBase, err)

	url := fmt.Sprintf("%s/environmentGroups/testgroup/environments", hostBase)
	t.Logf("URL: %v", url)

	client := &http.Client{}

	jsonStr := []byte(`{"environmentName": "testenv","secret": "12345"}`)
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonStr))

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, 201)

	t.Logf("Got Response: %v\n", resp.Body)
}

//Path: /environmentGroups/{environmentGroupID}/environments/{environment}
//Method: GET
func TestGetEnvironment(t *testing.T) {
	server, hostBase, err := setup()
	assert.Nil(t, err)
	t.Logf("Got %v, %v, %v\n", server.Router, hostBase, err)

	url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv", hostBase)
	t.Logf("URL: %v", url)

	client := &http.Client{}

	resp, err := client.Get(url)

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, 200)
}

//Path: /environmentGroups/{environmentGroupID}/environments/{environment}
//Method: PATCH
func TestUpdateEnvironment(t *testing.T) {
	server, hostBase, err := setup()
	assert.Nil(t, err)
	t.Logf("Got %v, %v, %v\n", server.Router, hostBase, err)

	url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv", hostBase)
	t.Logf("URL: %v", url)

	client := &http.Client{}

	jsonStr := []byte(`{"environmentName": "testenv","secret": "54321"}`)
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonStr))
	// resp, err := client.Update(url, "application/json", bytes.NewBuffer(jsonStr))
	resp, err := client.Do(req)

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, 200)

}

//Path: /environmentGroups/{environmentGroupID}/environments/{environment}
//Method: DELETE
func TestDeleteEnvironment(t *testing.T) {

}

//Path: /environmentGroups/{environmentGroupID}/environments/{environment}/deployments
//Method: GET
func TestGetDeployments(t *testing.T) {

}

//Path: /environmentGroups/{environmentGroupID}/environments/{environment}/deployments
//Method: POST
func TestCreateDeployment(t *testing.T) {

}

//Path: /environmentGroups/{environmentGroupID}/environments/{environment}/deployments/{deployment}
//Method: GET
func TestGetDeployment(t *testing.T) {

}

//Path: /environmentGroups/{environmentGroupID}/environments/{environment}/deployments/{deployment}
//Method: PATCH
func TestUpdateDeployment(t *testing.T) {

}

//Path: /environmentGroups/{environmentGroupID}/environments/{environment}/deployments/{deployment}
//Method: DELETE
func TestDeleteDeployment(t *testing.T) {

}
*/
