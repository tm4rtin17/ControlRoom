// Package k8s serves /api/k8s/* — read-only cluster inspection plus write actions.
// If Client is nil the module is disabled and every handler returns 503.
package k8s

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/rs/zerolog"

	"github.com/tm4rtin17/controlroom/internal/api/middleware"
	"github.com/tm4rtin17/controlroom/internal/k8s"
	"github.com/tm4rtin17/controlroom/internal/store"
)

// Deps mirrors the pattern used by api/services and api/containers.
type Deps struct {
	Client *k8s.Client // nil → module disabled
	DB     *store.DB
	Logger zerolog.Logger
}

const (
	wsLogWriteWait = 5 * time.Second
	wsLogReadWait  = 70 * time.Second
	wsLogPing      = 30 * time.Second

	tailDefault = 200
	tailMax     = 5000
)

// dns1123RE matches a DNS-1123 subdomain (k8s name and namespace format).
var dns1123RE = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

func validK8sName(s string) bool {
	return len(s) > 0 && len(s) <= 253 && dns1123RE.MatchString(s)
}

// MountHTTP registers REST endpoints on the authenticated router group.
func MountHTTP(authed fiber.Router, d Deps) {
	g := authed.Group("/k8s")
	g.Get("/nodes", d.nodesHandler)
	g.Get("/nodes/:name", d.nodeDetailHandler)
	g.Post("/nodes/:name/cordon", d.cordonNodeHandler)
	g.Get("/namespaces", d.namespacesHandler)
	g.Get("/workloads", d.workloadsHandler)
	g.Get("/workloads/:namespace/:kind/:name", d.workloadDetailHandler)
	g.Post("/workloads/:namespace/:kind/:name/restart", d.restartWorkloadHandler)
	g.Post("/workloads/:namespace/:kind/:name/scale", d.scaleWorkloadHandler)
	g.Get("/pods", d.podsHandler)
	g.Get("/pods/:namespace/:name", d.podDetailHandler)
	g.Delete("/pods/:namespace/:name", d.deletePodHandler)
	g.Get("/services", d.servicesHandler)
	g.Get("/services/:namespace/:name", d.serviceDetailHandler)
}

// MountWS registers WebSocket routes on the ws group.
func MountWS(wsGroup fiber.Router, d Deps) {
	g := wsGroup.Group("/k8s")
	g.Get("/pods/:namespace/:name/logs", websocket.New(d.podLogsWS, websocket.Config{HandshakeTimeout: 5 * time.Second}))
}

func (d Deps) requireClient() error {
	if d.Client == nil {
		return fiber.NewError(http.StatusServiceUnavailable, "kubernetes not available on this host")
	}
	return nil
}

type nodesResp struct {
	Nodes []k8s.Node `json:"nodes"`
}

func (d Deps) nodesHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	nodes, err := d.Client.ListNodes(c.Context())
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "list nodes: "+err.Error())
	}
	return c.JSON(nodesResp{Nodes: nodes})
}

func (d Deps) nodeDetailHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	name := c.Params("name")
	if !validK8sName(name) {
		return fiber.NewError(http.StatusBadRequest, "invalid node name")
	}
	detail, err := d.Client.GetNode(c.Context(), name)
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "get node: "+err.Error())
	}
	return c.JSON(detail)
}

type namespacesResp struct {
	Namespaces []k8s.Namespace `json:"namespaces"`
}

func (d Deps) namespacesHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	nss, err := d.Client.ListNamespaces(c.Context())
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "list namespaces: "+err.Error())
	}
	return c.JSON(namespacesResp{Namespaces: nss})
}

type workloadsResp struct {
	Workloads []k8s.Workload `json:"workloads"`
}

func (d Deps) workloadsHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	wls, err := d.Client.ListWorkloads(c.Context(), c.Query("namespace"))
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "list workloads: "+err.Error())
	}
	return c.JSON(workloadsResp{Workloads: wls})
}

