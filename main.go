package main

import (
	"fmt"

	"github.com/30x/enrober/pkg/server"
)

func main() {

	enrober := server.NewServer()
	err := enrober.Start()
	if err != nil {
		fmt.Printf("Error starting enrober: %v\n", err)
	}

	return

}
