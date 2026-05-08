// Package network provides read-only enumeration of interfaces (via the `ip`
// command's JSON output) and CRUD over UFW rules.
//
// We deliberately shell out to `ip -j` rather than pull in vishvananda/netlink
// — the JSON is small, stable, and saves a heavy transitive dependency on a
// rarely-used surface. Editing interfaces / netplan is out of scope for v0.1
// (deferred to M9 polish).
package network

import (
	"context"
	"encoding/json"
	"os/exec"
	"sort"
)

// Interface is the SPA-facing view.
type Interface struct {
	Name    string   `json:"name"`
	MAC     string   `json:"mac"`
	State   string   `json:"state"` // "UP" / "DOWN" / "UNKNOWN"
	MTU     int      `json:"mtu"`
	Type    string   `json:"type,omitempty"`
	IPs     []string `json:"ips"` // "192.168.1.5/24"
	Flags   []string `json:"flags"`
	Stats   Stats    `json:"stats"`
}

type Stats struct {
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
}

// rawAddrInfo is the slice element in `ip -j addr show`.
type rawAddrInfo struct {
	Family    string `json:"family"`
	Local     string `json:"local"`
	PrefixLen int    `json:"prefixlen"`
	Scope     string `json:"scope"`
}

type rawIface struct {
	IfName    string        `json:"ifname"`
	Operstate string        `json:"operstate"`
	MTU       int           `json:"mtu"`
	Address   string        `json:"address"`
	LinkType  string        `json:"link_type"`
	Flags     []string      `json:"flags"`
	AddrInfo  []rawAddrInfo `json:"addr_info"`
	Stats64   *struct {
		RX struct{ Bytes uint64 `json:"bytes"` } `json:"rx"`
		TX struct{ Bytes uint64 `json:"bytes"` } `json:"tx"`
	} `json:"stats64,omitempty"`
}

// List returns the host's interfaces, sorted by name with `lo` last.
func List(ctx context.Context) ([]Interface, error) {
	cmd := exec.CommandContext(ctx, "ip", "-j", "-s", "addr", "show")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var raws []rawIface
	if err := json.Unmarshal(out, &raws); err != nil {
		return nil, err
	}

	out2 := make([]Interface, 0, len(raws))
	for _, r := range raws {
		ifc := Interface{
			Name:  r.IfName,
			MAC:   r.Address,
			State: r.Operstate,
			MTU:   r.MTU,
			Type:  r.LinkType,
			Flags: r.Flags,
		}
		for _, a := range r.AddrInfo {
			if a.Local == "" {
				continue
			}
			ip := a.Local
			if a.PrefixLen > 0 {
				ip = a.Local + "/" + itoa(a.PrefixLen)
			}
			ifc.IPs = append(ifc.IPs, ip)
		}
		if r.Stats64 != nil {
			ifc.Stats.RxBytes = r.Stats64.RX.Bytes
			ifc.Stats.TxBytes = r.Stats64.TX.Bytes
		}
		out2 = append(out2, ifc)
	}

	sort.Slice(out2, func(i, j int) bool {
		ai, aj := out2[i].Name, out2[j].Name
		if ai == "lo" {
			return false
		}
		if aj == "lo" {
			return true
		}
		return ai < aj
	})
	return out2, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := "0123456789"
	out := make([]byte, 0, 4)
	for n > 0 {
		out = append([]byte{digits[n%10]}, out...)
		n /= 10
	}
	return string(out)
}
