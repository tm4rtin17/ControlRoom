package k8s

import (
	"context"
	"regexp"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// configmap key rule: ^[a-zA-Z0-9._-]+$
var cmKeyRE = regexp.MustCompile(`^[a-zA-Z0-9._\-]+$`)

// ValidConfigMapKey reports whether s is a valid configmap data key.
func ValidConfigMapKey(s string) bool {
	return len(s) > 0 && cmKeyRE.MatchString(s)
}

// ConfigMap is the list-level representation (no data values).
type ConfigMap struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Keys      []string `json:"keys"` // never nil; lex-sorted
	Age       string   `json:"age"`  // RFC3339
}

// ConfigMapDetail is the full detail view.
type ConfigMapDetail struct {
	ConfigMap   ConfigMap         `json:"configmap"`
	Labels      map[string]string `json:"labels"`      // never nil
	Annotations map[string]string `json:"annotations"` // never nil
	Data        map[string]string `json:"data"`        // never nil
	BinaryKeys  []string          `json:"binary_keys"` // never nil
	Events      []Event           `json:"events"`      // never nil
}

// sortedKeys returns the sorted keys of a map[string]string.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ListConfigMaps lists configmaps. namespace="" means all namespaces.
func (c *Client) ListConfigMaps(ctx context.Context, namespace string) ([]ConfigMap, error) {
	list, err := c.cs.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]ConfigMap, 0, len(list.Items))
	for _, cm := range list.Items {
		keys := sortedKeys(cm.Data)
		if keys == nil {
			keys = []string{}
		}
		out = append(out, ConfigMap{
			Name:      cm.Name,
			Namespace: cm.Namespace,
			Keys:      keys,
			Age:       cm.CreationTimestamp.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

// GetConfigMap returns full detail for a configmap including events.
func (c *Client) GetConfigMap(ctx context.Context, namespace, name string) (*ConfigMapDetail, error) {
	cm, err := c.cs.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	labels := cm.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	annotations := cm.Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}
	data := cm.Data
	if data == nil {
		data = map[string]string{}
	}

	binaryKeys := make([]string, 0, len(cm.BinaryData))
	for k := range cm.BinaryData {
		binaryKeys = append(binaryKeys, k)
	}
	sort.Strings(binaryKeys)

	keys := sortedKeys(cm.Data)
	if keys == nil {
		keys = []string{}
	}

	events, err := c.ListEvents(ctx, namespace, "ConfigMap", name)
	if err != nil {
		events = []Event{}
	}

	return &ConfigMapDetail{
		ConfigMap: ConfigMap{
			Name:      cm.Name,
			Namespace: cm.Namespace,
			Keys:      keys,
			Age:       cm.CreationTimestamp.UTC().Format(time.RFC3339),
		},
		Labels:      labels,
		Annotations: annotations,
		Data:        data,
		BinaryKeys:  binaryKeys,
		Events:      events,
	}, nil
}

// UpdateConfigMap replaces the data field of a configmap preserving binaryData.
// Uses Get+Update for resourceVersion conflict detection (409 on conflict).
func (c *Client) UpdateConfigMap(ctx context.Context, namespace, name string, newData map[string]string) error {
	cm, err := c.cs.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	cm.Data = newData
	_, err = c.cs.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}
