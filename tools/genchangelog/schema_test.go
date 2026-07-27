package main

import (
	"context"
	"reflect"
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestLoadSnapshotsSortsVersionsNumerically(t *testing.T) {
	directory := t.TempDir()
	for _, version := range []string{"v1.0.10", "v1.1.0", "v1.0.2"} {
		if err := writeSnapshot(directory, Snapshot{
			Version:     version,
			Resources:   map[string]Entity{},
			DataSources: map[string]Entity{},
		}); err != nil {
			t.Fatal(err)
		}
	}

	snapshots, err := loadSnapshots(directory)
	if err != nil {
		t.Fatal(err)
	}
	var versions []string
	for _, snapshot := range snapshots {
		versions = append(versions, snapshot.Version)
	}
	want := []string{"v1.0.2", "v1.0.10", "v1.1.0"}
	if !reflect.DeepEqual(versions, want) {
		t.Fatalf("versions = %v, want %v", versions, want)
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
