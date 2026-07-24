package providerschema

import (
	"encoding/json"
	"testing"
)

func TestDiffTracksSchemaFlagsNestingAndPermissions(t *testing.T) {
	t.Parallel()
	old := NewSnapshot("v1.0.0")
	old.Resources["chalk_example"] = Entity{
		Schema: map[string]Node{
			"name": {
				Kind:     "attribute",
				Type:     json.RawMessage(`"string"`),
				Optional: true,
			},
			"config": {
				Kind:        "attribute",
				NestingMode: "single",
				Optional:    true,
				Children: map[string]Node{
					"enabled": {Kind: "attribute", Type: json.RawMessage(`"bool"`), Optional: true},
				},
			},
		},
		Permissions: []Permission{{Name: "project.create"}},
	}
	current := NewSnapshot("v1.1.0")
	current.Resources["chalk_example"] = Entity{
		Schema: map[string]Node{
			"name": {
				Kind:      "attribute",
				Type:      json.RawMessage(`"number"`),
				Required:  true,
				Sensitive: true,
			},
			"config": {
				Kind:        "attribute",
				NestingMode: "list",
				Optional:    true,
				Children: map[string]Node{
					"enabled": {Kind: "attribute", Type: json.RawMessage(`"bool"`), Optional: true},
					"mode":    {Kind: "attribute", Type: json.RawMessage(`"string"`), Computed: true},
				},
			},
		},
		Permissions: []Permission{
			{Name: "deploy.read"},
			{Name: "project.create", TeamScoped: true},
		},
	}
	current.DataSources["chalk_lookup"] = Entity{}

	changes := Diff(old, current)
	wantKinds := map[string]bool{
		ChangeType:            false,
		ChangeOptional:        false,
		ChangeRequired:        false,
		ChangeSensitive:       false,
		ChangeNesting:         false,
		ChangeNodeAdded:       false,
		ChangePermissionAdded: false,
		ChangePermissionScope: false,
		ChangeEntityAdded:     false,
	}
	for _, change := range changes {
		if _, exists := wantKinds[change.Kind]; exists {
			wantKinds[change.Kind] = true
		}
	}
	for kind, found := range wantKinds {
		if !found {
			t.Errorf("missing change kind %s in %#v", kind, changes)
		}
	}
}

func TestDiffRecursesThroughNestedChildren(t *testing.T) {
	t.Parallel()
	old := NewSnapshot("v1.0.0")
	current := NewSnapshot("v1.1.0")
	old.Resources["chalk_example"] = Entity{Schema: map[string]Node{
		"outer": {
			Kind:        "attribute",
			NestingMode: "list",
			Children: map[string]Node{
				"inner": {Kind: "attribute", Type: json.RawMessage(`"string"`), Optional: true},
			},
		},
	}}
	current.Resources["chalk_example"] = Entity{Schema: map[string]Node{
		"outer": {Kind: "attribute", NestingMode: "list", Children: map[string]Node{}},
	}}
	changes := Diff(old, current)
	if len(changes) != 1 || changes[0].Kind != ChangeNodeRemoved || changes[0].Path != "outer.inner" {
		t.Fatalf("nested changes = %#v", changes)
	}
}

func TestDiffNodeKindChangeIsReportedOnce(t *testing.T) {
	t.Parallel()
	old := NewSnapshot("v1.0.0")
	current := NewSnapshot("v1.1.0")
	old.Resources["chalk_example"] = Entity{Schema: map[string]Node{
		"config": {Kind: "attribute", Type: json.RawMessage(`"string"`), Optional: true},
	}}
	current.Resources["chalk_example"] = Entity{Schema: map[string]Node{
		"config": {Kind: "block", NestingMode: "list"},
	}}
	changes := Diff(old, current)
	if len(changes) != 1 || changes[0].Kind != ChangeNodeKind {
		t.Fatalf("kind changes = %#v", changes)
	}
}

func TestContractEqualIgnoresVersion(t *testing.T) {
	t.Parallel()
	first := NewSnapshot("v1.0.0")
	second := NewSnapshot("v1.0.1")
	first.Resources["chalk_example"] = Entity{}
	second.Resources["chalk_example"] = Entity{}
	if !ContractEqual(first, second) {
		t.Fatal("identical contracts with different versions should compare equal")
	}
}

func TestFormatTypeDescribesObjectShape(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`["object",{"b":"number","a":["list","string"]},["b"]]`)
	want := "object({a=list(string), b=number}) optional(b)"
	if got := FormatType(raw); got != want {
		t.Fatalf("FormatType() = %q, want %q", got, want)
	}
}
