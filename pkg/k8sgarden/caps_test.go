package k8sgarden_test

import (
	"code.cloudfoundry.org/k8s-garden-client/pkg/k8sgarden"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ProcessCaps", func() {
	It("returns privilegedMaxCaps for root user when privileged is true", func() {
		caps := k8sgarden.ProcessCaps("root", true)
		Expect(caps).To(Equal(k8sgarden.PrivilegedMaxCaps))
	})

	It("returns unprivilegedMaxCaps for root user when privileged is false", func() {
		caps := k8sgarden.ProcessCaps("root", false)
		Expect(caps).To(Equal(k8sgarden.UnprivilegedMaxCaps))
	})

	It("returns nonRootMaxCaps for non-root user when privileged is true", func() {
		caps := k8sgarden.ProcessCaps("vcap", true)
		Expect(caps).To(Equal(k8sgarden.NonRootMaxCaps))
	})

	It("returns unprivilegedMaxCaps for non-root user when privileged is false", func() {
		caps := k8sgarden.ProcessCaps("vcap", false)
		Expect(caps).To(Equal(k8sgarden.UnprivilegedMaxCaps))
	})
})
