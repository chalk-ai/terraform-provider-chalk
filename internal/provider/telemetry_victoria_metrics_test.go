package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func victoriaMetricsTestObject(size types.String, storage, insert, auth types.Int32) types.Object {
	return types.ObjectValueMust(victoriaMetricsAttrTypes, map[string]attr.Value{
		"storage_size": size, "storage_replicas": storage, "insert_replicas": insert, "auth_replicas": auth,
	})
}

func TestVictoriaMetricsOptionalFields(t *testing.T) {
	t.Parallel()
	empty := types.ObjectNull(victoriaMetricsAttrTypes)
	var diags diag.Diagnostics
	assert.Nil(t, victoriaMetricsToProto(context.Background(), empty, &diags))
	serverDefaults := &serverv1.VictoriaMetricsSpec{
		StorageSize: new("500Gi"), StorageReplicas: proto.Int32(1),
		InsertReplicas: proto.Int32(1), AuthReplicas: proto.Int32(1), RetentionPeriod: new("30d"),
	}
	assert.True(t, victoriaMetricsFromProto(serverDefaults, empty).IsNull())

	// Adopting disk size does not adopt the server's defaults for replicas.
	prior := victoriaMetricsTestObject(types.StringValue("1Ti"), types.Int32Null(), types.Int32Null(), types.Int32Null())
	read := victoriaMetricsFromProto(serverDefaults, prior)
	assert.Equal(t, types.StringValue("500Gi"), read.Attributes()["storage_size"])
	assert.True(t, read.Attributes()["storage_replicas"].IsNull())
	spec := victoriaMetricsToProto(context.Background(), prior, &diags)
	require.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, "1Ti", spec.GetStorageSize())
	assert.Nil(t, spec.StorageReplicas)
	assert.Nil(t, spec.RetentionPeriod)
	assert.True(t, victoriaMetricsFromProto(nil, prior).IsNull())
}

func TestVictoriaMetricsUpdateMask(t *testing.T) {
	t.Parallel()
	nilObject := types.ObjectNull(victoriaMetricsAttrTypes)
	full := victoriaMetricsTestObject(types.StringValue("1Ti"), types.Int32Value(4), types.Int32Value(2), types.Int32Value(2))
	cases := []struct {
		name        string
		plan, state types.Object
		want        []string
	}{
		{"adopt", full, nilObject, []string{"victoria_metrics.storage_size", "victoria_metrics.storage_replicas", "victoria_metrics.insert_replicas", "victoria_metrics.auth_replicas"}},
		{"no change", full, full, nil},
		{"relinquish", nilObject, full, nil},
		{"unknown object", types.ObjectUnknown(victoriaMetricsAttrTypes), full, nil},
		{"one changed leaf", victoriaMetricsTestObject(types.StringNull(), types.Int32Null(), types.Int32Value(3), types.Int32Unknown()), full, []string{"victoria_metrics.insert_replicas"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTelemetryUpdateMask(&TelemetryResourceModel{VictoriaMetrics: tt.plan}, &TelemetryResourceModel{VictoriaMetrics: tt.state})
			assert.Equal(t, tt.want, got)
			_, err := fieldmaskpb.New(&serverv1.TelemetryDeploymentSpec{}, got...)
			require.NoError(t, err)
		})
	}
}

