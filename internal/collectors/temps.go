package collectors

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// readTemperatures walks /sys/class/thermal/thermal_zone* and returns one
// reading per zone. Values are millidegrees C in the kernel; we divide.
//
// /sys/class/hwmon/hwmonN/temp*_input is more comprehensive (motherboard
// sensors, etc.) but inconsistent across hardware; thermal zones are the
// stable lowest common denominator and what the RPi exposes for SoC temp.
func readTemperatures() ([]Temperature, error) {
	matches, err := filepath.Glob("/sys/class/thermal/thermal_zone*")
	if err != nil {
		return nil, err
	}

	out := make([]Temperature, 0, len(matches))
	for _, dir := range matches {
		raw, err := os.ReadFile(filepath.Join(dir, "temp"))
		if err != nil {
			continue
		}
		mC, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
		if err != nil {
			continue
		}
		t := Temperature{
			Name:    filepath.Base(dir),
			Celsius: float64(mC) / 1000,
		}
		if labelBytes, err := os.ReadFile(filepath.Join(dir, "type")); err == nil {
			t.Label = strings.TrimSpace(string(labelBytes))
		} else {
			t.Label = t.Name
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Celsius > out[j].Celsius })
	return out, nil
}
