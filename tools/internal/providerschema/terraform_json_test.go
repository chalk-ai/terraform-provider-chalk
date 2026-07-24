package providerschema

import (
	"testing"
)

func TestSnapshotFromTerraformJSONRecursesNestedAttributesAndBlocks(t *testing.T) {
	t.Parallel()
	input := []byte(`{
	  "format_version": "1.0",
	  "provider_schemas": {
	    "registry.terraform.io/chalk-ai/chalk": {
	      "resource_schemas": {
	        "chalk_example": {
	          "version": 0,
	          "block": {
	            "description": "Example.\n\n**Required permissions:** \u0060project.create\u0060 *(team-scoped)*",
	            "attributes": {
	              "name": {"type": "string", "required": true},
	              "config": {
	                "nested_type": {
	                  "nesting_mode": "list",
	                  "attributes": {
	                    "secret": {"type": "string", "optional": true, "sensitive": true}
	                  }
	                },
	                "optional": true
	              }
	            },
	            "block_types": {
	              "legacy": {
	                "nesting_mode": "set",
	                "min_items": 1,
	                "block": {
	                  "attributes": {
	                    "enabled": {"type": "bool", "optional": true}
	                  }
	                }
	              }
	            }
	          }
	        }
	      },
	      "data_source_schemas": {}
	    }
	  }
	}`)
	snapshot, err := SnapshotFromTerraformJSON(input, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	entity := snapshot.Resources["chalk_example"]
	if !entity.Schema["name"].Required {
		t.Fatal("required leaf attribute was not preserved")
	}
	config := entity.Schema["config"]
	if config.NestingMode != "list" || !config.Optional {
		t.Fatalf("nested config = %#v", config)
	}
	if !config.Children["secret"].Sensitive {
		t.Fatal("nested sensitive attribute was not preserved")
	}
	legacy := entity.Schema["legacy"]
	if legacy.Kind != "block" || legacy.NestingMode != "set" || legacy.MinItems != 1 {
		t.Fatalf("legacy block = %#v", legacy)
	}
	if got := entity.Permissions; len(got) != 1 || got[0].Name != "project.create" || !got[0].TeamScoped {
		t.Fatalf("permissions = %#v", got)
	}
}

func TestSnapshotFromTerraformJSONRequiresOneProvider(t *testing.T) {
	t.Parallel()
	if _, err := SnapshotFromTerraformJSON([]byte(`{"provider_schemas":{}}`), "v1.0.0"); err == nil {
		t.Fatal("expected empty provider schema to fail")
	}
}
