package main

import (
	"fmt"
	"os"

	"github.com/30x/enrober/pkg/server"

	"k8s.io/kubernetes/pkg/client/restclient"
)

func main() {

	//Default to local client
	clientConfig := restclient.Config{
		Host: "127.0.0.1:8080",
	}

	envState := os.Getenv("DEPLOY_STATE")

	switch envState {
	case "PROD":
		fmt.Printf("DEPLOY_STATE set to PROD\n")
		clientConfig.Host = ""
	case "DEV":
		fmt.Printf("DEPLOY_STATE set to DEV\n")
		clientConfig.Host = "127.0.0.1:8080"
	default:
		fmt.Printf("Defaulting to Local Dev Setup\n")
	}

	err := server.Init(clientConfig)
	if err != nil {
		fmt.Printf("Unable to create Deployment Manager: %v\n", err)
		return
	}

	server := server.NewServer()
	err = server.Start()
	if err != nil {
		fmt.Printf("Error starting server\n")
	}

	return

}
