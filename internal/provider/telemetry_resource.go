package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

var _ resource.Resource = &TelemetryResource{}
var _ resource.ResourceWithImportState = &TelemetryResource{}

func NewTelemetryResource() resource.Resource {
	return &TelemetryResource{}
}

type TelemetryResource struct {
	client *ClientManager
}

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

type TelemetryResourceModel struct {
	Id                       types.String                   `tfsdk:"id"`
	Namespace                types.String                   `tfsdk:"namespace"`
	KubeClusterId            types.String                   `tfsdk:"kube_cluster_id"`
	OtelCollectorSpec        *OtelCollectorSpecModel        `tfsdk:"otel_collector_spec"`
	ClickhouseDeploymentSpec *ClickhouseDeploymentSpecModel `tfsdk:"clickhouse_deployment_spec"`
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
				Attributes:          otelCollectorSpecSchema.Attributes,
			},
			"clickhouse_deployment_spec": schema.SingleNestedAttribute{
				MarkdownDescription: "Clickhouse deployment specification",
				Optional:            true,
				Attributes:          clickhouseDeploymentSchema.Attributes,
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

// buildTelemetryDeploymentSpec builds a TelemetryDeploymentSpec proto from a Terraform model.
func buildTelemetryDeploymentSpec(data *TelemetryResourceModel) *serverv1.TelemetryDeploymentSpec {
	spec := &serverv1.TelemetryDeploymentSpec{
		Namespace: data.Namespace.ValueStringPointer(),
	}

	if data.ClickhouseDeploymentSpec != nil {
		ch := &serverv1.ClickHouseSpec{
			ClickHouseVersion: data.ClickhouseDeploymentSpec.Version.ValueString(),
		}
		if !data.ClickhouseDeploymentSpec.GatewayId.IsNull() {
			ch.GatewayId = data.ClickhouseDeploymentSpec.GatewayId.ValueStringPointer()
		}
		if data.ClickhouseDeploymentSpec.Storage != nil {
			ch.Storage = &serverv1.KubePersistentVolumeClaim{
				Storage:          data.ClickhouseDeploymentSpec.Storage.Storage.ValueString(),
				StorageClassName: data.ClickhouseDeploymentSpec.Storage.StorageClassName.ValueString(),
			}
		}
		if data.ClickhouseDeploymentSpec.Request != nil {
			ch.Request = &serverv1.KubeResourceConfig{
				Cpu:              data.ClickhouseDeploymentSpec.Request.CPU.ValueString(),
				Memory:           data.ClickhouseDeploymentSpec.Request.Memory.ValueString(),
				EphemeralStorage: data.ClickhouseDeploymentSpec.Request.EphemeralStorage.ValueString(),
				Storage:          data.ClickhouseDeploymentSpec.Request.Storage.ValueString(),
			}
		}
		if data.ClickhouseDeploymentSpec.Limit != nil {
			ch.Limit = &serverv1.KubeResourceConfig{
				Cpu:              data.ClickhouseDeploymentSpec.Limit.CPU.ValueString(),
				Memory:           data.ClickhouseDeploymentSpec.Limit.Memory.ValueString(),
				EphemeralStorage: data.ClickhouseDeploymentSpec.Limit.EphemeralStorage.ValueString(),
				Storage:          data.ClickhouseDeploymentSpec.Limit.Storage.ValueString(),
			}
		}
		spec.ClickHouse = ch
	}

	if data.OtelCollectorSpec != nil {
		otel := &serverv1.OtelCollectorSpec{
			OtelCollectorVersion: data.OtelCollectorSpec.Version.ValueString(),
		}
		if data.OtelCollectorSpec.Request != nil {
			otel.Request = &serverv1.KubeResourceConfig{
				Cpu:              data.OtelCollectorSpec.Request.CPU.ValueString(),
				Memory:           data.OtelCollectorSpec.Request.Memory.ValueString(),
				EphemeralStorage: data.OtelCollectorSpec.Request.EphemeralStorage.ValueString(),
				Storage:          data.OtelCollectorSpec.Request.Storage.ValueString(),
			}
		}
		if data.OtelCollectorSpec.Limit != nil {
			otel.Limit = &serverv1.KubeResourceConfig{
				Cpu:              data.OtelCollectorSpec.Limit.CPU.ValueString(),
				Memory:           data.OtelCollectorSpec.Limit.Memory.ValueString(),
				EphemeralStorage: data.OtelCollectorSpec.Limit.EphemeralStorage.ValueString(),
				Storage:          data.OtelCollectorSpec.Limit.Storage.ValueString(),
			}
		}
		spec.Otel = otel
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
		chModel := &ClickhouseDeploymentSpecModel{
			Version: types.StringValue(ch.ClickHouseVersion),
		}
		if ch.GatewayId != nil {
			chModel.GatewayId = types.StringPointerValue(ch.GatewayId)
		} else {
			chModel.GatewayId = types.StringNull()
		}
		if ch.Storage != nil {
			chModel.Storage = &KubePersistentVolumeClaimModel{
				Storage:          optionalStringValue(ch.Storage.Storage),
				StorageClassName: optionalStringValue(ch.Storage.StorageClassName),
			}
		}
		if ch.Request != nil {
			chModel.Request = &KubeResourceConfigModel{
				CPU:              optionalStringValue(ch.Request.Cpu),
				Memory:           optionalStringValue(ch.Request.Memory),
				EphemeralStorage: optionalStringValue(ch.Request.EphemeralStorage),
				Storage:          optionalStringValue(ch.Request.Storage),
			}
		}
		if ch.Limit != nil {
			chModel.Limit = &KubeResourceConfigModel{
				CPU:              optionalStringValue(ch.Limit.Cpu),
				Memory:           optionalStringValue(ch.Limit.Memory),
				EphemeralStorage: optionalStringValue(ch.Limit.EphemeralStorage),
				Storage:          optionalStringValue(ch.Limit.Storage),
			}
		}
		data.ClickhouseDeploymentSpec = chModel
	} else {
		data.ClickhouseDeploymentSpec = nil
	}

	if spec.Otel != nil {
		otel := spec.Otel
		otelModel := &OtelCollectorSpecModel{
			Version: types.StringValue(otel.OtelCollectorVersion),
		}
		if otel.Request != nil {
			otelModel.Request = &KubeResourceConfigModel{
				CPU:              optionalStringValue(otel.Request.Cpu),
				Memory:           optionalStringValue(otel.Request.Memory),
				EphemeralStorage: optionalStringValue(otel.Request.EphemeralStorage),
				Storage:          optionalStringValue(otel.Request.Storage),
			}
		}
		if otel.Limit != nil {
			otelModel.Limit = &KubeResourceConfigModel{
				CPU:              optionalStringValue(otel.Limit.Cpu),
				Memory:           optionalStringValue(otel.Limit.Memory),
				EphemeralStorage: optionalStringValue(otel.Limit.EphemeralStorage),
				Storage:          optionalStringValue(otel.Limit.Storage),
			}
		}
		data.OtelCollectorSpec = otelModel
	} else {
		data.OtelCollectorSpec = nil
	}
}

// buildTelemetryUpdateMask compares plan and state to determine which top-level spec fields changed.
func buildTelemetryUpdateMask(data, state *TelemetryResourceModel) []string {
	var paths []string

	// Compare click_house block: any change triggers the whole top-level field
	planHasCH := data.ClickhouseDeploymentSpec != nil
	stateHasCH := state.ClickhouseDeploymentSpec != nil
	if planHasCH != stateHasCH {
		paths = append(paths, "click_house")
	} else if planHasCH && stateHasCH {
		plan := data.ClickhouseDeploymentSpec
		st := state.ClickhouseDeploymentSpec
		if !plan.Version.Equal(st.Version) ||
			!plan.GatewayId.Equal(st.GatewayId) ||
			clickhouseStorageChanged(plan.Storage, st.Storage) ||
			kubeResourceChanged(plan.Request, st.Request) ||
			kubeResourceChanged(plan.Limit, st.Limit) {
			paths = append(paths, "click_house")
		}
	}

	// Compare otel block
	planHasOtel := data.OtelCollectorSpec != nil
	stateHasOtel := state.OtelCollectorSpec != nil
	if planHasOtel != stateHasOtel {
		paths = append(paths, "otel")
	} else if planHasOtel && stateHasOtel {
		plan := data.OtelCollectorSpec
		st := state.OtelCollectorSpec
		if !plan.Version.Equal(st.Version) ||
			kubeResourceChanged(plan.Request, st.Request) ||
			kubeResourceChanged(plan.Limit, st.Limit) {
			paths = append(paths, "otel")
		}
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

func clickhouseStorageChanged(plan, state *KubePersistentVolumeClaimModel) bool {
	if (plan == nil) != (state == nil) {
		return true
	}
	if plan == nil {
		return false
	}
	return !plan.Storage.Equal(state.Storage) || !plan.StorageClassName.Equal(state.StorageClassName)
}

func kubeResourceChanged(plan, state *KubeResourceConfigModel) bool {
	if (plan == nil) != (state == nil) {
		return true
	}
	if plan == nil {
		return false
	}
	return !plan.CPU.Equal(state.CPU) ||
		!plan.Memory.Equal(state.Memory) ||
		!plan.EphemeralStorage.Equal(state.EphemeralStorage) ||
		!plan.Storage.Equal(state.Storage)
}

func (r *TelemetryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TelemetryResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	bc := r.client.NewBuilderClient(ctx)

	createReq := &serverv1.CreateTelemetryDeploymentRequest{
		ClusterId: data.KubeClusterId.ValueString(),
		Spec:      buildTelemetryDeploymentSpec(&data),
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
		updateReq := &serverv1.UpdateTelemetryDeploymentRequest{
			TelemetryDeploymentId: data.Id.ValueString(),
			Spec:                  buildTelemetryDeploymentSpec(&data),
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