// validWorkloadKind accepts deployment, statefulset, daemonset (case-insensitive).
func validWorkloadKind(kind string) bool {
	switch strings.ToLower(kind) {
	case "deployment", "statefulset", "daemonset":
		return true
	}
	return false
}

func (d Deps) workloadDetailHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	namespace := c.Params("namespace")
	kind := c.Params("kind")
	name := c.Params("name")
	if !validK8sName(namespace) {
		return fiber.NewError(http.StatusBadRequest, "invalid namespace")
	}
	if !validWorkloadKind(kind) {
		return fiber.NewError(http.StatusBadRequest, fmt.Sprintf("invalid workload kind %q: must be deployment, statefulset, or daemonset", kind))
	}
	if !validK8sName(name) {
		return fiber.NewError(http.StatusBadRequest, "invalid workload name")
	}
	detail, err := d.Client.GetWorkload(c.Context(), namespace, kind, name)
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "get workload: "+err.Error())
	}
	return c.JSON(detail)
}

type podsResp struct {
	Pods []k8s.Pod `json:"pods"`
}

func (d Deps) podsHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	pods, err := d.Client.ListPods(c.Context(), c.Query("namespace"))
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "list pods: "+err.Error())
	}
	return c.JSON(podsResp{Pods: pods})
}

func (d Deps) podDetailHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	namespace := c.Params("namespace")
	name := c.Params("name")
	if !validK8sName(namespace) {
		return fiber.NewError(http.StatusBadRequest, "invalid namespace")
	}
	if !validK8sName(name) {
		return fiber.NewError(http.StatusBadRequest, "invalid pod name")
	}
	detail, err := d.Client.GetPod(c.Context(), namespace, name)
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "get pod: "+err.Error())
	}
	return c.JSON(detail)
}

type servicesResp struct {
	Services []k8s.Service `json:"services"`
}

func (d Deps) servicesHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	svcs, err := d.Client.ListServices(c.Context(), c.Query("namespace"))
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "list services: "+err.Error())
	}
	return c.JSON(servicesResp{Services: svcs})
}

func (d Deps) serviceDetailHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	namespace := c.Params("namespace")
	name := c.Params("name")
	if !validK8sName(namespace) {
		return fiber.NewError(http.StatusBadRequest, "invalid namespace")
	}
	if !validK8sName(name) {
		return fiber.NewError(http.StatusBadRequest, "invalid service name")
	}
	detail, err := d.Client.GetService(c.Context(), namespace, name)
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "get service: "+err.Error())
	}
	return c.JSON(detail)
}

// ---- write action handlers ----

func (d Deps) restartWorkloadHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	namespace := c.Params("namespace")
	kind := c.Params("kind")
	name := c.Params("name")
	if !validK8sName(namespace) {
		return fiber.NewError(http.StatusBadRequest, "invalid namespace")
	}
	if !validWorkloadKind(kind) {
		return fiber.NewError(http.StatusBadRequest, fmt.Sprintf("invalid workload kind %q: must be deployment, statefulset, or daemonset", kind))
	}
	if !validK8sName(name) {
		return fiber.NewError(http.StatusBadRequest, "invalid workload name")
	}

	normalKind := strings.ToLower(kind)
	err := d.Client.RestartWorkload(c.Context(), namespace, normalKind, name)
	entry := store.AuditEntry{
		IP:      c.IP(),
		Action:  "k8s." + normalKind + ".restart",
		Target:  namespace + "/" + name,
		Outcome: "success",
	}
	if u := middleware.CurrentUser(c); u != nil {
		entry.UserID = u.ID
	}
	if err != nil {
		entry.Outcome = "failure"
		entry.Detail = map[string]any{"error": err.Error()}
		_ = d.DB.WriteAudit(c.Context(), entry)
		return fiber.NewError(http.StatusInternalServerError, "restart failed: "+err.Error())
	}
	_ = d.DB.WriteAudit(c.Context(), entry)
	return c.JSON(fiber.Map{"ok": true, "message": "rollout restarted"})
}

