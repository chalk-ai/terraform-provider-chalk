package provider

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &EnvironmentGatewayBindingResource{}
	_ resource.ResourceWithImportState = &EnvironmentGatewayBindingResource{}
)

func NewEnvironmentGatewayBindingResource() resource.Resource {
	return &EnvironmentGatewayBindingResource{}
}

type EnvironmentGatewayBindingResource struct {
	client *ClientManager
}

type EnvironmentGatewayBindingResourceModel struct {
	EnvironmentID    types.String `tfsdk:"environment_id"`
	ClusterGatewayID types.String `tfsdk:"cluster_gateway_id"`
}

func (r *EnvironmentGatewayBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_gateway_binding"
}

func (r *EnvironmentGatewayBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a binding between a Chalk environment and a gateway.",
		Attributes: map[string]schema.Attribute{
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the environment to bind to the gateway.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cluster_gateway_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the cluster gateway to bind to the environment.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *EnvironmentGatewayBindingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EnvironmentGatewayBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EnvironmentGatewayBindingResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	createRequest := &serverv1.CreateBindingEnvironmentGatewayRequest{
		EnvironmentId:    data.EnvironmentID.ValueString(),
		ClusterGatewayId: data.ClusterGatewayID.ValueString(),
	}

	_, err := cloudComponentsClient.CreateBindingEnvironmentGateway(ctx, connect.NewRequest(createRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating environment gateway binding",
			fmt.Sprintf("Could not create environment gateway binding: %s", err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentGatewayBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EnvironmentGatewayBindingResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	getRequest := &serverv1.GetBindingEnvironmentGatewayRequest{
		EnvironmentId: data.EnvironmentID.ValueString(),
	}

	response, err := cloudComponentsClient.GetBindingEnvironmentGateway(ctx, connect.NewRequest(getRequest))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading environment gateway binding",
			fmt.Sprintf("Could not read environment gateway binding: %s", err.Error()),
		)
		return
	}

	data.EnvironmentID = types.StringValue(response.Msg.EnvironmentId)
	data.ClusterGatewayID = types.StringValue(response.Msg.ClusterGatewayId)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentGatewayBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Environment gateway bindings cannot be updated. They must be deleted and recreated.",
	)
}

func (r *EnvironmentGatewayBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EnvironmentGatewayBindingResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	deleteRequest := &serverv1.DeleteBindingEnvironmentGatewayRequest{
		EnvironmentId: data.EnvironmentID.ValueString(),
	}

	_, err := cloudComponentsClient.DeleteBindingEnvironmentGateway(ctx, connect.NewRequest(deleteRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting environment gateway binding",
			fmt.Sprintf("Could not delete environment gateway binding: %s", err.Error()),
		)
		return
	}
}

func (r *EnvironmentGatewayBindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("environment_id"), req, resp)
}
