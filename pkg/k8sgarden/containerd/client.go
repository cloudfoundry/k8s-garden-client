package containerd

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	ctrdclient "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/continuity/fs"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/opencontainers/image-spec/identity"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

//go:generate go tool counterfeiter -generate

//counterfeiter:generate github.com/containerd/containerd/v2/client.Task
//counterfeiter:generate github.com/containerd/containerd/v2/client.Process
//counterfeiter:generate github.com/containerd/containerd/v2/client.Image

//counterfeiter:generate . Client
type Client interface {
	IsServing(ctx context.Context) (bool, error)
	LoadTasks(ctx context.Context, statuses []corev1.ContainerStatus) (map[string]ctrdclient.Task, error)
	Pull(ctx context.Context, ref, username, password string) (ctrdclient.Image, int64, error)
	Delete(ctx context.Context, img ctrdclient.Image) error
}

type clientWrapper struct {
	client *ctrdclient.Client
}

func NewClientWrapper(client *ctrdclient.Client) Client {
	return &clientWrapper{client: client}
}

func (w *clientWrapper) IsServing(ctx context.Context) (bool, error) {
	return w.client.IsServing(ctx)
}

func (w *clientWrapper) LoadTasks(ctx context.Context, statuses []corev1.ContainerStatus) (map[string]ctrdclient.Task, error) {
	taskMap := make(map[string]ctrdclient.Task)
	for _, status := range statuses {
		containerdID, _ := strings.CutPrefix(status.ContainerID, "containerd://")
		cntr, err := w.client.LoadContainer(ctx, containerdID)
		if err != nil {
			return nil, err
		}
		task, err := cntr.Task(ctx, nil)
		if err != nil {
			return nil, err
		}
		taskMap[status.Name] = task
	}

	return taskMap, nil
}

func (w *clientWrapper) Pull(ctx context.Context, ref, username, password string) (ctrdclient.Image, int64, error) {
	parsedRef, err := name.ParseReference(ref)
	if err != nil {
		return nil, 0, err
	}

	opts := []ctrdclient.RemoteOpt{
		ctrdclient.WithPullUnpack,
	}

	if username != "" && password != "" {
		// When credentials are provided, bypass any local cache mirrors and pull directly
		// from the registry. Cache mirrors typically don't forward authentication properly.
		// We define a custom RegistryHosts function that ignores the containerd hosts.toml
		// configuration and connects directly to the upstream registry.
		authorizer := docker.NewDockerAuthorizer(docker.WithAuthCreds(
			func(s string) (string, string, error) {
				return username, password, nil
			},
		))

		opts = append(opts, ctrdclient.WithResolver(docker.NewResolver(docker.ResolverOptions{
			Hosts: func(host string) ([]docker.RegistryHost, error) {
				// Create a registry host that connects directly to the upstream
				// registry, bypassing any local caching proxies
				hostConfig := docker.RegistryHost{
					Client:       http.DefaultClient,
					Authorizer:   authorizer,
					Host:         host,
					Scheme:       "https",
					Path:         "/v2",
					Capabilities: docker.HostCapabilityPull | docker.HostCapabilityResolve,
				}

				// Special handling for docker.io which uses registry-1.docker.io
				if host == "docker.io" {
					hostConfig.Host = "registry-1.docker.io"
				}

				return []docker.RegistryHost{hostConfig}, nil
			},
		})))
	}

	img, err := w.client.Pull(ctx, parsedRef.Name(), opts...)
	if err != nil {
		return nil, 0, err
	}

	diffIDs, err := img.RootFS(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get rootfs: %w", err)
	}

	snapshotter := w.client.SnapshotService("")
	finalChainID := identity.ChainID(diffIDs).String()
	viewKey := fmt.Sprintf("temp-view-%d", time.Now().UnixNano())
	mounts, err := snapshotter.View(ctx, viewKey, finalChainID) // .View always returns read-only mounts
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create view snapshot: %w", err)
	}

	defer func() {
		_ = snapshotter.Remove(ctx, viewKey)
	}()

	var totalSize int64
	if err := mount.WithTempMount(ctx, mounts, func(root string) error {
		usage, err := fs.DiskUsage(ctx, root)
		if err != nil {
			return err
		}

		totalSize = usage.Size
		return nil
	}); err != nil {
		return nil, 0, fmt.Errorf("failed to calculate mounted size: %w", err)
	}

	return img, totalSize, nil
}

func (w *clientWrapper) Delete(ctx context.Context, img ctrdclient.Image) error {
	return w.client.ImageService().Delete(ctx, img.Name(), images.DeleteTarget(ptr.To(img.Target())))
}
