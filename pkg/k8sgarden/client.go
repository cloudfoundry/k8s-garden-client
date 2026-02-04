package k8sgarden

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/properties"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/k8s-garden-client/pkg/k8sgarden/containerd"
	"code.cloudfoundry.org/k8s-garden-client/pkg/k8sgarden/kubelet"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep/cmd/rep/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AppGUIDLabelKey   = "cloudfoundry.org/app-guid"
	OrgGUIDLabelKey   = "cloudfoundry.org/org-guid"
	SpaceGUIDLabel    = "cloudfoundry.org/space-guid"
	WorkloadTypeKey   = "cloudfoundry.org/workload-type"
	OwnerNameLabelKey = "cloudfoundry.org/owner-name"

	appContainerName     = "app"
	sidecarContainerName = "sidecar"

	apiOperationTimeout = 10 * time.Second
)

var alphanum = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = alphanum[rand.IntN(len(alphanum))]
	}
	return string(b)
}

type client struct {
	k8sclient            ctrlclient.Client
	kubeletClient        kubelet.Client
	containerdClient     containerd.Client
	logger               lager.Logger
	node                 *corev1.Node
	containers           *containerMap
	portManager          PortManager
	cmdRunner            commandrunner.CommandRunner
	nstarRunner          rundmc.NstarRunner
	userLookupper        users.UserLookupper
	propertyManager      *properties.Manager
	trustedCertsDir      string
	nodeCPU              int64
	nodeMemoryInB        int64
	sidecarRootfs        string
	enableContainerProxy bool
	workloadsNamespace   string
}

var _ garden.Client = &client{}

func NewClient(logger lager.Logger, k8sclient ctrlclient.Client, containerdClient containerd.Client, kubeletClient kubelet.Client, cmdRunner commandrunner.CommandRunner, nstarRunner rundmc.NstarRunner, userLookupper users.UserLookupper, repConfig config.RepConfig, sidecarRootfs, workloadsNamespace string) (garden.Client, error) {
	node := &corev1.Node{}
	if err := k8sclient.Get(context.Background(), ctrlclient.ObjectKey{Name: os.Getenv("NODE_NAME")}, node); err != nil {
		return nil, fmt.Errorf("failed to get node %s: %w", os.Getenv("NODE_NAME"), err)
	}

	nodeCPU, _ := node.Status.Capacity.Cpu().AsInt64()
	nodeMemoryBytes, _ := node.Status.Capacity.Memory().AsInt64()

	containerMap, propertyManager, err := containerRestoreInfo(k8sclient, workloadsNamespace)
	if err != nil {
		return nil, err
	}

	return &client{
		kubeletClient:        kubeletClient,
		k8sclient:            k8sclient,
		containerdClient:     containerdClient,
		logger:               logger,
		node:                 node,
		nodeCPU:              nodeCPU,
		nodeMemoryInB:        nodeMemoryBytes,
		sidecarRootfs:        sidecarRootfs,
		trustedCertsDir:      repConfig.TrustedSystemCertificatesPath,
		enableContainerProxy: repConfig.EnableContainerProxy,
		cmdRunner:            cmdRunner,
		nstarRunner:          nstarRunner,
		userLookupper:        userLookupper,
		containers:           containerMap,
		portManager:          newPortManager(),
		propertyManager:      propertyManager,
		workloadsNamespace:   workloadsNamespace,
	}, nil
}

