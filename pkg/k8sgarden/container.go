package k8sgarden

import (
	"fmt"
	"io"
	"sync"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/processes"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/lager/v3"
	ctrdclient "github.com/containerd/containerd/v2/client"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
	corev1 "k8s.io/api/core/v1"
)

var (
	Caps = []string{"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FOWNER", "CAP_FSETID", "CAP_KILL", "CAP_SETGID", "CAP_SETUID", "CAP_SETPCAP", "CAP_NET_BIND_SERVICE", "CAP_NET_RAW", "CAP_SYS_CHROOT", "CAP_MKNOD", "CAP_AUDIT_WRITE", "CAP_SETFCAP"}
)

type container struct {
	log             lager.Logger
	pod             *corev1.Pod
	env             []string
	cpuAssignment   float64
	rootfsSize      uint64
	nstar           rundmc.NstarRunner
	userLookupper   users.UserLookupper
	taskMap         map[string]ctrdclient.Task
	propertyManager gardener.PropertyManager
	mu              sync.RWMutex
}

func NewContainer(
	log lager.Logger,
	pod *corev1.Pod,
	env []string,
	cpuAssignment float64,
	nstar rundmc.NstarRunner,
	userLookupper users.UserLookupper,
	propertyManager gardener.PropertyManager,
	rootfsSize uint64,
	taskMap map[string]ctrdclient.Task,
) *container {
	return &container{
		log:             log,
		pod:             pod,
		env:             env,
		cpuAssignment:   cpuAssignment,
		rootfsSize:      rootfsSize,
		nstar:           nstar,
		userLookupper:   userLookupper,
		taskMap:         taskMap,
		propertyManager: propertyManager,
		mu:              sync.RWMutex{},
	}
}

// Handle implements [garden.Container].
func (c *container) Handle() string {
	return c.pod.GetName()
}

// Info implements [garden.Container].
func (c *container) Info() (garden.ContainerInfo, error) {
	portMapping := []garden.PortMapping{}
	for _, container := range c.pod.Spec.Containers {
		if len(container.Ports) == 0 {
			continue
		}

		for _, containerPort := range container.Ports {
			portMapping = append(portMapping, garden.PortMapping{
				HostPort:      uint32(containerPort.HostPort),
				ContainerPort: uint32(containerPort.ContainerPort),
			})
		}
	}

	return garden.ContainerInfo{
		State:         "active",
		Events:        []string{},
		HostIP:        c.pod.Status.HostIP,
		ContainerIP:   c.pod.Status.PodIP,
		ContainerIPv6: "",
		ExternalIP:    c.pod.Status.HostIP,
		MappedPorts:   portMapping,
	}, nil
}

// Run implements [garden.Container].
func (c *container) Run(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	c.log.Info("running-process", lager.Data{"processSpec": spec})
	targetContainer := appContainerName
	if spec.Image.URI != "" {
		targetContainer = sidecarContainerName
	}

	task := c.taskMap[targetContainer]
	execUser, err := c.userLookupper.Lookup(fmt.Sprintf("/proc/%d/root", task.Pid()), spec.User)
	if err != nil {
		return nil, fmt.Errorf("get user %q: %w", spec.User, err)
	}

	processSpec := &specs.Process{
		Args: append([]string{spec.Path}, spec.Args...),
		Env:  c.env,
		Cwd:  spec.Dir,
		User: specs.User{
			UID:      uint32(execUser.Uid),
			GID:      uint32(execUser.Gid),
			Username: spec.User,
		},
		Capabilities: &specs.LinuxCapabilities{
			Bounding:    Caps,
			Inheritable: Caps,
		},
		NoNewPrivileges: false,
	}
	processSpec.Env = processes.UnixEnvFor(goci.Bndl{Spec: specs.Spec{Process: processSpec}}, spec, execUser.Uid)
	processSpec.Env = append(processSpec.Env, spec.Env...)

	if spec.Dir == "" {
		processSpec.Cwd = execUser.Home
	}

	id := spec.ID
	if id == "" {
		id = uuid.NewString()
	}

	return NewProcess(
		c.log.Session("process", lager.Data{"processID": id}),
		id,
		processSpec,
		io,
		task,
	), nil
}

