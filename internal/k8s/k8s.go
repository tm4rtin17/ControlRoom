package k8s

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ---- domain types (JSON shape is contract with the frontend) ----

type Node struct {
	Name      string        `json:"name"`
	Status    string        `json:"status"`    // "Ready" | "NotReady" | "Unknown"
	Roles     []string      `json:"roles"`     // never nil
	Version   string        `json:"version"`
	OS        string        `json:"os"`
	Arch      string        `json:"arch"`
	Addresses []NodeAddress `json:"addresses"` // never nil
	Capacity  NodeCapacity  `json:"capacity"`
	Age       string        `json:"age"`
}

type NodeAddress struct {
	Type    string `json:"type"`
	Address string `json:"address"`
}

type NodeCapacity struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Pods   string `json:"pods"`
}

type Namespace struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "Active" | "Terminating"
	Age    string `json:"age"`
}

type Workload struct {
	Kind      string            `json:"kind"`      // "Deployment" | "StatefulSet" | "DaemonSet"
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Ready     WorkloadReady     `json:"ready"`
	Images    []string          `json:"images"` // never nil
	Age       string            `json:"age"`
	Labels    map[string]string `json:"labels"` // never nil
}

type WorkloadReady struct {
	Current int32 `json:"current"`
	Desired int32 `json:"desired"`
}

type Pod struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Status    string   `json:"status"`
	Ready     PodReady `json:"ready"`
	Restarts  int32    `json:"restarts"` // sum across containers
	Node      string   `json:"node"`
	PodIP     string   `json:"pod_ip"`
	Age       string   `json:"age"`
	Images    []string `json:"images"` // never nil
}

type PodReady struct {
	Current int32 `json:"current"`
	Total   int32 `json:"total"`
}

type Service struct {
	Name       string        `json:"name"`
	Namespace  string        `json:"namespace"`
	Type       string        `json:"type"` // ClusterIP | NodePort | LoadBalancer
	ClusterIP  string        `json:"cluster_ip"`
	ExternalIP string        `json:"external_ip"` // empty if none
	Ports      []ServicePort `json:"ports"`       // never nil
	Age        string        `json:"age"`
}

type ServicePort struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort string `json:"target_port"` // intstr → string
	Protocol   string `json:"protocol"`
	NodePort   int32  `json:"node_port,omitempty"`
}

// ---- list methods ----

func (c *Client) ListNodes(ctx context.Context) ([]Node, error) {
	list, err := c.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]Node, 0, len(list.Items))
	for _, n := range list.Items {
		out = append(out, nodeFrom(n))
	}
	return out, nil
}

func (c *Client) ListNamespaces(ctx context.Context) ([]Namespace, error) {
	list, err := c.cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]Namespace, 0, len(list.Items))
	for _, ns := range list.Items {
		out = append(out, Namespace{
			Name:   ns.Name,
			Status: string(ns.Status.Phase),
			Age:    ns.CreationTimestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return out, nil
}

// ListWorkloads returns Deployments, StatefulSets, and DaemonSets.
// namespace="" means all namespaces.
func (c *Client) ListWorkloads(ctx context.Context, namespace string) ([]Workload, error) {
	var out []Workload

	deps, err := c.cs.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, d := range deps.Items {
		labels := d.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		images := containerImages(d.Spec.Template.Spec.Containers)
		out = append(out, Workload{
			Kind:      "Deployment",
			Name:      d.Name,
			Namespace: d.Namespace,
			Ready:     WorkloadReady{Current: d.Status.ReadyReplicas, Desired: derefInt32(d.Spec.Replicas)},
			Images:    images,
			Age:       d.CreationTimestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
			Labels:    labels,
		})
	}

	ssets, err := c.cs.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, s := range ssets.Items {
		labels := s.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		images := containerImages(s.Spec.Template.Spec.Containers)
		out = append(out, Workload{
			Kind:      "StatefulSet",
			Name:      s.Name,
			Namespace: s.Namespace,
			Ready:     WorkloadReady{Current: s.Status.ReadyReplicas, Desired: derefInt32(s.Spec.Replicas)},
			Images:    images,
			Age:       s.CreationTimestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
			Labels:    labels,
		})
	}

	dsets, err := c.cs.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, ds := range dsets.Items {
		labels := ds.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		images := containerImages(ds.Spec.Template.Spec.Containers)
		out = append(out, Workload{
			Kind:      "DaemonSet",
			Name:      ds.Name,
			Namespace: ds.Namespace,
			Ready:     WorkloadReady{Current: ds.Status.NumberReady, Desired: ds.Status.DesiredNumberScheduled},
			Images:    images,
			Age:       ds.CreationTimestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
			Labels:    labels,
		})
	}

	if out == nil {
		out = []Workload{}
	}
	return out, nil
}

func (c *Client) ListPods(ctx context.Context, namespace string) ([]Pod, error) {
	list, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]Pod, 0, len(list.Items))
	for _, p := range list.Items {
		out = append(out, podFrom(p))
	}
	return out, nil
}

