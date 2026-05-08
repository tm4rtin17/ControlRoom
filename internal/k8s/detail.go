package k8s

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ---- detail domain types ----

type Event struct {
	Type    string `json:"type"`   // "Normal" | "Warning"
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Source  string `json:"source"` // "kubelet" | controller name
	Count   int32  `json:"count"`
	First   string `json:"first"` // RFC3339
	Last    string `json:"last"`  // RFC3339
}

type Condition struct {
	Type               string `json:"type"`
	Status             string `json:"status"` // "True" | "False" | "Unknown"
	Reason             string `json:"reason"`
	Message            string `json:"message"`
	LastTransitionTime string `json:"last_transition_time"` // RFC3339
}

type Taint struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Effect string `json:"effect"`
}

type NodeDetail struct {
	Node        Node         `json:"node"`
	Conditions  []Condition  `json:"conditions"`  // never nil
	Allocatable NodeCapacity `json:"allocatable"`
	Taints      []Taint      `json:"taints"`  // never nil
	Events      []Event      `json:"events"`  // never nil
}

type WorkloadDetail struct {
	Workload   Workload          `json:"workload"`
	Conditions []Condition       `json:"conditions"` // never nil
	Selector   map[string]string `json:"selector"`   // never nil
	Strategy   string            `json:"strategy"`
	Events     []Event           `json:"events"` // never nil
}

type ContainerStatus struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restart_count"`
	State        string `json:"state"`   // "running" | "waiting" | "terminated"
	Reason       string `json:"reason"`  // for waiting/terminated
	Started      string `json:"started"` // RFC3339; "" when not running
}

type PodDetail struct {
	Pod        Pod               `json:"pod"`
	Conditions []Condition       `json:"conditions"` // never nil
	Containers []ContainerStatus `json:"containers"` // never nil
	NodeName   string            `json:"node_name"`
	QoSClass   string            `json:"qos_class"`
	Events     []Event           `json:"events"` // never nil
}

type EndpointAddr struct {
	IP       string `json:"ip"`
	NodeName string `json:"node_name"`
	Ready    bool   `json:"ready"`
}

type ServiceDetail struct {
	Service   Service           `json:"service"`
	Endpoints []EndpointAddr    `json:"endpoints"` // never nil
	Selector  map[string]string `json:"selector"`  // never nil
	Events    []Event           `json:"events"`    // never nil
}

// ---- Get* methods ----

func (c *Client) GetNode(ctx context.Context, name string) (*NodeDetail, error) {
	n, err := c.cs.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	conditions := make([]Condition, 0, len(n.Status.Conditions))
	for _, cond := range n.Status.Conditions {
		conditions = append(conditions, Condition{
			Type:               string(cond.Type),
			Status:             string(cond.Status),
			Reason:             cond.Reason,
			Message:            cond.Message,
			LastTransitionTime: cond.LastTransitionTime.UTC().Format(time.RFC3339),
		})
	}

	taints := make([]Taint, 0, len(n.Spec.Taints))
	for _, t := range n.Spec.Taints {
		taints = append(taints, Taint{
			Key:    t.Key,
			Value:  t.Value,
			Effect: string(t.Effect),
		})
	}

	alloc := n.Status.Allocatable
	events, err := c.ListEvents(ctx, "", "Node", name)
	if err != nil {
		events = []Event{}
	}

	return &NodeDetail{
		Node:       nodeFrom(*n),
		Conditions: conditions,
		Allocatable: NodeCapacity{
			CPU:    alloc.Cpu().String(),
			Memory: alloc.Memory().String(),
			Pods:   alloc.Pods().String(),
		},
		Taints: taints,
		Events: events,
	}, nil
}

func (c *Client) GetWorkload(ctx context.Context, namespace, kind, name string) (*WorkloadDetail, error) {
	switch strings.ToLower(kind) {
	case "deployment":
		return c.getDeploymentDetail(ctx, namespace, name)
	case "statefulset":
		return c.getStatefulSetDetail(ctx, namespace, name)
	case "daemonset":
		return c.getDaemonSetDetail(ctx, namespace, name)
	default:
		return nil, fmt.Errorf("unknown workload kind: %s", kind)
	}
}

