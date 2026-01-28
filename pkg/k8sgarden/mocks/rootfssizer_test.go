package mocks_test

import (
	"code.cloudfoundry.org/executor/initializer/configuration"
	"code.cloudfoundry.org/k8s-garden-client/pkg/k8sgarden/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RootfsSizerMock", func() {
	var rootfsSizer configuration.RootFSSizer

	BeforeEach(func() {
		rootfsSizer = mocks.NewRootFSSizer()
	})

	It("always returns 0 for RootFSSizeFromPath", func() {
		size := rootfsSizer.RootFSSizeFromPath("any/path")
		Expect(size).To(Equal(uint64(0)))
	})
})
