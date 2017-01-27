package helper

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"k8s.io/client-go/pkg/api/v1"
	"os"
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
