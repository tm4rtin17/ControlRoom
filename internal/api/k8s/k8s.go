// Package k8s serves /api/k8s/* — read-only cluster inspection plus write actions.
// If Client is nil the module is disabled and every handler returns 503.
package k8s

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/rs/zerolog"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/remotecommand"

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

	wsExecWriteWait    = 5 * time.Second
	wsExecReadWait     = 70 * time.Second
	wsExecPingInterval = 30 * time.Second
	wsExecIdleTimeout  = 30 * time.Minute
	wsExecIdleCheck    = 15 * time.Second

	execCmdMaxLen   = 256
	execSizeQueueCh = 8
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
	g.Get("/configmaps", d.listConfigMapsHandler)
	g.Get("/configmaps/:namespace/:name", d.configMapDetailHandler)
	g.Put("/configmaps/:namespace/:name", d.updateConfigMapHandler)
}

// MountWS registers WebSocket routes on the ws group.
func MountWS(wsGroup fiber.Router, d Deps) {
	g := wsGroup.Group("/k8s")
	g.Get("/pods/:namespace/:name/logs", websocket.New(d.podLogsWS, websocket.Config{HandshakeTimeout: 5 * time.Second}))
	g.Get("/pods/:namespace/:name/exec", websocket.New(d.podExecWS, websocket.Config{HandshakeTimeout: 5 * time.Second}))
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

// ---- configmap handlers ----

const cmSizeLimit = 1 << 20 // 1 MiB

type configMapsResp struct {
	ConfigMaps []k8s.ConfigMap `json:"configmaps"`
}

func (d Deps) listConfigMapsHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	cms, err := d.Client.ListConfigMaps(c.Context(), c.Query("namespace"))
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "list configmaps: "+err.Error())
	}
	return c.JSON(configMapsResp{ConfigMaps: cms})
}

func (d Deps) configMapDetailHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	namespace := c.Params("namespace")
	name := c.Params("name")
	if !validK8sName(namespace) {
		return fiber.NewError(http.StatusBadRequest, "invalid namespace")
	}
	if !validK8sName(name) {
		return fiber.NewError(http.StatusBadRequest, "invalid configmap name")
	}
	detail, err := d.Client.GetConfigMap(c.Context(), namespace, name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return fiber.NewError(http.StatusNotFound, "configmap not found")
		}
		if k8serrors.IsForbidden(err) {
			return fiber.NewError(http.StatusForbidden, "forbidden")
		}
		return fiber.NewError(http.StatusInternalServerError, "get configmap: "+err.Error())
	}
	return c.JSON(detail)
}

type updateConfigMapReq struct {
	Data map[string]string `json:"data"`
}

func (d Deps) updateConfigMapHandler(c *fiber.Ctx) error {
	if err := d.requireClient(); err != nil {
		return err
	}
	namespace := c.Params("namespace")
	name := c.Params("name")
	if !validK8sName(namespace) {
		return fiber.NewError(http.StatusBadRequest, "invalid namespace")
	}
	if !validK8sName(name) {
		return fiber.NewError(http.StatusBadRequest, "invalid configmap name")
	}

	var body updateConfigMapReq
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(http.StatusBadRequest, "invalid request body")
	}
	if body.Data == nil {
		return fiber.NewError(http.StatusBadRequest, "data is required")
	}

	// Validate each key and compute total size.
	var totalSize int
	for k, v := range body.Data {
		if !k8s.ValidConfigMapKey(k) {
			return fiber.NewError(http.StatusBadRequest, fmt.Sprintf("invalid configmap key %q: must match ^[a-zA-Z0-9._-]+$", k))
		}
		totalSize += len(k) + len(v)
	}
	if totalSize > cmSizeLimit {
		return fiber.NewError(http.StatusRequestEntityTooLarge, fmt.Sprintf("data size %d bytes exceeds 1 MiB limit", totalSize))
	}

	err := d.Client.UpdateConfigMap(c.Context(), namespace, name, body.Data)
	entry := store.AuditEntry{
		IP:      c.IP(),
		Action:  "k8s.configmap.update",
		Target:  namespace + "/" + name,
		Outcome: "success",
		Detail:  map[string]any{"key_count": len(body.Data), "size_bytes": totalSize},
	}
	if u := middleware.CurrentUser(c); u != nil {
		entry.UserID = u.ID
	}
	if err != nil {
		entry.Outcome = "failure"
		entry.Detail = map[string]any{"key_count": len(body.Data), "size_bytes": totalSize, "error": err.Error()}
		_ = d.DB.WriteAudit(c.Context(), entry)
		if k8serrors.IsNotFound(err) {
			return fiber.NewError(http.StatusNotFound, "configmap not found")
		}
		if k8serrors.IsForbidden(err) {
			return fiber.NewError(http.StatusForbidden, "forbidden")
		}
		if k8serrors.IsConflict(err) {
			return fiber.NewError(http.StatusConflict, "configmap was modified by another process; please reload and retry")
		}
		return fiber.NewError(http.StatusInternalServerError, "update configmap: "+err.Error())
	}
	_ = d.DB.WriteAudit(c.Context(), entry)
	return c.JSON(fiber.Map{"ok": true})
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

