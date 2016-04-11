package server_test

import (
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
    
    "testing"
)

func TestEnrober(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Enrober Server Suite")
}