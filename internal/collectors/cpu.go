package collectors

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// cpuTimes is the parsed jiffy counters for a single line of /proc/stat.
//
// Linux man stat(5):
//   cpu  user nice system idle iowait irq softirq steal guest guest_nice
type cpuTimes struct {
	user, nice, system, idle, iowait, irq, softirq, steal uint64
}

func (c cpuTimes) total() uint64 {
	return c.user + c.nice + c.system + c.idle + c.iowait + c.irq + c.softirq + c.steal
}

func (c cpuTimes) active() uint64 {
	// Everything except idle + iowait counts as "busy."
	return c.total() - c.idle - c.iowait
}

type cpuStat struct {
	overall cpuTimes
	perCore []cpuTimes
}

func readCPUStat() (cpuStat, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return cpuStat{}, err
	}
	defer f.Close()

	var s cpuStat
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu") {
			break
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		t := parseCPULine(fields[1:])
		if fields[0] == "cpu" {
			s.overall = t
		} else {
			s.perCore = append(s.perCore, t)
		}
	}
	return s, sc.Err()
}

func parseCPULine(values []string) cpuTimes {
	var t cpuTimes
	if len(values) > 0 {
		t.user, _ = strconv.ParseUint(values[0], 10, 64)
	}
	if len(values) > 1 {
		t.nice, _ = strconv.ParseUint(values[1], 10, 64)
	}
	if len(values) > 2 {
		t.system, _ = strconv.ParseUint(values[2], 10, 64)
	}
	if len(values) > 3 {
		t.idle, _ = strconv.ParseUint(values[3], 10, 64)
	}
	if len(values) > 4 {
		t.iowait, _ = strconv.ParseUint(values[4], 10, 64)
	}
	if len(values) > 5 {
		t.irq, _ = strconv.ParseUint(values[5], 10, 64)
	}
	if len(values) > 6 {
		t.softirq, _ = strconv.ParseUint(values[6], 10, 64)
	}
	if len(values) > 7 {
		t.steal, _ = strconv.ParseUint(values[7], 10, 64)
	}
	return t
}

// percentDelta returns active/total * 100 over the period between two samples.
// Returns 0 if either delta is non-positive (insufficient sample data).
func percentDelta(prev, curr cpuTimes) float64 {
	totalDelta := int64(curr.total()) - int64(prev.total())
	activeDelta := int64(curr.active()) - int64(prev.active())
	if totalDelta <= 0 {
		return 0
	}
	pct := float64(activeDelta) / float64(totalDelta) * 100
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct
}

// readCPUInfo reads /proc/cpuinfo for the model name + frequency. Falls back
// to "Hardware" / "Model" on ARM where "model name" isn't populated.
func readCPUInfo() (model string, freqMHz float64) {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "", 0
	}
	defer f.Close()

	var hardware, modelArm string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, val, ok := splitProcLine(sc.Text())
		if !ok {
			continue
		}
		switch key {
		case "model name":
			if model == "" {
				model = val
			}
		case "Hardware":
			hardware = val
		case "Model":
			modelArm = val
		case "cpu MHz":
			if freqMHz == 0 {
				freqMHz, _ = strconv.ParseFloat(val, 64)
			}
		}
	}
	if model == "" {
		switch {
		case modelArm != "":
			model = modelArm
		case hardware != "":
			model = hardware
		default:
			model = "Unknown CPU"
		}
	}
	return model, freqMHz
}

func splitProcLine(line string) (key, val string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}
