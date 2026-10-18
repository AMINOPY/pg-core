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
