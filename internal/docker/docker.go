// Package docker is a thin wrapper around docker/docker/client that surfaces
// only what ControlRoom needs (list/inspect/lifecycle/logs/stats), plus a fake
// implementation for tests.
//
// When the Docker socket isn't reachable the constructor returns
// ErrUnavailable; the api/containers handlers surface that as 503 and the SPA
// hides the navigation entry.
package docker

import (
	"context"
	"errors"
	"time"
)

// Container is the slim view returned by List. Compose-aware fields are
// extracted from the standard `com.docker.compose.*` labels.
type Container struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Image          string            `json:"image"`
	State          string            `json:"state"`  // "running" | "exited" | "paused" | …
	Status         string            `json:"status"` // human string from the daemon
	CreatedAt      time.Time         `json:"created_at"`
	Ports          []Port            `json:"ports"`
	Labels         map[string]string `json:"labels"`
	ComposeProject string            `json:"compose_project"`
	ComposeService string            `json:"compose_service"`
}

type Port struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port,omitempty"`
	Protocol    string `json:"protocol"`
}

// ContainerDetail is the full inspect view; heavier than List, fetched on demand.
type ContainerDetail struct {
	Container
	Command     []string          `json:"command"`
	Env         []string          `json:"env"` // raw VAR=value pairs; client may mask before render
	Mounts      []Mount           `json:"mounts"`
	Networks    []NetworkAttach   `json:"networks"`
	StartedAt   time.Time         `json:"started_at"`
	FinishedAt  time.Time         `json:"finished_at"`
	RestartPolicy string          `json:"restart_policy"`
	ExitCode    int               `json:"exit_code"`
	Health      string            `json:"health,omitempty"`
}

type Mount struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
	RW          bool   `json:"rw"`
}

type NetworkAttach struct {
	Name        string `json:"name"`
	IPAddress   string `json:"ip_address"`
	Gateway     string `json:"gateway"`
	MacAddress  string `json:"mac_address"`
}

// ContainerStats is one normalized sample. CPU% is computed against the
// previous sample by the daemon; we just relay what it tells us.
type ContainerStats struct {
	Timestamp        time.Time `json:"ts"`
	CPUPercent       float64   `json:"cpu_pct"`
	MemoryUsage      uint64    `json:"mem_usage"`
	MemoryLimit      uint64    `json:"mem_limit"`
	MemoryPercent    float64   `json:"mem_pct"`
	NetRxBytes       uint64    `json:"net_rx"`
	NetTxBytes       uint64    `json:"net_tx"`
	BlockReadBytes   uint64    `json:"blk_read"`
	BlockWriteBytes  uint64    `json:"blk_write"`
	PIDs             uint64    `json:"pids"`
}

// ListOptions combines the small set of filters the SPA actually uses.
type ListOptions struct {
	All bool // include stopped
}

type Client interface {
	List(ctx context.Context, opts ListOptions) ([]Container, error)
	Inspect(ctx context.Context, id string) (*ContainerDetail, error)
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string, timeout time.Duration) error
	Restart(ctx context.Context, id string, timeout time.Duration) error
	Remove(ctx context.Context, id string, force bool) error
	Logs(ctx context.Context, id string, tail int) (<-chan LogLine, error)
	Stats(ctx context.Context, id string) (<-chan ContainerStats, error)
	Close() error
}

// LogLine is a single demuxed log line. Stream is "stdout" or "stderr"
// (lowercase) — derived from docker's 8-byte multiplexed prefix.
type LogLine struct {
	Stream string `json:"stream"`
	Line   string `json:"line"`
}

// ErrUnavailable signals the Docker daemon is not reachable; bubbled up by
// New() and used by handlers to return 503 with code "containers.unavailable".
var ErrUnavailable = errors.New("docker daemon is not available")
