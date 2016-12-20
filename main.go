package main

import (
	"fmt"
	"os"

	"github.com/30x/enrober/pkg/server"
)

func main() {

	envState := os.Getenv("DEPLOY_STATE")

	state := server.StateLocal

	switch envState {
	case "PROD":
		fmt.Printf("DEPLOY_STATE set to PROD\n")
		state = server.StateCluster
	case "DEV_CONTAINER":
		fmt.Printf("DEPLOY_STATE set to DEV_CONTAINER\n")
		state = server.StateCluster
	case "DEV":
		fmt.Printf("DEPLOY_STATE set to DEV\n")
		state = server.StateLocal
	default:
		fmt.Printf("Defaulting to Local Dev Setup\n")

	}

	err := server.Init(state)
	if err != nil {
		fmt.Printf("Error initializing server: %v\n", err)
		return
	}

	server := server.NewServer()
	err = server.Start()
	if err != nil {
		fmt.Printf("Error starting server\n")
	}

	return

}
