package k8s

import (
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// PodExec opens a SPDY exec stream to the named container and pipes stdin to
// the container, multiplexing stdout+stderr back to the provided writers.
// The provided context cancels the stream when done. sizeQueue may be nil to
// skip TTY resize tracking.
func (c *Client) PodExec(
	ctx context.Context,
	namespace, pod, container string,
	command []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
	sizeQueue remotecommand.TerminalSizeQueue,
) error {
	req := c.cs.CoreV1().RESTClient().Post().
		Resource("pods").Name(pod).Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.cfg, "POST", req.URL())
	if err != nil {
		return err
	}

	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             stdin,
		Stdout:            stdout,
		Stderr:            stderr,
		Tty:               true,
		TerminalSizeQueue: sizeQueue,
	})
}
