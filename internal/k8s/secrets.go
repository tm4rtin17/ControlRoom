package k8s

import (
	"context"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Secret is the list-level representation (no data values).
type Secret struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Type      string   `json:"type"` // K8s SecretType e.g. "Opaque", "kubernetes.io/tls"
	Keys      []string `json:"keys"` // never nil; lex-sorted
	Age       string   `json:"age"`  // RFC3339
}

// SecretDetail is the full detail view.
type SecretDetail struct {
	Secret      Secret            `json:"secret"`
	Labels      map[string]string `json:"labels"`      // never nil
	Annotations map[string]string `json:"annotations"` // never nil
	Data        map[string]string `json:"data"`        // never nil; values are base64-decoded plaintext
	Events      []Event           `json:"events"`      // never nil
}

// sortedSecretKeys returns alphabetically sorted keys from a map[string][]byte.
func sortedSecretKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ListSecrets lists secrets. namespace="" means all namespaces.
func (c *Client) ListSecrets(ctx context.Context, namespace string) ([]Secret, error) {
	list, err := c.cs.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]Secret, 0, len(list.Items))
	for _, s := range list.Items {
		keys := sortedSecretKeys(s.Data)
		if keys == nil {
			keys = []string{}
		}
		out = append(out, Secret{
			Name:      s.Name,
			Namespace: s.Namespace,
			Type:      string(s.Type),
			Keys:      keys,
			Age:       s.CreationTimestamp.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

// GetSecret returns full detail for a secret including events.
// client-go already base64-decodes s.Data values; we cast []byte to string directly.
func (c *Client) GetSecret(ctx context.Context, namespace, name string) (*SecretDetail, error) {
	s, err := c.cs.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	labels := s.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	annotations := s.Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}

	data := make(map[string]string, len(s.Data))
	for k, v := range s.Data {
		data[k] = string(v)
	}

	keys := sortedSecretKeys(s.Data)
	if keys == nil {
		keys = []string{}
	}

	events, err := c.ListEvents(ctx, namespace, "Secret", name)
	if err != nil {
		events = []Event{}
	}

	return &SecretDetail{
		Secret: Secret{
			Name:      s.Name,
			Namespace: s.Namespace,
			Type:      string(s.Type),
			Keys:      keys,
			Age:       s.CreationTimestamp.UTC().Format(time.RFC3339),
		},
		Labels:      labels,
		Annotations: annotations,
		Data:        data,
		Events:      events,
	}, nil
}
