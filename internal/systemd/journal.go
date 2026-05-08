package systemd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"syscall"
)

// TailUnit shells out to journalctl and streams stdout lines through the
// returned channel. The child process is killed when the context is cancelled
// (via SetPgid + Kill on the process group, so any subprocesses go too).
//
// `n` is the initial backlog; subsequent lines arrive as they come.
func TailUnit(ctx context.Context, unit string, n int) (<-chan string, error) {
	if !ValidUnitName(unit) {
		return nil, fmt.Errorf("invalid unit name")
	}
	if n < 0 {
		n = 0
	}

	cmd := exec.CommandContext(ctx, "journalctl",
		"-u", unit,
		"-f",
		"-n", strconv.Itoa(n),
		"--output=short-iso",
		"--no-pager",
	)
	// Run in its own process group so killing it tears down any children.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stderr = io.Discard

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start journalctl: %w", err)
	}

	ch := make(chan string, 32)

	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(stdout)
		// Allow long lines (journal messages can exceed default 64K).
		scanner.Buffer(make([]byte, 0, 64*1024), 1*1024*1024)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case ch <- scanner.Text():
			}
		}
	}()

	go func() {
		<-ctx.Done()
		// Kill the whole pgrp; CommandContext kills only the leader.
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		_ = cmd.Wait()
	}()

	return ch, nil
}