// ---- pod exec WebSocket ----

// execInitMsg is the first text frame the client must send.
type execInitMsg struct {
	Rows      int      `json:"rows"`
	Cols      int      `json:"cols"`
	Container string   `json:"container"`
	Command   []string `json:"command,omitempty"`
}

// execControlMsg is a subsequent text frame from the client.
type execControlMsg struct {
	Type string `json:"type"`
	Rows int    `json:"rows,omitempty"`
	Cols int    `json:"cols,omitempty"`
}

// execErrorFrame is a fatal text frame sent server → client before close.
type execErrorFrame struct {
	Type string `json:"type"`
	Err  string `json:"err"`
}

// wsSizeQueue implements remotecommand.TerminalSizeQueue via a buffered channel.
type wsSizeQueue struct {
	ch chan remotecommand.TerminalSize
}

func (q *wsSizeQueue) Next() *remotecommand.TerminalSize {
	s, ok := <-q.ch
	if !ok {
		return nil
	}
	return &s
}

// wsWriter serialises binary writes to the WebSocket with a per-write deadline.
// It is used for both stdout and stderr (merged).
type wsExecWriter struct {
	conn     *websocket.Conn
	bytesOut *atomic.Int64
}

func (w *wsExecWriter) Write(p []byte) (int, error) {
	_ = w.conn.SetWriteDeadline(time.Now().Add(wsExecWriteWait))
	if err := w.conn.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	w.bytesOut.Add(int64(len(p)))
	return len(p), nil
}

func clampExecDim(n int) int {
	if n < 1 {
		return 1
	}
	if n > 500 {
		return 500
	}
	return n
}

