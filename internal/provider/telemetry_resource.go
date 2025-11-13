package provider

import (
	"context"
	"fmt"
	"github.com/chalk-ai/terraform-provider-chalk/internal/client"
	"github.com/cockroachdb/errors"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &TelemetryResource{}
var _ resource.ResourceWithImportState = &TelemetryResource{}

func NewTelemetryResource() resource.Resource {
	return &TelemetryResource{}
}

type TelemetryResource struct {
	client *client.Manager
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

	clientManager, ok := req.ProviderData.(*client.Manager)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ClientManager, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = clientManager
}

func (r *TelemetryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TelemetryResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	bc, err := r.client.NewBuilderClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("client error", errors.Wrap(err, "get builder client").Error())
		return
	}

	// Convert terraform model to proto request
	createReq := &serverv1.CreateTelemetryDeploymentRequest{
		ClusterId: data.KubeClusterId.ValueString(),
		Spec: &serverv1.TelemetryDeploymentSpec{
			Namespace: data.Namespace.ValueStringPointer(),
		},
	}

	// Convert listeners
	if data.ClickhouseDeploymentSpec != nil {
		createReq.Spec.ClickHouse = &serverv1.ClickHouseSpec{
			ClickHouseVersion: data.ClickhouseDeploymentSpec.Version.ValueString(),
		}
		if !data.ClickhouseDeploymentSpec.GatewayId.IsNull() {
			createReq.Spec.ClickHouse.GatewayId = data.ClickhouseDeploymentSpec.GatewayId.ValueStringPointer()
		}
		if data.ClickhouseDeploymentSpec.Storage != nil {
			createReq.Spec.ClickHouse.Storage = &serverv1.KubePersistentVolumeClaim{
				Storage:          data.ClickhouseDeploymentSpec.Storage.Storage.ValueString(),
				StorageClassName: data.ClickhouseDeploymentSpec.Storage.StorageClassName.ValueString(),
			}
		}
		if data.ClickhouseDeploymentSpec.Request != nil {
			createReq.Spec.ClickHouse.Request = &serverv1.KubeResourceConfig{
				Storage:          data.ClickhouseDeploymentSpec.Request.Storage.ValueString(),
				Cpu:              data.ClickhouseDeploymentSpec.Request.CPU.ValueString(),
				Memory:           data.ClickhouseDeploymentSpec.Request.Memory.ValueString(),
				EphemeralStorage: data.ClickhouseDeploymentSpec.Request.EphemeralStorage.ValueString(),
			}
		}
		if data.ClickhouseDeploymentSpec.Limit != nil {
			createReq.Spec.ClickHouse.Limit = &serverv1.KubeResourceConfig{
				Storage:          data.ClickhouseDeploymentSpec.Limit.Storage.ValueString(),
				Cpu:              data.ClickhouseDeploymentSpec.Limit.CPU.ValueString(),
				Memory:           data.ClickhouseDeploymentSpec.Limit.Memory.ValueString(),
				EphemeralStorage: data.ClickhouseDeploymentSpec.Limit.EphemeralStorage.ValueString(),
			}
		}
	}
	if data.OtelCollectorSpec != nil {
		createReq.Spec.Otel = &serverv1.OtelCollectorSpec{
			OtelCollectorVersion: data.OtelCollectorSpec.Version.ValueString(),
		}
		if data.OtelCollectorSpec.Request != nil {
			createReq.Spec.Otel.Request = &serverv1.KubeResourceConfig{
				Storage:          data.OtelCollectorSpec.Request.Storage.ValueString(),
				Cpu:              data.OtelCollectorSpec.Request.CPU.ValueString(),
				Memory:           data.OtelCollectorSpec.Request.Memory.ValueString(),
				EphemeralStorage: data.OtelCollectorSpec.Request.EphemeralStorage.ValueString(),
			}
		}
		if data.OtelCollectorSpec.Limit != nil {
			createReq.Spec.Otel.Limit = &serverv1.KubeResourceConfig{
				Storage:          data.OtelCollectorSpec.Limit.Storage.ValueString(),
				Cpu:              data.OtelCollectorSpec.Limit.CPU.ValueString(),
				Memory:           data.OtelCollectorSpec.Limit.Memory.ValueString(),
				EphemeralStorage: data.OtelCollectorSpec.Limit.EphemeralStorage.ValueString(),
			}
		}
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

	// Create auth client first
	// Create builder client
	bc, err := r.client.NewBuilderClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("client error", errors.Wrap(err, "get builder client").Error())
		return
	}

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

	// Update the model with the fetched data
	spec := telemetry.Msg.Deployment.Spec
	if spec != nil {
		data.Namespace = types.StringPointerValue(spec.Namespace)
		data.KubeClusterId = types.StringValue(telemetry.Msg.Deployment.ClusterId)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TelemetryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Note: According to the proto definition, there's no UpdateClusterTelemetryDeployment method
	// So we'll need to delete and recreate, but this would be disruptive
	// For now, we'll return an error indicating updates are not supported
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Cluster telemetryu updates are not supported by the Chalk API. Please recreate the resource if changes are needed.",
	)
}

func (r *TelemetryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TelemetryResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	// Create builder client
	bc, err := r.client.NewBuilderClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("client error", errors.Wrap(err, "get builder client").Error())
		return
	}

	deleteRequest := &serverv1.DeleteTelemetryDeploymentRequest{
		ClusterId: data.KubeClusterId.ValueString(),
		Namespace: data.Namespace.ValueStringPointer(),
	}

	_, err = bc.DeleteTelemetryDeployment(ctx, connect.NewRequest(deleteRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Chalk Cluster Telemetry Deployment",
			fmt.Sprintf("Could not delete cluster telemetry %s-%s: %v", data.KubeClusterId.ValueString(), data.Namespace.ValueString(), err),
		)
		return
	}
	tflog.Trace(ctx, "deleted a chalk_cluster_telemetry resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TelemetryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
