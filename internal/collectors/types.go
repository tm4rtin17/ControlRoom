// Package collectors gathers a snapshot of system metrics from /proc, /sys,
// and a few standard system files. The Aggregator owns the small bit of state
// (previous CPU and net counters) needed to compute rates, and caches its
// most recent snapshot for ~900ms so multiple WebSocket clients ticking at 1Hz
// don't each re-read /proc.
package collectors

// Overview is the JSON-serializable snapshot returned to the SPA.
type Overview struct {
	Host          Host           `json:"host"`
	UptimeSeconds float64        `json:"uptime_seconds"`
	LoadAvg       LoadAverages   `json:"load_avg"`
	CPU           CPU            `json:"cpu"`
	Memory        Memory         `json:"memory"`
	Disks         []Disk         `json:"disks"`
	Temperatures  []Temperature  `json:"temperatures"`
	Network       []NetInterface `json:"network"`
}

type Host struct {
	Hostname string `json:"hostname"`
	Distro   string `json:"distro"`
	Kernel   string `json:"kernel"`
	Arch     string `json:"arch"`
}

type LoadAverages struct {
	One     float64 `json:"one"`
	Five    float64 `json:"five"`
	Fifteen float64 `json:"fifteen"`
}

type CPU struct {
	Model      string    `json:"model"`
	FreqMHz    float64   `json:"freq_mhz"`
	Cores      int       `json:"cores"`
	OverallPct float64   `json:"overall_pct"`
	PerCorePct []float64 `json:"per_core_pct"`
}

type Memory struct {
	Total     uint64 `json:"total"`
	Used      uint64 `json:"used"`
	Free      uint64 `json:"free"`
	Available uint64 `json:"available"`
	Cached    uint64 `json:"cached"`
	Buffers   uint64 `json:"buffers"`
	SwapTotal uint64 `json:"swap_total"`
	SwapUsed  uint64 `json:"swap_used"`
}

type Disk struct {
	Mount      string `json:"mount"`
	Filesystem string `json:"filesystem"`
	Total      uint64 `json:"total"`
	Used       uint64 `json:"used"`
	Free       uint64 `json:"free"`
}

type Temperature struct {
	Name    string  `json:"name"`
	Label   string  `json:"label"`
	Celsius float64 `json:"celsius"`
}

type NetInterface struct {
	Name       string `json:"name"`
	RxBytes    uint64 `json:"rx_bytes"`
	TxBytes    uint64 `json:"tx_bytes"`
	RxRate     uint64 `json:"rx_rate"` // bytes per second
	TxRate     uint64 `json:"tx_rate"`
}
