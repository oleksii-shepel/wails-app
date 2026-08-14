package pluginpackage

import (
	"path/filepath"
	"testing"
)

func TestStateStoreDefaultsAreConservative(t *testing.T) {
	store, err := NewStateStoreAt(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Get("example")
	if err != nil {
		t.Fatal(err)
	}
	if state.Enabled || state.Trusted || state.Quarantined {
		t.Fatalf("unexpected permissive default: %+v", state)
	}
	if len(state.GrantedPermissions) != 0 {
		t.Fatalf("expected no permissions: %+v", state.GrantedPermissions)
	}
	if state.LastActivationStatus != ActivationNever {
		t.Fatalf("status = %q", state.LastActivationStatus)
	}
}

func TestStateStorePersistsUserDecisions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, _ := NewStateStoreAt(path)
	enabled, trusted := true, true
	permissions := []string{"network", "commands", "commands"}
	state, err := store.Update("example", PackageStatePatch{Enabled: &enabled, Trusted: &trusted, GrantedPermissions: &permissions})
	if err != nil {
		t.Fatal(err)
	}
	if !state.Enabled || !state.Trusted {
		t.Fatalf("state not updated: %+v", state)
	}
	if len(state.GrantedPermissions) != 2 || state.GrantedPermissions[0] != "commands" || state.GrantedPermissions[1] != "network" {
		t.Fatalf("permissions = %#v", state.GrantedPermissions)
	}
	reopened, _ := NewStateStoreAt(path)
	persisted, err := reopened.Get("example")
	if err != nil {
		t.Fatal(err)
	}
	if !persisted.Enabled || !persisted.Trusted || len(persisted.GrantedPermissions) != 2 {
		t.Fatalf("not persisted: %+v", persisted)
	}
}

func TestReconcileNeverBroadensTrustOrPermissions(t *testing.T) {
	store, _ := NewStateStoreAt(filepath.Join(t.TempDir(), "state.json"))
	enabled, trusted := true, false
	permissions := []string{"commands"}
	_, _ = store.Update("example", PackageStatePatch{Enabled: &enabled, Trusted: &trusted, GrantedPermissions: &permissions})
	if err := store.Reconcile([]DiscoveredPackage{{Valid: true, Manifest: Manifest{ID: "example", Version: "2.0.0"}}}); err != nil {
		t.Fatal(err)
	}
	state, _ := store.Get("example")
	if !state.Enabled || state.Trusted {
		t.Fatalf("decisions changed: %+v", state)
	}
	if len(state.GrantedPermissions) != 1 || state.GrantedPermissions[0] != "commands" {
		t.Fatalf("permissions changed: %#v", state.GrantedPermissions)
	}
	if state.LastSeenVersion != "2.0.0" {
		t.Fatalf("version = %q", state.LastSeenVersion)
	}
}

func TestQuarantineAndActivationHealth(t *testing.T) {
	store, _ := NewStateStoreAt(filepath.Join(t.TempDir(), "state.json"))
	state, err := store.Quarantine("example", "activation panic")
	if err != nil {
		t.Fatal(err)
	}
	if !state.Quarantined || state.LastActivationStatus != ActivationFailed {
		t.Fatalf("bad quarantine state: %+v", state)
	}
	state, err = store.Update("example", PackageStatePatch{ClearQuarantine: true})
	if err != nil {
		t.Fatal(err)
	}
	if state.Quarantined {
		t.Fatalf("quarantine not cleared: %+v", state)
	}
	state, err = store.RecordActivation("example", true, "ignored")
	if err != nil {
		t.Fatal(err)
	}
	if state.LastActivationStatus != ActivationSuccess || state.LastActivationError != "" || state.LastActivatedAt == 0 {
		t.Fatalf("bad activation state: %+v", state)
	}
}

func TestReconcileDropsTrustWhenPackageVersionChanges(t *testing.T) {
	store, _ := NewStateStoreAt(filepath.Join(t.TempDir(), "state.json"))
	enabled, trusted := true, true
	version := "1.0.0"
	_, err := store.Update("example", PackageStatePatch{Enabled: &enabled, Trusted: &trusted, TrustedVersion: &version})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Reconcile([]DiscoveredPackage{{Valid: true, Manifest: Manifest{ID: "example", Version: "2.0.0"}}}); err != nil {
		t.Fatal(err)
	}
	state, err := store.Get("example")
	if err != nil {
		t.Fatal(err)
	}
	if state.Trusted || state.TrustedVersion != "" {
		t.Fatalf("trust survived package update: %+v", state)
	}
	if !state.Enabled {
		t.Fatalf("package update should not disable explicit enablement: %+v", state)
	}
}

func TestValidateGrantedPermissionsRejectsUndeclaredPermission(t *testing.T) {
	manifest := Manifest{ID: "example", Permissions: []string{"commands"}}
	if err := ValidateGrantedPermissions(manifest, []string{"commands"}); err != nil {
		t.Fatalf("expected declared grant to pass: %v", err)
	}
	if err := ValidateGrantedPermissions(manifest, []string{"process"}); err == nil {
		t.Fatal("expected undeclared grant to fail")
	}
}
