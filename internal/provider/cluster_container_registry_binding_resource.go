package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &ClusterContainerRegistryBindingResource{}
	_ resource.ResourceWithImportState = &ClusterContainerRegistryBindingResource{}
)

func NewClusterContainerRegistryBindingResource() resource.Resource {
	return &ClusterContainerRegistryBindingResource{}
}

type ClusterContainerRegistryBindingResource struct {
	client *client.Manager
}

type ClusterContainerRegistryBindingResourceModel struct {
	ClusterID           types.String `tfsdk:"cluster_id"`
	ContainerRegistryID types.String `tfsdk:"container_registry_id"`
}

func (r *ClusterContainerRegistryBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_container_registry_binding"
}

func (r *ClusterContainerRegistryBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a binding between a Chalk cluster and a container registry.",
		Attributes: map[string]schema.Attribute{
			"cluster_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the cluster to bind to the container registry.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"container_registry_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the container registry to bind to the cluster.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *ClusterContainerRegistryBindingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ClusterContainerRegistryBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ClusterContainerRegistryBindingResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	createRequest := &serverv1.CreateBindingClusterContainerRegistryRequest{
		ClusterId:           data.ClusterID.ValueString(),
		ContainerRegistryId: data.ContainerRegistryID.ValueString(),
	}

	_, err := cloudComponentsClient.CreateBindingClusterContainerRegistry(ctx, connect.NewRequest(createRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating cluster container registry binding",
			fmt.Sprintf("Could not create cluster container registry binding: %s", err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterContainerRegistryBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ClusterContainerRegistryBindingResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	getRequest := &serverv1.GetBindingClusterContainerRegistryRequest{
		ClusterId: data.ClusterID.ValueString(),
	}

	response, err := cloudComponentsClient.GetBindingClusterContainerRegistry(ctx, connect.NewRequest(getRequest))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading cluster container registry binding",
			fmt.Sprintf("Could not read cluster container registry binding: %s", err.Error()),
		)
		return
	}

	data.ClusterID = types.StringValue(response.Msg.GetClusterId())
	data.ContainerRegistryID = types.StringValue(response.Msg.GetContainerRegistryId())

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterContainerRegistryBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Cluster container registry bindings cannot be updated. They must be deleted and recreated.",
	)
}

func (r *ClusterContainerRegistryBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ClusterContainerRegistryBindingResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	deleteRequest := &serverv1.DeleteBindingClusterContainerRegistryRequest{
		ClusterId: data.ClusterID.ValueString(),
	}

	_, err := cloudComponentsClient.DeleteBindingClusterContainerRegistry(ctx, connect.NewRequest(deleteRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting cluster container registry binding",
			fmt.Sprintf("Could not delete cluster container registry binding: %s", err.Error()),
		)
		return
	}
}

func (r *ClusterContainerRegistryBindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("cluster_id"), req, resp)
}
