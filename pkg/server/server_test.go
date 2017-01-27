package server

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"
)

var _ = Describe("Server Test", func() {
	ServerTests := func(testServer *Server, hostBase string) {

		client := &http.Client{}

		It("Get OK Status", func() {
			url := fmt.Sprintf("%s/environments/status", hostBase)

			req, err := http.NewRequest("GET", url, nil)

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on GET. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200")
		})

		It("Create Deployment from PTS URL", func() {
			url := fmt.Sprintf("%s/environments/testorg1:testenv1/deployments", hostBase)

			jsonStr := []byte(`{
				"deploymentName": "testdep1",
    			"replicas": 1,
    			"edgePaths": [{
    				"basePath": "base",
    				"containerPort": "9000",
    				"targetPath": "target"
				}],
    			"revision": 1,
				"envVars": [{
					"name": "test1",
					"value": "value1"
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
				"edgePaths": [{
    				"basePath": "base",
    				"containerPort": "9000",
    				"targetPath": "target"
				}],
				"ptsURL": "https://api.myjson.com/bins/119h9",
				"envVars": [{
					"name": "test1",
					"value": "value2"
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

		It("Create Duplicate Deployment", func() {
			url := fmt.Sprintf("%s/environments/testorg1:testenv1/deployments", hostBase)

			jsonStr := []byte(`{
				"deploymentName": "testdep1",
    			"replicas": 1,
    			"edgePaths": [{
    				"basePath": "base",
    				"containerPort": "9000",
    				"targetPath": "target"
				}],
    			"ptsURL": "https://api.myjson.com/bins/2p9z1",
				"envVars": [{
					"name": "test1",
					"value": "value1"
				}]
			}`)

			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on POST. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(409), "Response should be 409 conflict")
		})

		It("Get Environment", func() {
			url := fmt.Sprintf("%s/environments/testorg1:testenv1", hostBase)

			req, err := http.NewRequest("GET", url, nil)

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on GET. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")
		})

		It("Update Environment", func() {
			url := fmt.Sprintf("%s/environments/testorg1:testenv1", hostBase)

			req, err := http.NewRequest("PATCH", url, bytes.NewBuffer([]byte("")))

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on PATCH. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")
		})

		It("Get Deployments", func() {
			url := fmt.Sprintf("%s/environments/testorg1:testenv1/deployments", hostBase)

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

	}

	Context("Local Testing", func() {
		server, hostBase, err := setup()
		os.Setenv("PTS_URL_HOST_RESTRICTION", "false")
		if err != nil {
			Fail(fmt.Sprintf("Failed to start server %s", err))
		}

		ServerTests(server, hostBase)
	})
})

//Initialize a server for testing
func setup() (*Server, string, error) {

	testServer := NewServer()

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
