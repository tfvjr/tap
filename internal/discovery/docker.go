package discovery

import (
	"context"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/tfvjr/tap/internal/model"
)

// DiscoverContainers connects to the local Docker daemon and returns every
// running container as a model.DevProcess. If Docker is not installed or the
// daemon is not reachable, the function returns an empty slice and a nil error
// because the absence of Docker is not considered an error condition.
func DiscoverContainers() ([]model.DevProcess, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		// Cannot create a client at all (e.g. Docker libraries missing).
		return []model.DevProcess{}, nil
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		// Docker daemon is not running or not accessible — graceful fallback.
		return []model.DevProcess{}, nil
	}

	result := make([]model.DevProcess, 0, len(containers))

	for _, c := range containers {
		portMappings := buildPortMappings(c.Ports)
		portBindings := buildPortBindings(c.Ports)
		containerName := containerDisplayName(c.Names)

		// Docker Compose sets labels that identify the project directory.
		projectPath := c.Labels["com.docker.compose.project.working_dir"]

		dp := model.DevProcess{
			PID:         nil, // Containers don't expose a meaningful host PID.
			Name:        c.Image,
			Command:     c.Command,
			Ports:       portBindings,
			Project:     "",
			ProjectPath: projectPath,
			CPUPercent:  0, // Docker stats API is streaming and expensive; skip for now.
			MemoryBytes: 0,
			StartTime:   time.Unix(c.Created, 0),
			Kind:        model.ProcessDocker,
			ContainerInfo: &model.ContainerInfo{
				ContainerID:   c.ID,
				ContainerName: containerName,
				Image:         c.Image,
				Status:        c.Status,
				PortMappings:  portMappings,
			},
		}

		result = append(result, dp)
	}

	return result, nil
}

// buildPortMappings converts the Docker port list into model.PortMapping
// entries. Only ports that have a public (host) mapping are included.
func buildPortMappings(ports []types.Port) []model.PortMapping {
	mappings := make([]model.PortMapping, 0, len(ports))
	for _, p := range ports {
		if p.PublicPort == 0 {
			continue
		}
		mappings = append(mappings, model.PortMapping{
			HostPort:      p.PublicPort,
			ContainerPort: p.PrivatePort,
		})
	}
	return mappings
}

// buildPortBindings converts the Docker port list into model.PortBinding
// entries suitable for the DevProcess.Ports field. Only ports that are
// published on the host are included.
func buildPortBindings(ports []types.Port) []model.PortBinding {
	bindings := make([]model.PortBinding, 0, len(ports))
	for _, p := range ports {
		if p.PublicPort == 0 {
			continue
		}

		addr := p.IP
		if addr == "" {
			addr = "0.0.0.0"
		}

		bindings = append(bindings, model.PortBinding{
			Port:     p.PublicPort,
			Protocol: p.Type, // "tcp" or "udp"
			Address:  addr,
		})
	}
	return bindings
}

// containerDisplayName picks a human-readable name from the Docker names
// slice. Docker prefixes every name with a leading "/" which is stripped.
func containerDisplayName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}
