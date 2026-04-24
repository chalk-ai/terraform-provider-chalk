package provider

import (
	"encoding/json"
	"testing"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

// Tests for INF-1287: specs_config_json must be populated from the server in Read
// so terraform plan does not show perpetual drift after import.
//
// The deprecated chalk_environment resource cannot be driven end-to-end through
// resource.Test today because chalk-go/testserver has no mocks for the legacy
// CreateEnvironment/UpdateEnvironment/ArchiveEnvironment RPCs it calls. These
// tests instead target applySpecConfigJsonToState, which is the entirety of the
// Read-side transformation the bug fix introduces. Test inputs are distilled
// from 194 production spec_config values in /tmp/chalk_public_Environment.json.

// jsonRoundtrip parses a JSON string, returns the Go structure. Used to compare
// two JSON strings for semantic equality independently of key order / whitespace.
func jsonRoundtrip(t *testing.T, s string) any {
	t.Helper()
	var v any
	require.NoError(t, json.Unmarshal([]byte(s), &v))
	return v
}

// TestApplySpecConfigJsonToState_EmptyMap: 3 of 194 prod envs store "{}". Read
// must map an empty SpecConfigJson map to a null state value, matching the
// behavior of baseUpdateStateFromEnvironment on the newer resources.
func TestApplySpecConfigJsonToState_EmptyMap(t *testing.T) {
	t.Parallel()

	data := &EnvironmentResourceModel{}
	e := &serverv1.Environment{SpecConfigJson: map[string]*structpb.Value{}}

	require.NoError(t, applySpecConfigJsonToState(data, e))
	assert.True(t, data.SpecsConfigJson.IsNull(),
		"empty map must map to NewNormalizedNull (flagged breaking case: HCL `\"{}\"` will drift to null)")
}

// TestApplySpecConfigJsonToState_NilMap: defensive case; SpecConfigJson absent.
func TestApplySpecConfigJsonToState_NilMap(t *testing.T) {
	t.Parallel()

	data := &EnvironmentResourceModel{}
	e := &serverv1.Environment{} // SpecConfigJson defaults to nil

	require.NoError(t, applySpecConfigJsonToState(data, e))
	assert.True(t, data.SpecsConfigJson.IsNull())
}

// TestApplySpecConfigJsonToState_SimpleServices: the most common shape
// (191/194 prod envs have top-level "services"). Covers dashed service keys
// (engine-grpc, offline-query-consumer — which appear in 170+ and 112+ envs
// respectively) and mixed leaf types (string, number, bool, array, object).
func TestApplySpecConfigJsonToState_SimpleServices(t *testing.T) {
	t.Parallel()

	specs, err := structpb.NewStruct(map[string]any{
		"services": map[string]any{
			"engine": map[string]any{
				"min_instances": 1,
				"max_instances": 5,
				"request":       map[string]any{"cpu": "2", "memory": "2Gi"},
			},
			"engine-grpc": map[string]any{
				"dual_serve_http_and_grpc":        true,
				"isolate_from_dataplane_services": false,
				"env_overrides": map[string]any{
					"CHALK_REMOTE_INVOKER_KIND": "subprocess",
					"DD_TRACE_ENABLED":          "0",
				},
			},
			"offline-query-consumer": map[string]any{
				"max_instances": 2,
			},
		},
		"resource_groups": []any{},
	})
	require.NoError(t, err)

	data := &EnvironmentResourceModel{}
	e := &serverv1.Environment{SpecConfigJson: specs.Fields}

	require.NoError(t, applySpecConfigJsonToState(data, e))
	require.False(t, data.SpecsConfigJson.IsNull())

	got := jsonRoundtrip(t, data.SpecsConfigJson.ValueString())
	topLevel := got.(map[string]any)
	assert.Contains(t, topLevel, "services")
	assert.Contains(t, topLevel, "resource_groups")

	services := topLevel["services"].(map[string]any)
	assert.Contains(t, services, "engine-grpc", "dashed service keys must survive the roundtrip")
	assert.Contains(t, services, "offline-query-consumer")
}

// TestApplySpecConfigJsonToState_KubernetesStyleKeys: real prod data uses keys
// like "node.kubernetes.io/instance-type" and "karpenter.sh/nodepool" under
// node_selector. These contain dots and slashes — valid JSON but worth pinning.
func TestApplySpecConfigJsonToState_KubernetesStyleKeys(t *testing.T) {
	t.Parallel()

	specs, err := structpb.NewStruct(map[string]any{
		"services": map[string]any{
			"engine": map[string]any{
				"node_selector": map[string]any{
					"node.kubernetes.io/instance-type":         "c7i.2xlarge",
					"karpenter.sh/nodepool":                    "chalk-node-pool",
					"karpenter.k8s.aws/instance-generation":    "7",
					"chalk.ai/nodepool":                        "eks-a-priority-nodepool-al2023",
				},
			},
		},
	})
	require.NoError(t, err)

	data := &EnvironmentResourceModel{}
	e := &serverv1.Environment{SpecConfigJson: specs.Fields}

	require.NoError(t, applySpecConfigJsonToState(data, e))

	got := jsonRoundtrip(t, data.SpecsConfigJson.ValueString())
	selector := got.(map[string]any)["services"].(map[string]any)["engine"].(map[string]any)["node_selector"].(map[string]any)
	assert.Equal(t, "c7i.2xlarge", selector["node.kubernetes.io/instance-type"])
	assert.Equal(t, "chalk-node-pool", selector["karpenter.sh/nodepool"])
	assert.Equal(t, "eks-a-priority-nodepool-al2023", selector["chalk.ai/nodepool"])
}

// TestApplySpecConfigJsonToState_TolerationsMixedOperators: 194 prod envs use
// operator=Equal (with a value field), 2 use operator=Exists (no value field).
// protojson must drop absent fields, not emit `"value": ""`.
func TestApplySpecConfigJsonToState_TolerationsMixedOperators(t *testing.T) {
	t.Parallel()

	specs, err := structpb.NewStruct(map[string]any{
		"services": map[string]any{
			"engine": map[string]any{
				"tolerations": []any{
					map[string]any{
						"effect":   "NoSchedule",
						"key":      "chalk.ai/nodepool",
						"operator": "Equal",
						"value":    "eks-a-priority-nodepool-al2023",
					},
					map[string]any{
						"effect":   "NoSchedule",
						"operator": "Exists",
					},
				},
			},
		},
	})
	require.NoError(t, err)

	data := &EnvironmentResourceModel{}
	e := &serverv1.Environment{SpecConfigJson: specs.Fields}

	require.NoError(t, applySpecConfigJsonToState(data, e))

	got := jsonRoundtrip(t, data.SpecsConfigJson.ValueString())
	tols := got.(map[string]any)["services"].(map[string]any)["engine"].(map[string]any)["tolerations"].([]any)
	require.Len(t, tols, 2)

	equalTol := tols[0].(map[string]any)
	assert.Equal(t, "Equal", equalTol["operator"])
	assert.Equal(t, "eks-a-priority-nodepool-al2023", equalTol["value"])

	existsTol := tols[1].(map[string]any)
	assert.Equal(t, "Exists", existsTol["operator"])
	_, hasValue := existsTol["value"]
	assert.False(t, hasValue, "Exists tolerations must NOT carry a value field after roundtrip")
}

// TestApplySpecConfigJsonToState_NestedResourceGroups: 32 of 194 prod envs have
// non-empty resource_groups (up to 17 per env), each carrying its own services
// subtree. Exercises arrays-of-objects with deeply nested object content
// (real prod depth reaches 8 levels).
func TestApplySpecConfigJsonToState_NestedResourceGroups(t *testing.T) {
	t.Parallel()

	specs, err := structpb.NewStruct(map[string]any{
		"resource_groups": []any{
			map[string]any{
				"resource_group_name": "card-auth",
				"services": map[string]any{
					"engine-grpc": map[string]any{
						"max_instances":            5,
						"min_instances":            5,
						"dual_serve_http_and_grpc": true,
						"env_overrides": map[string]any{
							"CHALK_REMOTE_INVOKER_ACTOR_POOL_SIZE": "2",
							"OTEL_TRACES_SAMPLER":                  "always_off",
						},
						"node_selector": map[string]any{
							"karpenter.sh/nodepool": "chalk-node-pool",
						},
						"request": map[string]any{
							"cpu":               "4",
							"memory":            "8Gi",
							"ephemeral_storage": "20Gi",
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	data := &EnvironmentResourceModel{}
	e := &serverv1.Environment{SpecConfigJson: specs.Fields}

	require.NoError(t, applySpecConfigJsonToState(data, e))

	got := jsonRoundtrip(t, data.SpecsConfigJson.ValueString())
	rgs := got.(map[string]any)["resource_groups"].([]any)
	require.Len(t, rgs, 1)
	rg := rgs[0].(map[string]any)
	assert.Equal(t, "card-auth", rg["resource_group_name"])

	engineGrpc := rg["services"].(map[string]any)["engine-grpc"].(map[string]any)
	assert.Equal(t, float64(5), engineGrpc["max_instances"], "integer values survive roundtrip as JSON numbers")
	assert.Equal(t, true, engineGrpc["dual_serve_http_and_grpc"])

	envOverrides := engineGrpc["env_overrides"].(map[string]any)
	assert.Equal(t, "2", envOverrides["CHALK_REMOTE_INVOKER_ACTOR_POOL_SIZE"])
}

// TestApplySpecConfigJsonToState_OnlineStoreCleanupConfig: 20 of 194 prod envs
// carry this key. The cron_schedule value contains spaces and asterisks, a
// good stress-test for string escaping.
func TestApplySpecConfigJsonToState_OnlineStoreCleanupConfig(t *testing.T) {
	t.Parallel()

	specs, err := structpb.NewStruct(map[string]any{
		"online_store_cleanup_config": map[string]any{
			"cron_schedule": "0 8 * * *",
			"disabled":      true,
			"dry_run":       false,
			"scan_size":     5000,
		},
	})
	require.NoError(t, err)

	data := &EnvironmentResourceModel{}
	e := &serverv1.Environment{SpecConfigJson: specs.Fields}

	require.NoError(t, applySpecConfigJsonToState(data, e))

	got := jsonRoundtrip(t, data.SpecsConfigJson.ValueString())
	cfg := got.(map[string]any)["online_store_cleanup_config"].(map[string]any)
	assert.Equal(t, "0 8 * * *", cfg["cron_schedule"], "cron schedule string with spaces/asterisks survives")
	assert.Equal(t, true, cfg["disabled"])
	assert.Equal(t, float64(5000), cfg["scan_size"])
}

// TestApplySpecConfigJsonToState_NormalizedEqualsPrettyInput: this is the core
// INF-1287 regression assertion. A user writes pretty-printed JSON in HCL;
// the server returns a structured map; Read re-serializes via protojson.Marshal
// (always compact). jsontypes.Normalized.Equal must treat the two as semantically
// equal so terraform plan shows no diff.
func TestApplySpecConfigJsonToState_NormalizedEqualsPrettyInput(t *testing.T) {
	t.Parallel()

	specs, err := structpb.NewStruct(map[string]any{
		"services": map[string]any{
			"engine": map[string]any{
				"min_instances": 1,
				"max_instances": 1,
				"request":       map[string]any{"cpu": "2", "memory": "6Gi"},
			},
		},
	})
	require.NoError(t, err)

	data := &EnvironmentResourceModel{}
	e := &serverv1.Environment{SpecConfigJson: specs.Fields}
	require.NoError(t, applySpecConfigJsonToState(data, e))

	// What the user writes in HCL: pretty-printed, different key order, extra whitespace.
	hclPretty := `{
    "services": {
        "engine": {
            "request": {
                "memory": "6Gi",
                "cpu": "2"
            },
            "max_instances": 1,
            "min_instances": 1
        }
    }
}`
	planValue := jsontypes.NewNormalizedValue(hclPretty)

	// The regression check: with the fix in place, state (compact, from protojson)
	// compares equal to plan (pretty, from HCL) under jsontypes.Normalized semantics.
	eq, diags := data.SpecsConfigJson.StringSemanticEquals(t.Context(), planValue)
	require.False(t, diags.HasError(), "%v", diags)
	assert.True(t, eq,
		"state (%s) must semantically equal pretty-printed HCL — this is the INF-1287 regression check",
		data.SpecsConfigJson.ValueString())
}
