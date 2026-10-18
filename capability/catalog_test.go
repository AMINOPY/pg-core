package capability

import "testing"

func TestAssessUsesAdapterByDefault(t *testing.T) {
	c := NewCatalog()
	err := c.Register(Manifest{
		ID: "engine-x", Version: "1", Capabilities: []string{"dns"}, License: "unknown",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Assess("engine-x", Host{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != Adapter {
		t.Fatalf("expected adapter mode, got %q", got.Mode)
	}
}

func TestAssessRequiresHostCapabilities(t *testing.T) {
	c := NewCatalog()
	if err := c.Register(Manifest{
		ID: "engine-y", Version: "1", Capabilities: []string{"tun"},
		Requires: []Requirement{{Name: "android-vpn"}},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := c.Assess("engine-y", Host{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != Unavailable || len(got.Missing) != 1 {
		t.Fatalf("unexpected assessment: %+v", got)
	}
}

func TestAssessChecksMinimumCapabilityVersion(t *testing.T) {
	c := NewCatalog()
	if err := c.Register(Manifest{
		ID: "engine-versioned", Version: "1", Capabilities: []string{"dns"},
		Requires: []Requirement{{Name: "resolver", MinimumVersion: "1.4.0"}},
	}); err != nil {
		t.Fatal(err)
	}

	tooOld, err := c.Assess("engine-versioned", Host{Capabilities: map[string]string{"resolver": "1.3.9"}})
	if err != nil {
		t.Fatal(err)
	}
	if tooOld.Mode != Unavailable || len(tooOld.Missing) != 1 {
		t.Fatalf("expected version requirement to fail, got %+v", tooOld)
	}

	supported, err := c.Assess("engine-versioned", Host{Capabilities: map[string]string{"resolver": "v1.4.1"}})
	if err != nil {
		t.Fatal(err)
	}
	if supported.Mode != Adapter {
		t.Fatalf("expected adapter mode for supported version, got %q", supported.Mode)
	}
}

func TestAssessAllowsReviewedLicenseForNativeAdoption(t *testing.T) {
	c := NewCatalog()
	if err := c.Register(Manifest{
		ID: "engine-z", Version: "2", Capabilities: []string{"health"},
		License: "MIT", SourceReuseAllowed: true,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := c.Assess("engine-z", Host{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != Native {
		t.Fatalf("expected native mode, got %q", got.Mode)
	}
}

func TestCatalogReturnsDefensiveCopies(t *testing.T) {
	c := NewCatalog()
	m := Manifest{
		ID: "engine-copy", Version: "1", Capabilities: []string{"dns"},
		Requires: []Requirement{{Name: "resolver"}},
	}
	if err := c.Register(m); err != nil {
		t.Fatal(err)
	}

	listed := c.List()
	listed[0].Capabilities[0] = "mutated"
	listed[0].Requires[0].Name = "mutated"

	assessed, err := c.Assess("engine-copy", Host{})
	if err != nil {
		t.Fatal(err)
	}
	if assessed.Manifest.Capabilities[0] != "dns" || assessed.Manifest.Requires[0].Name != "resolver" {
		t.Fatalf("catalog data was mutated through returned slices: %+v", assessed.Manifest)
	}
}

func TestRegisterRejectsDuplicateID(t *testing.T) {
	c := NewCatalog()
	m := Manifest{ID: "same", Version: "1", Capabilities: []string{"health"}}
	if err := c.Register(m); err != nil {
		t.Fatal(err)
	}
	if err := c.Register(m); err == nil {
		t.Fatal("expected duplicate error")
	}
}
