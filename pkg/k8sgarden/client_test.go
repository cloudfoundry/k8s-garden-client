package k8sgarden_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/initializer"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/rundmcfakes"
	"code.cloudfoundry.org/guardian/rundmc/users/usersfakes"
	"code.cloudfoundry.org/k8s-garden-client/pkg/k8sgarden"
	"code.cloudfoundry.org/k8s-garden-client/pkg/k8sgarden/containerd/containerdfakes"
	"code.cloudfoundry.org/k8s-garden-client/pkg/k8sgarden/kubelet/kubeletfakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/rep/cmd/rep/config"
	ctrdclient "github.com/containerd/containerd/v2/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var _ = Describe("Client", func() {
	var (
		logger               *lagertest.TestLogger
		k8sClient            ctrlclient.Client
		fakeContainerdClient *containerdfakes.FakeClient
		fakeKubeletClient    *kubeletfakes.FakeClient
		fakeCmdRunner        *fake_command_runner.FakeCommandRunner
		fakeNstarRunner      *rundmcfakes.FakeNstarRunner
		fakeUserLookupper    *usersfakes.FakeUserLookupper
		repConfig            config.RepConfig
		sidecarRootfs        string
		testNode             *corev1.Node
		scheme               *runtime.Scheme
		gardenClient         garden.Client
		err                  error
		tempDir              string

		failPodCreation bool
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("k8sgarden-test")
		fakeContainerdClient = &containerdfakes.FakeClient{}
		fakeKubeletClient = &kubeletfakes.FakeClient{}
		fakeCmdRunner = fake_command_runner.New()
		fakeNstarRunner = &rundmcfakes.FakeNstarRunner{}
		fakeUserLookupper = &usersfakes.FakeUserLookupper{}
		sidecarRootfs = "sidecar-rootfs"

		failPodCreation = false

		tempDir = GinkgoT().TempDir()

		repConfig = config.RepConfig{
			ExecutorConfig: initializer.ExecutorConfig{
				EnableContainerProxy:          true,
				TrustedSystemCertificatesPath: filepath.Join(tempDir, "trusted-certs"),
			},
		}
		repConfig.InstanceIdentityCredDir = filepath.Join(tempDir, "instance-identity")
		repConfig.ContainerProxyConfigPath = filepath.Join(tempDir, "container-proxy")
		repConfig.VolumeMountedFiles = filepath.Join(tempDir, "volume-mounted-files")

		Expect(os.MkdirAll(repConfig.InstanceIdentityCredDir, 0755)).To(Succeed())
		Expect(os.MkdirAll(repConfig.ContainerProxyConfigPath, 0755)).To(Succeed())
		Expect(os.MkdirAll(repConfig.VolumeMountedFiles, 0755)).To(Succeed())

		scheme = runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())

		testNode = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
			},
			Status: corev1.NodeStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("4"),
					corev1.ResourceMemory:           resource.MustParse("8Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
					corev1.ResourcePods:             resource.MustParse("110"),
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("3800m"),
					corev1.ResourceMemory:           resource.MustParse("7Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("95Gi"),
					corev1.ResourcePods:             resource.MustParse("110"),
				},
			},
		}
		Expect(os.Setenv("NODE_NAME", "test-node")).To(Succeed())

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithInterceptorFuncs(interceptor.Funcs{
				Create: func(ctx context.Context, c ctrlclient.WithWatch, obj ctrlclient.Object, opts ...ctrlclient.CreateOption) error {
					if pod, ok := obj.(*corev1.Pod); ok {
						if pod.Status.Phase != corev1.PodRunning {
							if failPodCreation {
								return errors.New("simulated pod creation failure")
							}

							pod.Status.Phase = corev1.PodRunning
							pod.Status.ContainerStatuses = []corev1.ContainerStatus{
								{Name: "app", ContainerID: "containerd://test", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
							}
						}
					}

					return c.Create(context.Background(), obj)
				},
			}).
			WithObjects(testNode).
			Build()

		gardenClient, err = k8sgarden.NewClient(
			logger,
			k8sClient,
			fakeContainerdClient,
			fakeKubeletClient,
			fakeCmdRunner,
			fakeNstarRunner,
			fakeUserLookupper,
			repConfig,
			sidecarRootfs,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.Unsetenv("NODE_NAME")).To(Succeed())
		if tempDir != "" {
			Expect(os.RemoveAll(tempDir)).To(Succeed())
		}
	})

	Describe("NewClient", func() {
		Context("when all dependencies are healthy", func() {
			It("creates a new garden client successfully", func() {
				gardenClient, err = k8sgarden.NewClient(
					logger,
					k8sClient,
					fakeContainerdClient,
					fakeKubeletClient,
					fakeCmdRunner,
					fakeNstarRunner,
					fakeUserLookupper,
					repConfig,
					sidecarRootfs,
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(gardenClient).NotTo(BeNil())
			})
		})

		Context("when NODE_NAME environment variable is not set", func() {
			BeforeEach(func() {
				Expect(os.Unsetenv("NODE_NAME")).To(Succeed())
			})

			It("fails to create client", func() {
				gardenClient, err = k8sgarden.NewClient(
					logger,
					k8sClient,
					fakeContainerdClient,
					fakeKubeletClient,
					fakeCmdRunner,
					fakeNstarRunner,
					fakeUserLookupper,
					repConfig,
					sidecarRootfs,
				)

				Expect(err).To(HaveOccurred())
				Expect(gardenClient).To(BeNil())
			})
		})
	})

	Describe("Capacity", func() {
		It("returns the node's allocatable resources", func() {
			capacity, err := gardenClient.Capacity()
			Expect(err).NotTo(HaveOccurred())

			expectedMemory := uint64(7 * 1024 * 1024 * 1024)
			Expect(capacity.MemoryInBytes).To(Equal(expectedMemory))

			expectedDisk := uint64(100 * 1024 * 1024 * 1024)
			Expect(capacity.DiskInBytes).To(Equal(expectedDisk))

			expectedSchedulableDisk := uint64(95 * 1024 * 1024 * 1024)
			Expect(capacity.SchedulableDiskInBytes).To(Equal(expectedSchedulableDisk))

			Expect(capacity.MaxContainers).To(Equal(uint64(110)))
		})
	})

	Describe("Ping", func() {
		Context("when containerd is serving", func() {
			BeforeEach(func() {
				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(testNode).
					Build()

				gardenClient, err = k8sgarden.NewClient(
					logger,
					k8sClient,
					fakeContainerdClient,
					fakeKubeletClient,
					fakeCmdRunner,
					fakeNstarRunner,
					fakeUserLookupper,
					repConfig,
					sidecarRootfs,
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns no error when containerd is serving", func() {
				fakeContainerdClient.IsServingReturns(true, nil)

				err := gardenClient.Ping()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns error when containerd IsServing fails", func() {
				fakeContainerdClient.IsServingReturns(false, errors.New("connection failed"))

				err := gardenClient.Ping()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to ping containerd"))
			})

			It("returns error when containerd is not serving", func() {
				fakeContainerdClient.IsServingReturns(false, nil)

				err := gardenClient.Ping()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("containerd is not serving"))
			})
		})
	})

	Describe("Containers and Lookup", func() {
		Context("when no containers exist", func() {
			It("returns an empty list", func() {
				containers, err := gardenClient.Containers(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(BeEmpty())
			})

			It("returns error when looking up non-existent container", func() {
				_, err := gardenClient.Lookup("non-existent-handle")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(garden.ContainerNotFoundError{Handle: "non-existent-handle"}))
			})
		})

		Context("when containers exist", func() {
			It("Restores the container state on startup", func() {
				orphanedPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "orphaned-pod-1",
						Namespace: "cf-workloads",
						Labels: map[string]string{
							k8sgarden.OwnerNameLabelKey: "executor",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "app", Image: "test-image"},
						},
					},
				}
				Expect(k8sClient.Create(context.Background(), orphanedPod)).To(Succeed())

				gardenClient, err = k8sgarden.NewClient(
					logger,
					k8sClient,
					fakeContainerdClient,
					fakeKubeletClient,
					fakeCmdRunner,
					fakeNstarRunner,
					fakeUserLookupper,
					repConfig,
					sidecarRootfs,
				)

				Expect(err).NotTo(HaveOccurred())

				var containerFilter = garden.Properties{
					executor.ContainerStateProperty: "all",
					executor.ContainerOwnerProperty: "executor",
				}

				containers, err := gardenClient.Containers(containerFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers[0].Handle()).To(Equal("orphaned-pod-1"))
				Expect(containers[0].Properties()).To(HaveKeyWithValue(executor.ContainerOwnerProperty, "executor"))
			})
		})
	})

	Describe("BulkMetrics", func() {
		Context("when kubelet returns metrics successfully", func() {
			It("returns an empty map when no containers match", func() {
				kubeletMetrics := map[string]executor.ContainerMetrics{
					"test-container": {
						MemoryUsageInBytes: 1024 * 1024 * 100,
						DiskUsageInBytes:   1024 * 1024 * 50,
						TimeSpentInCPU:     1000000000,
					},
				}
				fakeKubeletClient.GetMetricsReturns(kubeletMetrics, nil)

				spec := garden.ContainerSpec{
					Handle: "test-container",
				}
				_, err := gardenClient.Create(spec)
				Expect(err).NotTo(HaveOccurred())

				metrics, err := gardenClient.BulkMetrics([]string{})
				Expect(err).NotTo(HaveOccurred())
				Expect(metrics).To(HaveLen(1))
			})

			It("returns error when container is not found in local map", func() {
				kubeletMetrics := map[string]executor.ContainerMetrics{
					"non-existent-container": {
						MemoryUsageInBytes: 1024 * 1024 * 100,
						DiskUsageInBytes:   1024 * 1024 * 50,
						TimeSpentInCPU:     1000000000,
					},
				}
				fakeKubeletClient.GetMetricsReturns(kubeletMetrics, nil)

				_, err = gardenClient.BulkMetrics([]string{"non-existent-container"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get container"))
			})
		})

		Context("when kubelet fails to return metrics", func() {
			It("returns an error", func() {
				fakeKubeletClient.GetMetricsReturns(nil, errors.New("kubelet connection failed"))

				_, err := gardenClient.BulkMetrics([]string{"some-handle"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get metrics from kubelet"))
			})
		})
	})

	Describe("Create", func() {
		var (
			fakeTask *containerdfakes.FakeTask
		)

		BeforeEach(func() {
			fakeTask = &containerdfakes.FakeTask{}
			fakeTask.PidReturns(12345)

			fakeContainerdClient.LoadTasksReturns(map[string]ctrdclient.Task{
				"app": fakeTask,
			}, nil)
		})

		It("creates a pod and loads containerd tasks successfully", func() {
			spec := garden.ContainerSpec{
				Handle: "test-container-1",
				Properties: garden.Properties{
					"network.container_workload": "app",
					"network.app_id":             "test-app-guid",
				},
				Env: []string{"TEST_ENV=value"},
				Limits: garden.Limits{
					Memory: garden.MemoryLimits{
						LimitInBytes: 256 * 1024 * 1024,
					},
					Disk: garden.DiskLimits{
						ByteHard: 1024 * 1024 * 1024,
					},
				},
				Image: garden.ImageRef{
					URI: "cflinuxfs4",
				},
				NetIn: []garden.NetIn{
					{
						HostPort:      0,
						ContainerPort: 80,
					},
				},
				BindMounts: []garden.BindMount{
					{
						SrcPath: "/host/data",
						DstPath: "/container/data",
						Mode:    garden.BindMountModeRO,
					},
					{
						SrcPath: repConfig.TrustedSystemCertificatesPath,
						DstPath: "/etc/ssl/certs",
						Mode:    garden.BindMountModeRO,
					},
				},
			}

			container, err := gardenClient.Create(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(container).NotTo(BeNil())
			Expect(container.Handle()).To(Equal("test-container-1"))

			var pod corev1.Pod
			err = k8sClient.Get(context.Background(), ctrlclient.ObjectKey{
				Name:      "test-container-1",
				Namespace: "cf-workloads",
			}, &pod)
			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Labels).To(HaveKeyWithValue(k8sgarden.AppGUIDLabelKey, "test-app-guid"))
			Expect(pod.Spec.Containers).To(HaveLen(2))
			Expect(pod.Spec.Containers[0].Image).To(Equal("cflinuxfs4"))
			Expect(pod.Spec.Containers[1].Ports).To(HaveLen(1))
			Expect(pod.Spec.Containers[1].Ports[0].HostPort).To(Equal(int32(62000)))
			Expect(pod.Spec.Containers[1].Ports[0].ContainerPort).To(Equal(int32(80)))

			Expect(pod.Spec.Volumes).To(HaveLen(4))
			Expect(pod.Spec.Containers[0].VolumeMounts).To(ContainElements(
				MatchFields(IgnoreExtras, Fields{
					"MountPath": Equal("/container/data"),
				}),
				MatchFields(IgnoreExtras, Fields{
					"MountPath": Equal("/etc/ssl/certs"),
					"ReadOnly":  BeTrue(),
				}),
			))
			Expect(pod.Spec.Volumes).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"VolumeSource": MatchFields(IgnoreExtras, Fields{
					"ConfigMap": Not(BeNil()),
				}),
			})))

			containers, err := gardenClient.Containers(nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(containers).To(HaveLen(1))
		})

		It("creates a docker app container successfully", func() {
			fakeContainerdClient.PullReturns(&containerdfakes.FakeImage{
				NameStub: func() string {
					return "docker.io/library/busybox:latest"
				},
			}, 9999, nil)

			spec := garden.ContainerSpec{
				Handle: "test-container-2",
				Limits: garden.Limits{
					Memory: garden.MemoryLimits{
						LimitInBytes: 256 * 1024 * 1024,
					},
					Disk: garden.DiskLimits{
						ByteHard: 1024 * 1024 * 1024,
					},
				},
				Image: garden.ImageRef{
					URI: "docker:///busybox:latest",
				},
			}

			container, err := gardenClient.Create(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(container).NotTo(BeNil())
			Expect(container.Handle()).To(Equal("test-container-2"))

			var pod corev1.Pod
			err = k8sClient.Get(context.Background(), ctrlclient.ObjectKey{
				Name:      "test-container-2",
				Namespace: "cf-workloads",
			}, &pod)
			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Spec.Containers[0].Image).To(Equal("docker.io/library/busybox:latest"))
			Expect(pod.Spec.Containers[0].Resources.Limits.StorageEphemeral().Value()).To(Equal(int64((1024 * 1024 * 1024) - 9999)))
		})

		It("returns an error if the image is larger than the disk limit", func() {
			fakeContainerdClient.PullReturns(&containerdfakes.FakeImage{
				NameStub: func() string {
					return "docker.io/library/busybox:latest"
				},
			}, 20, nil)

			spec := garden.ContainerSpec{
				Handle: "test-container-2",
				Limits: garden.Limits{
					Memory: garden.MemoryLimits{
						LimitInBytes: 256 * 1024 * 1024,
					},
					Disk: garden.DiskLimits{
						ByteHard: 10,
					},
				},
				Image: garden.ImageRef{
					URI: "docker:///busybox:latest",
				},
			}

			container, err := gardenClient.Create(spec)
			Expect(err).To(MatchError("image size 20 exceeds container disk limit 10"))
			Expect(container).To(BeNil())
		})

		It("returns error when creating a container with an existing name", func() {
			spec := garden.ContainerSpec{
				Handle: "test-container-2",
				Limits: garden.Limits{
					Memory: garden.MemoryLimits{
						LimitInBytes: 256 * 1024 * 1024,
					},
					Disk: garden.DiskLimits{
						ByteHard: 1024 * 1024 * 1024,
					},
				},
				Image: garden.ImageRef{
					URI: "cflinuxfs4",
				},
			}

			_, err := gardenClient.Create(spec)
			Expect(err).NotTo(HaveOccurred())

			_, err = gardenClient.Create(spec)
			Expect(err).To(MatchError("Handle 'test-container-2' already in use"))
		})

		It("returns error when pod creation fails", func() {
			spec := garden.ContainerSpec{
				Handle: "test-container-3",
				Limits: garden.Limits{
					Memory: garden.MemoryLimits{
						LimitInBytes: 256 * 1024 * 1024,
					},
					Disk: garden.DiskLimits{
						ByteHard: 1024 * 1024 * 1024,
					},
				},
				Image: garden.ImageRef{
					URI: "cflinuxfs4",
				},
			}

			conflictPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-container-3",
					Namespace: "cf-workloads",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "test"},
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), conflictPod)).To(Succeed())

			_, err := gardenClient.Create(spec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create pod"))
		})

		It("returns error when containerd task loading fails", func() {
			fakeContainerdClient.LoadTasksReturns(nil, errors.New("containerd connection failed"))

			spec := garden.ContainerSpec{
				Handle: "test-container-4",
				Limits: garden.Limits{
					Memory: garden.MemoryLimits{
						LimitInBytes: 256 * 1024 * 1024,
					},
					Disk: garden.DiskLimits{
						ByteHard: 1024 * 1024 * 1024,
					},
				},
				Image: garden.ImageRef{
					URI: "cflinuxfs4",
				},
			}

			_, err := gardenClient.Create(spec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to load containerD container"))
		})
	})

	Describe("Destroy", func() {
		Context("when container does not exist", func() {
			It("returns an error", func() {
				err := gardenClient.Destroy("non-existent-handle")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(garden.ContainerNotFoundError{Handle: "non-existent-handle"}))
			})
		})

		Context("when container exists", func() {
			var fakeTask *containerdfakes.FakeTask

			BeforeEach(func() {
				fakeTask = &containerdfakes.FakeTask{}
				fakeTask.PidReturns(12345)

				spec := garden.ContainerSpec{
					Handle: "container-to-destroy",
					Limits: garden.Limits{
						Memory: garden.MemoryLimits{
							LimitInBytes: 256 * 1024 * 1024,
						},
						Disk: garden.DiskLimits{
							ByteHard: 1024 * 1024 * 1024,
						},
					},
					Image: garden.ImageRef{
						URI: "cflinuxfs4",
					},
				}

				_, err := gardenClient.Create(spec)
				Expect(err).NotTo(HaveOccurred())
			})

			It("deletes the pod and removes container from tracking", func() {
				containers, err := gardenClient.Containers(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(HaveLen(1))

				err = gardenClient.Destroy("container-to-destroy")
				Expect(err).NotTo(HaveOccurred())

				var pod corev1.Pod
				err = k8sClient.Get(context.Background(), ctrlclient.ObjectKey{
					Name:      "container-to-destroy",
					Namespace: "cf-workloads",
				}, &pod)
				Expect(err).To(HaveOccurred())

				containers, err = gardenClient.Containers(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(BeEmpty())
			})

			It("can be looked up before destroy", func() {
				container, err := gardenClient.Lookup("container-to-destroy")
				Expect(err).NotTo(HaveOccurred())
				Expect(container.Handle()).To(Equal("container-to-destroy"))
			})

			It("cannot be looked up after destroy", func() {
				err := gardenClient.Destroy("container-to-destroy")
				Expect(err).NotTo(HaveOccurred())

				_, err = gardenClient.Lookup("container-to-destroy")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(garden.ContainerNotFoundError{Handle: "container-to-destroy"}))
			})
		})
	})
})