func (d Deps) podExecWS(c *websocket.Conn) {
	defer func() { _ = c.Close() }()

	if d.Client == nil {
		_ = c.WriteJSON(execErrorFrame{Type: "error", Err: "kubernetes not available"})
		return
	}

	namespace := c.Params("namespace")
	name := c.Params("name")
	if !validK8sName(namespace) {
		_ = c.WriteJSON(execErrorFrame{Type: "error", Err: "invalid namespace"})
		return
	}
	if !validK8sName(name) {
		_ = c.WriteJSON(execErrorFrame{Type: "error", Err: "invalid pod name"})
		return
	}

	_ = c.SetReadDeadline(time.Now().Add(wsExecReadWait))
	c.SetPongHandler(func(string) error { return c.SetReadDeadline(time.Now().Add(wsExecReadWait)) })

	// Expect text init frame first.
	mt, raw, err := c.ReadMessage()
	if err != nil {
		return
	}
	if mt != websocket.TextMessage {
		_ = c.WriteJSON(execErrorFrame{Type: "error", Err: "expected JSON init frame first"})
		return
	}

	var init execInitMsg
	if err := json.Unmarshal(raw, &init); err != nil {
		_ = c.WriteJSON(execErrorFrame{Type: "error", Err: "invalid init frame: " + err.Error()})
		return
	}

	// Validate container (required, DNS-1123 label).
	if init.Container == "" {
		_ = c.WriteJSON(execErrorFrame{Type: "error", Err: "container is required"})
		return
	}
	if !validK8sName(init.Container) {
		_ = c.WriteJSON(execErrorFrame{Type: "error", Err: "invalid container name"})
		return
	}

	// Validate / default command.
	command := init.Command
	if len(command) == 0 {
		command = []string{"/bin/sh"}
	} else {
		for i, part := range command {
			if part == "" {
				_ = c.WriteJSON(execErrorFrame{Type: "error", Err: fmt.Sprintf("command[%d] is empty", i)})
				return
			}
			if len(part) > execCmdMaxLen {
				_ = c.WriteJSON(execErrorFrame{Type: "error", Err: fmt.Sprintf("command[%d] exceeds max length", i)})
				return
			}
		}
	}

	rows := clampExecDim(init.Rows)
	cols := clampExecDim(init.Cols)

	// Audit user.
	userID := int64(0)
	if u, ok := c.Locals(middleware.CtxUser).(*store.User); ok && u != nil {
		userID = u.ID
	}
	startedAt := time.Now()

	_ = d.DB.WriteAudit(context.Background(), store.AuditEntry{
		UserID:  userID,
		IP:      c.RemoteAddr().String(),
		Action:  "k8s.pod.exec.start",
		Target:  namespace + "/" + name,
		Outcome: "success",
		Detail:  map[string]any{"container": init.Container, "command": command},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Byte counters.
	var bytesIn, bytesOut atomic.Int64

	// Stdin pipe: WS read loop → stdinW → SPDY.
	stdinR, stdinW := io.Pipe()

	// Size queue.
	sq := &wsSizeQueue{ch: make(chan remotecommand.TerminalSize, execSizeQueueCh)}
	// Seed with initial size.
	sq.ch <- remotecommand.TerminalSize{Width: uint16(cols), Height: uint16(rows)}

	// stdout/stderr writer.
	writer := &wsExecWriter{conn: c, bytesOut: &bytesOut}

	// Launch SPDY exec in background.
	execDone := make(chan error, 1)
	go func() {
		execDone <- d.Client.PodExec(ctx, namespace, name, init.Container, command, stdinR, writer, writer, sq)
		_ = stdinR.Close()
	}()

	// Activity tracker for idle timeout.
	lastActivity := time.Now()
	activityCh := make(chan struct{}, 1)
	bumpActivity := func() {
		lastActivity = time.Now()
		select {
		case activityCh <- struct{}{}:
		default:
		}
	}

	// Idle / ping goroutine.
	stopIdle := make(chan struct{})
	go func() {
		defer close(stopIdle)
		ticker := time.NewTicker(wsExecIdleCheck)
		ping := time.NewTicker(wsExecPingInterval)
		defer ticker.Stop()
		defer ping.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-activityCh:
				continue
			case <-ping.C:
				_ = c.SetWriteDeadline(time.Now().Add(wsExecWriteWait))
				if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
					cancel()
					return
				}
			case <-ticker.C:
				if time.Since(lastActivity) > wsExecIdleTimeout {
					_ = c.SetWriteDeadline(time.Now().Add(wsExecWriteWait))
					_ = c.WriteJSON(execErrorFrame{Type: "error", Err: "idle timeout"})
					cancel()
					return
				}
			}
		}
	}()

	// WS read loop: binary → stdin, text → resize.
	var readLoopErr error
	for {
		mt, data, err := c.ReadMessage()
		if err != nil {
			readLoopErr = err
			break
		}
		bumpActivity()
		switch mt {
		case websocket.BinaryMessage:
			bytesIn.Add(int64(len(data)))
			if _, err := stdinW.Write(data); err != nil {
				goto done
			}
		case websocket.TextMessage:
			var ctrl execControlMsg
			if jsonErr := json.Unmarshal(data, &ctrl); jsonErr != nil {
				continue
			}
			if ctrl.Type == "resize" {
				r := clampExecDim(ctrl.Rows)
				col := clampExecDim(ctrl.Cols)
				select {
				case sq.ch <- remotecommand.TerminalSize{Width: uint16(col), Height: uint16(r)}:
				default:
				}
			}
		}
	}
done:
	// Cancel context and close stdin pipe so SPDY and size queue both unblock.
	cancel()
	_ = stdinW.Close()
	close(sq.ch)

	// Wait for exec to finish; capture any error to decide if we send an error frame.
	execErr := <-execDone

	<-stopIdle

	// Only send an error frame if exec failed (not a normal shell exit) and the
	// read loop didn't already detect a closed WS.
	if execErr != nil && readLoopErr == nil {
		_ = c.SetWriteDeadline(time.Now().Add(wsExecWriteWait))
		_ = c.WriteJSON(execErrorFrame{Type: "error", Err: execErr.Error()})
	}

	_ = d.DB.WriteAudit(context.Background(), store.AuditEntry{
		UserID:  userID,
		IP:      c.RemoteAddr().String(),
		Action:  "k8s.pod.exec.end",
		Target:  namespace + "/" + name,
		Outcome: "success",
		Detail: map[string]any{
			"container":   init.Container,
			"command":     command,
			"duration_ms": time.Since(startedAt).Milliseconds(),
			"bytes_in":    bytesIn.Load(),
			"bytes_out":   bytesOut.Load(),
		},
	})
}
