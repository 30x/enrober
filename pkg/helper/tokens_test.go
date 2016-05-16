package helper

import (
	"fmt"
	"testing"
)

//Maybe more thorough testing? Unsure how to test random numbers...

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
