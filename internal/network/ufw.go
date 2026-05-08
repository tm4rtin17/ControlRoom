package network

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// UFWStatus is the parsed `ufw status numbered` view.
type UFWStatus struct {
	Active  bool      `json:"active"`
	Logging string    `json:"logging,omitempty"`
	Default string    `json:"default,omitempty"`
	Rules   []UFWRule `json:"rules"`
}

type UFWRule struct {
	Index  int    `json:"index"`
	To     string `json:"to"`
	Action string `json:"action"`
	From   string `json:"from"`
}

// Status runs `sudo -n ufw status numbered` and parses the human-readable
// output. We use sudo because UFW state is root-only.
func Status(ctx context.Context) (*UFWStatus, error) {
	cmd := exec.CommandContext(ctx, "sudo", "-n", "ufw", "status", "numbered")
	cmd.Env = append(cmd.Env, "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseUFWStatus(out), nil
}

// numberedRE matches the table rows: "[ N] To Action From"
//
//	[ 1] 22                         ALLOW IN    Anywhere
//	[ 2] 80,443/tcp                 ALLOW IN    Anywhere (v6)
var numberedRE = regexp.MustCompile(`^\[\s*(\d+)\]\s+(.+?)\s{2,}([A-Z][A-Z ]+?)\s{2,}(.+?)\s*$`)

func parseUFWStatus(out []byte) *UFWStatus {
	st := &UFWStatus{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "Status:"):
			val := strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
			st.Active = val == "active"
		case strings.HasPrefix(line, "Logging:"):
			st.Logging = strings.TrimSpace(strings.TrimPrefix(line, "Logging:"))
		case strings.HasPrefix(line, "Default:"):
			st.Default = strings.TrimSpace(strings.TrimPrefix(line, "Default:"))
		default:
			if m := numberedRE.FindStringSubmatch(line); m != nil {
				idx, _ := strconv.Atoi(m[1])
				st.Rules = append(st.Rules, UFWRule{
					Index:  idx,
					To:     strings.TrimSpace(m[2]),
					Action: strings.TrimSpace(m[3]),
					From:   strings.TrimSpace(m[4]),
				})
			}
		}
	}
	return st
}

// AddRuleSpec is the limited API surface exposed to the SPA — anything more
// elaborate goes via the terminal page.
type AddRuleSpec struct {
	Action   string `json:"action"`              // "allow" | "deny" | "reject" | "limit"
	Port     string `json:"port,omitempty"`       // "22" or "80,443"
	Protocol string `json:"protocol,omitempty"`   // "tcp" | "udp"
	From     string `json:"from,omitempty"`       // "192.168.1.0/24" | "10.0.0.5"
	Comment  string `json:"comment,omitempty"`
}

var (
	allowedActions = map[string]bool{"allow": true, "deny": true, "reject": true, "limit": true}
	portRE         = regexp.MustCompile(`^(\d{1,5})(:\d{1,5})?(,\d{1,5}(:\d{1,5})?)*$`)
	cidrRE         = regexp.MustCompile(`^[0-9a-fA-F.:]+(/\d{1,3})?$`)
	commentRE      = regexp.MustCompile(`^[a-zA-Z0-9 _.\-:]{1,64}$`)
)

func (a AddRuleSpec) validate() error {
	if !allowedActions[a.Action] {
		return fmt.Errorf("action must be one of allow/deny/reject/limit")
	}
	if a.Port != "" && !portRE.MatchString(a.Port) {
		return fmt.Errorf("invalid port spec %q", a.Port)
	}
	if a.Protocol != "" && a.Protocol != "tcp" && a.Protocol != "udp" {
		return fmt.Errorf("protocol must be tcp or udp")
	}
	if a.From != "" && !cidrRE.MatchString(a.From) {
		return fmt.Errorf("invalid from address %q", a.From)
	}
	if a.Comment != "" && !commentRE.MatchString(a.Comment) {
		return fmt.Errorf("invalid comment")
	}
	return nil
}

// Args formats this spec for `ufw <args>`. Order matters; see ufw(8).
func (a AddRuleSpec) Args() []string {
	args := []string{a.Action}
	if a.From != "" {
		args = append(args, "from", a.From, "to", "any")
	}
	if a.Port != "" {
		port := a.Port
		if a.Protocol != "" {
			port = port + "/" + a.Protocol
		}
		if a.From != "" {
			args = append(args, "port", port)
		} else {
			args = append(args, port)
		}
	}
	if a.Comment != "" {
		args = append(args, "comment", a.Comment)
	}
	return args
}

// AddRule adds a UFW rule built from spec. Spec is validated first so we
// never pass arbitrary user strings to sudo.
func AddRule(ctx context.Context, spec AddRuleSpec) error {
	if err := spec.validate(); err != nil {
		return err
	}
	args := append([]string{"-n", "ufw"}, spec.Args()...)
	return run(ctx, "sudo", args...)
}

// DeleteRule removes the rule at the given 1-based index.
func DeleteRule(ctx context.Context, index int) error {
	if index < 1 || index > 9999 {
		return fmt.Errorf("invalid rule index")
	}
	// `ufw --force delete N` skips the y/n prompt.
	return run(ctx, "sudo", "-n", "ufw", "--force", "delete", strconv.Itoa(index))
}

func Enable(ctx context.Context) error {
	return run(ctx, "sudo", "-n", "ufw", "--force", "enable")
}

func Disable(ctx context.Context) error {
	return run(ctx, "sudo", "-n", "ufw", "disable")
}

func run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
