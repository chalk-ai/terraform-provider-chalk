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

func TestRenderChangelog(t *testing.T) {
	baseline := Snapshot{
		Version: "v1.0.2",
		Resources: map[string]Entity{
			"chalk_example": {
				Attributes: map[string]Attribute{
					"count": {Type: "string"},
				},
				Permissions: "**Required permissions:** `old.read`",
			},
		},
		DataSources: map[string]Entity{},
	}
	live := Snapshot{
		Version: "v1.0.2",
		Resources: map[string]Entity{
			"chalk_example": {
				Attributes: map[string]Attribute{
					"count":   {Type: "number"},
					"enabled": {Type: "bool"},
				},
				Permissions: "**Required permissions:** `new.read`",
			},
		},
		DataSources: map[string]Entity{"chalk_lookup": {}},
	}

	want := `---
subcategory: ""
page_title: "Chalk Provider: Changelog"
description: |-
  Changes to Chalk Terraform resources, data sources, attributes, and required permissions.
---

# Chalk provider changelog

For migration guidance and non-schema changes, see the [project changelog](https://github.com/chalk-ai/terraform-provider-chalk/blob/main/CHANGELOG.md).

## Unreleased

### Resources

- Changed attribute ` + "`chalk_example.count`" + ` type from ` + "`string`" + ` to ` + "`number`" + `.
- Added attribute ` + "`chalk_example.enabled`" + ` (` + "`bool`" + `).

### Data sources

- Added ` + "`chalk_lookup`" + `.

### Required permissions

- ` + "`chalk_example`" + ` permissions changed from ` + "`old.read`" + ` to ` + "`new.read`" + `.
`
	got := string(renderChangelog(live, []Snapshot{baseline}))
	if got != want {
		t.Fatalf("rendered changelog:\n%s\nwant:\n%s", got, want)
	}
}