// StreamIn implements [garden.Container].
func (c *container) StreamIn(spec garden.StreamInSpec) error {
	c.log.Info("stream-in-starting", lager.Data{"path": spec.Path, "user": spec.User})
	defer c.log.Info("stream-in-completed", lager.Data{"path": spec.Path, "user": spec.User})

	if err := c.nstar.StreamIn(c.log.Session("nstar"), int(c.taskMap[appContainerName].Pid()), spec.Path, spec.User, spec.TarStream); err != nil {
		c.log.Error("nstar-failed", err)
		return fmt.Errorf("stream-in: nstar: %s", err)
	}

	return nil
}

// StreamOut implements [garden.Container].
func (c *container) StreamOut(spec garden.StreamOutSpec) (io.ReadCloser, error) {
	c.log.Info("stream-out-starting", lager.Data{"path": spec.Path, "user": spec.User})
	defer c.log.Info("stream-out-completed", lager.Data{"path": spec.Path, "user": spec.User})

	stream, err := c.nstar.StreamOut(c.log.Session("nstar"), int(c.taskMap[appContainerName].Pid()), spec.Path, spec.User)
	if err != nil {
		c.log.Error("nstar-failed", err)
		return nil, fmt.Errorf("stream-out: nstar: %s", err)
	}

	return stream, nil
}

func (c *container) Properties() (garden.Properties, error) {
	return c.propertyManager.All(c.Handle())
}

func (c *container) Property(name string) (string, error) {
	if prop, ok := c.propertyManager.Get(c.Handle(), name); ok {
		return prop, nil
	}

	return "", fmt.Errorf("property does not exist: %s", name)
}

func (c *container) SetProperty(name string, value string) error {
	c.propertyManager.Set(c.Handle(), name, value)
	return nil
}

func (c *container) RemoveProperty(name string) error {
	// we explicitly stopped handling this in 2016, see git blame + commit log -- COPIED FROM GARDEN
	_ = c.propertyManager.Remove(c.Handle(), name)
	return nil
}

func (c *container) SetGraceTime(t time.Duration) error {
	c.propertyManager.Set(c.Handle(), gardener.GraceTimeKey, fmt.Sprintf("%d", t))
	return nil
}

// Attach implements [garden.Container].
func (c *container) Attach(processID string, io garden.ProcessIO) (garden.Process, error) {
	panic("unimplemented")
}

// BulkNetOut implements [garden.Container].
func (c *container) BulkNetOut(netOutRules []garden.NetOutRule) error {
	panic("unimplemented")
}

// CurrentBandwidthLimits implements [garden.Container].
func (c *container) CurrentBandwidthLimits() (garden.BandwidthLimits, error) {
	panic("unimplemented")
}

// CurrentCPULimits implements [garden.Container].
func (c *container) CurrentCPULimits() (garden.CPULimits, error) {
	panic("unimplemented")
}

// CurrentDiskLimits implements [garden.Container].
func (c *container) CurrentDiskLimits() (garden.DiskLimits, error) {
	panic("unimplemented")
}

// CurrentMemoryLimits implements [garden.Container].
func (c *container) CurrentMemoryLimits() (garden.MemoryLimits, error) {
	panic("unimplemented")
}

// Metrics implements [garden.Container].
func (c *container) Metrics() (garden.Metrics, error) {
	panic("unimplemented")
}

// NetIn implements [garden.Container].
func (c *container) NetIn(hostPort uint32, containerPort uint32) (uint32, uint32, error) {
	panic("unimplemented")
}

// NetOut implements [garden.Container].
func (c *container) NetOut(netOutRule garden.NetOutRule) error {
	panic("unimplemented")
}

// Stop implements [garden.Container].
func (c *container) Stop(kill bool) error {
	panic("unimplemented")
}
