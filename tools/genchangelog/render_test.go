package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chalk-ai/terraform-provider-chalk/tools/internal/providerschema"
)

func TestRenderChangelogIncludesUnreleasedVersionsAndBaseline(t *testing.T) {
	t.Parallel()
	baseline := providerschema.NewSnapshot("v1.0.0")
	baseline.Resources["chalk_example"] = providerschema.Entity{}
	released := providerschema.NewSnapshot("v1.0.1")
	released.Resources["chalk_example"] = providerschema.Entity{Schema: map[string]providerschema.Node{
		"name": {Kind: "attribute", Type: json.RawMessage(`"string"`), Optional: true},
	}}
	live := providerschema.NewSnapshot("v1.0.1")
	live.Resources["chalk_example"] = providerschema.Entity{Schema: map[string]providerschema.Node{
		"name":  {Kind: "attribute", Type: json.RawMessage(`"string"`), Optional: true},
		"count": {Kind: "attribute", Type: json.RawMessage(`"number"`), Computed: true},
	}}
	v100, _ := providerschema.ParseVersion("v1.0.0")
	v101, _ := providerschema.ParseVersion("v1.0.1")
	snapshots := []versionedSnapshot{
		{version: v100, value: baseline},
		{version: v101, value: released},
	}

	first, err := renderChangelog(live, snapshots)
	if err != nil {
		t.Fatal(err)
	}
	second, err := renderChangelog(live, snapshots)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("rendering is not deterministic")
	}
	text := string(first)
	for _, want := range []string{
		"## Unreleased",
		"Added computed number attribute to `chalk_example.count`.",
		"## v1.0.1",
		"Added optional string attribute to `chalk_example.name`.",
		"## v1.0.0",
		"Baseline snapshot.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered changelog is missing %q\n%s", want, text)
		}
	}
	if strings.Index(text, "## v1.0.1") > strings.Index(text, "## v1.0.0") {
		t.Fatal("versions are not rendered newest-first")
	}
}

func TestDescribePermissionChangeIncludesEntityKindAndScope(t *testing.T) {
	t.Parallel()
	change := providerschema.Change{
		EntityKind: "data_source",
		Entity:     "chalk_environment",
		Path:       "deploy.read",
		Kind:       providerschema.ChangePermissionAdded,
		After:      "team-scoped",
	}
	got := describeChange(change)
	want := "Data source `chalk_environment` now requires `deploy.read` (team-scoped)."
	if got != want {
		t.Fatalf("describeChange() = %q, want %q", got, want)
	}
}
