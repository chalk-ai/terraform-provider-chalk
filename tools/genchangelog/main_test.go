package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chalk-ai/terraform-provider-chalk/tools/internal/providerschema"
)

func TestWriteNewSnapshotIsIdempotentAndRefusesConflicts(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	first := providerschema.NewSnapshot("v1.0.0")
	first.Resources["chalk_example"] = providerschema.Entity{}
	if err := writeNewSnapshot(directory, first, nil); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadSnapshots(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeNewSnapshot(directory, first, loaded); err != nil {
		t.Fatalf("idempotent snapshot failed: %v", err)
	}

	conflict := providerschema.NewSnapshot("v1.0.0")
	conflict.Resources["chalk_example"] = providerschema.Entity{Schema: map[string]providerschema.Node{
		"name": {Kind: "attribute", Type: json.RawMessage(`"string"`)},
	}}
	if err := writeNewSnapshot(directory, conflict, loaded); err == nil {
		t.Fatal("expected conflicting snapshot overwrite to fail")
	}

	older := providerschema.NewSnapshot("v0.9.0")
	if err := writeNewSnapshot(directory, older, loaded); err == nil {
		t.Fatal("expected an older snapshot to fail")
	}
}

func TestCheckSnapshot(t *testing.T) {
	t.Parallel()
	live := providerschema.NewSnapshot("v1.0.1")
	live.Resources["chalk_example"] = providerschema.Entity{}
	version, _ := providerschema.ParseVersion("v1.0.0")
	snapshots := []versionedSnapshot{{
		version: version,
		value: func() *providerschema.Snapshot {
			value := providerschema.NewSnapshot("v1.0.0")
			value.Resources["chalk_example"] = providerschema.Entity{}
			return value
		}(),
	}}
	newerVersion, _ := providerschema.ParseVersion("v1.0.1")
	snapshots = append(snapshots, versionedSnapshot{version: newerVersion, value: live})
	if err := checkSnapshot("v1.0.1", live, snapshots); err != nil {
		t.Fatal(err)
	}
	if err := checkSnapshot("v1.0.0", live, snapshots); err == nil {
		t.Fatal("expected non-latest snapshot check to fail")
	}
	if err := checkSnapshot("v1.0.2", live, snapshots); err == nil {
		t.Fatal("expected missing snapshot check to fail")
	}
}

func TestLoadSnapshotsUsesSemanticVersionOrder(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	for _, version := range []string{"v0.9.10", "v0.9.9"} {
		snapshot := providerschema.NewSnapshot(version)
		data, err := providerschema.MarshalSnapshot(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, version+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	snapshots, err := loadSnapshots(directory)
	if err != nil {
		t.Fatal(err)
	}
	if snapshots[0].value.Version != "v0.9.9" || snapshots[1].value.Version != "v0.9.10" {
		t.Fatalf("snapshot order = %s, %s", snapshots[0].value.Version, snapshots[1].value.Version)
	}
}
