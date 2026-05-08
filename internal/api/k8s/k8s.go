// Package k8s serves /api/k8s/* — read-only cluster inspection.
// If Client is nil the module is disabled and every handler returns 503.
package k8s

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/tm4rtin17/controlroom/internal/k8s"
)

// Deps mirrors the pattern used by api/services and api/containers.
type Deps struct {
	Client *k8s.Client // nil → module disabled
	Logger zerolog.Logger
}

// MountHTTP registers GET routes on the authenticated router group.
func MountHTTP(authed fiber.Router, d Deps) {
	g := authed.Group("/k8s")
	g.Get("/nodes", d.nodesHandler)
	g.Get("/namespaces", d.namespacesHandler)
	g.Get("/workloads", d.workloadsHandler)
	g.Get("/pods", d.podsHandler)
	g.Get("/services", d.servicesHandler)
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