func (c *Client) ListServices(ctx context.Context, namespace string) ([]Service, error) {
	list, err := c.cs.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]Service, 0, len(list.Items))
	for _, s := range list.Items {
		out = append(out, serviceFrom(s))
	}
	return out, nil
}

// ---- conversion helpers ----

const rolePrefix = "node-role.kubernetes.io/"

func nodeFrom(n corev1.Node) Node {
	roles := []string{}
	for k := range n.Labels {
		if strings.HasPrefix(k, rolePrefix) {
			roles = append(roles, strings.TrimPrefix(k, rolePrefix))
		}
	}

	addrs := make([]NodeAddress, 0, len(n.Status.Addresses))
	for _, a := range n.Status.Addresses {
		addrs = append(addrs, NodeAddress{Type: string(a.Type), Address: a.Address})
	}

	status := "Unknown"
	for _, cond := range n.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			switch cond.Status {
			case corev1.ConditionTrue:
				status = "Ready"
			case corev1.ConditionFalse:
				status = "NotReady"
			}
			break
		}
	}

	cap := n.Status.Capacity
	return Node{
		Name:      n.Name,
		Status:    status,
		Roles:     roles,
		Version:   n.Status.NodeInfo.KubeletVersion,
		OS:        n.Status.NodeInfo.OperatingSystem,
		Arch:      n.Status.NodeInfo.Architecture,
		Addresses: addrs,
		Capacity: NodeCapacity{
			CPU:    cap.Cpu().String(),
			Memory: cap.Memory().String(),
			Pods:   cap.Pods().String(),
		},
		Age: n.CreationTimestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func podFrom(p corev1.Pod) Pod {
	var restarts int32
	var readyCurrent int32
	for _, cs := range p.Status.ContainerStatuses {
		restarts += cs.RestartCount
		if cs.Ready {
			readyCurrent++
		}
	}
	images := make([]string, 0, len(p.Spec.Containers))
	for _, c := range p.Spec.Containers {
		images = append(images, c.Image)
	}
	return Pod{
		Name:      p.Name,
		Namespace: p.Namespace,
		Status:    string(p.Status.Phase),
		Ready:     PodReady{Current: readyCurrent, Total: int32(len(p.Spec.Containers))},
		Restarts:  restarts,
		Node:      p.Spec.NodeName,
		PodIP:     p.Status.PodIP,
		Age:       p.CreationTimestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
		Images:    images,
	}
}

func serviceFrom(s corev1.Service) Service {
	externalIP := ""
	if len(s.Status.LoadBalancer.Ingress) > 0 {
		ing := s.Status.LoadBalancer.Ingress[0]
		if ing.IP != "" {
			externalIP = ing.IP
		} else {
			externalIP = ing.Hostname
		}
	}
	ports := make([]ServicePort, 0, len(s.Spec.Ports))
	for _, p := range s.Spec.Ports {
		ports = append(ports, ServicePort{
			Name:       p.Name,
			Port:       p.Port,
			TargetPort: p.TargetPort.String(),
			Protocol:   string(p.Protocol),
			NodePort:   p.NodePort,
		})
	}
	return Service{
		Name:       s.Name,
		Namespace:  s.Namespace,
		Type:       string(s.Spec.Type),
		ClusterIP:  s.Spec.ClusterIP,
		ExternalIP: externalIP,
		Ports:      ports,
		Age:        s.CreationTimestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func containerImages(containers []corev1.Container) []string {
	images := make([]string, 0, len(containers))
	for _, c := range containers {
		images = append(images, c.Image)
	}
	return images
}

func derefInt32(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}