func (c *client) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	c.logger.Info("bulk-metrics-start")
	defer c.logger.Info("bulk-metrics-end")

	metricsMap := make(map[string]garden.ContainerMetricsEntry, len(handles))
	metrics, err := c.kubeletClient.GetMetrics(c.logger, handles)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics from kubelet: %w", err)
	}

	for handle, metric := range metrics {
		ctr, err := c.containers.Get(handle)
		if err != nil {
			return nil, fmt.Errorf("failed to get container %s for metrics: %w", handle, err)
		}

		metricEntry := garden.ContainerMetricsEntry{
			Metrics: garden.Metrics{
				MemoryStat: garden.ContainerMemoryStat{
					TotalUsageTowardLimit: metric.MemoryUsageInBytes,
				},
				CPUStat: garden.ContainerCPUStat{
					Usage: uint64(metric.TimeSpentInCPU),
				},
				DiskStat: garden.ContainerDiskStat{
					TotalBytesUsed: metric.DiskUsageInBytes + uint64(ctr.rootfsSize),
				},
				Age:            time.Duration(metric.ContainerAgeInNanoseconds),
				CPUEntitlement: uint64(ctr.cpuAssignment * float64(metric.ContainerAgeInNanoseconds)),
			},
		}

		if metric.RxInBytes != nil && metric.TxInBytes != nil {
			metricEntry.Metrics.NetworkStat = &garden.ContainerNetworkStat{
				RxBytes: *metric.RxInBytes,
				TxBytes: *metric.TxInBytes,
			}
		}

		metricsMap[handle] = metricEntry
	}

	return metricsMap, nil
}

func (c *client) Capacity() (garden.Capacity, error) {
	capacity := garden.Capacity{
		MemoryInBytes:          uint64(c.node.Status.Allocatable.Memory().Value()),
		DiskInBytes:            uint64(c.node.Status.Capacity.StorageEphemeral().Value()),
		SchedulableDiskInBytes: uint64(c.node.Status.Allocatable.StorageEphemeral().Value()),
		MaxContainers:          uint64(c.node.Status.Capacity.Pods().Value()),
	}

	return capacity, nil
}

func (c *client) Containers(properties garden.Properties) ([]garden.Container, error) {
	log := c.logger.Session("list-containers")

	log.Info("starting")
	defer log.Info("finished")

	containers := c.containers.List()
	if properties == nil {
		properties = garden.Properties{}
	}
	if _, ok := properties["garden.state"]; !ok {
		properties["garden.state"] = "created"
	} else if properties["garden.state"] == "all" {
		delete(properties, "garden.state")
	}

	var matchedContainers []garden.Container
	for _, cntr := range containers {
		matched := c.propertyManager.MatchesAll(cntr.Handle(), properties)
		if matched {
			matchedContainers = append(matchedContainers, cntr)
		}
	}

	return matchedContainers, nil
}

