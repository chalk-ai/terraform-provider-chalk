package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

var _ resource.Resource = &TelemetryResource{}
var _ resource.ResourceWithImportState = &TelemetryResource{}

// Attribute type maps used to construct types.Object values and null objects.
var kubePVCAttrTypes = map[string]attr.Type{
	"storage":            types.StringType,
	"storage_class_name": types.StringType,
}

var otelCollectorSpecAttrTypes = map[string]attr.Type{
	"version": types.StringType,
	"request": types.ObjectType{AttrTypes: kubeResourceConfigAttrTypes},
	"limit":   types.ObjectType{AttrTypes: kubeResourceConfigAttrTypes},
}

var aggregatorSpecAttrTypes = map[string]attr.Type{
	"image_version": types.StringType,
	"request":       types.ObjectType{AttrTypes: kubeResourceConfigAttrTypes},
}

var clickhouseDeploymentSpecAttrTypes = map[string]attr.Type{
	"version":    types.StringType,
	"request":    types.ObjectType{AttrTypes: kubeResourceConfigAttrTypes},
	"limit":      types.ObjectType{AttrTypes: kubeResourceConfigAttrTypes},
	"storage":    types.ObjectType{AttrTypes: kubePVCAttrTypes},
	"gateway_id": types.StringType,
}

func NewTelemetryResource() resource.Resource {
	return &TelemetryResource{}
}

type TelemetryResource struct {
	client *ClientManager
}

// Intermediate structs used only for .As() deserialization within buildTelemetryDeploymentSpec.
type KubePersistentVolumeClaimModel struct {
	Storage          types.String `tfsdk:"storage"`
	StorageClassName types.String `tfsdk:"storage_class_name"`
}

type ClickhouseDeploymentSpecModel struct {
	Version   types.String                    `tfsdk:"version"`
	Request   *KubeResourceConfigModel        `tfsdk:"request"`
	Limit     *KubeResourceConfigModel        `tfsdk:"limit"`
	Storage   *KubePersistentVolumeClaimModel `tfsdk:"storage"`
	GatewayId types.String                    `tfsdk:"gateway_id"`
}

type OtelCollectorSpecModel struct {
	Version types.String             `tfsdk:"version"`
	Request *KubeResourceConfigModel `tfsdk:"request"`
	Limit   *KubeResourceConfigModel `tfsdk:"limit"`
}

type AggregatorSpecModel struct {
	ImageVersion types.String             `tfsdk:"image_version"`
	Request      *KubeResourceConfigModel `tfsdk:"request"`
}

type TelemetryResourceModel struct {
	Id                       types.String `tfsdk:"id"`
	Namespace                types.String `tfsdk:"namespace"`
	KubeClusterId            types.String `tfsdk:"kube_cluster_id"`
	OtelCollectorSpec        types.Object `tfsdk:"otel_collector_spec"`
	ClickhouseDeploymentSpec types.Object `tfsdk:"clickhouse_deployment_spec"`
	AggregatorSpec           types.Object `tfsdk:"aggregator_spec"`
}

func (r *TelemetryResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_telemetry"
}

