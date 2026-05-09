package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// RestartWorkload triggers a controller-driven rollout by patching the pod
// template annotation — the same mechanism as `kubectl rollout restart`.
// kind is case-insensitive: deployment | statefulset | daemonset.
func (c *Client) RestartWorkload(ctx context.Context, namespace, kind, name string) error {
	restartedAt := time.Now().UTC().Format(time.RFC3339)
	patch := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"kubectl.kubernetes.io/restartedAt": restartedAt,
					},
				},
			},
		},
	}
	raw, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal patch: %w", err)
	}

	switch strings.ToLower(kind) {
	case "deployment":
		_, err = c.cs.AppsV1().Deployments(namespace).Patch(ctx, name, types.MergePatchType, raw, metav1.PatchOptions{})
	case "statefulset":
		_, err = c.cs.AppsV1().StatefulSets(namespace).Patch(ctx, name, types.MergePatchType, raw, metav1.PatchOptions{})
	case "daemonset":
		_, err = c.cs.AppsV1().DaemonSets(namespace).Patch(ctx, name, types.MergePatchType, raw, metav1.PatchOptions{})
	default:
		return fmt.Errorf("unknown workload kind: %s", kind)
	}
	return err
}

// ScaleWorkload updates the scale subresource for a Deployment or StatefulSet.
// DaemonSets cannot be scaled and callers should reject that kind before calling.
// Returns the post-update replica count.
func (c *Client) ScaleWorkload(ctx context.Context, namespace, kind, name string, replicas int32) (int32, error) {
	switch strings.ToLower(kind) {
	case "deployment":
		s, err := c.cs.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
		if err != nil {
			return 0, err
		}
		s.Spec.Replicas = replicas
		updated, err := c.cs.AppsV1().Deployments(namespace).UpdateScale(ctx, name, s, metav1.UpdateOptions{})
		if err != nil {
			return 0, err
		}
		return updated.Spec.Replicas, nil
	case "statefulset":
		s, err := c.cs.AppsV1().StatefulSets(namespace).GetScale(ctx, name, metav1.GetOptions{})
		if err != nil {
			return 0, err
		}
		s.Spec.Replicas = replicas
		updated, err := c.cs.AppsV1().StatefulSets(namespace).UpdateScale(ctx, name, s, metav1.UpdateOptions{})
		if err != nil {
			return 0, err
		}
		return updated.Spec.Replicas, nil
	default:
		return 0, fmt.Errorf("unknown or unscalable workload kind: %s", kind)
	}
}

// DeletePod deletes a single pod. The owning controller (Deployment/STS/DS/Job)
// will recreate it. gracePeriod nil means use the pod's own terminationGracePeriodSeconds.
// force=true sets gracePeriodSeconds=0 and propagationPolicy=Background.
func (c *Client) DeletePod(ctx context.Context, namespace, name string, gracePeriod *int64, force bool) error {
	opts := metav1.DeleteOptions{}
	if force {
		zero := int64(0)
		opts.GracePeriodSeconds = &zero
		bg := metav1.DeletePropagationBackground
		opts.PropagationPolicy = &bg
	} else if gracePeriod != nil {
		opts.GracePeriodSeconds = gracePeriod
	}
	return c.cs.CoreV1().Pods(namespace).Delete(ctx, name, opts)
}

// CordonNode patches spec.unschedulable on the named node.
// cordoned=true cordons; cordoned=false uncordons.
func (c *Client) CordonNode(ctx context.Context, name string, cordoned bool) error {
	patch := map[string]any{
		"spec": map[string]any{
			"unschedulable": cordoned,
		},
	}
	raw, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal patch: %w", err)
	}
	_, err = c.cs.CoreV1().Nodes().Patch(ctx, name, types.MergePatchType, raw, metav1.PatchOptions{})
	return err
}
