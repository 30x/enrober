package main

import (
	"fmt"
	"os"

	"github.com/30x/enrober/pkg/server"

	"k8s.io/kubernetes/pkg/client/restclient"
)

func main() {

	clientConfig := restclient.Config{
		Host: "127.0.0.1:8080",
	}
	envState := os.Getenv("DEPLOY_STATE")
	switch envState {
	case "PROD":
		clientConfig.Host = ""
	case "DEV":
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
	server.Start()

}
