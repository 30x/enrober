package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Server Test", func() {
	ServerTests := func(testServer *Server, hostBase string) {

		client := &http.Client{}

		//Higher scoped secret value
		var globalPrivate string
		var globalPublic string

		It("Create Environment", func() {
			url := fmt.Sprintf("%s/environments", hostBase)

			jsonStr := []byte(`{"environmentName": "testorg1:testenv1", "hostNames": ["testhost1"]}`)
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)
			Expect(err).Should(BeNil(), "Shouldn't get an error on POST. Error: %v", err)

			respStore := environmentResponse{}

			err = json.NewDecoder(resp.Body).Decode(&respStore)
			Expect(err).Should(BeNil(), "Error decoding response: %v", err)

			//Store the private-api-key in higher scope
			globalPrivate = string(respStore.PrivateSecret)

			//Store the public-api-key in higher scope
			globalPublic = string(respStore.PublicSecret)

			Expect(respStore.PrivateSecret).ShouldNot(BeNil())
			Expect(respStore.PublicSecret).ShouldNot(BeNil())

			Expect(resp.StatusCode).Should(Equal(201), "Response should be 201 Created")
		})

		It("Create Environment with duplicated Host Name", func() {
			url := fmt.Sprintf("%s/environments", hostBase)

			jsonStr := []byte(`{"environmentName": "testorg2:testenv2", "hostNames": ["testhost1"]}`)
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on POST. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(500), "Response should be 500 Internal Server Error")
		})

		It("Update Environment", func() {
			url := fmt.Sprintf("%s/environments/testorg1:testenv1", hostBase)

			jsonStr := []byte(`{"hostNames": ["testhost2"]}`)
			req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)
			Expect(err).Should(BeNil(), "Shouldn't get an error on PATCH. Error: %v", err)

			respStore := environmentResponse{}

			err = json.NewDecoder(resp.Body).Decode(&respStore)
			Expect(err).Should(BeNil(), "Error decoding response: %v", err)

			//Make sure that private-api-key wasn't changed
			Expect(string(respStore.PrivateSecret)).Should(Equal(globalPrivate))

			//Make sure that public-api-key wasn't changed
			Expect(string(respStore.PublicSecret)).Should(Equal(globalPublic))

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")
		})

		It("Create Deployment from PTS URL", func() {
			url := fmt.Sprintf("%s/environments/testorg1:testenv1/deployments", hostBase)

			jsonStr := []byte(`{
				"deploymentName": "testdep1",
				"publicHosts": "deploy.k8s.public",
				"privateHosts": "deploy.k8s.private",
    			"replicas": 1,
    			"ptsURL": "https://api.myjson.com/bins/2p9z1",
				"envVars": [{
					"name": "test1",
					"value": "test3"
				},
				{
					"name": "test2",
					"value": "test4"
   				}] 
			}`)

			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on POST. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(201), "Response should be 201 Created")

		})

		It("Update Deployment from PTS URL", func() {
			//Need to wait a little before we run an update
			//Should look into a better fix
			time.Sleep(2000 * time.Millisecond)
			url := fmt.Sprintf("%s/environments/testorg1:testenv1/deployments/testdep1", hostBase)

			jsonStr := []byte(`{
				"replicas": 3,
				"ptsURL": "https://api.myjson.com/bins/119h9",
				"envVars": [{
					"name": "test1",
					"value": "test3"
				},
				{
					"name": "test2",
					"value": "test4"
				}] 
			}`)

			req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on PATCH. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")

		})

		It("Get Deployment testdep1", func() {
			url := fmt.Sprintf("%s/environments/testorg1:testenv1/deployments/testdep1", hostBase)

			req, err := http.NewRequest("GET", url, nil)

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on GET. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")

		})

		It("Get Environment", func() {
			url := fmt.Sprintf("%s/environments/testorg1:testenv1", hostBase)

			req, err := http.NewRequest("GET", url, nil)

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on GET. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")
		})

		It("Get Logs for Deployment testdep1", func() {
			//Need to wait for container to start
			time.Sleep(5000 * time.Millisecond)

			url := fmt.Sprintf("%s/environments/testorg1:testenv1/deployments/testdep1/logs", hostBase)

			req, err := http.NewRequest("GET", url, nil)

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on GET. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")
		})

		It("Delete Deployment testdep1", func() {
			url := fmt.Sprintf("%s/environments/testorg1:testenv1/deployments/testdep1", hostBase)

			req, err := http.NewRequest("DELETE", url, nil)

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on DELETE. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(204), "Response should be 200 OK")

		})

		It("Delete Environment", func() {
			url := fmt.Sprintf("%s/environments/testorg1:testenv1", hostBase)

			req, err := http.NewRequest("DELETE", url, nil)

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on DELETE. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(204), "Response should be 204 No Content")
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
func setup() (*Server, string, error) {
	kubeHost := os.Getenv("KUBE_HOST")
	testServer := NewServer()

	if kubeHost == "" {
		kubeHost = "127.0.0.1:8080"
	}

	err := SetState(StateLocal)
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

	hostBase := "http://localhost:9000"

	return testServer, hostBase, nil
}
