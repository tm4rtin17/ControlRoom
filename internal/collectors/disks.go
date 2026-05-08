package collectors

import (
	"bufio"
	"os"
	"sort"
	"strings"
	"syscall"
)

// pseudoFS lists filesystem types that aren't real disks; we hide them from
// the dashboard. Add more as edge cases come up.
var pseudoFS = map[string]bool{
	"proc":         true,
	"sysfs":        true,
	"cgroup":       true,
	"cgroup2":      true,
	"devtmpfs":     true,
	"devpts":       true,
	"tmpfs":        true,
	"mqueue":       true,
	"hugetlbfs":    true,
	"pstore":       true,
	"securityfs":   true,
	"debugfs":      true,
	"tracefs":      true,
	"configfs":     true,
	"fusectl":      true,
	"binfmt_misc":  true,
	"autofs":       true,
	"rpc_pipefs":   true,
	"nsfs":         true,
	"squashfs":     true, // snap mounts; useful but noisy
	"overlay":      true,
	"ramfs":        true,
}

func readDisks() ([]Disk, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := make(map[string]bool)
	var out []Disk

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		mount, fsType := fields[1], fields[2]
		if pseudoFS[fsType] {
			continue
		}
		// Hide bind-mounted duplicates: we keep the first sighting per mount.
		if seen[mount] {
			continue
		}
		seen[mount] = true

		var st syscall.Statfs_t
		if err := syscall.Statfs(mount, &st); err != nil {
			continue
		}
		bs := uint64(st.Bsize)
		total := st.Blocks * bs
		free := st.Bavail * bs
		if total == 0 {
			continue
		}
		out = append(out, Disk{
			Mount:      mount,
			Filesystem: fsType,
			Total:      total,
			Used:       total - free,
			Free:       free,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	// Largest first — easier scan on the dashboard.
	sort.Slice(out, func(i, j int) bool { return out[i].Total > out[j].Total })
	return out, nil
}
