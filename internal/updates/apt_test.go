package updates

import "testing"

func TestParseUpgradable(t *testing.T) {
	t.Parallel()
	input := []byte(`Listing... Done
curl/jammy-updates,jammy-security 7.81.0-1ubuntu1.18 amd64 [upgradable from: 7.81.0-1ubuntu1.16]
libssl3/jammy-security 3.0.2-0ubuntu1.18 amd64 [upgradable from: 3.0.2-0ubuntu1.15]
nginx-core/jammy 1.18.0-6ubuntu14.5 amd64 [upgradable from: 1.18.0-6ubuntu14.4]
`)
	pkgs := parseUpgradable(input)
	if len(pkgs) != 3 {
		t.Fatalf("expected 3 packages, got %d", len(pkgs))
	}

	if got := pkgs[0]; got.Name != "curl" || got.Source != "jammy-updates" || got.OldVersion != "7.81.0-1ubuntu1.16" || got.NewVersion != "7.81.0-1ubuntu1.18" {
		t.Errorf("curl parsed wrong: %+v", got)
	}
	if !pkgs[0].Security {
		t.Errorf("curl should be flagged Security (jammy-security in source list)")
	}
	if !pkgs[1].Security {
		t.Errorf("libssl3 should be flagged Security")
	}
	if pkgs[2].Security {
		t.Errorf("nginx-core should NOT be flagged Security")
	}
}

func TestParseUpgradableEmpty(t *testing.T) {
	t.Parallel()
	pkgs := parseUpgradable([]byte("Listing... Done\n"))
	if len(pkgs) != 0 {
		t.Fatalf("expected 0, got %d", len(pkgs))
	}
}

func TestParseUpgradableMalformed(t *testing.T) {
	t.Parallel()
	pkgs := parseUpgradable([]byte("garbage\nlines\nhere\nno-bracket-here\n"))
	if len(pkgs) != 0 {
		t.Fatalf("expected 0 from garbage, got %d", len(pkgs))
	}
}
