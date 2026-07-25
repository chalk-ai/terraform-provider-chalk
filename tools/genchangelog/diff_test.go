package main

import (
	"reflect"
	"testing"
)

func TestDiffSnapshotsTracksTicketScope(t *testing.T) {
	old := Snapshot{
		Resources: map[string]Entity{
			"chalk_old": {},
			"chalk_example": {
				Attributes: map[string]Attribute{
					"removed":  {Type: "string"},
					"typed":    {Type: "string"},
					"required": {Type: "string"},
				},
				Permissions: "**Required permissions:** `old.read`",
			},
		},
		DataSources: map[string]Entity{"chalk_old_data": {}},
	}
	current := Snapshot{
		Resources: map[string]Entity{
			"chalk_new": {},
			"chalk_example": {
				Attributes: map[string]Attribute{
					"added":    {Type: "bool"},
					"typed":    {Type: "number"},
					"required": {Type: "string", Required: true},
				},
				Permissions: "**Required permissions:** `new.read`",
			},
		},
		DataSources: map[string]Entity{"chalk_new_data": {}},
	}

	var kinds []string
	for _, change := range diffSnapshots(old, current) {
		kinds = append(kinds, change.Kind)
	}
	want := []string{
		changeEntityAdded,
		changeEntityRemoved,
		changePermissions,
		changeAttributeAdded,
		changeAttributeRemoved,
		changeRequired,
		changeAttributeType,
		changeEntityAdded,
		changeEntityRemoved,
	}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("change kinds = %v, want %v", kinds, want)
	}
}

func TestRenderChangelogIsDeterministic(t *testing.T) {
	baseline := Snapshot{
		Version:     "v1.0.2",
		Resources:   map[string]Entity{},
		DataSources: map[string]Entity{},
	}
	first := renderChangelog(baseline, []Snapshot{baseline})
	second := renderChangelog(baseline, []Snapshot{baseline})
	if !reflect.DeepEqual(first, second) {
		t.Fatal("rendered changelog is not deterministic")
	}
}
