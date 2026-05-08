package collectors

import (
	"bufio"
	"os"
	"sort"
	"strings"
)

// netSample is the cumulative byte counter pair for one interface.
type netSample struct {
	rx uint64
	tx uint64
}

// readNetSamples returns the current cumulative counters keyed by interface.
// Loopback is included (so the rate is visible if curious) but filtered out
// of the final Overview.
func readNetSamples() (map[string]netSample, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := make(map[string]netSample)
	sc := bufio.NewScanner(f)
	skipped := 0
	for sc.Scan() {
		// First two lines are headers.
		if skipped < 2 {
			skipped++
			continue
		}
		line := sc.Text()
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		name := strings.TrimSpace(line[:colon])
		fields := strings.Fields(line[colon+1:])
		if len(fields) < 9 {
			continue
		}
		var s netSample
		s.rx = parseUintOr(fields[0], 0)
		s.tx = parseUintOr(fields[8], 0)
		out[name] = s
	}
	return out, sc.Err()
}

// netDiff turns two cumulative samples into the rate-bearing NetInterface list.
// Counter wraps (rare on 64-bit) emit a 0 rate rather than a misleading
// massive number.
func netDiff(prev, curr map[string]netSample, elapsedSec float64) []NetInterface {
	if elapsedSec <= 0 {
		elapsedSec = 1
	}
	out := make([]NetInterface, 0, len(curr))
	for name, c := range curr {
		if name == "lo" {
			continue
		}
		ni := NetInterface{Name: name, RxBytes: c.rx, TxBytes: c.tx}
		if p, ok := prev[name]; ok {
			if c.rx >= p.rx {
				ni.RxRate = uint64(float64(c.rx-p.rx) / elapsedSec)
			}
			if c.tx >= p.tx {
				ni.TxRate = uint64(float64(c.tx-p.tx) / elapsedSec)
			}
		}
		out = append(out, ni)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
