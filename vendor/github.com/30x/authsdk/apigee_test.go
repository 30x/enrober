package authsdk

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os"
)

var _ = Describe("Apigee", func() {

	It("Parse Token", func() {

		testToken := os.Getenv("TEST_APIGEE_TOKEN")

		Expect(testToken).ShouldNot(BeNil(), "TEST_APIGEE_TOKEN must be defined in the environment to execute this test")

		jwtToken, err := NewApigeeJWTToken(testToken)

		Expect(err).Should(BeNil(), "Should not return an error creating a valid token")

		//if could not find directory, it's a fail

		Expect(jwtToken.GetSubject()).Should(Equal("38927727-932a-4706-acec-56d382077f34"))

		Expect(jwtToken.GetEmail()).Should(Equal("tnine@apigee.com"))

		Expect(jwtToken.GetUsername()).Should(Equal("tnine@apigee.com"))

		isAdmin, err := jwtToken.IsOrgAdmin("michaelarusso")

		Expect(err).Should(BeNil(), "Should not return an error creating a valid token")

		Expect(isAdmin).Should(Equal(true), "admin should be true")

	})

})
