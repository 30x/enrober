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
		t.Fatal("Expected false got true")
	}

	if validatePath(testPathFail) == true {
		t.Fatal("Expected false got true")
	}

	if validatePath(testPathPass) == false {
		t.Fatal("Expected true got false")
	}

}

func TestMultipleEdgePorts(t *testing.T) {
	singularPortPaths := []EdgePath{
		{
			BasePath:      "/test1",
			ContainerPort: "9000",
			TargetPath:    "/",
		},
		{
			BasePath:      "/test2",
			ContainerPort: "9000",
			TargetPath:    "/2",
		},
		{
			BasePath:      "/test3",
			ContainerPort: "9000",
			TargetPath:    "/3",
		},
	}
	if multipleEdgePorts(singularPortPaths) {
		t.Fatal("Expected false got true")
	}

	multiplePortPaths := []EdgePath{
		{
			BasePath:      "/test1",
			ContainerPort: "9000",
			TargetPath:    "/",
		},
		{
			BasePath:      "/test2",
			ContainerPort: "3000",
			TargetPath:    "/2",
		},
		{
			BasePath:      "/test3",
			ContainerPort: "9000",
			TargetPath:    "/3",
		},
	}
	if !multipleEdgePorts(multiplePortPaths) {
		t.Fatal("Expected true got false")
	}
}

func TestUniqueEdgePorts(t *testing.T) {
	singularPortPaths := []EdgePath{
		{
			BasePath:      "/test1",
			ContainerPort: "9000",
			TargetPath:    "/",
		},
		{
			BasePath:      "/test2",
			ContainerPort: "9000",
			TargetPath:    "/2",
		},
		{
			BasePath:      "/test3",
			ContainerPort: "9000",
			TargetPath:    "/3",
		},
	}
	expctedSingularResult := []string{"9000"}
	singularResult := uniqueEdgePorts(singularPortPaths)
	if !reflect.DeepEqual(singularResult, expctedSingularResult) {
		t.Fatalf("Expected %v got %v", expctedSingularResult, singularResult)
	}

	multiplePortPaths := []EdgePath{
		{
			BasePath:      "/test1",
			ContainerPort: "9000",
			TargetPath:    "/",
		},
		{
			BasePath:      "/test2",
			ContainerPort: "3000",
			TargetPath:    "/2",
		},
		{
			BasePath:      "/test3",
			ContainerPort: "9000",
			TargetPath:    "/3",
		},
	}
	expctedMultipleResult := []string{"9000", "3000"}
	multipleResult := uniqueEdgePorts(multiplePortPaths)
	if !reflect.DeepEqual(multipleResult, expctedMultipleResult) {
		t.Fatalf("Expected %v got %v", expctedMultipleResult, multipleResult)
	}
}
