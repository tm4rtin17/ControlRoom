package systemd

import (
	"context"
	"testing"
)

func TestFakeListAndFilter(t *testing.T) {
	t.Parallel()
	f := NewFake([]UnitDetail{
		{Unit: Unit{Name: "nginx.service", Description: "Web server", ActiveState: "active", SubState: "running"}},
		{Unit: Unit{Name: "ssh.service", Description: "OpenSSH server", ActiveState: "active", SubState: "running"}},
		{Unit: Unit{Name: "cron.service", Description: "Scheduler", ActiveState: "inactive", SubState: "dead"}},
		{Unit: Unit{Name: "tmp.mount", Description: "tmp dir", ActiveState: "active", SubState: "mounted"}},
	})
	ctx := context.Background()

	all, err := f.List(ctx, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Fatalf("expected 4 units, got %d", len(all))
	}

	services, err := f.List(ctx, ListFilter{Suffixes: []string{"service"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 3 {
		t.Fatalf("expected 3 services, got %d", len(services))
	}

	web, err := f.List(ctx, ListFilter{Search: "web"})
	if err != nil {
		t.Fatal(err)
	}
	if len(web) != 1 || web[0].Name != "nginx.service" {
		t.Fatalf("expected nginx, got %+v", web)
	}

	active, err := f.List(ctx, ListFilter{States: []string{"active"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 3 {
		t.Fatalf("expected 3 active, got %d", len(active))
	}
}

func TestFakeStartStop(t *testing.T) {
	t.Parallel()
	f := NewFake([]UnitDetail{
		{Unit: Unit{Name: "nginx.service", ActiveState: "inactive", SubState: "dead"}},
	})
	ctx := context.Background()

	if err := f.Start(ctx, "nginx.service"); err != nil {
		t.Fatal(err)
	}
	d, err := f.Get(ctx, "nginx.service")
	if err != nil {
		t.Fatal(err)
	}
	if d.ActiveState != "active" {
		t.Fatalf("expected active, got %s", d.ActiveState)
	}

	if err := f.Stop(ctx, "nginx.service"); err != nil {
		t.Fatal(err)
	}
	d, _ = f.Get(ctx, "nginx.service")
	if d.ActiveState != "inactive" {
		t.Fatalf("expected inactive, got %s", d.ActiveState)
	}
}

func TestValidUnitName(t *testing.T) {
	t.Parallel()
	good := []string{
		"nginx.service",
		"ssh.socket",
		"systemd-resolved.service",
		"getty@tty1.service",
		"mnt-data.mount",
		"app.path",
	}
	bad := []string{
		"",
		"nginx",
		"nginx.service ",
		"../etc/passwd",
		"nginx.service; rm -rf /",
		"nginx.exe",
	}
	for _, n := range good {
		if !ValidUnitName(n) {
			t.Errorf("expected %q valid", n)
		}
	}
	for _, n := range bad {
		if ValidUnitName(n) {
			t.Errorf("expected %q invalid", n)
		}
	}
}
