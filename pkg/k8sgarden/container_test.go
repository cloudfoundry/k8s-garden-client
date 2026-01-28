package k8sgarden_test

import (
	"errors"
	"io"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/properties"
	"code.cloudfoundry.org/guardian/rundmc/rundmcfakes"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/guardian/rundmc/users/usersfakes"
	"code.cloudfoundry.org/k8s-garden-client/pkg/k8sgarden"
	"code.cloudfoundry.org/k8s-garden-client/pkg/k8sgarden/containerd/containerdfakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
	ctrdclient "github.com/containerd/containerd/v2/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Container", func() {
	var (
		testContainer     garden.Container
		logger            *lagertest.TestLogger
		pod               *corev1.Pod
		env               []string
		fakeNstarRunner   *rundmcfakes.FakeNstarRunner
		fakeUserLookupper *usersfakes.FakeUserLookupper
		fakeAppTask       *containerdfakes.FakeTask
		fakeSidecarTask   *containerdfakes.FakeTask
		taskMap           map[string]ctrdclient.Task
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("container-test")
		fakeNstarRunner = &rundmcfakes.FakeNstarRunner{}
		fakeUserLookupper = &usersfakes.FakeUserLookupper{}
		fakeAppTask = &containerdfakes.FakeTask{}
		fakeAppTask.IDReturns("app-task")
		fakeAppTask.PidReturns(12345)

		fakeSidecarTask = &containerdfakes.FakeTask{}
		fakeSidecarTask.IDReturns("sidecar-task")
		fakeSidecarTask.PidReturns(67890)

		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-container",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "app",
						Ports: []corev1.ContainerPort{
							{ContainerPort: 8080, HostPort: 30080},
							{ContainerPort: 9090, HostPort: 30090},
						},
					},
				},
			},
			Status: corev1.PodStatus{
				HostIP: "10.0.0.1",
				PodIP:  "10.244.0.1",
			},
		}

		env = []string{"HOME=/home/vcap", "PATH=/usr/bin"}
		taskMap = map[string]ctrdclient.Task{
			"app":     fakeAppTask,
			"sidecar": fakeSidecarTask,
		}

		testContainer = k8sgarden.NewContainer(logger, pod, env, 2.0, fakeNstarRunner, fakeUserLookupper, properties.NewManager(), 0, taskMap)
	})

	Describe("Handle", func() {
		It("returns the pod name as handle", func() {
			Expect(testContainer.Handle()).To(Equal("test-container"))
		})
	})

	Describe("Info", func() {
		It("returns container info object with IPs", func() {
			info, err := testContainer.Info()
			Expect(err).NotTo(HaveOccurred())
			Expect(info.State).To(Equal("active"))
			Expect(info.HostIP).To(Equal("10.0.0.1"))
			Expect(info.ContainerIP).To(Equal("10.244.0.1"))
			Expect(info.ExternalIP).To(Equal("10.0.0.1"))
			Expect(info.MappedPorts).To(HaveLen(2))
			Expect(info.MappedPorts[0]).To(Equal(garden.PortMapping{HostPort: 30080, ContainerPort: 8080}))
			Expect(info.MappedPorts[1]).To(Equal(garden.PortMapping{HostPort: 30090, ContainerPort: 9090}))
		})
	})

	Describe("Run", func() {
		It("runs process with correct user lookup and environment configuration", func() {
			fakeUserLookupper.LookupReturns(&users.ExecUser{
				Uid:  1000,
				Gid:  2000,
				Home: "/home/vcap",
			}, nil)

			proc, err := testContainer.Run(garden.ProcessSpec{
				ID:   "process-1",
				Path: "/bin/sh",
				Args: []string{"-c", "echo hello"},
				Dir:  "/app",
				User: "vcap",
				Env:  []string{"APP_ENV=production"},
			}, garden.ProcessIO{})

			Expect(err).NotTo(HaveOccurred())
			Expect(proc).NotTo(BeNil())
			Expect(fakeUserLookupper.LookupCallCount()).To(Equal(1))
			rootPath, user := fakeUserLookupper.LookupArgsForCall(0)
			Expect(rootPath).To(Equal("/proc/12345/root"))
			Expect(user).To(Equal("vcap"))

			process, ok := proc.(k8sgarden.Process)
			Expect(ok).To(BeTrue())
			Expect(process.Spec().Args).To(Equal([]string{"/bin/sh", "-c", "echo hello"}))
			Expect(process.Spec().Cwd).To(Equal("/app"))
			Expect(process.Spec().Env).To(ContainElements("APP_ENV=production", "HOME=/home/vcap", "PATH=/usr/bin"))
			Expect(process.Spec().User.UID).To(Equal(uint32(1000)))
			Expect(process.Spec().User.GID).To(Equal(uint32(2000)))
			Expect(process.Task().ID()).To(Equal("app-task"))
		})

		It("runs process in sidecar container when image URI is specified", func() {
			fakeUserLookupper.LookupReturns(&users.ExecUser{
				Uid:  1000,
				Gid:  2000,
				Home: "/root",
			}, nil)

			proc, err := testContainer.Run(garden.ProcessSpec{
				Path:  "/usr/bin/curl",
				Args:  []string{"http://example.com"},
				User:  "root",
				Image: garden.ImageRef{URI: "docker://curl"},
			}, garden.ProcessIO{})

			Expect(err).NotTo(HaveOccurred())
			Expect(proc).NotTo(BeNil())

			Expect(fakeUserLookupper.LookupCallCount()).To(Equal(1))
			rootPath, user := fakeUserLookupper.LookupArgsForCall(0)
			Expect(rootPath).To(Equal("/proc/67890/root"))
			Expect(user).To(Equal("root"))

			process, ok := proc.(k8sgarden.Process)
			Expect(ok).To(BeTrue())
			Expect(process.Spec().Args).To(Equal([]string{"/usr/bin/curl", "http://example.com"}))
			Expect(process.Spec().Cwd).To(Equal("/root"))
			Expect(process.Task().ID()).To(Equal("sidecar-task"))
		})

		It("handles empty process directory by using user home", func() {
			fakeUserLookupper.LookupReturns(&users.ExecUser{
				Uid:  1000,
				Gid:  2000,
				Home: "/home/vcap",
			}, nil)

			proc, err := testContainer.Run(garden.ProcessSpec{
				Path: "/bin/ls",
				User: "vcap",
				Dir:  "", // empty dir
			}, garden.ProcessIO{})

			Expect(err).NotTo(HaveOccurred())
			Expect(proc).NotTo(BeNil())

			process, ok := proc.(k8sgarden.Process)
			Expect(ok).To(BeTrue())
			Expect(process.Spec().Cwd).To(Equal("/home/vcap"))
		})

		It("generates UUID when process ID is not provided", func() {
			fakeUserLookupper.LookupReturns(&users.ExecUser{
				Uid:  1000,
				Gid:  2000,
				Home: "/home/vcap",
			}, nil)

			proc, err := testContainer.Run(garden.ProcessSpec{
				Path: "/bin/echo",
				User: "vcap",
				ID:   "", // empty ID
			}, garden.ProcessIO{})

			Expect(err).NotTo(HaveOccurred())
			Expect(proc.ID()).NotTo(BeEmpty())
		})

		It("handles errors from user lookup, StreamIn and StreamOut", func() {
			// Test Run error when user lookup fails
			fakeUserLookupper.LookupReturns(nil, errors.New("user not found"))
			_, err := testContainer.Run(garden.ProcessSpec{
				Path: "/bin/sh",
				User: "invalid-user",
			}, garden.ProcessIO{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("user not found"))
		})
	})

	Describe("StreamIn", func() {
		It("streams data into the container using nstar", func() {
			tarStream := io.NopCloser(strings.NewReader("tar-data"))
			err := testContainer.StreamIn(garden.StreamInSpec{
				Path:      "/app/data",
				User:      "vcap",
				TarStream: tarStream,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeNstarRunner.StreamInCallCount()).To(Equal(1))
			_, pid, path, user, stream := fakeNstarRunner.StreamInArgsForCall(0)
			Expect(pid).To(Equal(12345))
			Expect(path).To(Equal("/app/data"))
			Expect(user).To(Equal("vcap"))
			Expect(stream).To(Equal(tarStream))

		})

		It("returns error when nstar StreamIn fails", func() {
			fakeNstarRunner.StreamInReturns(errors.New("stream-in failed"))
			err := testContainer.StreamIn(garden.StreamInSpec{
				Path:      "/app/data",
				User:      "vcap",
				TarStream: io.NopCloser(strings.NewReader("data")),
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("stream-in failed"))
		})
	})

	Describe("StreamOut", func() {
		It("streams data out of the container using nstar", func() {
			fakeReadCloser := io.NopCloser(strings.NewReader("stream-out-data"))
			fakeNstarRunner.StreamOutReturns(fakeReadCloser, nil)
			reader, err := testContainer.StreamOut(garden.StreamOutSpec{
				Path: "/app/logs",
				User: "vcap",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(reader).To(Equal(fakeReadCloser))
			Expect(fakeNstarRunner.StreamOutCallCount()).To(Equal(1))
			_, outPid, outPath, outUser := fakeNstarRunner.StreamOutArgsForCall(0)
			Expect(outPid).To(Equal(12345))
			Expect(outPath).To(Equal("/app/logs"))
			Expect(outUser).To(Equal("vcap"))
		})

		It("returns error when nstar StreamOut fails", func() {
			fakeNstarRunner.StreamOutReturns(nil, errors.New("stream-out failed"))
			_, err := testContainer.StreamOut(garden.StreamOutSpec{
				Path: "/app/logs",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("stream-out failed"))
		})
	})
})
