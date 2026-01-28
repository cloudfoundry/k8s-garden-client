package k8sgarden_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestK8sGarden(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "K8sGarden Suite")
}
