package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                   = &UnmanagedCloudStorageResource{}
	_ resource.ResourceWithImportState    = &UnmanagedCloudStorageResource{}
	_ resource.ResourceWithValidateConfig = &UnmanagedCloudStorageResource{}
)

func NewUnmanagedCloudStorageResource() resource.Resource {
	return &UnmanagedCloudStorageResource{}
}

// UnmanagedCloudStorageResource registers a reference to an existing bucket plus
// the cloud credential used to reach it. Chalk does not provision the bucket.
type UnmanagedCloudStorageResource struct {
	client *client.Manager
}

func (r *UnmanagedCloudStorageResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_unmanaged_cloud_storage"
}

func (r *UnmanagedCloudStorageResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = cloudStorageSchema(false)
}

// ValidateConfig enforces the URI-vs-kind pairing at plan time when kind is set,
// mirroring the server-side check. When kind is omitted the server infers it, so
// there is nothing to validate client-side.
func (r *UnmanagedCloudStorageResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data cloudStorageResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Kind.IsNull() || data.Kind.IsUnknown() || data.Uri.IsNull() || data.Uri.IsUnknown() {
		return
	}

	if ok, reason := validateStorageURIForKind(data.Kind.ValueString(), data.Uri.ValueString()); !ok {
		resp.Diagnostics.AddAttributeError(
			path.Root("uri"),
			"Invalid storage URI for kind",
			fmt.Sprintf("%s, got %q", reason, data.Uri.ValueString()),
		)
	}
}

func (r *UnmanagedCloudStorageResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureCloudManager(req, resp)
}

func (r *UnmanagedCloudStorageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data cloudStorageResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	credId := data.CloudCredentialId.ValueString()
	response, err := r.client.NewCloudComponentsClient(ctx).CreateCloudComponentStorage(ctx, connect.NewRequest(&serverv1.CreateCloudComponentStorageRequest{
		Storage: &serverv1.CloudComponentStorageRequest{
			Kind:              data.Kind.ValueString(), // empty when unset; server infers from the credential
			Spec:              &serverv1.CloudComponentStorage{Uri: data.Uri.ValueString()},
			Managed:           false,
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
	tflog.Trace(ctx, "created a chalk_unmanaged_cloud_storage resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UnmanagedCloudStorageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
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
			"Error reading unmanaged cloud storage",
			"Could not read unmanaged cloud storage "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	setCloudStorageState(&data, response.Msg.GetStorage())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UnmanagedCloudStorageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Unmanaged cloud storages cannot be updated. They must be deleted and recreated.",
	)
}

func (r *UnmanagedCloudStorageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
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
			"Error deleting unmanaged cloud storage",
			"Could not delete unmanaged cloud storage "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}
	tflog.Trace(ctx, "deleted a chalk_unmanaged_cloud_storage resource")
}

func (r *UnmanagedCloudStorageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