func (r *TelemetryResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	kubeResourceConfigSchema := schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"cpu": schema.StringAttribute{
				MarkdownDescription: "CPU resource specification",
				Optional:            true,
			},
			"memory": schema.StringAttribute{
				MarkdownDescription: "Memory resource specification",
				Optional:            true,
			},
			"ephemeral_storage": schema.StringAttribute{
				MarkdownDescription: "Ephemeral storage resource specification",
				Optional:            true,
			},
			"storage": schema.StringAttribute{
				MarkdownDescription: "Storage resource specification",
				Optional:            true,
			},
		},
	}

	kubePersistentVolumeClaimSchema := schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"storage": schema.StringAttribute{
				MarkdownDescription: "Storage resource specification",
				Optional:            true,
			},
			"storage_class_name": schema.StringAttribute{
				MarkdownDescription: "Storage class name",
				Optional:            true,
			},
		},
	}

	clickhouseDeploymentSchema := schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"version": schema.StringAttribute{
				MarkdownDescription: "Clickhouse version",
				Required:            true,
			},
			"request": schema.SingleNestedAttribute{
				MarkdownDescription: "Request resource specification",
				Optional:            true,
				Attributes:          kubeResourceConfigSchema.Attributes,
			},
			"limit": schema.SingleNestedAttribute{
				MarkdownDescription: "Limit resource specification",
				Optional:            true,
				Attributes:          kubeResourceConfigSchema.Attributes,
			},
			"storage": schema.SingleNestedAttribute{
				MarkdownDescription: "Storage resource specification",
				Optional:            true,
				Attributes:          kubePersistentVolumeClaimSchema.Attributes,
			},
			"gateway_id": schema.StringAttribute{
				MarkdownDescription: "Gateway resource specification",
				Optional:            true,
			},
		},
	}

	otelCollectorSpecSchema := schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"version": schema.StringAttribute{
				MarkdownDescription: "Otel collector specification",
				Required:            true,
			},
			"request": schema.SingleNestedAttribute{
				MarkdownDescription: "Request resource specification",
				Optional:            true,
				Attributes:          kubeResourceConfigSchema.Attributes,
			},
			"limit": schema.SingleNestedAttribute{
				MarkdownDescription: "Limit resource specification",
				Optional:            true,
				Attributes:          kubeResourceConfigSchema.Attributes,
			},
		},
	}

	aggregatorSpecSchema := schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"image_version": schema.StringAttribute{
				MarkdownDescription: "Aggregator image version",
				Required:            true,
			},
			"request": schema.SingleNestedAttribute{
				MarkdownDescription: "Request resource specification",
				Optional:            true,
				Attributes:          kubeResourceConfigSchema.Attributes,
			},
		},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk telemetry resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Telemetry deployment identifier",
				Computed:            true,
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"kube_cluster_id": schema.StringAttribute{
				MarkdownDescription: "Kubernetes cluster ID",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"namespace": schema.StringAttribute{
				MarkdownDescription: "Kubernetes namespace for the telemetry deployment",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"otel_collector_spec": schema.SingleNestedAttribute{
				MarkdownDescription: "Otel collector specification",
				Optional:            true,
				Computed:            true,
				Attributes:          otelCollectorSpecSchema.Attributes,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
			},
			"clickhouse_deployment_spec": schema.SingleNestedAttribute{
				MarkdownDescription: "Clickhouse deployment specification",
				Optional:            true,
				Computed:            true,
				Attributes:          clickhouseDeploymentSchema.Attributes,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
			},
			"aggregator_spec": schema.SingleNestedAttribute{
				MarkdownDescription: "Aggregator specification",
				Optional:            true,
				Computed:            true,
				Attributes:          aggregatorSpecSchema.Attributes,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *TelemetryResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ClientManager)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ClientManager, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

// kubeResourceConfigObject converts a KubeResourceConfig proto to a types.Object.
func kubeResourceConfigObject(rc *serverv1.KubeResourceConfig) types.Object {
	if rc == nil {
		return types.ObjectNull(kubeResourceConfigAttrTypes)
	}
	return types.ObjectValueMust(kubeResourceConfigAttrTypes, map[string]attr.Value{
		"cpu":               optionalStringValue(rc.Cpu),
		"memory":            optionalStringValue(rc.Memory),
		"ephemeral_storage": optionalStringValue(rc.EphemeralStorage),
		"storage":           optionalStringValue(rc.Storage),
	})
}

// kubePVCObject converts a KubePersistentVolumeClaim proto to a types.Object.
func kubePVCObject(pvc *serverv1.KubePersistentVolumeClaim) types.Object {
	if pvc == nil {
		return types.ObjectNull(kubePVCAttrTypes)
	}
	return types.ObjectValueMust(kubePVCAttrTypes, map[string]attr.Value{
		"storage":            optionalStringValue(pvc.Storage),
		"storage_class_name": optionalStringValue(pvc.StorageClassName),
	})
}

// buildTelemetryDeploymentSpec builds a TelemetryDeploymentSpec proto from a Terraform model.
func buildTelemetryDeploymentSpec(ctx context.Context, data *TelemetryResourceModel, diags *diag.Diagnostics) *serverv1.TelemetryDeploymentSpec {
	spec := &serverv1.TelemetryDeploymentSpec{
		Namespace: data.Namespace.ValueStringPointer(),
	}

	if !data.ClickhouseDeploymentSpec.IsNull() && !data.ClickhouseDeploymentSpec.IsUnknown() {
		var chModel ClickhouseDeploymentSpecModel
		diags.Append(data.ClickhouseDeploymentSpec.As(ctx, &chModel, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil
		}
		ch := &serverv1.ClickHouseSpec{
			ClickHouseVersion: chModel.Version.ValueString(),
		}
		if !chModel.GatewayId.IsNull() {
			ch.GatewayId = chModel.GatewayId.ValueStringPointer()
		}
		if chModel.Storage != nil {
			ch.Storage = &serverv1.KubePersistentVolumeClaim{
				Storage:          chModel.Storage.Storage.ValueString(),
				StorageClassName: chModel.Storage.StorageClassName.ValueString(),
			}
		}
		if chModel.Request != nil {
			ch.Request = &serverv1.KubeResourceConfig{
				Cpu:              chModel.Request.CPU.ValueString(),
				Memory:           chModel.Request.Memory.ValueString(),
				EphemeralStorage: chModel.Request.EphemeralStorage.ValueString(),
				Storage:          chModel.Request.Storage.ValueString(),
			}
		}
		if chModel.Limit != nil {
			ch.Limit = &serverv1.KubeResourceConfig{
				Cpu:              chModel.Limit.CPU.ValueString(),
				Memory:           chModel.Limit.Memory.ValueString(),
				EphemeralStorage: chModel.Limit.EphemeralStorage.ValueString(),
				Storage:          chModel.Limit.Storage.ValueString(),
			}
		}
		spec.ClickHouse = ch
	}

	if !data.OtelCollectorSpec.IsNull() && !data.OtelCollectorSpec.IsUnknown() {
		var otelModel OtelCollectorSpecModel
		diags.Append(data.OtelCollectorSpec.As(ctx, &otelModel, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil
		}
		otel := &serverv1.OtelCollectorSpec{
			OtelCollectorVersion: otelModel.Version.ValueString(),
		}
		if otelModel.Request != nil {
			otel.Request = &serverv1.KubeResourceConfig{
				Cpu:              otelModel.Request.CPU.ValueString(),
				Memory:           otelModel.Request.Memory.ValueString(),
				EphemeralStorage: otelModel.Request.EphemeralStorage.ValueString(),
				Storage:          otelModel.Request.Storage.ValueString(),
			}
		}
		if otelModel.Limit != nil {
			otel.Limit = &serverv1.KubeResourceConfig{
				Cpu:              otelModel.Limit.CPU.ValueString(),
				Memory:           otelModel.Limit.Memory.ValueString(),
				EphemeralStorage: otelModel.Limit.EphemeralStorage.ValueString(),
				Storage:          otelModel.Limit.Storage.ValueString(),
			}
		}
		spec.Otel = otel
	}

	if !data.AggregatorSpec.IsNull() && !data.AggregatorSpec.IsUnknown() {
		var aggModel AggregatorSpecModel
		diags.Append(data.AggregatorSpec.As(ctx, &aggModel, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil
		}
		agg := &serverv1.AggregatorSpec{
			ImageVersion: aggModel.ImageVersion.ValueString(),
		}
		if aggModel.Request != nil {
			agg.Request = &serverv1.KubeResourceConfig{
				Cpu:              aggModel.Request.CPU.ValueString(),
				Memory:           aggModel.Request.Memory.ValueString(),
				EphemeralStorage: aggModel.Request.EphemeralStorage.ValueString(),
				Storage:          aggModel.Request.Storage.ValueString(),
			}
		}
		spec.Aggregator = agg
	}

	return spec
}

// updateStateFromTelemetrySpec updates a Terraform model from a TelemetryDeploymentSpec proto.
func updateStateFromTelemetrySpec(data *TelemetryResourceModel, spec *serverv1.TelemetryDeploymentSpec) {
	if spec == nil {
		return
	}

	data.Namespace = types.StringPointerValue(spec.Namespace)

	if spec.ClickHouse != nil {
		ch := spec.ClickHouse
		gatewayId := types.StringNull()
		if ch.GatewayId != nil {
			gatewayId = types.StringPointerValue(ch.GatewayId)
		}
		data.ClickhouseDeploymentSpec = types.ObjectValueMust(clickhouseDeploymentSpecAttrTypes, map[string]attr.Value{
			"version":    types.StringValue(ch.ClickHouseVersion),
			"gateway_id": gatewayId,
			"request":    kubeResourceConfigObject(ch.Request),
			"limit":      kubeResourceConfigObject(ch.Limit),
			"storage":    kubePVCObject(ch.Storage),
		})
	} else {
		data.ClickhouseDeploymentSpec = types.ObjectNull(clickhouseDeploymentSpecAttrTypes)
	}

	if spec.Otel != nil {
		otel := spec.Otel
		data.OtelCollectorSpec = types.ObjectValueMust(otelCollectorSpecAttrTypes, map[string]attr.Value{
			"version": types.StringValue(otel.OtelCollectorVersion),
			"request": kubeResourceConfigObject(otel.Request),
			"limit":   kubeResourceConfigObject(otel.Limit),
		})
	} else {
		data.OtelCollectorSpec = types.ObjectNull(otelCollectorSpecAttrTypes)
	}

	if spec.Aggregator != nil {
		agg := spec.Aggregator
		data.AggregatorSpec = types.ObjectValueMust(aggregatorSpecAttrTypes, map[string]attr.Value{
			"image_version": types.StringValue(agg.ImageVersion),
			"request":       kubeResourceConfigObject(agg.Request),
		})
	} else {
		data.AggregatorSpec = types.ObjectNull(aggregatorSpecAttrTypes)
	}
}

// buildTelemetryUpdateMask compares plan and state to determine which top-level spec fields changed.
func buildTelemetryUpdateMask(data, state *TelemetryResourceModel) []string {
	var paths []string
	// Skip Unknown plan values — Terraform hasn't determined them yet (Computed field with no prior state).
	if !data.ClickhouseDeploymentSpec.IsUnknown() && !data.ClickhouseDeploymentSpec.Equal(state.ClickhouseDeploymentSpec) {
		paths = append(paths, "click_house")
	}
	if !data.OtelCollectorSpec.IsUnknown() && !data.OtelCollectorSpec.Equal(state.OtelCollectorSpec) {
		paths = append(paths, "otel")
	}
	if !data.AggregatorSpec.IsUnknown() && !data.AggregatorSpec.Equal(state.AggregatorSpec) {
		paths = append(paths, "aggregator")
	}
	return paths
}

// optionalStringValue converts an empty string to a null types.String, and a non-empty
// string to a types.StringValue. This prevents spurious drift when the server returns
// empty strings for fields the user did not configure.
func optionalStringValue(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func (r *TelemetryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TelemetryResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	bc := r.client.NewBuilderClient(ctx)

	spec := buildTelemetryDeploymentSpec(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &serverv1.CreateTelemetryDeploymentRequest{
		ClusterId: data.KubeClusterId.ValueString(),
		Spec:      spec,
	}

	createResp, err := bc.CreateTelemetryDeployment(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Cluster Telemetry Deployment",
			fmt.Sprintf("Could not create cluster telemetry: %v", err),
		)
		return
	}

	data.Id = types.StringValue(createResp.Msg.GetTelemetryDeploymentId())

	// The Create response only returns an ID, not the full spec. Resolve any Unknown
	// computed spec fields to null so state is concrete. Read will populate them from
	// the server on the next plan.
	if data.OtelCollectorSpec.IsUnknown() {
		data.OtelCollectorSpec = types.ObjectNull(otelCollectorSpecAttrTypes)
	}
	if data.ClickhouseDeploymentSpec.IsUnknown() {
		data.ClickhouseDeploymentSpec = types.ObjectNull(clickhouseDeploymentSpecAttrTypes)
	}
	if data.AggregatorSpec.IsUnknown() {
		data.AggregatorSpec = types.ObjectNull(aggregatorSpecAttrTypes)
	}

	tflog.Trace(ctx, "created a chalk_cluster_telemetry resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TelemetryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TelemetryResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	bc := r.client.NewBuilderClient(ctx)

	getReq := &serverv1.GetTelemetryDeploymentRequest{
		Identifier: &serverv1.GetTelemetryDeploymentRequest_TelemetryId{
			TelemetryId: data.Id.ValueString(),
		},
	}

	telemetry, err := bc.GetTelemetryDeployment(ctx, connect.NewRequest(getReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Chalk Cluster Telemetry",
			fmt.Sprintf("Could not read cluster telemetry [%s]: %v", data.Id.ValueString(), err),
		)
		return
	}

	deployment := telemetry.Msg.Deployment
	if deployment != nil {
		data.KubeClusterId = types.StringValue(deployment.ClusterId)
		updateStateFromTelemetrySpec(&data, deployment.Spec)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TelemetryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TelemetryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state TelemetryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bc := r.client.NewBuilderClient(ctx)

	updateMaskPaths := buildTelemetryUpdateMask(&data, &state)

	if len(updateMaskPaths) > 0 {
		spec := buildTelemetryDeploymentSpec(ctx, &data, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}

		updateReq := &serverv1.UpdateTelemetryDeploymentRequest{
			TelemetryDeploymentId: data.Id.ValueString(),
			Spec:                  spec,
			UpdateMask: &fieldmaskpb.FieldMask{
				Paths: updateMaskPaths,
			},
		}

		updateResp, err := bc.UpdateTelemetryDeployment(ctx, connect.NewRequest(updateReq))
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Chalk Cluster Telemetry Deployment",
				fmt.Sprintf("Could not update cluster telemetry: %v", err),
			)
			return
		}

		if updateResp.Msg.Deployment != nil {
			updateStateFromTelemetrySpec(&data, updateResp.Msg.Deployment.Spec)
		}
	}

	tflog.Trace(ctx, "updated a chalk_cluster_telemetry resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TelemetryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TelemetryResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	bc := r.client.NewBuilderClient(ctx)

	deleteRequest := &serverv1.DeleteTelemetryDeploymentRequest{
		ClusterId: data.KubeClusterId.ValueString(),
		Namespace: data.Namespace.ValueStringPointer(),
	}

	_, err := bc.DeleteTelemetryDeployment(ctx, connect.NewRequest(deleteRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Chalk Cluster Telemetry Deployment",
			fmt.Sprintf("Could not delete cluster telemetry %s-%s: %v", data.KubeClusterId.ValueString(), data.Namespace.ValueString(), err),
		)
		return
	}
	tflog.Trace(ctx, "deleted a chalk_cluster_telemetry resource")
}

func (r *TelemetryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}