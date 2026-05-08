package collectors

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

func readHost() Host {
	hostname, _ := os.Hostname()
	return Host{
		Hostname: hostname,
		Distro:   readDistro(),
		Kernel:   readKernel(),
		Arch:     runtime.GOARCH,
	}
}

func readDistro() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "unknown"
	}
	defer f.Close()

	values := make(map[string]string)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := line[:idx]
		val := strings.Trim(line[idx+1:], `"`)
		values[key] = val
	}
	if pretty := values["PRETTY_NAME"]; pretty != "" {
		return pretty
	}
	if name := values["NAME"]; name != "" {
		if v := values["VERSION"]; v != "" {
			return fmt.Sprintf("%s %s", name, v)
		}
		return name
	}
	return "unknown"
}

func readKernel() string {
	if data, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		return strings.TrimSpace(string(data))
	}
	return "unknown"
}

func readUptime() float64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[0], 64)
	return v
}

func readLoadAvg() LoadAverages {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return LoadAverages{}
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return LoadAverages{}
	}
	var la LoadAverages
	la.One, _ = strconv.ParseFloat(fields[0], 64)
	la.Five, _ = strconv.ParseFloat(fields[1], 64)
	la.Fifteen, _ = strconv.ParseFloat(fields[2], 64)
	return la
}

func parseUintOr(s string, def uint64) uint64 {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return def
	}
	return v
}
