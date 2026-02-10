package model

import "time"

// Snapshot captures the full system state at a point in time.
type Snapshot struct {
	Timestamp         time.Time    `json:"timestamp"`
	Projects          []Project    `json:"projects"`
	Unattributed      []DevProcess `json:"unattributed"`
	SystemCPU         float64      `json:"system_cpu"`
	SystemMemoryTotal uint64       `json:"system_memory_total"`
	SystemMemoryUsed  uint64       `json:"system_memory_used"`
}
