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
)

var (
	_ resource.Resource                = &TelemetryBindingResource{}
	_ resource.ResourceWithImportState = &TelemetryBindingResource{}
)

func NewTelemetryBindingResource() resource.Resource {
	return &TelemetryBindingResource{}
}

type TelemetryBindingResource struct {
	client *ClientManager
}

type TelemetryBindingResourceModel struct {
	ClusterID             types.String `tfsdk:"cluster_id"`
	TelemetryDeploymentID types.String `tfsdk:"telemetry_deployment_id"`
}

func (r *TelemetryBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_telemetry_binding"
}

func (r *TelemetryBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a binding between a Chalk cluster and a gateway.",
		Attributes: map[string]schema.Attribute{
			"cluster_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the cluster to bind to the gateway.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"telemetry_deployment_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the cluster gateway to bind to the cluster.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *TelemetryBindingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TelemetryBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TelemetryBindingResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	createRequest := &serverv1.CreateBindingClusterTelemetryDeploymentRequest{
		ClusterId:             data.ClusterID.ValueString(),
		TelemetryDeploymentId: data.TelemetryDeploymentID.ValueString(),
	}

	_, err := cloudComponentsClient.CreateBindingClusterTelemetryDeployment(ctx, connect.NewRequest(createRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating telemetry binding",
			fmt.Sprintf("Could not create telemetry binding: %s", err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TelemetryBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TelemetryBindingResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	getRequest := &serverv1.GetBindingClusterTelemetryDeploymentRequest{
		ClusterId: data.ClusterID.ValueString(),
	}

	response, err := cloudComponentsClient.GetBindingClusterTelemetryDeployment(ctx, connect.NewRequest(getRequest))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading telemetry binding",
			fmt.Sprintf("Could not read telemetry binding: %s", err.Error()),
		)
		return
	}

	data.ClusterID = types.StringValue(response.Msg.ClusterId)
	data.TelemetryDeploymentID = types.StringValue(response.Msg.TelemetryDeploymentId)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TelemetryBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Telemetry bindings cannot be updated. They must be deleted and recreated.",
	)
}

func (r *TelemetryBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TelemetryBindingResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	deleteRequest := &serverv1.DeleteBindingClusterTelemetryDeploymentRequest{
		ClusterId: data.ClusterID.ValueString(),
	}

	_, err := cloudComponentsClient.DeleteBindingClusterTelemetryDeployment(ctx, connect.NewRequest(deleteRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting telemetry binding",
			fmt.Sprintf("Could not delete telemetry binding: %s", err.Error()),
		)
		return
	}
}

func (r *TelemetryBindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("cluster_id"), req, resp)
}
