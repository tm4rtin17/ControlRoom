package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// DockerClient is the production Client implementation.
type DockerClient struct {
	cli *client.Client
}

// New attempts to connect to the Docker daemon using DOCKER_HOST env or the
// default socket. Returns ErrUnavailable if the daemon is not reachable.
func New(ctx context.Context, host string) (*DockerClient, error) {
	opts := []client.Opt{client.WithAPIVersionNegotiation()}
	if host != "" {
		opts = append(opts, client.WithHost("unix://"+host))
	} else {
		opts = append(opts, client.FromEnv)
	}
	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, err.Error())
	}

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := cli.Ping(pingCtx); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, err.Error())
	}
	return &DockerClient{cli: cli}, nil
}

func (c *DockerClient) Close() error {
	if c == nil || c.cli == nil {
		return nil
	}
	return c.cli.Close()
}

func (c *DockerClient) List(ctx context.Context, opts ListOptions) ([]Container, error) {
	summaries, err := c.cli.ContainerList(ctx, container.ListOptions{All: opts.All})
	if err != nil {
		return nil, err
	}
	out := make([]Container, 0, len(summaries))
	for _, s := range summaries {
		c := Container{
			ID:        shortID(s.ID),
			Name:      primaryName(s.Names),
			Image:     s.Image,
			State:     s.State,
			Status:    s.Status,
			CreatedAt: time.Unix(s.Created, 0),
			Labels:    s.Labels,
		}
		c.ComposeProject = s.Labels["com.docker.compose.project"]
		c.ComposeService = s.Labels["com.docker.compose.service"]
		for _, p := range s.Ports {
			c.Ports = append(c.Ports, Port{
				IP:          p.IP,
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Protocol:    p.Type,
			})
		}
		out = append(out, c)
	}
	return out, nil
}

func (c *DockerClient) Inspect(ctx context.Context, id string) (*ContainerDetail, error) {
	j, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}
	createdAt, _ := time.Parse(time.RFC3339Nano, j.Created)
	startedAt, _ := time.Parse(time.RFC3339Nano, j.State.StartedAt)
	finishedAt, _ := time.Parse(time.RFC3339Nano, j.State.FinishedAt)

	d := &ContainerDetail{
		Container: Container{
			ID:        shortID(j.ID),
			Name:      strings.TrimPrefix(j.Name, "/"),
			Image:     j.Config.Image,
			State:     j.State.Status,
			Status:    j.State.Status,
			CreatedAt: createdAt,
			Labels:    j.Config.Labels,
		},
		Command:    j.Config.Entrypoint,
		Env:        j.Config.Env,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		ExitCode:   j.State.ExitCode,
	}
	if j.Config.Cmd != nil {
		d.Command = append(d.Command, j.Config.Cmd...)
	}
	d.ComposeProject = j.Config.Labels["com.docker.compose.project"]
	d.ComposeService = j.Config.Labels["com.docker.compose.service"]
	if j.HostConfig != nil {
		d.RestartPolicy = string(j.HostConfig.RestartPolicy.Name)
	}
	if j.State.Health != nil {
		d.Health = j.State.Health.Status
	}
	for _, m := range j.Mounts {
		d.Mounts = append(d.Mounts, Mount{
			Type:        string(m.Type),
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
			RW:          m.RW,
		})
	}
	if j.NetworkSettings != nil {
		for name, net := range j.NetworkSettings.Networks {
			d.Networks = append(d.Networks, NetworkAttach{
				Name:       name,
				IPAddress:  net.IPAddress,
				Gateway:    net.Gateway,
				MacAddress: net.MacAddress,
			})
		}
	}
	return d, nil
}

func (c *DockerClient) Start(ctx context.Context, id string) error {
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (c *DockerClient) Stop(ctx context.Context, id string, timeout time.Duration) error {
	t := int(timeout.Seconds())
	return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &t})
}

func (c *DockerClient) Restart(ctx context.Context, id string, timeout time.Duration) error {
	t := int(timeout.Seconds())
	return c.cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &t})
}

func (c *DockerClient) Remove(ctx context.Context, id string, force bool) error {
	return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force, RemoveVolumes: false})
}

func (c *DockerClient) Logs(ctx context.Context, id string, tail int) (<-chan LogLine, error) {
	if tail < 0 {
		tail = 0
	}
	rc, err := c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
		Tail:       fmtInt(tail),
	})
	if err != nil {
		return nil, err
	}

	ch := make(chan LogLine, 64)
	go func() {
		defer close(ch)
		defer rc.Close()
		demuxLogs(ctx, rc, ch)
	}()
	return ch, nil
}

func (c *DockerClient) Stats(ctx context.Context, id string) (<-chan ContainerStats, error) {
	resp, err := c.cli.ContainerStats(ctx, id, true)
	if err != nil {
		return nil, err
	}

	ch := make(chan ContainerStats, 8)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		dec := json.NewDecoder(resp.Body)
		for {
			if ctx.Err() != nil {
				return
			}
			var raw container.StatsResponse
			if err := dec.Decode(&raw); err != nil {
				return
			}
			s := normalizeStats(raw)
			select {
			case <-ctx.Done():
				return
			case ch <- s:
			}
		}
	}()
	return ch, nil
}

// ---- helpers ----

func primaryName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	const digits = "0123456789"
	if n < 0 {
		return "0"
	}
	out := make([]byte, 0, 8)
	for n > 0 {
		out = append([]byte{digits[n%10]}, out...)
		n /= 10
	}
	return string(out)
}
