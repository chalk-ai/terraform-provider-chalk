package main

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestLoadReleasesSortsVersionsNumerically(t *testing.T) {
	directory := t.TempDir()
	for _, version := range []string{"v1.0.10", "v1.1.0", "v1.0.2"} {
		if err := writeJSON(filepath.Join(directory, version+".json"), Release{Version: version}); err != nil {
			t.Fatal(err)
		}
	}

	releases, err := loadReleases(directory)
	if err != nil {
		t.Fatal(err)
	}
	var versions []string
	for _, release := range releases {
		versions = append(versions, release.Version)
	}
	want := []string{"v1.0.2", "v1.0.10", "v1.1.0"}
	if !reflect.DeepEqual(versions, want) {
		t.Fatalf("versions = %v, want %v", versions, want)
	}
}

func TestCaptureReleaseStoresDiffAndAdvancesCurrentSnapshot(t *testing.T) {
	directory := t.TempDir()
	current := Snapshot{
		Version: "v1.0.8",
		Resources: map[string]Entity{
			"chalk_example": {Attributes: map[string]Attribute{"count": {Type: "string"}}},
		},
		DataSources: map[string]Entity{},
	}
	if err := writeCurrentSnapshot(directory, current); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(directory, "releases", "v1.0.8.json"), Release{Version: "v1.0.8"}); err != nil {
		t.Fatal(err)
	}

	next := Snapshot{
		Version: "v1.0.9",
		Resources: map[string]Entity{
			"chalk_example": {Attributes: map[string]Attribute{"count": {Type: "number"}}},
		},
		DataSources: map[string]Entity{},
	}
	if err := captureRelease(directory, next); err != nil {
		t.Fatal(err)
	}

	gotCurrent, err := loadCurrentSnapshot(directory)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotCurrent, next) {
		t.Fatalf("current snapshot = %#v, want %#v", gotCurrent, next)
	}

	releases, err := loadReleases(filepath.Join(directory, "releases"))
	if err != nil {
		t.Fatal(err)
	}
	wantReleases := []Release{
		{Version: "v1.0.8"},
		{
			Version: "v1.0.9",
			Changes: []Change{{
				EntityKind: "resource",
				Entity:     "chalk_example",
				Attribute:  "count",
				Kind:       changeAttributeType,
				Before:     "string",
				After:      "number",
			}},
		},
	}
	if !reflect.DeepEqual(releases, wantReleases) {
		t.Fatalf("releases = %#v, want %#v", releases, wantReleases)
	}
}

func TestCaptureReleaseWithoutChangesStillRecordsVersion(t *testing.T) {
	directory := t.TempDir()
	current := Snapshot{
		Version:     "v1.0.8",
		Resources:   map[string]Entity{},
		DataSources: map[string]Entity{},
	}
	if err := writeCurrentSnapshot(directory, current); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(directory, "releases", "v1.0.8.json"), Release{Version: "v1.0.8"}); err != nil {
		t.Fatal(err)
	}

	next := current
	next.Version = "v1.0.9"
	if err := captureRelease(directory, next); err != nil {
		t.Fatal(err)
	}

	releases, err := loadReleases(filepath.Join(directory, "releases"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Release{{Version: "v1.0.8"}, {Version: "v1.0.9"}}
	if !reflect.DeepEqual(releases, want) {
		t.Fatalf("releases = %#v, want %#v", releases, want)
	}
	if err := captureRelease(directory, next); err != nil {
		t.Fatalf("recapturing unchanged release: %v", err)
	}
}

func TestCollectResourceSchemaFlattensAttributesAndBlocks(t *testing.T) {
	attributes := map[string]rschema.Attribute{
		"config": rschema.SingleNestedAttribute{
			Optional: true,
			Attributes: map[string]rschema.Attribute{
				"name": rschema.StringAttribute{Required: true},
			},
		},
		"labels": rschema.MapAttribute{
			ElementType: types.StringType,
			Optional:    true,
		},
	}
	blocks := map[string]rschema.Block{
		"rule": rschema.ListNestedBlock{
			NestedObject: rschema.NestedBlockObject{
				Attributes: map[string]rschema.Attribute{
					"enabled": rschema.BoolAttribute{Optional: true},
				},
			},
		},
	}

	got := map[string]Attribute{}
	if err := collectResourceSchema(context.Background(), got, "", attributes, blocks); err != nil {
		t.Fatal(err)
	}
	want := map[string]Attribute{
		"config":       {Type: "object"},
		"config.name":  {Type: "string", Required: true},
		"labels":       {Type: "map(string)"},
		"rule.enabled": {Type: "bool"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("attributes = %#v, want %#v", got, want)
	}
}
