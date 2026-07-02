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
	_ resource.Resource                = &ManagedCloudStorageResource{}
	_ resource.ResourceWithImportState = &ManagedCloudStorageResource{}
)

func NewManagedCloudStorageResource() resource.Resource {
	return &ManagedCloudStorageResource{}
}

// ManagedCloudStorageResource registers a Chalk-managed cloud storage: Chalk owns
// the bucket and derives its uri, so the user supplies only the cloud credential
// (and optionally the kind).
type ManagedCloudStorageResource struct {
	client *client.Manager
}

func (r *ManagedCloudStorageResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_cloud_storage"
}

func (r *ManagedCloudStorageResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = cloudStorageSchema(true)
}

func (r *ManagedCloudStorageResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureCloudManager(req, resp)
}

func (r *ManagedCloudStorageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data cloudStorageResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	credId := data.CloudCredentialId.ValueString()
	response, err := r.client.NewCloudComponentsClient(ctx).CreateCloudComponentStorage(ctx, connect.NewRequest(&serverv1.CreateCloudComponentStorageRequest{
		Storage: &serverv1.CloudComponentStorageRequest{
			Kind:              data.Kind.ValueString(),           // empty when unset; server infers from the credential
			Spec:              &serverv1.CloudComponentStorage{}, // managed: Chalk derives the uri
			Managed:           true,
			CloudCredentialId: &credId,
		},
	}))
	if err != nil {
		summary, detail := describeCloudStorageCreateError(err)
		resp.Diagnostics.AddError(summary, detail)
		return
	}
	if response.Msg.GetStorage() == nil {
		resp.Diagnostics.AddError(
			"Empty create response",
			"The server returned no storage in the create response. This is unexpected; please report it to the provider developers.",
		)
		return
	}

	setCloudStorageState(&data, response.Msg.GetStorage())
	tflog.Trace(ctx, "created a chalk_managed_cloud_storage resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ManagedCloudStorageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data cloudStorageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	response, err := r.client.NewCloudComponentsClient(ctx).GetCloudComponentStorage(ctx, connect.NewRequest(&serverv1.GetCloudComponentStorageRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading managed cloud storage",
			"Could not read managed cloud storage "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	setCloudStorageState(&data, response.Msg.GetStorage())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ManagedCloudStorageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Managed cloud storages cannot be updated. They must be deleted and recreated.",
	)
}

func (r *ManagedCloudStorageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data cloudStorageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.NewCloudComponentsClient(ctx).DeleteCloudComponentStorage(ctx, connect.NewRequest(&serverv1.DeleteCloudComponentStorageRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		resp.Diagnostics.AddError(
			"Error deleting managed cloud storage",
			"Could not delete managed cloud storage "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}
	tflog.Trace(ctx, "deleted a chalk_managed_cloud_storage resource")
}

func (r *ManagedCloudStorageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
