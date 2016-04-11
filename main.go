package main

import (
	"fmt"
	"os"

	"k8s.io/kubernetes/pkg/client/restclient"
)

//Global Variables
var clientConfig = restclient.Config{
	Host: "127.0.0.1:8080",
}

func main() {

	envState := os.Getenv("DEPLOY_STATE")
	switch envState {
	case "PROD":

	case "DEV":

	default:
		fmt.Printf("Defaulting to Local Dev Setup\n")
	}

	// server := server.NewServer()

	//TODO: Should encapsulate this
	// http.ListenAndServe(":9000", server.router)

}
