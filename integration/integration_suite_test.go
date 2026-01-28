package integration_test

import (
	"net"
	"os/exec"
	"testing"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/lager/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	bbsClient      bbs.InternalClient
	client         *kubernetes.Clientset
	logger         lager.Logger
	portForwardCmd *exec.Cmd
	err            error
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite", Label("integration"))
}

var _ = BeforeSuite(func() {
	logger = lager.NewLogger("integration-tests")
	logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))

	client, err = newClient()
	Expect(err).NotTo(HaveOccurred())

	go func() {
		portForwardCmd := exec.Command("kubectl", "port-forward", "svc/bbs", "8889:8889", "-n", "default")
		Expect(portForwardCmd.Start()).To(Succeed())
	}()
	Eventually(checkBBS, "10s", "500ms").Should(Succeed())

	bbsClient, err = bbs.NewClient("https://localhost:8889", "../certs/ca.crt", "../certs/tls.crt", "../certs/tls.key", 0, 0)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	if portForwardCmd != nil {
		Expect(portForwardCmd.Process.Kill()).To(Succeed())
	}
})

func checkBBS() error {
	conn, err := net.DialTimeout("tcp", "localhost:8889", 1*time.Second)
	if err == nil {
		return conn.Close()
	}

	return err
}

func newClient() (*kubernetes.Clientset, error) {
	apiCfg, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, err
	}

	restCfg, err := clientcmd.NewDefaultClientConfig(*apiCfg, nil).ClientConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(restCfg)
}
