// Package updates wraps `apt` for the homelab updates dashboard. All commands
// run with LC_ALL=C so we get a stable, English output to parse.
package updates

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Package is one row in `apt list --upgradable` output.
type Package struct {
	Name       string `json:"name"`
	Source     string `json:"source"` // e.g. "jammy-security"
	NewVersion string `json:"new_version"`
	OldVersion string `json:"old_version"`
	Arch       string `json:"arch"`
	Security   bool   `json:"security"`
}

// Upgradable returns the parsed list of packages with available upgrades.
// Errors from the underlying command surface as-is; an empty list is normal
// (no upgrades).
func Upgradable(ctx context.Context) ([]Package, error) {
	cmd := exec.CommandContext(ctx, "apt", "list", "--upgradable")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseUpgradable(out), nil
}

// parseUpgradable handles lines like:
//
//	Listing... Done
//	curl/jammy-updates 7.81.0-1ubuntu1.18 amd64 [upgradable from: 7.81.0-1ubuntu1.16]
//
// Anything that doesn't match is silently skipped; we trust apt to remain
// roughly consistent across versions.
func parseUpgradable(out []byte) []Package {
	var pkgs []Package
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "Listing") {
			continue
		}
		p, ok := parseUpgradableLine(line)
		if !ok {
			continue
		}
		pkgs = append(pkgs, p)
	}
	return pkgs
}

func parseUpgradableLine(line string) (Package, bool) {
	// Split into roughly three parts: "name/source version arch [upgradable from: oldver]"
	openBracket := strings.Index(line, "[")
	if openBracket < 0 {
		return Package{}, false
	}
	head := strings.TrimSpace(line[:openBracket])
	tail := line[openBracket:]

	headFields := strings.Fields(head)
	if len(headFields) < 3 {
		return Package{}, false
	}
	nameSource := headFields[0]
	slash := strings.Index(nameSource, "/")
	if slash < 0 {
		return Package{}, false
	}
	name := nameSource[:slash]
	source := nameSource[slash+1:]
	// Source like "jammy-updates,jammy-security" — pick the first.
	if comma := strings.Index(source, ","); comma >= 0 {
		source = source[:comma]
	}

	pkg := Package{
		Name:       name,
		Source:     source,
		NewVersion: headFields[1],
		Arch:       headFields[2],
		Security:   strings.Contains(strings.ToLower(headFields[0]), "security"),
	}

	// Tail like "[upgradable from: 7.81.0-1ubuntu1.16]"
	tail = strings.TrimSuffix(strings.TrimPrefix(tail, "["), "]")
	if idx := strings.Index(tail, ":"); idx >= 0 {
		pkg.OldVersion = strings.TrimSpace(tail[idx+1:])
	}
	return pkg, true
}

// commonEnv adds the locale + non-interactive defaults used by every apt run.
func commonEnv() []string {
	return append(os.Environ(),
		"LC_ALL=C",
		"DEBIAN_FRONTEND=noninteractive",
	)
}

// RunUpdate runs `sudo -n apt-get update`, streaming all output to w.
// The supplied context is honored for cancellation.
func RunUpdate(ctx context.Context, w io.Writer) error {
	cmd := exec.CommandContext(ctx, "sudo", "-n", "apt-get", "update")
	cmd.Env = commonEnv()
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

// RunUpgrade runs `sudo -n apt-get -y upgrade` with --force-confold so any
// changed config file is left in place rather than prompting the operator.
func RunUpgrade(ctx context.Context, w io.Writer) error {
	cmd := exec.CommandContext(ctx, "sudo", "-n", "apt-get",
		"-y",
		"-o", `Dpkg::Options::=--force-confold`,
		"upgrade",
	)
	cmd.Env = commonEnv()
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

// RunReboot triggers a system reboot via systemd. Used by the updates page's
// reboot-required banner.
func RunReboot(ctx context.Context, w io.Writer) error {
	cmd := exec.CommandContext(ctx, "sudo", "-n", "systemctl", "reboot")
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

// RebootRequired returns true if /var/run/reboot-required (the standard
// Debian/Ubuntu marker file) exists.
func RebootRequired() bool {
	_, err := os.Stat("/var/run/reboot-required")
	return err == nil
}