func (c *Client) getDeploymentDetail(ctx context.Context, namespace, name string) (*WorkloadDetail, error) {
	d, err := c.cs.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	conditions := make([]Condition, 0, len(d.Status.Conditions))
	for _, cond := range d.Status.Conditions {
		conditions = append(conditions, Condition{
			Type:               string(cond.Type),
			Status:             string(cond.Status),
			Reason:             cond.Reason,
			Message:            cond.Message,
			LastTransitionTime: cond.LastTransitionTime.UTC().Format(time.RFC3339),
		})
	}

	selector := selectorLabels(d.Spec.Selector)
	strategy := string(d.Spec.Strategy.Type)

	labels := d.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	images := containerImages(d.Spec.Template.Spec.Containers)

	events, err := c.ListEvents(ctx, namespace, "Deployment", name)
	if err != nil {
		events = []Event{}
	}

	return &WorkloadDetail{
		Workload: Workload{
			Kind:      "Deployment",
			Name:      d.Name,
			Namespace: d.Namespace,
			Ready:     WorkloadReady{Current: d.Status.ReadyReplicas, Desired: derefInt32(d.Spec.Replicas)},
			Images:    images,
			Age:       d.CreationTimestamp.UTC().Format(time.RFC3339),
			Labels:    labels,
		},
		Conditions: conditions,
		Selector:   selector,
		Strategy:   strategy,
		Events:     events,
	}, nil
}

func (c *Client) getStatefulSetDetail(ctx context.Context, namespace, name string) (*WorkloadDetail, error) {
	s, err := c.cs.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	conditions := make([]Condition, 0, len(s.Status.Conditions))
	for _, cond := range s.Status.Conditions {
		conditions = append(conditions, Condition{
			Type:               string(cond.Type),
			Status:             string(cond.Status),
			Reason:             cond.Reason,
			Message:            cond.Message,
			LastTransitionTime: cond.LastTransitionTime.UTC().Format(time.RFC3339),
		})
	}

	selector := selectorLabels(s.Spec.Selector)
	strategy := string(s.Spec.UpdateStrategy.Type)

	labels := s.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	images := containerImages(s.Spec.Template.Spec.Containers)

	events, err := c.ListEvents(ctx, namespace, "StatefulSet", name)
	if err != nil {
		events = []Event{}
	}

	return &WorkloadDetail{
		Workload: Workload{
			Kind:      "StatefulSet",
			Name:      s.Name,
			Namespace: s.Namespace,
			Ready:     WorkloadReady{Current: s.Status.ReadyReplicas, Desired: derefInt32(s.Spec.Replicas)},
			Images:    images,
			Age:       s.CreationTimestamp.UTC().Format(time.RFC3339),
			Labels:    labels,
		},
		Conditions: conditions,
		Selector:   selector,
		Strategy:   strategy,
		Events:     events,
	}, nil
}

func (c *Client) getDaemonSetDetail(ctx context.Context, namespace, name string) (*WorkloadDetail, error) {
	ds, err := c.cs.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// DaemonSets don't have conditions in status; return empty slice.
	conditions := []Condition{}

	selector := selectorLabels(ds.Spec.Selector)
	strategy := string(ds.Spec.UpdateStrategy.Type)

	labels := ds.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	images := containerImages(ds.Spec.Template.Spec.Containers)

	events, err := c.ListEvents(ctx, namespace, "DaemonSet", name)
	if err != nil {
		events = []Event{}
	}

	return &WorkloadDetail{
		Workload: Workload{
			Kind:      "DaemonSet",
			Name:      ds.Name,
			Namespace: ds.Namespace,
			Ready:     WorkloadReady{Current: ds.Status.NumberReady, Desired: ds.Status.DesiredNumberScheduled},
			Images:    images,
			Age:       ds.CreationTimestamp.UTC().Format(time.RFC3339),
			Labels:    labels,
		},
		Conditions: conditions,
		Selector:   selector,
		Strategy:   strategy,
		Events:     events,
	}, nil
}

func (c *Client) GetPod(ctx context.Context, namespace, name string) (*PodDetail, error) {
	p, err := c.cs.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	conditions := make([]Condition, 0, len(p.Status.Conditions))
	for _, cond := range p.Status.Conditions {
		conditions = append(conditions, Condition{
			Type:               string(cond.Type),
			Status:             string(cond.Status),
			Reason:             cond.Reason,
			Message:            cond.Message,
			LastTransitionTime: cond.LastTransitionTime.UTC().Format(time.RFC3339),
		})
	}

	// Build a map of container status by name for O(1) lookup.
	statusByName := make(map[string]corev1.ContainerStatus, len(p.Status.ContainerStatuses))
	for _, cs := range p.Status.ContainerStatuses {
		statusByName[cs.Name] = cs
	}

	containers := make([]ContainerStatus, 0, len(p.Spec.Containers))
	for _, spec := range p.Spec.Containers {
		cs, ok := statusByName[spec.Name]
		state := "waiting"
		reason := ""
		started := ""
		if ok {
			state, reason, started = containerState(cs.State)
		}
		ready := ok && cs.Ready
		var restarts int32
		if ok {
			restarts = cs.RestartCount
		}
		containers = append(containers, ContainerStatus{
			Name:         spec.Name,
			Image:        spec.Image,
			Ready:        ready,
			RestartCount: restarts,
			State:        state,
			Reason:       reason,
			Started:      started,
		})
	}

	events, err := c.ListEvents(ctx, namespace, "Pod", name)
	if err != nil {
		events = []Event{}
	}

	return &PodDetail{
		Pod:        podFrom(*p),
		Conditions: conditions,
		Containers: containers,
		NodeName:   p.Spec.NodeName,
		QoSClass:   string(p.Status.QOSClass),
		Events:     events,
	}, nil
}

