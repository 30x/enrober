package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/30x/enrober/pkg/server"

	"k8s.io/kubernetes/pkg/client/restclient"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type environmentRequest struct {
	Name      string   `json:"name"`
	HostNames []string `json:"hostNames"`
}

type environmentResponse struct {
	Name          string   `json:"name"`
	HostNames     []string `json:"hostNames,omitempty"`
	PublicSecret  []byte   `json:"publicSecret"`
	PrivateSecret []byte   `json:"privateSecret"`
}

var _ = Describe("Server Test", func() {
	ServerTests := func(testServer *server.Server, hostBase string) {

		client := &http.Client{}

		//Higher scoped secret value
		var globalPrivate string
		var globalPublic string

		It("Create Environment", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments", hostBase)

			jsonStr := []byte(`{"environmentName": "testenv1", "hostNames": ["testhost1"]}`)
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
			req.Header.Add("Authorization", `Bearer eyJhbGciOiJSUzI1NiJ9.eyJqdGkiOiI1MmI5MjRlZS0wZTQwLTRjNjktYmZiMC04MjEwN2Q3ZWM2MGQiLCJzdWIiOiJlNTdjNDg1MS04YTJlLTRlYzItYmEyYy1jY2RiMDJiYjIwNmMiLCJzY29wZSI6WyJzY2ltLm1lIiwib3BlbmlkIiwicGFzc3dvcmQud3JpdGUiLCJhcHByb3ZhbHMubWUiLCJvYXV0aC5hcHByb3ZhbHMiXSwiY2xpZW50X2lkIjoiZWRnZWNsaSIsImNpZCI6ImVkZ2VjbGkiLCJhenAiOiJlZGdlY2xpIiwiZ3JhbnRfdHlwZSI6InBhc3N3b3JkIiwidXNlcl9pZCI6ImU1N2M0ODUxLThhMmUtNGVjMi1iYTJjLWNjZGIwMmJiMjA2YyIsIm9yaWdpbiI6InVzZXJncmlkIiwidXNlcl9uYW1lIjoiamJvd2VuQGFwaWdlZS5jb20iLCJlbWFpbCI6Impib3dlbkBhcGlnZWUuY29tIiwiYXV0aF90aW1lIjoxNDY0Nzk2MDIyLCJyZXZfc2lnIjoiNGVlODcwYjgiLCJpYXQiOjE0NjQ3OTYwMjIsImV4cCI6MTQ2NDc5NzgyMiwiaXNzIjoiaHR0cHM6Ly9sb2dpbi5hcGlnZWUuY29tL29hdXRoL3Rva2VuIiwiemlkIjoidWFhIiwiYXVkIjpbImVkZ2VjbGkiLCJzY2ltIiwib3BlbmlkIiwicGFzc3dvcmQiLCJhcHByb3ZhbHMiLCJvYXV0aCJdfQ.LyP58pOr-lj1yIdaLGH_oCDm-HivLIiBM-wgETiFXALi_ZQLoJTqvLLFmgzmspShAyrbHfbrALrHH5DwRHE3skU6L8mnetTA0Q9cBZHq2pgkNsnw7S2K8A1jMZQEuijnoOZbnQ1v2JhgbHtnr-b_aLMT7WS58oWx0yj6mqL242P3AHdo1RHVmig_jNEn6fiVTCU9pTx9IuX5uB8RYahWhfI6T2bs4EqPdUr0MdwGhu6QPgP2cPNITdzfj2u9xXIUXOzYBxLGc6rPVIpmxKqEbjkdkVOqoNOof75AkegXkULBZk6so50yNGsEkBk4aChg5gDR-KRw4a1yYkNLMJ__vg`)

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
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments", hostBase)

			jsonStr := []byte(`{"environmentName": "testenv2", "hostNames": ["testhost1"]}`)
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
			req.Header.Add("Authorization", `Bearer eyJhbGciOiJSUzI1NiJ9.eyJqdGkiOiI1MmI5MjRlZS0wZTQwLTRjNjktYmZiMC04MjEwN2Q3ZWM2MGQiLCJzdWIiOiJlNTdjNDg1MS04YTJlLTRlYzItYmEyYy1jY2RiMDJiYjIwNmMiLCJzY29wZSI6WyJzY2ltLm1lIiwib3BlbmlkIiwicGFzc3dvcmQud3JpdGUiLCJhcHByb3ZhbHMubWUiLCJvYXV0aC5hcHByb3ZhbHMiXSwiY2xpZW50X2lkIjoiZWRnZWNsaSIsImNpZCI6ImVkZ2VjbGkiLCJhenAiOiJlZGdlY2xpIiwiZ3JhbnRfdHlwZSI6InBhc3N3b3JkIiwidXNlcl9pZCI6ImU1N2M0ODUxLThhMmUtNGVjMi1iYTJjLWNjZGIwMmJiMjA2YyIsIm9yaWdpbiI6InVzZXJncmlkIiwidXNlcl9uYW1lIjoiamJvd2VuQGFwaWdlZS5jb20iLCJlbWFpbCI6Impib3dlbkBhcGlnZWUuY29tIiwiYXV0aF90aW1lIjoxNDY0Nzk2MDIyLCJyZXZfc2lnIjoiNGVlODcwYjgiLCJpYXQiOjE0NjQ3OTYwMjIsImV4cCI6MTQ2NDc5NzgyMiwiaXNzIjoiaHR0cHM6Ly9sb2dpbi5hcGlnZWUuY29tL29hdXRoL3Rva2VuIiwiemlkIjoidWFhIiwiYXVkIjpbImVkZ2VjbGkiLCJzY2ltIiwib3BlbmlkIiwicGFzc3dvcmQiLCJhcHByb3ZhbHMiLCJvYXV0aCJdfQ.LyP58pOr-lj1yIdaLGH_oCDm-HivLIiBM-wgETiFXALi_ZQLoJTqvLLFmgzmspShAyrbHfbrALrHH5DwRHE3skU6L8mnetTA0Q9cBZHq2pgkNsnw7S2K8A1jMZQEuijnoOZbnQ1v2JhgbHtnr-b_aLMT7WS58oWx0yj6mqL242P3AHdo1RHVmig_jNEn6fiVTCU9pTx9IuX5uB8RYahWhfI6T2bs4EqPdUr0MdwGhu6QPgP2cPNITdzfj2u9xXIUXOzYBxLGc6rPVIpmxKqEbjkdkVOqoNOof75AkegXkULBZk6so50yNGsEkBk4aChg5gDR-KRw4a1yYkNLMJ__vg`)

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on POST. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(500), "Response should be 500 Internal Server Error")
		})

		It("Update Environment", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1", hostBase)

			jsonStr := []byte(`{"hostNames": ["testhost2"]}`)
			req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonStr))
			req.Header.Add("Authorization", `Bearer eyJhbGciOiJSUzI1NiJ9.eyJqdGkiOiI1MmI5MjRlZS0wZTQwLTRjNjktYmZiMC04MjEwN2Q3ZWM2MGQiLCJzdWIiOiJlNTdjNDg1MS04YTJlLTRlYzItYmEyYy1jY2RiMDJiYjIwNmMiLCJzY29wZSI6WyJzY2ltLm1lIiwib3BlbmlkIiwicGFzc3dvcmQud3JpdGUiLCJhcHByb3ZhbHMubWUiLCJvYXV0aC5hcHByb3ZhbHMiXSwiY2xpZW50X2lkIjoiZWRnZWNsaSIsImNpZCI6ImVkZ2VjbGkiLCJhenAiOiJlZGdlY2xpIiwiZ3JhbnRfdHlwZSI6InBhc3N3b3JkIiwidXNlcl9pZCI6ImU1N2M0ODUxLThhMmUtNGVjMi1iYTJjLWNjZGIwMmJiMjA2YyIsIm9yaWdpbiI6InVzZXJncmlkIiwidXNlcl9uYW1lIjoiamJvd2VuQGFwaWdlZS5jb20iLCJlbWFpbCI6Impib3dlbkBhcGlnZWUuY29tIiwiYXV0aF90aW1lIjoxNDY0Nzk2MDIyLCJyZXZfc2lnIjoiNGVlODcwYjgiLCJpYXQiOjE0NjQ3OTYwMjIsImV4cCI6MTQ2NDc5NzgyMiwiaXNzIjoiaHR0cHM6Ly9sb2dpbi5hcGlnZWUuY29tL29hdXRoL3Rva2VuIiwiemlkIjoidWFhIiwiYXVkIjpbImVkZ2VjbGkiLCJzY2ltIiwib3BlbmlkIiwicGFzc3dvcmQiLCJhcHByb3ZhbHMiLCJvYXV0aCJdfQ.LyP58pOr-lj1yIdaLGH_oCDm-HivLIiBM-wgETiFXALi_ZQLoJTqvLLFmgzmspShAyrbHfbrALrHH5DwRHE3skU6L8mnetTA0Q9cBZHq2pgkNsnw7S2K8A1jMZQEuijnoOZbnQ1v2JhgbHtnr-b_aLMT7WS58oWx0yj6mqL242P3AHdo1RHVmig_jNEn6fiVTCU9pTx9IuX5uB8RYahWhfI6T2bs4EqPdUr0MdwGhu6QPgP2cPNITdzfj2u9xXIUXOzYBxLGc6rPVIpmxKqEbjkdkVOqoNOof75AkegXkULBZk6so50yNGsEkBk4aChg5gDR-KRw4a1yYkNLMJ__vg`)

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
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1/deployments", hostBase)

			jsonStr := []byte(`{
				"deploymentName": "testdep1",
				"publicHosts": "deploy.k8s.public",
				"privateHosts": "deploy.k8s.private",
    			"replicas": 1,
    			"ptsURL": "https://api.myjson.com/bins/2g3im"}`)

			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on POST. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(201), "Response should be 201 Created")

		})

		It("Update Deployment from PTS URL", func() {
			//Need to wait a little before we run an update
			//Should look into a better fix
			time.Sleep(50 * time.Millisecond)
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1/deployments/testdep1", hostBase)

			jsonStr := []byte(`{
    				"replicas": 3,
					"ptsURL": "https://api.myjson.com/bins/3p3vy"
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
				"privateHosts": "deploy.k8s.private",
    			"replicas": 1,
				"pts":     
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
						"name": "testpod2",
						"labels": {
							"app": "web2"
						},
						"annotations": {
							"publicPaths": "80:/ 90:/2",
      						"privatePaths": "80:/ 90:/2"
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

			//TODO: Maybe more thorough checking of response
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
						"name": "testpod2",
						"labels": {
							"app": "web2"
						},
						"annotations": {
							"publicPaths": "80:/ 100:/2",
      						"privatePaths": "80:/ 100:/2"
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
								"value": "100"
							}],
							"ports": [{
								"containerPort": 100
							}]
						}]
					}
				}
			}`)

			req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonStr))

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on PATCH. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(200), "Response should be 200 OK")

			//TODO: Maybe more thorough checking of response

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

			Expect(resp.StatusCode).Should(Equal(204), "Response should be 200 OK")

		})

		It("Delete Deployment testdep2", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1/deployments/testdep2", hostBase)
			req, err := http.NewRequest("DELETE", url, nil)

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on DELETE. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(204), "Response should be 200 OK")

		})

		It("Delete Environment", func() {
			url := fmt.Sprintf("%s/environmentGroups/testgroup/environments/testenv1", hostBase)

			req, err := http.NewRequest("DELETE", url, nil)

			resp, err := client.Do(req)

			Expect(err).Should(BeNil(), "Shouldn't get an error on DELETE. Error: %v", err)

			Expect(resp.StatusCode).Should(Equal(204), "Response should be 200 OK")
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