func (d Deps) scaleWorkloadHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	namespace := c.Params("namespace")
	kind := c.Params("kind")
	name := c.Params("name")
	if !validK8sName(namespace) {
		return fiber.NewError(http.StatusBadRequest, "invalid namespace")
	}
	normalKind := strings.ToLower(kind)
	if normalKind == "daemonset" {
		return fiber.NewError(http.StatusBadRequest, "daemonsets cannot be scaled")
	}
	if normalKind != "deployment" && normalKind != "statefulset" {
		return fiber.NewError(http.StatusBadRequest, fmt.Sprintf("invalid workload kind %q: must be deployment or statefulset", kind))
	}
	if !validK8sName(name) {
		return fiber.NewError(http.StatusBadRequest, "invalid workload name")
	}

	var body struct {
		Replicas *int32 `json:"replicas"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(http.StatusBadRequest, "invalid request body")
	}
	if body.Replicas == nil {
		return fiber.NewError(http.StatusBadRequest, "replicas is required")
	}
	if *body.Replicas < 0 || *body.Replicas > 1000 {
		return fiber.NewError(http.StatusBadRequest, "replicas must be between 0 and 1000")
	}

	newCount, err := d.Client.ScaleWorkload(c.Context(), namespace, normalKind, name, *body.Replicas)
	entry := store.AuditEntry{
		IP:      c.IP(),
		Action:  "k8s." + normalKind + ".scale",
		Target:  namespace + "/" + name,
		Outcome: "success",
		Detail:  map[string]any{"replicas": *body.Replicas},
	}
	if u := middleware.CurrentUser(c); u != nil {
		entry.UserID = u.ID
	}
	if err != nil {
		entry.Outcome = "failure"
		entry.Detail = map[string]any{"replicas": *body.Replicas, "error": err.Error()}
		_ = d.DB.WriteAudit(c.Context(), entry)
		return fiber.NewError(http.StatusInternalServerError, "scale failed: "+err.Error())
	}
	_ = d.DB.WriteAudit(c.Context(), entry)
	return c.JSON(fiber.Map{"ok": true, "replicas": newCount})
}

func (d Deps) deletePodHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	namespace := c.Params("namespace")
	name := c.Params("name")
	if !validK8sName(namespace) {
		return fiber.NewError(http.StatusBadRequest, "invalid namespace")
	}
	if !validK8sName(name) {
		return fiber.NewError(http.StatusBadRequest, "invalid pod name")
	}

	force := c.Query("force") == "true"
	var gracePeriod *int64
	if g := c.Query("grace"); g != "" {
		var n int64
		if _, err := fmt.Sscanf(g, "%d", &n); err != nil || n < 0 || n > 600 {
			return fiber.NewError(http.StatusBadRequest, "grace must be an integer between 0 and 600")
		}
		gracePeriod = &n
	}

	detail := map[string]any{"force": force}
	if gracePeriod != nil {
		detail["grace"] = *gracePeriod
	}

	err := d.Client.DeletePod(c.Context(), namespace, name, gracePeriod, force)
	entry := store.AuditEntry{
		IP:      c.IP(),
		Action:  "k8s.pod.delete",
		Target:  namespace + "/" + name,
		Outcome: "success",
		Detail:  detail,
	}
	if u := middleware.CurrentUser(c); u != nil {
		entry.UserID = u.ID
	}
	if err != nil {
		entry.Outcome = "failure"
		entry.Detail = map[string]any{"force": force, "error": err.Error()}
		_ = d.DB.WriteAudit(c.Context(), entry)
		return fiber.NewError(http.StatusInternalServerError, "delete pod failed: "+err.Error())
	}
	_ = d.DB.WriteAudit(c.Context(), entry)
	return c.JSON(fiber.Map{"ok": true})
}

func (d Deps) cordonNodeHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	name := c.Params("name")
	if !validK8sName(name) {
		return fiber.NewError(http.StatusBadRequest, "invalid node name")
	}

	var body struct {
		Cordoned *bool `json:"cordoned"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(http.StatusBadRequest, "invalid request body")
	}
	if body.Cordoned == nil {
		return fiber.NewError(http.StatusBadRequest, "cordoned is required")
	}

	action := "k8s.node.cordon"
	if !*body.Cordoned {
		action = "k8s.node.uncordon"
	}

	err := d.Client.CordonNode(c.Context(), name, *body.Cordoned)
	entry := store.AuditEntry{
		IP:      c.IP(),
		Action:  action,
		Target:  name,
		Outcome: "success",
		Detail:  map[string]any{"cordoned": *body.Cordoned},
	}
	if u := middleware.CurrentUser(c); u != nil {
		entry.UserID = u.ID
	}
	if err != nil {
		entry.Outcome = "failure"
		entry.Detail = map[string]any{"cordoned": *body.Cordoned, "error": err.Error()}
		_ = d.DB.WriteAudit(c.Context(), entry)
		return fiber.NewError(http.StatusInternalServerError, action+" failed: "+err.Error())
	}
	_ = d.DB.WriteAudit(c.Context(), entry)
	return c.JSON(fiber.Map{"ok": true, "cordoned": *body.Cordoned})
}