func (c *Client) GetService(ctx context.Context, namespace, name string) (*ServiceDetail, error) {
	s, err := c.cs.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	selector := s.Spec.Selector
	if selector == nil {
		selector = map[string]string{}
	}

	// Fetch endpoints for this service.
	ep, err := c.cs.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
	endpoints := []EndpointAddr{}
	if err == nil {
		for _, subset := range ep.Subsets {
			for _, addr := range subset.Addresses {
				nodeName := ""
				if addr.NodeName != nil {
					nodeName = *addr.NodeName
				}
				endpoints = append(endpoints, EndpointAddr{IP: addr.IP, NodeName: nodeName, Ready: true})
			}
			for _, addr := range subset.NotReadyAddresses {
				nodeName := ""
				if addr.NodeName != nil {
					nodeName = *addr.NodeName
				}
				endpoints = append(endpoints, EndpointAddr{IP: addr.IP, NodeName: nodeName, Ready: false})
			}
		}
	}

	events, err := c.ListEvents(ctx, namespace, "Service", name)
	if err != nil {
		events = []Event{}
	}

	return &ServiceDetail{
		Service:   serviceFrom(*s),
		Endpoints: endpoints,
		Selector:  selector,
		Events:    events,
	}, nil
}

// ListEvents lists events whose involvedObject matches (namespace, kind, name).
// For cluster-scoped objects (e.g. Node), pass empty namespace.
func (c *Client) ListEvents(ctx context.Context, namespace, kind, name string) ([]Event, error) {
	// Field selector targets the specific object to avoid fetching all events.
	fieldSelector := fmt.Sprintf("involvedObject.kind=%s,involvedObject.name=%s", kind, name)
	if namespace != "" {
		fieldSelector += fmt.Sprintf(",involvedObject.namespace=%s", namespace)
	}

	list, err := c.cs.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, err
	}

	out := make([]Event, 0, len(list.Items))
	for _, ev := range list.Items {
		out = append(out, eventFrom(ev))
	}
	return out, nil
}

// ---- conversion helpers ----

func eventFrom(ev corev1.Event) Event {
	source := ev.Source.Component
	if source == "" {
		source = ev.ReportingController
	}

	// Prefer eventTime over firstTimestamp for newer events.
	first := ev.FirstTimestamp.UTC().Format(time.RFC3339)
	if ev.FirstTimestamp.IsZero() && !ev.EventTime.IsZero() {
		first = ev.EventTime.UTC().Format(time.RFC3339)
	}
	last := ev.LastTimestamp.UTC().Format(time.RFC3339)
	if ev.LastTimestamp.IsZero() && !ev.EventTime.IsZero() {
		last = ev.EventTime.UTC().Format(time.RFC3339)
	}

	count := ev.Count
	if count == 0 {
		count = 1
	}

	return Event{
		Type:    ev.Type,
		Reason:  ev.Reason,
		Message: ev.Message,
		Source:  source,
		Count:   count,
		First:   first,
		Last:    last,
	}
}

func containerState(s corev1.ContainerState) (state, reason, started string) {
	if s.Running != nil {
		started = s.Running.StartedAt.UTC().Format(time.RFC3339)
		return "running", "", started
	}
	if s.Waiting != nil {
		return "waiting", s.Waiting.Reason, ""
	}
	if s.Terminated != nil {
		return "terminated", s.Terminated.Reason, ""
	}
	return "waiting", "", ""
}

func selectorLabels(sel *metav1.LabelSelector) map[string]string {
	if sel == nil || sel.MatchLabels == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(sel.MatchLabels))
	for k, v := range sel.MatchLabels {
		out[k] = v
	}
	return out
}

// PodLogs opens a streaming log reader for the given pod/container.
// container may be empty if the pod has exactly one container.
// sinceSeconds may be nil (no time floor).
func (c *Client) PodLogs(ctx context.Context, namespace, name, container string, tail int64, sinceSeconds *int64) (io.ReadCloser, error) {
	opts := &corev1.PodLogOptions{
		Follow:       true,
		TailLines:    &tail,
		SinceSeconds: sinceSeconds,
	}
	if container != "" {
		opts.Container = container
	}
	req := c.cs.CoreV1().Pods(namespace).GetLogs(name, opts)
	return req.Stream(ctx)
}
