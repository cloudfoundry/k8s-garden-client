package kubelet_test

import (
	"io"
	"net/http"
	"time"

	"code.cloudfoundry.org/k8s-garden-client/pkg/k8sgarden/kubelet"
	"code.cloudfoundry.org/lager/v3"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	statsapi "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
)

var megabyte = uint64(1024 * 1024)

func newPodStats(name, namespace string, cpuUsageNs, diskUsageInBytes, memoryUsageInBytes uint64) statsapi.PodStats {
	return statsapi.PodStats{
		PodRef: statsapi.PodReference{
			Name:      name,
			Namespace: namespace,
		},
		StartTime: v1.Now(),
		Containers: []statsapi.ContainerStats{
			{
				Name: "app",
				CPU: &statsapi.CPUStats{
					UsageCoreNanoSeconds: &cpuUsageNs,
				},
				Memory: &statsapi.MemoryStats{
					WorkingSetBytes: &memoryUsageInBytes,
				},
			},
			{
				Name: "other-container",
			},
		},
		CPU:    &statsapi.CPUStats{},
		Memory: &statsapi.MemoryStats{},
		EphemeralStorage: &statsapi.FsStats{
			UsedBytes: &diskUsageInBytes,
		},
	}
}

var _ = Describe("Client", func() {
	var (
		client kubelet.Client
		logger lager.Logger
	)

	BeforeEach(func() {
		client = kubelet.NewClient(http.DefaultClient, "127.0.0.1", "10250")
		logger = lager.NewLogger("test")
		logger.RegisterSink(lager.NewPrettySink(io.Discard, lager.DEBUG))
	})

	Describe("GetMetrics", func() {
		It("returns an empty map when no GUIDs are provided", func() {
			httpmock.RegisterResponder("GET", "https://127.0.0.1:10250/stats/summary", httpmock.NewJsonResponderOrPanic(
				200,
				&statsapi.Summary{
					Pods: []statsapi.PodStats{
						newPodStats("pod-1", "default", 1000, 100*megabyte, 300*megabyte),
						newPodStats("pod-2", "default", 2000, 200*megabyte, 400*megabyte),
					},
				},
			))
			metrics, err := client.GetMetrics(logger, []string{"pod-1"})
			Expect(err).NotTo(HaveOccurred())
			Expect(metrics).To(HaveLen(1))

			podMetrics := metrics["pod-1"]
			Expect(podMetrics.MemoryUsageInBytes).To(Equal(300 * megabyte))
			Expect(podMetrics.DiskUsageInBytes).To(Equal(100 * megabyte))
			Expect(podMetrics.TimeSpentInCPU).To(Equal(time.Duration(1000)))
		})
	})
})
