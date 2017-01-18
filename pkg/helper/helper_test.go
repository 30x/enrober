package helper

import (
	"fmt"
	"testing"
	"net/http"
	"net/http/httptest"
	"reflect"

	"k8s.io/client-go/pkg/api/v1"

)

func TestGenerateRandomBytes(t *testing.T) {
	bytes, err := GenerateRandomBytes(32)
	if err != nil {
		t.Error("Error from GenerateRandomBytes\n")
	}
	fmt.Printf("Got Bytes: %v\n", bytes)

}

func TestGenerateRandomString(t *testing.T) {
	token, err := GenerateRandomString(32)
	if err != nil {
		t.Error("Error from GenerateRandomString\n")
	}
	fmt.Printf("Got Token: %v\n", token)

}

func TestGetPTSFromURL(t *testing.T) {
	ts := startMockServer()

	mockRequest := http.Request{}
	mockPTS	:= v1.PodTemplateSpec{
		ObjectMeta: v1.ObjectMeta{
			Name: "nginx",
			Labels: map[string]string{
				"component": "nginx2",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: "frontend",
					Image: "nginx",
					Ports: []v1.ContainerPort{
						{
							ContainerPort: 8000,
						},
					},
				},
			},
		},
	}

	ptsResp, err := GetPTSFromURL(ts.URL + "/ptsURL", &mockRequest)
	if err != nil {
		t.Fatalf("Error when calling GETPTSFromURL: %v.", err)
	}
	if !reflect.DeepEqual(ptsResp, mockPTS) {
		t.Fatalf("Expected %s, got %s", mockPTS, ptsResp)
	}

}

// Starts mock httptest server that returns the used apigee resources, all other resources return 404
func startMockServer() *httptest.Server {
	var jsonPTS = `{
		"metadata": {
			"name": "nginx",
			"labels": {
				"component": "nginx2"
			}
		},
		"spec": {
			"containers": [{
				"name": "frontend",
				"image": "nginx",
				"ports": [{
					"containerPort": 8000
				}]
			}]
		}
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ptsURL":
			fmt.Fprintln(w, jsonPTS)
		default:
			w.WriteHeader(404)
		}
	}))

	return ts
}