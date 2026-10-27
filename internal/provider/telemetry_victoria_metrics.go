package provider

import (
	"context"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var victoriaMetricsAttrTypes = map[string]attr.Type{
	"storage_size":     types.StringType,
	"storage_replicas": types.Int32Type,
	"insert_replicas":  types.Int32Type,
	"auth_replicas":    types.Int32Type,
}

type victoriaMetricsModel struct {
	StorageSize     types.String `tfsdk:"storage_size"`
	StorageReplicas types.Int32  `tfsdk:"storage_replicas"`
	InsertReplicas  types.Int32  `tfsdk:"insert_replicas"`
	AuthReplicas    types.Int32  `tfsdk:"auth_replicas"`
}

func victoriaMetricsSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: "VictoriaMetrics sizing. Only explicitly configured fields are managed. Omitting a field or removing this object leaves the server's existing settings unchanged. At least one field must be set.",
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"storage_size": schema.StringAttribute{
				MarkdownDescription: "Persistent volume size per vmstorage replica, for example `1Ti`. Existing volumes can only grow.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.AtLeastOneOf(
						path.MatchRelative().AtParent().AtName("storage_replicas"),
						path.MatchRelative().AtParent().AtName("insert_replicas"),
						path.MatchRelative().AtParent().AtName("auth_replicas"),
					),
				},
			},
			"storage_replicas": schema.Int32Attribute{
				MarkdownDescription: "Number of vmstorage replicas. Increasing this shards new data across the replicas without rebalancing historical data. Decreasing the count is not supported.",
				Optional:            true,
				Validators:          []validator.Int32{int32validator.AtLeast(1)},
			},
			"insert_replicas": schema.Int32Attribute{
				MarkdownDescription: "Number of vminsert replicas.",
				Optional:            true,
				Validators:          []validator.Int32{int32validator.AtLeast(1)},
			},
			"auth_replicas": schema.Int32Attribute{
				MarkdownDescription: "Number of vmauth replicas.",
				Optional:            true,
				Validators:          []validator.Int32{int32validator.AtLeast(1)},
			},
		},
	}
}

func victoriaMetricsToProto(ctx context.Context, value types.Object, diags *diag.Diagnostics) *serverv1.VictoriaMetricsSpec {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	var m victoriaMetricsModel
	diags.Append(value.As(ctx, &m, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil
	}
	return &serverv1.VictoriaMetricsSpec{
		StorageSize:     m.StorageSize.ValueStringPointer(),
		StorageReplicas: m.StorageReplicas.ValueInt32Pointer(),
		InsertReplicas:  m.InsertReplicas.ValueInt32Pointer(),
		AuthReplicas:    m.AuthReplicas.ValueInt32Pointer(),
	}
}

// Keep unowned settings null even when the server supplies effective defaults.
func victoriaMetricsFromProto(p *serverv1.VictoriaMetricsSpec, prior types.Object) types.Object {
	if prior.IsNull() || prior.IsUnknown() {
		return prior
	}
	if p == nil {
		return types.ObjectNull(victoriaMetricsAttrTypes)
	}
	values := map[string]attr.Value{
		"storage_size":     types.StringPointerValue(p.StorageSize),
		"storage_replicas": types.Int32PointerValue(p.StorageReplicas),
		"insert_replicas":  types.Int32PointerValue(p.InsertReplicas),
		"auth_replicas":    types.Int32PointerValue(p.AuthReplicas),
	}
	for name, value := range prior.Attributes() {
		if value.IsNull() {
			values[name] = value
		}
	}
	return types.ObjectValueMust(victoriaMetricsAttrTypes, values)
}

// Mask only changed, configured leaves to preserve scheduling, retention and other
// settings managed by the server or another client. Removal relinquishes ownership.
func victoriaMetricsMaskPaths(plan, state types.Object) []string {
	if plan.IsNull() || plan.IsUnknown() {
		return nil
	}
	var paths []string
	prior := state.Attributes()
	for _, name := range []string{"storage_size", "storage_replicas", "insert_replicas", "auth_replicas"} {
		value := plan.Attributes()[name]
		if !value.IsNull() && !value.IsUnknown() && !value.Equal(prior[name]) {
			paths = append(paths, "victoria_metrics."+name)
		}
	}
	return paths
}
