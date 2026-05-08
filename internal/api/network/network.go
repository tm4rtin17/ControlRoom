// Package network serves /api/network/*.
package network

import (
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/tm4rtin17/controlroom/internal/api/middleware"
	netpkg "github.com/tm4rtin17/controlroom/internal/network"
	"github.com/tm4rtin17/controlroom/internal/store"
)

type Deps struct {
	DB     *store.DB
	Logger zerolog.Logger
}

func MountHTTP(authed fiber.Router, d Deps) {
	g := authed.Group("/network")
	g.Get("/interfaces", d.listIfaces)
	g.Get("/firewall", d.firewallStatus)
	g.Post("/firewall/rules", d.addRule)
	g.Delete("/firewall/rules/:index", d.deleteRule)
	g.Post("/firewall/enable", d.enableFirewall)
	g.Post("/firewall/disable", d.disableFirewall)
}

// ---- handlers ----

type ifaceResp struct {
	Interfaces []netpkg.Interface `json:"interfaces"`
}

func (d Deps) listIfaces(c *fiber.Ctx) error {
	ifaces, err := netpkg.List(c.Context())
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "list interfaces: "+err.Error())
	}
	return c.JSON(ifaceResp{Interfaces: ifaces})
}

func (d Deps) firewallStatus(c *fiber.Ctx) error {
	st, err := netpkg.Status(c.Context())
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "ufw status: "+err.Error())
	}
	return c.JSON(st)
}

func (d Deps) addRule(c *fiber.Ctx) error {
	var spec netpkg.AddRuleSpec
	if err := c.BodyParser(&spec); err != nil {
		return fiber.NewError(http.StatusBadRequest, "invalid body")
	}
	if err := netpkg.AddRule(c.Context(), spec); err != nil {
		_ = d.audit(c, "network.firewall.rule_add", "", "failure", map[string]any{"spec": spec, "error": err.Error()})
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	_ = d.audit(c, "network.firewall.rule_add", "", "success", map[string]any{"spec": spec})
	return c.JSON(fiber.Map{"ok": true})
}

func (d Deps) deleteRule(c *fiber.Ctx) error {
	idx, err := strconv.Atoi(c.Params("index"))
	if err != nil || idx < 1 {
		return fiber.NewError(http.StatusBadRequest, "invalid index")
	}
	if err := netpkg.DeleteRule(c.Context(), idx); err != nil {
		_ = d.audit(c, "network.firewall.rule_delete", strconv.Itoa(idx), "failure", map[string]string{"error": err.Error()})
		return fiber.NewError(http.StatusInternalServerError, err.Error())
	}
	_ = d.audit(c, "network.firewall.rule_delete", strconv.Itoa(idx), "success", nil)
	return c.JSON(fiber.Map{"ok": true})
}

func (d Deps) enableFirewall(c *fiber.Ctx) error {
	if err := netpkg.Enable(c.Context()); err != nil {
		_ = d.audit(c, "network.firewall.enable", "", "failure", map[string]string{"error": err.Error()})
		return fiber.NewError(http.StatusInternalServerError, err.Error())
	}
	_ = d.audit(c, "network.firewall.enable", "", "success", nil)
	return c.JSON(fiber.Map{"ok": true})
}

func (d Deps) disableFirewall(c *fiber.Ctx) error {
	if err := netpkg.Disable(c.Context()); err != nil {
		_ = d.audit(c, "network.firewall.disable", "", "failure", map[string]string{"error": err.Error()})
		return fiber.NewError(http.StatusInternalServerError, err.Error())
	}
	_ = d.audit(c, "network.firewall.disable", "", "success", nil)
	return c.JSON(fiber.Map{"ok": true})
}

func (d Deps) audit(c *fiber.Ctx, action, target, outcome string, detail any) error {
	entry := store.AuditEntry{
		IP: c.IP(), Action: action, Target: target, Outcome: outcome, Detail: detail,
	}
	if u := middleware.CurrentUser(c); u != nil {
		entry.UserID = u.ID
	}
	return d.DB.WriteAudit(c.Context(), entry)
}
