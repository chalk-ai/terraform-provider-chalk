package providerschema

import (
	"encoding/json"
	"testing"
)

func TestParseVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value string
		valid bool
	}{
		{value: "v0.9.18", valid: true},
		{value: "v1.0.2", valid: true},
		{value: "1.0.2", valid: false},
		{value: "v1.0", valid: false},
		{value: "v01.0.0", valid: false},
		{value: "v1.0.0-beta", valid: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.value, func(t *testing.T) {
			t.Parallel()
			_, err := ParseVersion(test.value)
			if (err == nil) != test.valid {
				t.Fatalf("ParseVersion(%q) error = %v, valid = %t", test.value, err, test.valid)
			}
		})
	}
}

func TestVersionCompareUsesSemanticOrder(t *testing.T) {
	t.Parallel()
	v9, _ := ParseVersion("v0.9.9")
	v10, _ := ParseVersion("v0.9.10")
	v1, _ := ParseVersion("v1.0.0")
	if v9.Compare(v10) >= 0 || v10.Compare(v1) >= 0 || v1.Compare(v1) != 0 {
		t.Fatal("semantic version comparison returned the wrong order")
	}
}

func TestParsePermissions(t *testing.T) {
	t.Parallel()
	permissions, err := ParsePermissions("Description.\n\n**Required permissions:** `deploy.read`, `project.create` *(team-scoped)*")
	if err != nil {
		t.Fatal(err)
	}
	want := []Permission{
		{Name: "deploy.read"},
		{Name: "project.create", TeamScoped: true},
	}
	data, _ := json.Marshal(permissions)
	wantData, _ := json.Marshal(want)
	if string(data) != string(wantData) {
		t.Fatalf("permissions = %s, want %s", data, wantData)
	}
}

func TestParsePermissionsRejectsMalformedMarkdown(t *testing.T) {
	t.Parallel()
	if _, err := ParsePermissions("**Required permissions:** deploy.read"); err == nil {
		t.Fatal("expected malformed permission documentation to fail")
	}
}

func TestMarshalSnapshotIsDeterministic(t *testing.T) {
	t.Parallel()
	snapshot := NewSnapshot("v1.0.0")
	snapshot.Resources["chalk_z"] = Entity{}
	snapshot.Resources["chalk_a"] = Entity{}
	first, err := MarshalSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MarshalSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("snapshot output changed between identical renders")
	}
	if first[len(first)-1] != '\n' {
		t.Fatal("snapshot output must end with a newline")
	}
}
