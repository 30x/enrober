package server

import (
	"fmt"
	"reflect"
	"testing"
)

func TestComposeHostsString(t *testing.T) {
	mockHosts := []string{"val1", "val2"}
	hostString := composeHostsJSON(mockHosts)
	fmt.Printf("Result String:\n%v\n", hostString)
	mockResult := "{" +
		"\"val1\":{}," +
		"\"val2\":{}" +
		"}"
	if hostString != mockResult {
		t.Fatalf("Expected\n%v\ngot\n%v", mockResult, hostString)
	}
}

func TestParseHoststoMap(t *testing.T) {
	mockHostString := "{" +
		"\"val1\": {}," +
		"\"val2\": {}" +
		"}"
	mockMap := map[string]HostsConfig{"val1": {}, "val2": {}}
	hostsMap, err := parseHoststoMap(mockHostString)
	fmt.Printf("Result: %v\n", hostsMap)
	if err != nil {
		t.Fatalf("Shouldn't get err: %v", err)
	}
	if !reflect.DeepEqual(hostsMap, mockMap) {
		t.Fatalf("Expected%v\ngot\n%v", mockMap, hostsMap)
	}
}

func TestComposePaths(t *testing.T) {
	mockPathsObj := []EdgePath{
		{
			BasePath:      "/base",
			ContainerPort: "9000",
			TargetPath:    "/target",
		},
	}
	mockJSON :=
		`[
  {
    "basePath": "/base",
    "containerPort": "9000",
    "targetPath": "/target"
  }
]`
	resultJSON, err := composePathsJSON(mockPathsObj)
	if err != nil || resultJSON != mockJSON {
		t.Fatalf("Expected\n%v\ngot\n%v", mockJSON, resultJSON)
	}
}

func TestValidatePath(t *testing.T) {
	testNoPrefix := "test"
	testPathFail := "/test/%2a/%"
	testPathPass := "/test/%2a/aa/a"

	if validatePath(testNoPrefix) == true {
		t.Fatalf("Expected false got true")
	}

	if validatePath(testPathFail) == true {
		t.Fatalf("Expected false got true")
	}

	if validatePath(testPathPass) == false {
		t.Fatalf("Expected true got false")
	}

}
