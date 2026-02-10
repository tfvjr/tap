package model

import "time"

// ProcessKind identifies whether a process is native, a Docker container, or a Podman container.
type ProcessKind string

const (
	ProcessNative ProcessKind = "native"
	ProcessDocker ProcessKind = "docker"
	ProcessPodman ProcessKind = "podman"
)

// DevProcess represents a single process or container tracked by Tap.
type DevProcess struct {
	PID           *int32         `json:"pid,omitempty"`           // nil for stopped containers
	Name          string         `json:"name"`                    // e.g., "node", "postgres"
	Command       string         `json:"command"`                 // Full command line
	Ports         []PortBinding  `json:"ports"`                   // Ports this process is listening on
	Project       string         `json:"project,omitempty"`       // Attributed project name
	ProjectPath   string         `json:"project_path,omitempty"`  // Project root directory
	CPUPercent    float64        `json:"cpu_percent"`
	MemoryBytes   uint64         `json:"memory_bytes"`
	StartTime     time.Time      `json:"start_time"`
	Kind          ProcessKind    `json:"kind"`                    // Native | DockerContainer | PodmanContainer
	ContainerInfo *ContainerInfo `json:"container_info,omitempty"`
}

// PortBinding describes a single port a process is listening on.
type PortBinding struct {
	Port     uint16 `json:"port"`
	Protocol string `json:"protocol"` // "tcp" | "udp"
	Address  string `json:"address"`  // "0.0.0.0", "127.0.0.1", "::1"
}

// ContainerInfo holds metadata specific to container-based processes.
type ContainerInfo struct {
	ContainerID   string        `json:"container_id"`
	ContainerName string        `json:"container_name"`
	Image         string        `json:"image"`
	Status        string        `json:"status"`
	PortMappings  []PortMapping `json:"port_mappings"`
}

// PortMapping describes a host-to-container port mapping.
type PortMapping struct {
	HostPort      uint16 `json:"host_port"`
	ContainerPort uint16 `json:"container_port"`
}

// Uptime returns how long the process has been running.
func (p *DevProcess) Uptime() time.Duration {
	return time.Since(p.StartTime)
}

// MemoryMB returns the process memory usage in megabytes.
func (p *DevProcess) MemoryMB() float64 {
	return float64(p.MemoryBytes) / (1024 * 1024)
}
