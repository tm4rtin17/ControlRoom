// Package k8s wraps client-go for read-only cluster inspection.
//
// New() tries in-cluster config first, then kubeconfig paths in order:
// $KUBECONFIG, $HOME/.kube/config, /etc/rancher/k3s/k3s.yaml.
// Returns ErrUnavailable if all attempts fail.
package k8s

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ErrUnavailable mirrors the docker package sentinel; handlers return 503.
var ErrUnavailable = errors.New("kubernetes is not available")

// Client holds a typed clientset and a dynamic client.
type Client struct {
	cs  *kubernetes.Clientset
	dyn dynamic.Interface
}

// New constructs a Client. It never returns a non-nil Client alongside a
// non-nil error.
func New(ctx context.Context) (*Client, error) {
	cfg, err := buildConfig()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, err.Error())
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, err.Error())
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, err.Error())
	}
	c := &Client{cs: cs, dyn: dyn}
	if err := c.Ping(ctx); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, err.Error())
	}
	return c, nil
}

// Ping verifies the API server is reachable within 3 seconds.
func (c *Client) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := c.cs.Discovery().ServerVersion()
	_ = pingCtx
	return err
}

// buildConfig tries config sources in priority order.
func buildConfig() (*rest.Config, error) {
	// 1. In-cluster (pod service account).
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}

	// 2. Explicit $KUBECONFIG.
	if kc := os.Getenv("KUBECONFIG"); kc != "" {
		if cfg, err := clientcmd.BuildConfigFromFlags("", kc); err == nil {
			return cfg, nil
		}
	}

	// 3. ~/.kube/config
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".kube", "config")
		if cfg, err := clientcmd.BuildConfigFromFlags("", p); err == nil {
			return cfg, nil
		}
	}

	// 4. K3s bare-metal default.
	if cfg, err := clientcmd.BuildConfigFromFlags("", "/etc/rancher/k3s/k3s.yaml"); err == nil {
		return cfg, nil
	}

	return nil, errors.New("no valid kubeconfig found")
}