func (c *client) Create(spec garden.ContainerSpec) (garden.Container, error) {
	c.logger.Info("create-container-start", lager.Data{"spec": spec})
	defer c.logger.Info("create-container-end")

	if c.containers.Exists(spec.Handle) {
		return nil, fmt.Errorf("Handle '%s' already in use", spec.Handle)
	}

	cpuAssignment := cpuQuantity(float64(spec.Limits.Memory.LimitInBytes)/(1024.0*1024.0), c.nodeCPU, c.nodeMemoryInB)
	ports := make([]corev1.ContainerPort, 0, len(spec.NetIn))
	for idx, netin := range spec.NetIn {
		hostPort := netin.HostPort
		if hostPort == 0 {
			var err error
			hostPort, err = c.portManager.Next()
			if err != nil {
				return nil, fmt.Errorf("failed to allocate host port: %w", err)
			}
			spec.NetIn[idx].HostPort = hostPort
		}

		ports = append(ports, corev1.ContainerPort{
			ContainerPort: int32(netin.ContainerPort),
			HostPort:      int32(hostPort),
			Protocol:      corev1.ProtocolTCP,
		})
	}

	baseImage := spec.Image.URI
	var (
		dockerEnv  []string
		rootfsSize uint64
		err        error
	)
	if cutImg, ok := strings.CutPrefix(baseImage, "docker:"); ok {
		img, imgSize, err := c.containerdClient.Pull(context.Background(), strings.TrimLeft(strings.ReplaceAll(cutImg, "#", ":"), "/"), spec.Image.Username, spec.Image.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to pull docker image: %w", err)
		}
		rootfsSize = uint64(imgSize)

		if rootfsSize >= spec.Limits.Disk.ByteHard {
			err = c.containerdClient.Delete(context.Background(), img)
			return nil, errors.Join(fmt.Errorf("image size %d exceeds container disk limit %d", rootfsSize, spec.Limits.Disk.ByteHard), err)
		}

		baseImage = img.Name()
		imgSpec, err := img.Spec(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get image config for image %s: %w", baseImage, err)
		}
		dockerEnv = imgSpec.Config.Env
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Handle,
			Namespace: c.workloadsNamespace,
			Labels:    podLabels(spec.Properties),
		},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken:  ptr.To(false),
			EnableServiceLinks:            ptr.To(false),
			NodeName:                      c.node.GetName(),
			TerminationGracePeriodSeconds: ptr.To(int64(5)),
			HostUsers:                     ptr.To(true), // should work with "false" too, but fails in KinD
			RestartPolicy:                 corev1.RestartPolicyNever,
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse(fmt.Sprintf("%dm", int(cpuAssignment*1000))),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: byteToQuantity(int64(spec.Limits.Memory.LimitInBytes), resource.BinarySI),
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "tmp",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "init-bin",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/lib/rep/bin/init",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:            appContainerName,
					Image:           baseImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Ports:           ports,
					Command:         []string{"/tmp/garden-init"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "tmp",
							MountPath: "/tmp",
						},
						{
							Name:      "init-bin",
							MountPath: "/tmp/garden-init",
							ReadOnly:  true,
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceEphemeralStorage: byteToQuantity(int64(spec.Limits.Disk.ByteHard-rootfsSize), resource.DecimalSI),
						},
					},
				},
			},
		},
	}

	if spec.Properties["network.container_workload"] == appContainerName { // can be one if these https://github.com/cloudfoundry/cloud_controller_ng/blob/169b6202c7a05f36e22cb1e6e7595e07f2872cf4/lib/cloud_controller/diego/protocol/container_network_info.rb#L7-L9
		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
			Name:    sidecarContainerName,
			Image:   c.sidecarRootfs,
			Command: []string{"/tmp/garden-init"},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "init-bin",
					MountPath: "/tmp/garden-init",
					ReadOnly:  true,
				},
			},
		})

		if c.enableContainerProxy {
			pod.Spec.Containers[0].Ports = nil // ports are handled by the sidecar
			pod.Spec.Containers[1].Ports = ports
		}
	}

	for _, mount := range spec.BindMounts {
		volName := "vol-" + randSeq(10)
		volumeSource := corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: mount.SrcPath,
			},
		}

		if mount.SrcPath == c.trustedCertsDir {
			volumeSource = corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "trusted-system-certs",
					},
				},
			}
		}

		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name:         volName,
			VolumeSource: volumeSource,
		})

		for i := range pod.Spec.Containers {
			pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts, corev1.VolumeMount{
				Name:      volName,
				MountPath: mount.DstPath,
				ReadOnly:  true,
			})
		}
	}

	for key, value := range spec.Properties {
		c.propertyManager.Set(pod.GetName(), key, value)
	}
	container := NewContainer(
		c.logger.Session(fmt.Sprintf("container-%s", spec.Handle)),
		pod,
		append(dockerEnv, spec.Env...),
		cpuAssignment,
		c.nstarRunner,
		c.userLookupper,
		c.propertyManager,
		rootfsSize,
		nil,
	)
	if err := c.containers.Add(spec.Handle, container); err != nil {
		return nil, err
	}

	if err := c.k8sclient.Create(context.Background(), pod); err != nil {
		return nil, fmt.Errorf("failed to create pod: %w", err)
	}

	// wait until the pod is running
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	for {
		if err := c.k8sclient.Get(ctx, ctrlclient.ObjectKeyFromObject(pod), pod); ctrlclient.IgnoreNotFound(err) != nil {
			return nil, fmt.Errorf("failed to get pod after creation: %w", err)
		}

		if pod.Status.Phase == corev1.PodRunning {
			break
		} else {
			c.logger.Info("waiting-for-pod-to-be-running", lager.Data{"pod-name": spec.Handle, "current-phase": pod.Status.Phase})
		}

		select {
		case <-ctx.Done():
			_ = c.k8sclient.Delete(context.Background(), pod, &ctrlclient.DeleteOptions{
				GracePeriodSeconds: ptr.To[int64](0),
			})
			return nil, fmt.Errorf("timed out waiting for pod to be running")
		case <-time.After(500 * time.Millisecond):
		}
	}

	container.taskMap, err = c.containerdClient.LoadTasks(context.Background(), pod.Status.ContainerStatuses)
	if err != nil {
		return nil, fmt.Errorf("failed to load containerD container: %w", err)
	}

	if err := container.SetProperty("garden.state", "created"); err != nil {
		return nil, err
	}

	return container, nil
}

