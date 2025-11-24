package provider

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &ClusterGatewayBindingResource{}
	_ resource.ResourceWithImportState = &ClusterGatewayBindingResource{}
)

func NewClusterGatewayBindingResource() resource.Resource {
	return &ClusterGatewayBindingResource{}
}

type ClusterGatewayBindingResource struct {
	client *client.Manager
}

type ClusterGatewayBindingResourceModel struct {
	ClusterID        types.String `tfsdk:"cluster_id"`
	ClusterGatewayID types.String `tfsdk:"cluster_gateway_id"`
}

func (r *ClusterGatewayBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_gateway_binding"
}

func (r *ClusterGatewayBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
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
			"cluster_gateway_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the cluster gateway to bind to the cluster.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *ClusterGatewayBindingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Manager)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Manager, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *ClusterGatewayBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ClusterGatewayBindingResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient, err := r.client.NewCloudComponentsClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cloud Components Client", err.Error())
		return
	}

	createRequest := &serverv1.CreateBindingClusterGatewayRequest{
		ClusterId:        data.ClusterID.ValueString(),
		ClusterGatewayId: data.ClusterGatewayID.ValueString(),
	}
	_, err = cloudComponentsClient.CreateBindingClusterGateway(ctx, connect.NewRequest(createRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating cluster gateway binding",
			fmt.Sprintf("Could not create cluster gateway binding: %s", err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterGatewayBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ClusterGatewayBindingResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient, err := r.client.NewCloudComponentsClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cloud Components Client", err.Error())
		return
	}

	getRequest := &serverv1.GetBindingClusterGatewayRequest{
		ClusterId: data.ClusterID.ValueString(),
	}

	response, err := cloudComponentsClient.GetBindingClusterGateway(ctx, connect.NewRequest(getRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading cluster gateway binding",
			fmt.Sprintf("Could not read cluster gateway binding: %s", err.Error()),
		)
		return
	}

	data.ClusterID = types.StringValue(response.Msg.ClusterId)
	data.ClusterGatewayID = types.StringValue(response.Msg.ClusterGatewayId)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterGatewayBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Cluster gateway bindings cannot be updated. They must be deleted and recreated.",
	)
}

func (r *ClusterGatewayBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ClusterGatewayBindingResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient, err := r.client.NewCloudComponentsClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cloud Components Client", err.Error())
		return
	}

	deleteRequest := &serverv1.DeleteBindingClusterGatewayRequest{
		ClusterId: data.ClusterID.ValueString(),
	}
	_, err = cloudComponentsClient.DeleteBindingClusterGateway(ctx, connect.NewRequest(deleteRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting cluster gateway binding",
			fmt.Sprintf("Could not delete cluster gateway binding: %s", err.Error()),
		)
		return
	}
}

func (r *ClusterGatewayBindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("cluster_id"), req, resp)
}
