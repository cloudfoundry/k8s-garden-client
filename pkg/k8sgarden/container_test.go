package k8sgarden_test

import (
	"errors"
	"io"
	"strings"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/properties"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/guardian/rundmc/users/usersfakes"
	"code.cloudfoundry.org/k8s-garden-client/pkg/containerd/containerdfakes"
	"code.cloudfoundry.org/k8s-garden-client/pkg/k8sgarden"
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
		fakeUserLookupper *usersfakes.FakeUserLookupper
		fakeAppTask       *containerdfakes.FakeTask
		fakeSidecarTask   *containerdfakes.FakeTask
		fakeProcess       *containerdfakes.FakeProcess
		exitChan          chan ctrdclient.ExitStatus
		taskMap           map[string]ctrdclient.Task
		sandboxPath       string
		expectedRootfs    string
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("container-test")
		fakeUserLookupper = &usersfakes.FakeUserLookupper{}

		exitChan = make(chan ctrdclient.ExitStatus, 1)
		fakeProcess = &containerdfakes.FakeProcess{}
		fakeProcess.WaitReturns(exitChan, nil)

		fakeAppTask = &containerdfakes.FakeTask{}
		fakeAppTask.IDReturns("app-task")
		fakeAppTask.PidReturns(12345)
		fakeAppTask.ExecReturns(fakeProcess, nil)

		fakeSidecarTask = &containerdfakes.FakeTask{}
		fakeSidecarTask.IDReturns("sidecar-task")
		fakeSidecarTask.PidReturns(67890)
		fakeSidecarTask.ExecReturns(fakeProcess, nil)

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

		sandboxPath = "/var/run/containerd/io.containerd.runtime.v2.task/k8s.io"
		// containerIDMap is populated by the client after pod creation and is not
		// set via NewContainer, so the container id segment is empty in tests.
		expectedRootfs = sandboxPath + "/rootfs"

		testContainer = k8sgarden.NewContainer(logger, pod, env, 2.0, fakeUserLookupper, properties.NewManager(), 0, taskMap, sandboxPath)
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
			Expect(rootPath).To(Equal(expectedRootfs))
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
			Expect(rootPath).To(Equal(expectedRootfs))
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

		It("returns error when user lookup fails", func() {
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
		BeforeEach(func() {
			fakeUserLookupper.LookupReturns(&users.ExecUser{Uid: 0, Gid: 0, Home: "/root"}, nil)
		})

		It("extracts the tar stream via the untar binary in the app container", func() {
			exitChan <- *ctrdclient.NewExitStatus(0, time.Now(), nil)

			tarStream := io.NopCloser(strings.NewReader("tar-data"))
			err := testContainer.StreamIn(garden.StreamInSpec{
				Path:      "/app/data",
				User:      "vcap",
				TarStream: tarStream,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeAppTask.ExecCallCount()).To(Equal(1))
			Expect(fakeSidecarTask.ExecCallCount()).To(Equal(0))

			_, id, spec, _ := fakeAppTask.ExecArgsForCall(0)
			Expect(id).To(HavePrefix("stream-in-"))
			Expect(spec.Args).To(Equal([]string{"/tmp/bin/untar", "/tmp/bin/tar", "vcap", "/app/data"}))
			Expect(spec.User.Username).To(Equal("root"))
		})

		It("defaults the user to root when none is provided", func() {
			exitChan <- *ctrdclient.NewExitStatus(0, time.Now(), nil)

			err := testContainer.StreamIn(garden.StreamInSpec{
				Path:      "/app/data",
				TarStream: io.NopCloser(strings.NewReader("tar-data")),
			})
			Expect(err).NotTo(HaveOccurred())

			_, _, spec, _ := fakeAppTask.ExecArgsForCall(0)
			Expect(spec.Args).To(Equal([]string{"/tmp/bin/untar", "/tmp/bin/tar", "root", "/app/data"}))
		})

		It("returns an error when the user lookup fails", func() {
			fakeUserLookupper.LookupReturns(nil, errors.New("user not found"))
			err := testContainer.StreamIn(garden.StreamInSpec{
				Path:      "/app/data",
				User:      "vcap",
				TarStream: io.NopCloser(strings.NewReader("tar-data")),
			})
			Expect(err).To(MatchError(ContainSubstring("stream-in: failed to run tar")))
			Expect(fakeAppTask.ExecCallCount()).To(Equal(0))
		})

		It("returns an error when tar exits non-zero", func() {
			exitChan <- *ctrdclient.NewExitStatus(3, time.Now(), nil)

			err := testContainer.StreamIn(garden.StreamInSpec{
				Path:      "/app/data",
				User:      "vcap",
				TarStream: io.NopCloser(strings.NewReader("tar-data")),
			})
			Expect(err).To(MatchError(ContainSubstring("stream-in: tar exited 3")))
		})
	})

	Describe("StreamOut", func() {
		BeforeEach(func() {
			fakeUserLookupper.LookupReturns(&users.ExecUser{Uid: 1000, Gid: 2000, Home: "/home/vcap"}, nil)
		})

		It("compresses a file via tar in the app container", func() {
			reader, err := testContainer.StreamOut(garden.StreamOutSpec{
				Path: "/app/logs",
				User: "vcap",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(reader).NotTo(BeNil())

			Eventually(fakeAppTask.ExecCallCount).Should(Equal(1))
			Expect(fakeSidecarTask.ExecCallCount()).To(Equal(0))

			_, id, spec, _ := fakeAppTask.ExecArgsForCall(0)
			Expect(id).To(HavePrefix("stream-out-"))
			Expect(spec.Args).To(Equal([]string{"/tmp/bin/tar", "--no-same-permissions", "--no-same-owner", "--xattrs", "--xattrs-include=*", "-C", "/app", "-cf", "-", "logs"}))
			Expect(spec.User.Username).To(Equal("vcap"))

			exitChan <- *ctrdclient.NewExitStatus(0, time.Now(), nil)
			data, readErr := io.ReadAll(reader)
			Expect(readErr).NotTo(HaveOccurred())
			Expect(data).To(BeEmpty())
		})

		It("compresses a directory's contents when the path ends with a slash", func() {
			reader, err := testContainer.StreamOut(garden.StreamOutSpec{
				Path: "/app/logs/",
				User: "vcap",
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(fakeAppTask.ExecCallCount).Should(Equal(1))
			_, _, spec, _ := fakeAppTask.ExecArgsForCall(0)
			Expect(spec.Args).To(Equal([]string{"/tmp/bin/tar", "--no-same-permissions", "--no-same-owner", "--xattrs", "--xattrs-include=*", "-C", "/app/logs/", "-cf", "-", "."}))

			exitChan <- *ctrdclient.NewExitStatus(0, time.Now(), nil)
			_, _ = io.ReadAll(reader)
		})

		It("defaults the user to root when none is provided", func() {
			reader, err := testContainer.StreamOut(garden.StreamOutSpec{
				Path: "/app/logs",
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(fakeAppTask.ExecCallCount).Should(Equal(1))
			_, _, spec, _ := fakeAppTask.ExecArgsForCall(0)
			Expect(spec.User.Username).To(Equal("root"))

			exitChan <- *ctrdclient.NewExitStatus(0, time.Now(), nil)
			_, _ = io.ReadAll(reader)
		})

		It("returns an error when the user lookup fails", func() {
			fakeUserLookupper.LookupReturns(nil, errors.New("user not found"))
			_, err := testContainer.StreamOut(garden.StreamOutSpec{
				Path: "/app/logs",
				User: "vcap",
			})
			Expect(err).To(MatchError(ContainSubstring("stream-out: failed to run tar")))
			Expect(fakeAppTask.ExecCallCount()).To(Equal(0))
		})

		It("propagates a non-zero tar exit through the reader", func() {
			reader, err := testContainer.StreamOut(garden.StreamOutSpec{
				Path: "/app/logs",
				User: "vcap",
			})
			Expect(err).NotTo(HaveOccurred())

			exitChan <- *ctrdclient.NewExitStatus(2, time.Now(), nil)
			_, readErr := io.ReadAll(reader)
			Expect(readErr).To(MatchError(ContainSubstring("stream-out: tar exited 2")))
		})
	})
})