func (c *client) Destroy(handle string) error {
	container, err := c.containers.Get(handle)
	if err != nil {
		return err
	}

	if err := deletePod(c.logger, container.pod, c.k8sclient); err != nil {
		return fmt.Errorf("failed to delete pod: %w", err)
	}

	for _, container := range container.pod.Spec.Containers {
		for _, port := range container.Ports {
			c.portManager.Release(uint32(port.HostPort))
		}
	}
	c.containers.Remove(handle)

	return c.propertyManager.DestroyKeySpace(handle)
}

func (c *client) Lookup(handle string) (garden.Container, error) {
	return c.containers.Get(handle)
}

func (c *client) Ping() error {
	containerdOk, err := c.containerdClient.IsServing(context.Background())
	if err != nil {
		return fmt.Errorf("failed to ping containerd: %w", err)
	}
	if !containerdOk {
		return errors.New("containerd is not serving")
	}

	return nil
}

func (c *client) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	panic("unimplemented")
}

func deletePod(logger lager.Logger, pod *corev1.Pod, clnt ctrlclient.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), apiOperationTimeout)
	defer cancel()

	if err := clnt.Delete(ctx, pod); err != nil {
		if ctrlclient.IgnoreNotFound(err) == nil {
			return nil
		}

		logger.Error("failed-to-delete-pod", err)
		return err
	}

	for {
		var p corev1.Pod
		if err := clnt.Get(ctx, ctrlclient.ObjectKeyFromObject(pod), &p); err != nil {
			if ctrlclient.IgnoreNotFound(err) == nil {
				return nil
			}

			logger.Error("failed-to-get-pod", err)
			return err
		}

		logger.Info("waiting-for-pod-deletion", lager.Data{"pod-name": pod.Name})
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func byteToQuantity(b int64, f resource.Format) resource.Quantity {
	return ptr.Deref(resource.NewQuantity(b, f), resource.Quantity{})
}

func cpuQuantity(memMb float64, nodeCPU, nodeMemoryInB int64) float64 {
	cpuPerShare := float64(nodeCPU) / (float64(nodeMemoryInB) / (1024.0 * 1024.0))
	return float64(memMb) * cpuPerShare
}

func podLabels(properties garden.Properties) map[string]string {
	labels := map[string]string{}

	appGUID, ok := properties["network.app_id"]
	if ok {
		labels[AppGUIDLabelKey] = appGUID
	}

	orgGUID, ok := properties["network.org_id"]
	if ok {
		labels[OrgGUIDLabelKey] = orgGUID
	}

	spaceGUID, ok := properties["network.space_id"]
	if ok {
		labels[SpaceGUIDLabel] = spaceGUID
	}

	workloadType, ok := properties["network.container_workload"]
	if ok {
		labels[WorkloadTypeKey] = workloadType
	}

	ownerName, ok := properties[executor.ContainerOwnerProperty]
	if ok {
		labels[OwnerNameLabelKey] = ownerName
	}

	return labels
}

func containerRestoreInfo(client ctrlclient.Client, workloadsNamespace string) (*containerMap, *properties.Manager, error) {
	podList := &corev1.PodList{}
	if err := client.List(context.Background(), podList, ctrlclient.InNamespace(workloadsNamespace)); err != nil {
		return nil, nil, fmt.Errorf("failed to list existing pods: %w", err)
	}

	containerMap := newContainerMap()
	propertyManager := properties.NewManager()

	for _, pod := range podList.Items {
		propertyManager.Set(pod.Name, executor.ContainerOwnerProperty, pod.Labels[OwnerNameLabelKey])

		container := NewContainer(
			nil,
			&pod,
			[]string{},
			0,
			nil,
			nil,
			propertyManager,
			0,
			nil,
		)
		err := containerMap.Add(pod.Name, container)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to add container to map: %w", err)
		}
	}

	return containerMap, propertyManager, nil
}
