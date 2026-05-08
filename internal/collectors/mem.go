package collectors

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func readMemory() (Memory, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return Memory{}, err
	}
	defer f.Close()

	values := make(map[string]uint64, 16)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, val, ok := splitProcLine(sc.Text())
		if !ok {
			continue
		}
		// Values look like "16332980 kB". Strip the unit and parse.
		fields := strings.Fields(val)
		if len(fields) == 0 {
			continue
		}
		n, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		values[key] = n * 1024 // kB → bytes
	}
	if err := sc.Err(); err != nil {
		return Memory{}, err
	}

	m := Memory{
		Total:     values["MemTotal"],
		Free:      values["MemFree"],
		Available: values["MemAvailable"],
		Cached:    values["Cached"],
		Buffers:   values["Buffers"],
		SwapTotal: values["SwapTotal"],
	}
	if swapFree := values["SwapFree"]; m.SwapTotal >= swapFree {
		m.SwapUsed = m.SwapTotal - swapFree
	}
	// Linux convention: "used" excludes Buffers + Cached + SReclaimable.
	used := int64(m.Total) - int64(m.Free) - int64(m.Buffers) - int64(m.Cached) - int64(values["SReclaimable"])
	if used > 0 {
		m.Used = uint64(used)
	}
	return m, nil
}
