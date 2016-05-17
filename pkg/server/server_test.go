package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/30x/enrober/pkg/server"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/restclient"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Server Test", func() {
	ServerTests := func(testServer *server.Server, hostBase string) {

		client := &http.Client{}

		//Higher scoped secret value
		var globalPrivate string
		var globalPublic string

		It("Create Environment", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments", hostBase)

			jsonStr := []byte(`{"environmentName": "testenv1","publicSecret": true, "privateSecret": true, "hostNames": ["testhost1"]}`)
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)
			Expect(err).Should(BeNil(), "Shouldn't get an error on POST. Error: %v", err)

			tempSecret := api.Secret{
				Data: make(map[string][]byte),
			}

			err = json.NewDecoder(resp.Body).Decode(&tempSecret.Data)
			Expect(err).Should(BeNil(), "Error decoding response: %v", err)

			//Store the private-api-key in higher scope
			globalPrivate = string(tempSecret.Data["private-api-key"])

			//Store the public-api-key in higher scope
			globalPublic = string(tempSecret.Data["public-api-key"])

			Expect(tempSecret.Data).ShouldNot(BeNil())

			Expect(resp.StatusCode).Should(Equal(201), "Response should be 201 Created")
		})

		It("Create Environment with duplicated Host Name", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments", hostBase)

			jsonStr := []byte(`{"environmentName": "testenv2","publicSecret": true, "privateSecret": true, "hostNames": ["testhost1"]}`)
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on POST. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(500), "Response should be 500 Internal Server Error")
		})

		It("Update Environment to not change privateSecret", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1", hostBase)

			jsonStr := []byte(`{"publicSecret": true, "privateSecret": false, "hostNames": ["testhost1"]}`)
			req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)
			Expect(err).Should(BeNil(), "Shouldn't get an error on PATCH. Error: %v", err)

			tempSecret := api.Secret{
				Data: make(map[string][]byte),
			}

			err = json.NewDecoder(resp.Body).Decode(&tempSecret.Data)
			Expect(err).Should(BeNil(), "Error decoding response: %v", err)

			//Make sure that private-api-key wasn't changed
			Expect(string(tempSecret.Data["private-api-key"])).Should(Equal(globalPrivate))

			//Make sure that public-api-key was changed
			Expect(string(tempSecret.Data["public-api-key"])).ShouldNot(Equal(globalPublic))

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")
		})

		It("Create Deployment from PTS URL", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1/deployments", hostBase)

			jsonStr := []byte(`{
				"deploymentName": "testdep1",
				"publicHosts": "deploy.k8s.public",
				"publicPaths": "80:/ 90:/2",
				"privateHosts": "deploy.k8s.private",
				"privatePaths": "80:/ 90:/2",
    			"replicas": 1,
    			"ptsURL": "https://api.myjson.com/bins/2aot6"}`)

			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on POST. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(201), "Response should be 200 OK")

		})

		It("Update Deployment from PTS URL", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1/deployments/testdep1", hostBase)

			jsonStr := []byte(`{
    				"replicas": 3,
					"ptsURL": "https://api.myjson.com/bins/2aot6"
					}`)

			req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on PATCH. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")

		})

		It("Create Deployment from direct PTS", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1/deployments", hostBase)

			jsonStr := []byte(`{
				"deploymentName": "testdep2",
 				"publicHosts": "deploy.k8s.public",
				"publicPaths": "80:/ 90:/2",
				"privateHosts": "deploy.k8s.private",
				"privatePaths": "80:/ 90:/2",
    			"replicas": 1,
				"ptsURL": "https://api.myjson.com/bins/4nja0",
				"pts":     
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
						"name": "nginx",
						"labels": {
							"app": "web"
						}
					},
					"spec": {
						"containers": [{
							"name": "nginx",
							"image": "nginx",
							"env": [{
								"name": "PORT",
								"value": "80"
							}],
							"ports": [{
								"containerPort": 80
							}]
						}, {
							"name": "test",
							"image": "jbowen/testapp:v0",
							"env": [{
								"name": "PORT",
								"value": "90"
							}],
							"ports": [{
								"containerPort": 90
							}]
						}]
					}
				}
			}`)

			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on POST. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(201), "Response should be 200 OK")

		})

		It("Update Deployment from direct PTS", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1/deployments/testdep2", hostBase)

			jsonStr := []byte(`{
				"trafficHosts": "deploy.k8s.local",
				"replicas": 3,
				"pts":     
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
						"name": "nginx",
						"labels": {
							"app": "web"
						}
					},
					"spec": {
						"containers": [{
							"name": "nginx",
							"image": "nginx",
							"env": [{
								"name": "PORT",
								"value": "80"
							}],
							"ports": [{
								"containerPort": 80
							}]
						}, {
							"name": "test",
							"image": "jbowen/testapp:v0",
							"env": [{
								"name": "PORT",
								"value": "90"
							}],
							"ports": [{
								"containerPort": 90
							}]
						}]
					}
				}
			}`)

			req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on PATCH. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")

		})

		It("Get Deployment testdep1", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1/deployments/testdep1", hostBase)

			req, err := http.NewRequest("GET", url, nil)

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on GET. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")

		})

		It("Get Deployment testdep2", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1/deployments/testdep2", hostBase)

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

		It("Delete Deployment testdep1", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1/deployments/testdep1", hostBase)
			req, err := http.NewRequest("DELETE", url, nil)

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on DELETE. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")

		})

		It("Delete Deployment testdep2", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1/deployments/testdep2", hostBase)
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
