package provider

import (
	"context"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &UnmanagedContainerRegistryResource{}
	_ resource.ResourceWithImportState = &UnmanagedContainerRegistryResource{}
)

func NewUnmanagedContainerRegistryResource() resource.Resource {
	return &UnmanagedContainerRegistryResource{}
}

// UnmanagedContainerRegistryResource registers a reference to an existing cloud
// container registry (GAR/ECR/ACR) plus the cloud credential used to reach it.
// Chalk does not provision the registry; the kind is derived by the server from
// `name`. Every attribute is replace-only: there is no update RPC, so any change
// forces recreation.
type UnmanagedContainerRegistryResource struct {
	client *client.Manager
}

func (r *UnmanagedContainerRegistryResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_unmanaged_container_registry"
}

func (r *UnmanagedContainerRegistryResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = cloudContainerRegistrySchema(false)
}

func (r *UnmanagedContainerRegistryResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureCloudManager(req, resp)
}

func (r *UnmanagedContainerRegistryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data cloudContainerRegistryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	credId := data.CloudCredentialId.ValueString()
	response, err := r.client.NewCloudComponentsClient(ctx).CreateCloudComponentContainerRegistry(ctx, connect.NewRequest(&serverv1.CreateCloudComponentContainerRegistryRequest{
		ContainerRegistry: &serverv1.CloudComponentContainerRegistryRequest{
			Spec:              &serverv1.CloudComponentContainerRegistry{Name: data.Name.ValueString()},
			Managed:           false,
			CloudCredentialId: &credId,
		},
	}))
	if err != nil {
		summary, detail := describeCloudContainerRegistryCreateError(err)
		resp.Diagnostics.AddError(summary, detail)
		return
	}
	if response.Msg.GetContainerRegistry() == nil {
		resp.Diagnostics.AddError(
			"Empty create response",
			"The server returned no container registry in the create response. This is unexpected; please report it to the provider developers.",
		)
		return
	}

	setCloudContainerRegistryState(&data, response.Msg.GetContainerRegistry())
	tflog.Trace(ctx, "created a chalk_unmanaged_container_registry resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UnmanagedContainerRegistryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data cloudContainerRegistryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	response, err := r.client.NewCloudComponentsClient(ctx).GetCloudComponentContainerRegistry(ctx, connect.NewRequest(&serverv1.GetCloudComponentContainerRegistryRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading unmanaged container registry",
			"Could not read unmanaged container registry "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	setCloudContainerRegistryState(&data, response.Msg.GetContainerRegistry())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UnmanagedContainerRegistryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Unmanaged container registries cannot be updated. They must be deleted and recreated.",
	)
}

func (r *UnmanagedContainerRegistryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data cloudContainerRegistryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.NewCloudComponentsClient(ctx).DeleteCloudComponentContainerRegistry(ctx, connect.NewRequest(&serverv1.DeleteCloudComponentContainerRegistryRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		resp.Diagnostics.AddError(
			"Error deleting unmanaged container registry",
			"Could not delete unmanaged container registry "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}
	tflog.Trace(ctx, "deleted a chalk_unmanaged_container_registry resource")
}

func (r *UnmanagedContainerRegistryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
