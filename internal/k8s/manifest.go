package k8s

import (
	"context"
	"fmt"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	sigsyaml "sigs.k8s.io/yaml"
)

// editableKinds lists the resource types that may be fetched and patched through
// the manifest API. Pods, Nodes, and Secrets are intentionally excluded.
var editableKinds = map[string]schema.GroupVersionResource{
	"deployment":  {Group: "apps", Version: "v1", Resource: "deployments"},
	"statefulset": {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"daemonset":   {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"service":     {Group: "", Version: "v1", Resource: "services"},
	"configmap":   {Group: "", Version: "v1", Resource: "configmaps"},
}

// editableGVK maps the same keys to the canonical GVK for the YAML apiVersion/kind header.
var editableGVK = map[string]schema.GroupVersionKind{
	"deployment":  {Group: "apps", Version: "v1", Kind: "Deployment"},
	"statefulset": {Group: "apps", Version: "v1", Kind: "StatefulSet"},
	"daemonset":   {Group: "apps", Version: "v1", Kind: "DaemonSet"},
	"service":     {Group: "", Version: "v1", Kind: "Service"},
	"configmap":   {Group: "", Version: "v1", Kind: "ConfigMap"},
}

// ErrManifestKind is returned when a kind is not in the editable allowlist.
type ErrManifestKind struct{ Kind string }

func (e ErrManifestKind) Error() string {
	return fmt.Sprintf("kind %q is not in the editable allowlist (deployment|statefulset|daemonset|service|configmap)", e.Kind)
}

// ErrManifestConflict is returned when the API server returns a 409 conflict.
var ErrManifestConflict = fmt.Errorf("conflict — resource was modified by another process; reload and retry")

// ErrManifestNotFound is returned when the resource does not exist.
var ErrManifestNotFound = fmt.Errorf("resource not found")

// ErrManifestInvalid is returned when the API server rejects with 422.
type ErrManifestInvalid struct{ Message string }

func (e ErrManifestInvalid) Error() string { return e.Message }

// ErrManifestForbidden is returned on 403 from the API server.
var ErrManifestForbidden = fmt.Errorf("forbidden")

// ErrManifestMismatch is returned when YAML metadata doesn't match URL params.
type ErrManifestMismatch struct{ Detail string }

func (e ErrManifestMismatch) Error() string { return e.Detail }

// stripMeta removes server-managed and conflict-prone fields before serialising.
// Keeps labels, annotations, spec. Removes status entirely.
func stripMeta(obj *unstructured.Unstructured) {
	meta, ok := obj.Object["metadata"].(map[string]interface{})
	if !ok {
		return
	}
	for _, field := range []string{
		"managedFields",
		"creationTimestamp",
		"uid",
		"resourceVersion",
		"generation",
	} {
		delete(meta, field)
	}
	obj.Object["metadata"] = meta
	delete(obj.Object, "status")
}

// GetManifest fetches a resource as cleaned YAML plus its resourceVersion and GVK.
func (c *Client) GetManifest(ctx context.Context, kind, namespace, name string) (yamlOut string, rv string, gvk schema.GroupVersionKind, err error) {
	normKind := strings.ToLower(kind)
	gvr, ok := editableKinds[normKind]
	if !ok {
		return "", "", schema.GroupVersionKind{}, ErrManifestKind{Kind: kind}
	}
	gvk = editableGVK[normKind]

	obj, err := c.dyn.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", "", gvk, mapManifestErr(err)
	}

	rv = obj.GetResourceVersion()

	stripMeta(obj)

	raw, err := sigsyaml.Marshal(obj.Object)
	if err != nil {
		return "", "", gvk, fmt.Errorf("marshal manifest: %w", err)
	}
	return string(raw), rv, gvk, nil
}

// ApplyManifest parses the YAML, cross-checks metadata against URL params,
// injects resourceVersion, and calls Update (PUT) on the dynamic client.
// Returns the post-update resourceVersion and any server warnings.
func (c *Client) ApplyManifest(ctx context.Context, kind, namespace, name, yamlText, resourceVersion string, dryRun bool) (newRV string, warnings []string, err error) {
	normKind := strings.ToLower(kind)
	gvr, ok := editableKinds[normKind]
	if !ok {
		return "", nil, ErrManifestKind{Kind: kind}
	}

	// Parse YAML → unstructured.
	var raw map[string]interface{}
	if unmarshalErr := sigsyaml.Unmarshal([]byte(yamlText), &raw); unmarshalErr != nil {
		return "", nil, fmt.Errorf("invalid YAML: %w", unmarshalErr)
	}
	if raw == nil {
		return "", nil, fmt.Errorf("invalid YAML: empty document")
	}

	obj := &unstructured.Unstructured{Object: raw}

	// Cross-check kind in YAML matches URL kind.
	yamlKind := strings.ToLower(obj.GetKind())
	if yamlKind != normKind {
		return "", nil, ErrManifestMismatch{Detail: fmt.Sprintf("YAML kind %q does not match URL kind %q", obj.GetKind(), kind)}
	}

	// Cross-check name and namespace in YAML match URL params.
	if obj.GetName() != name {
		return "", nil, ErrManifestMismatch{Detail: fmt.Sprintf("YAML metadata.name %q does not match URL name %q", obj.GetName(), name)}
	}
	if obj.GetNamespace() != namespace {
		return "", nil, ErrManifestMismatch{Detail: fmt.Sprintf("YAML metadata.namespace %q does not match URL namespace %q", obj.GetNamespace(), namespace)}
	}

	// Inject resourceVersion for conflict detection.
	obj.SetResourceVersion(resourceVersion)

	var dryRunOpts []string
	if dryRun {
		dryRunOpts = []string{metav1.DryRunAll}
	}

	result, err := c.dyn.Resource(gvr).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{DryRun: dryRunOpts})
	if err != nil {
		return "", nil, mapManifestErr(err)
	}

	return result.GetResourceVersion(), []string{}, nil
}

// mapManifestErr translates typed k8s API errors to the sentinel errors above.
func mapManifestErr(err error) error {
	switch {
	case k8serrors.IsConflict(err):
		return ErrManifestConflict
	case k8serrors.IsNotFound(err):
		return ErrManifestNotFound
	case k8serrors.IsInvalid(err):
		return ErrManifestInvalid{Message: err.Error()}
	case k8serrors.IsForbidden(err):
		return ErrManifestForbidden
	}
	return err
}
