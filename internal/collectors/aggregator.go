package collectors

import (
	"sync"
	"time"
)

// cacheTTL is just under the WS push interval (1s) so concurrent ticks share
// a single /proc read but each tick still gets a fresh CPU/net delta.
const cacheTTL = 900 * time.Millisecond

// Aggregator keeps the per-process state needed to compute rate-based metrics
// (CPU %, net B/s) across snapshots. Safe for concurrent use.
type Aggregator struct {
	mu sync.Mutex

	// State for delta computation.
	prevCPU       cpuStat
	prevNet       map[string]netSample
	prevSampledAt time.Time

	// Cached snapshot.
	cached     Overview
	cachedAt   time.Time
	cachedErrs []string

	// Cached static-ish CPU info; refreshed on first snapshot only.
	cpuModel    string
	cpuFreqMHz  float64
	cpuInfoRead bool
}

func NewAggregator() *Aggregator {
	return &Aggregator{prevNet: map[string]netSample{}}
}

// Snapshot returns the current Overview, recomputing from /proc and /sys if
// the cached value is stale. Errors from individual collectors are surfaced
// in the returned []string but do not fail the whole snapshot — best-effort.
func (a *Aggregator) Snapshot() (Overview, []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.cachedAt.IsZero() && time.Since(a.cachedAt) < cacheTTL {
		return a.cached, a.cachedErrs
	}

	now := time.Now()
	elapsed := now.Sub(a.prevSampledAt).Seconds()
	if a.prevSampledAt.IsZero() {
		elapsed = 0
	}

	var errs []string

	// Static-ish CPU metadata: read once, then re-use.
	if !a.cpuInfoRead {
		a.cpuModel, a.cpuFreqMHz = readCPUInfo()
		a.cpuInfoRead = true
	}

	currCPU, err := readCPUStat()
	if err != nil {
		errs = append(errs, "cpu: "+err.Error())
	}
	currNet, err := readNetSamples()
	if err != nil {
		errs = append(errs, "net: "+err.Error())
	}
	mem, err := readMemory()
	if err != nil {
		errs = append(errs, "mem: "+err.Error())
	}
	disks, err := readDisks()
	if err != nil {
		errs = append(errs, "disks: "+err.Error())
	}
	temps, err := readTemperatures()
	if err != nil {
		errs = append(errs, "temps: "+err.Error())
	}

	// CPU percentages from delta.
	cpu := CPU{
		Model:   a.cpuModel,
		FreqMHz: a.cpuFreqMHz,
		Cores:   len(currCPU.perCore),
	}
	if elapsed > 0 {
		cpu.OverallPct = percentDelta(a.prevCPU.overall, currCPU.overall)
		cpu.PerCorePct = make([]float64, len(currCPU.perCore))
		for i, c := range currCPU.perCore {
			if i < len(a.prevCPU.perCore) {
				cpu.PerCorePct[i] = percentDelta(a.prevCPU.perCore[i], c)
			}
		}
	} else {
		cpu.PerCorePct = make([]float64, len(currCPU.perCore))
	}

	ov := Overview{
		Host:          readHost(),
		UptimeSeconds: readUptime(),
		LoadAvg:       readLoadAvg(),
		CPU:           cpu,
		Memory:        mem,
		Disks:         disks,
		Temperatures:  temps,
		Network:       netDiff(a.prevNet, currNet, elapsed),
	}

	a.prevCPU = currCPU
	a.prevNet = currNet
	a.prevSampledAt = now
	a.cached = ov
	a.cachedAt = now
	a.cachedErrs = errs
	return ov, errs
}