// ---- WebSocket ----

type wsLogFrame struct {
	Type   string `json:"type"`             // "line" | "error"
	Stream string `json:"stream,omitempty"` // always "stdout"
	Line   string `json:"line,omitempty"`
	Err    string `json:"err,omitempty"`
}

func (d Deps) podLogsWS(c *websocket.Conn) {
	defer func() { _ = c.Close() }()

	if d.Client == nil {
		_ = c.WriteJSON(wsLogFrame{Type: "error", Err: "kubernetes not available"})
		return
	}

	namespace := c.Params("namespace")
	name := c.Params("name")
	if !validK8sName(namespace) {
		_ = c.WriteJSON(wsLogFrame{Type: "error", Err: "invalid namespace"})
		return
	}
	if !validK8sName(name) {
		_ = c.WriteJSON(wsLogFrame{Type: "error", Err: "invalid pod name"})
		return
	}

	container := c.Query("container")
	// container is validated by the API server if provided; empty is fine (k8s picks the only one).

	tail := int64(tailDefault)
	if v := c.Query("tail"); v != "" {
		var n int64
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			if n > tailMax {
				n = tailMax
			}
			tail = n
		}
	}

	var sinceSeconds *int64
	if v := c.Query("since"); v != "" {
		if dur, err := time.ParseDuration(v); err == nil && dur > 0 {
			secs := int64(dur.Seconds())
			sinceSeconds = &secs
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = c.SetReadDeadline(time.Now().Add(wsLogReadWait))
	c.SetPongHandler(func(string) error { return c.SetReadDeadline(time.Now().Add(wsLogReadWait)) })

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()

	rc, err := d.Client.PodLogs(ctx, namespace, name, container, tail, sinceSeconds)
	if err != nil {
		_ = c.WriteJSON(wsLogFrame{Type: "error", Err: err.Error()})
		return
	}
	defer rc.Close()

	lines := make(chan string, 64)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(rc)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
	}()

	ping := time.NewTicker(wsLogPing)
	defer ping.Stop()

	for {
		select {
		case <-closed:
			return
		case <-ping.C:
			_ = c.SetWriteDeadline(time.Now().Add(wsLogWriteWait))
			if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case line, ok := <-lines:
			if !ok {
				return
			}
			_ = c.SetWriteDeadline(time.Now().Add(wsLogWriteWait))
			if err := c.WriteJSON(wsLogFrame{Type: "line", Stream: "stdout", Line: line}); err != nil {
				return
			}
		}
	}
}