// Exercise Terraform's planning and refresh against an API that hydrates defaults
// and preserves unmasked VictoriaMetrics settings, including externally owned ones.
func TestTelemetryResourceVictoriaMetrics(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(server.Close)
	var current *serverv1.TelemetryDeploymentSpec
	deployment := func() *serverv1.TelemetryDeployment {
		return &serverv1.TelemetryDeployment{Id: "test-telemetry-id", ClusterId: "test-cluster-id", Spec: proto.Clone(current).(*serverv1.TelemetryDeploymentSpec)}
	}
	server.OnCreateTelemetryDeployment().WithBehavior(func(req proto.Message) (proto.Message, error) {
		current = proto.Clone(req.(*serverv1.CreateTelemetryDeploymentRequest).Spec).(*serverv1.TelemetryDeploymentSpec)
		if current.VictoriaMetrics == nil {
			current.VictoriaMetrics = &serverv1.VictoriaMetricsSpec{}
		}
		vm := current.VictoriaMetrics
		if vm.StorageSize == nil {
			vm.StorageSize = new("500Gi")
		}
		if vm.StorageReplicas == nil {
			vm.StorageReplicas = proto.Int32(1)
		}
		if vm.InsertReplicas == nil {
			vm.InsertReplicas = proto.Int32(1)
		}
		if vm.AuthReplicas == nil {
			vm.AuthReplicas = proto.Int32(1)
		}
		vm.RetentionPeriod = new("60d")
		vm.StorageRequest = &serverv1.KubeResourceConfig{Cpu: "1", Memory: "2Gi"}
		return &serverv1.CreateTelemetryDeploymentResponse{TelemetryDeploymentId: "test-telemetry-id"}, nil
	})
	server.OnGetTelemetryDeployment().WithBehavior(func(proto.Message) (proto.Message, error) {
		return &serverv1.GetTelemetryDeploymentResponse{Deployment: deployment()}, nil
	})
	server.OnUpdateTelemetryDeployment().WithBehavior(func(req proto.Message) (proto.Message, error) {
		r := req.(*serverv1.UpdateTelemetryDeploymentRequest)
		require.Equal(t, []string{"victoria_metrics.storage_size", "victoria_metrics.storage_replicas", "victoria_metrics.insert_replicas", "victoria_metrics.auth_replicas"}, r.UpdateMask.Paths)
		vm := current.VictoriaMetrics
		vm.StorageSize = r.Spec.VictoriaMetrics.StorageSize
		vm.StorageReplicas = r.Spec.VictoriaMetrics.StorageReplicas
		vm.InsertReplicas = r.Spec.VictoriaMetrics.InsertReplicas
		vm.AuthReplicas = r.Spec.VictoriaMetrics.AuthReplicas
		return &serverv1.UpdateTelemetryDeploymentResponse{Deployment: deployment()}, nil
	})
	server.OnDeleteTelemetryDeployment().Return(&serverv1.DeleteTelemetryDeploymentResponse{})
	config := func(block string) string {
		return providerConfig(server.URL) + fmt.Sprintf(`
resource "chalk_telemetry" "test" {
 kube_cluster_id = "test-cluster-id"
 %s
}
`, block)
	}
	upgraded := config(`victoria_metrics = {
 storage_size = "1Ti"
 storage_replicas = 4
 insert_replicas = 2
 auth_replicas = 2
}`)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: config(""), Check: resource.TestCheckNoResourceAttr("chalk_telemetry.test", "victoria_metrics")},
			{Config: upgraded, Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("chalk_telemetry.test", "victoria_metrics.storage_size", "1Ti"),
				resource.TestCheckResourceAttr("chalk_telemetry.test", "victoria_metrics.storage_replicas", "4"),
				resource.TestCheckResourceAttr("chalk_telemetry.test", "victoria_metrics.insert_replicas", "2"),
				resource.TestCheckResourceAttr("chalk_telemetry.test", "victoria_metrics.auth_replicas", "2"),
				func(*terraform.State) error {
					assert.Equal(t, "60d", current.VictoriaMetrics.GetRetentionPeriod())
					assert.Equal(t, "2Gi", current.VictoriaMetrics.StorageRequest.Memory)
					return nil
				},
			)},
			{Config: upgraded, PlanOnly: true},
			{Config: config(""), Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckNoResourceAttr("chalk_telemetry.test", "victoria_metrics"),
				func(*terraform.State) error {
					assert.Len(t, server.GetCapturedRequests("UpdateTelemetryDeployment"), 1)
					assert.Equal(t, int32(4), current.VictoriaMetrics.GetStorageReplicas())
					return nil
				},
			)},
		},
	})
}

func TestTelemetryResourceVictoriaMetricsCreatePartial(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerTelemetry(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{{
			Config: providerConfig(server.URL) + `
resource "chalk_telemetry" "test" {
 kube_cluster_id = "test-cluster-id"
 victoria_metrics = { storage_size = "1Ti" }
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("chalk_telemetry.test", "victoria_metrics.storage_size", "1Ti"),
				resource.TestCheckNoResourceAttr("chalk_telemetry.test", "victoria_metrics.storage_replicas"),
				func(*terraform.State) error {
					req := server.GetCapturedRequests("CreateTelemetryDeployment")[0].(*serverv1.CreateTelemetryDeploymentRequest)
					require.NotNil(t, req.Spec.VictoriaMetrics)
					assert.Equal(t, "1Ti", req.Spec.VictoriaMetrics.GetStorageSize())
					assert.Nil(t, req.Spec.VictoriaMetrics.StorageReplicas)
					return nil
				},
			),
		}},
	})
}

func TestTelemetryResourceVictoriaMetricsValidation(t *testing.T) {
	for _, tt := range []struct{ name, body, pattern string }{
		{"empty", "{}", "At least one attribute"},
		{"empty size", `{storage_size = ""}`, "length must be at least 1"},
		{"zero storage", "{storage_replicas = 0}", "must be at least 1"},
		{"negative insert", "{insert_replicas = -1}", "must be at least 1"},
		{"zero auth", "{auth_replicas = 0}", "must be at least 1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := setupMockBuilderServerTelemetry(t)
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
				Steps: []resource.TestStep{{
					Config: providerConfig(server.URL) + fmt.Sprintf(`
resource "chalk_telemetry" "test" {
 kube_cluster_id = "test-cluster-id"
 victoria_metrics = %s
}
`, tt.body),
					ExpectError: regexp.MustCompile(tt.pattern),
				}},
			})
		})
	}
}
