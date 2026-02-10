package model

// Project groups related processes under a single project name.
type Project struct {
	Name        string       `json:"name"`
	Path        string       `json:"path"`
	Processes   []DevProcess `json:"processes"`
	TotalCPU    float64      `json:"total_cpu"`
	TotalMemory uint64       `json:"total_memory"`
}

// Recalculate sums CPU and memory usage from all processes in the project.
func (p *Project) Recalculate() {
	var cpu float64
	var mem uint64
	for _, proc := range p.Processes {
		cpu += proc.CPUPercent
		mem += proc.MemoryBytes
	}
	p.TotalCPU = cpu
	p.TotalMemory = mem
}
