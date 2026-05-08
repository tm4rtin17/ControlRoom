package docker

import (
	"time"

	"github.com/docker/docker/api/types/container"
)

// normalizeStats turns docker's wire-format stats into our flat ContainerStats.
//
// CPU% formula (per Docker's official client):
//   cpu_delta    = current - previous total CPU usage (ns)
//   system_delta = current - previous total system CPU usage (ns)
//   pct          = (cpu_delta / system_delta) * num_cpus * 100
func normalizeStats(s container.StatsResponse) ContainerStats {
	out := ContainerStats{
		Timestamp: s.Read,
	}
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage) - float64(s.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(s.CPUStats.SystemUsage) - float64(s.PreCPUStats.SystemUsage)
	cpus := float64(s.CPUStats.OnlineCPUs)
	if cpus == 0 {
		cpus = float64(len(s.CPUStats.CPUUsage.PercpuUsage))
		if cpus == 0 {
			cpus = 1
		}
	}
	if cpuDelta > 0 && sysDelta > 0 {
		out.CPUPercent = (cpuDelta / sysDelta) * cpus * 100
		if out.CPUPercent < 0 {
			out.CPUPercent = 0
		}
	}

	out.MemoryUsage = effectiveMemory(s)
	out.MemoryLimit = s.MemoryStats.Limit
	if out.MemoryLimit > 0 {
		out.MemoryPercent = (float64(out.MemoryUsage) / float64(out.MemoryLimit)) * 100
	}

	for _, n := range s.Networks {
		out.NetRxBytes += n.RxBytes
		out.NetTxBytes += n.TxBytes
	}
	for _, b := range s.BlkioStats.IoServiceBytesRecursive {
		switch b.Op {
		case "Read", "read":
			out.BlockReadBytes += b.Value
		case "Write", "write":
			out.BlockWriteBytes += b.Value
		}
	}
	out.PIDs = s.PidsStats.Current

	if out.Timestamp.IsZero() {
		out.Timestamp = time.Now()
	}
	return out
}

// effectiveMemory mirrors `docker stats` — usage minus the kernel cache that
// would be reclaimable if needed.
func effectiveMemory(s container.StatsResponse) uint64 {
	usage := s.MemoryStats.Usage
	if v, ok := s.MemoryStats.Stats["cache"]; ok && v <= usage {
		return usage - v
	}
	if v, ok := s.MemoryStats.Stats["inactive_file"]; ok && v <= usage {
		return usage - v
	}
	return usage
}
