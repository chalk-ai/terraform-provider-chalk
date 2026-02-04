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
	_ resource.Resource                = &PrivateGatewayBindingResource{}
	_ resource.ResourceWithImportState = &PrivateGatewayBindingResource{}
)

func NewPrivateGatewayBindingResource() resource.Resource {
	return &PrivateGatewayBindingResource{}
}

type PrivateGatewayBindingResource struct {
	client *ClientManager
}

type PrivateGatewayBindingResourceModel struct {
	ClusterID        types.String `tfsdk:"cluster_id"`
	PrivateGatewayID types.String `tfsdk:"private_gateway_id"`
}

func (r *PrivateGatewayBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_private_gateway_binding"
}

func (r *PrivateGatewayBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a binding between a Chalk cluster and a private gateway.",
		Attributes: map[string]schema.Attribute{
			"cluster_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the cluster to bind to the private gateway.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"private_gateway_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the private gateway to bind to the cluster.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *PrivateGatewayBindingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *PrivateGatewayBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PrivateGatewayBindingResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	createRequest := &serverv1.CreateBindingPrivateGatewayRequest{
		ClusterId:        data.ClusterID.ValueString(),
		PrivateGatewayId: data.PrivateGatewayID.ValueString(),
	}

	_, err := cloudComponentsClient.CreateBindingPrivateGateway(ctx, connect.NewRequest(createRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating cluster private gateway binding",
			fmt.Sprintf("Could not create cluster private gateway binding: %s", err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PrivateGatewayBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PrivateGatewayBindingResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	getRequest := &serverv1.GetBindingPrivateGatewayRequest{
		ClusterId: data.ClusterID.ValueString(),
	}

	response, err := cloudComponentsClient.GetBindingPrivateGateway(ctx, connect.NewRequest(getRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading cluster private gateway binding",
			fmt.Sprintf("Could not read cluster private gateway binding: %s", err.Error()),
		)
		return
	}

	data.ClusterID = types.StringValue(response.Msg.ClusterId)
	data.PrivateGatewayID = types.StringValue(response.Msg.PrivateGatewayId)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PrivateGatewayBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Cluster private gateway bindings cannot be updated. They must be deleted and recreated.",
	)
}

func (r *PrivateGatewayBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PrivateGatewayBindingResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	deleteRequest := &serverv1.DeleteBindingPrivateGatewayRequest{
		ClusterId: data.ClusterID.ValueString(),
	}

	_, err := cloudComponentsClient.DeleteBindingPrivateGateway(ctx, connect.NewRequest(deleteRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting cluster private gateway binding",
			fmt.Sprintf("Could not delete cluster private gateway binding: %s", err.Error()),
		)
		return
	}
}

func (r *PrivateGatewayBindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("cluster_id"), req, resp)
}
