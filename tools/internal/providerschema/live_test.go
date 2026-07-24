package providerschema

import (
	"context"
	"testing"
)

func TestLiveSnapshotExtractsProviderContract(t *testing.T) {
	t.Parallel()
	snapshot, err := LiveSnapshot(context.Background(), "v1.0.2")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Resources) == 0 || len(snapshot.DataSources) == 0 {
		t.Fatalf("live snapshot has %d resources and %d data sources", len(snapshot.Resources), len(snapshot.DataSources))
	}
	project, exists := snapshot.Resources["chalk_project"]
	if !exists {
		t.Fatal("live snapshot is missing chalk_project")
	}
	if len(project.Permissions) == 0 {
		t.Fatal("chalk_project permissions were not extracted")
	}
	assertValidNodes(t, project.Schema)
	for name, entity := range snapshot.Resources {
		if entity.Schema == nil {
			t.Errorf("resource %s has a nil schema", name)
		}
		assertValidNodes(t, entity.Schema)
	}
	for name, entity := range snapshot.DataSources {
		if entity.Schema == nil {
			t.Errorf("data source %s has a nil schema", name)
		}
		assertValidNodes(t, entity.Schema)
	}
}

func assertValidNodes(t *testing.T, nodes map[string]Node) {
	t.Helper()
	for name, node := range nodes {
		if node.Kind != "attribute" && node.Kind != "block" {
			t.Errorf("%s has invalid kind %q", name, node.Kind)
		}
		if node.Kind == "attribute" && node.NestingMode == "" && len(node.Type) == 0 {
			t.Errorf("%s leaf attribute has no type", name)
		}
		assertValidNodes(t, node.Children)
	}
}
