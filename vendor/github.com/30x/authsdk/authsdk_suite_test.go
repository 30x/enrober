package authsdk

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestShipyard(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auth SDK Suite")
}
