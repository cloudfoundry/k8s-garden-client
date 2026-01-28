package mocks_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMocksSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mocks Suite")
}
