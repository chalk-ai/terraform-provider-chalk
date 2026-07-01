package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &ClusterCloudStorageBindingResource{}
	_ resource.ResourceWithImportState = &ClusterCloudStorageBindingResource{}
)

func NewClusterCloudStorageBindingResource() resource.Resource {
	return &ClusterCloudStorageBindingResource{}
}

type ClusterCloudStorageBindingResource struct {
	client *client.Manager
}

type ClusterCloudStorageBindingResourceModel struct {
	Id             types.String `tfsdk:"id"`
	ClusterId      types.String `tfsdk:"cluster_id"`
	CloudStorageId types.String `tfsdk:"cloud_storage_id"`
	StorageRole    types.String `tfsdk:"storage_role"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func (r *ClusterCloudStorageBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_cloud_storage_binding"
}

func (r *ClusterCloudStorageBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Binds a `chalk_cloud_storage` to a cluster for a given storage role.\n\n" +
			"Bindings are keyed by `(cluster_id, storage_role)` — a cluster has at most one storage per role, and the binding is identified by that pair rather than by `cloud_storage_id`. " +
			"Every attribute is replace-only (there is no update RPC).\n\n" +
			"~> **Delete removes whatever occupies the (cluster, role) slot.** Because deletes target `(cluster_id, storage_role)` (not a binding id), a stale plan can delete a binding that was created out-of-band for the same cluster and role. " +
			"If the storage occupying a slot is reassigned outside Terraform, the next plan reconciles `cloud_storage_id` and schedules a recreate.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Binding identifier.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cluster_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the cluster to bind the cloud storage to. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cloud_storage_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the `chalk_cloud_storage` to bind. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"storage_role": schema.StringAttribute{
				MarkdownDescription: cloudStorageRoleMarkdown,
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(cloudStorageRoleValues...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp at which the binding was created.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp at which the binding was last updated.",
				Computed:            true,
			},
		},
	}
}

func (r *ClusterCloudStorageBindingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ClusterCloudStorageBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ClusterCloudStorageBindingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := parseCloudStorageRole(data.StorageRole.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("storage_role"), "Invalid storage role", err.Error())
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	response, err := cloudComponentsClient.CreateBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.CreateBindingClusterCloudStorageRequest{
		ClusterId:      data.ClusterId.ValueString(),
		CloudStorageId: data.CloudStorageId.ValueString(),
		StorageRole:    role,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeAlreadyExists {
			resp.Diagnostics.AddError(
				"Cluster already has a binding for this role",
				fmt.Sprintf("Cluster %q already has a cloud storage binding for role %q. Remove the existing binding first, or import it. (%s)",
					data.ClusterId.ValueString(), data.StorageRole.ValueString(), err.Error()),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error creating cluster cloud storage binding",
			fmt.Sprintf("Could not create cluster cloud storage binding: %s", err.Error()),
		)
		return
	}

	if response.Msg.GetBinding() == nil {
		resp.Diagnostics.AddError(
			"Empty create response",
			"The server returned no binding in the create response. This is unexpected; please report it to the provider developers.",
		)
		return
	}

	setClusterCloudStorageBindingState(&data, response.Msg.GetBinding())

	tflog.Trace(ctx, "created a chalk_cluster_cloud_storage_binding resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterCloudStorageBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ClusterCloudStorageBindingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := parseCloudStorageRole(data.StorageRole.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("storage_role"), "Invalid storage role", err.Error())
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	// Get is keyed by (cluster_id, storage_role) and IGNORES cloud_storage_id, so it
	// returns whatever storage currently occupies this slot.
	response, err := cloudComponentsClient.GetBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.GetBindingClusterCloudStorageRequest{
		ClusterId:   data.ClusterId.ValueString(),
		StorageRole: role,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading cluster cloud storage binding",
			fmt.Sprintf("Could not read cluster cloud storage binding: %s", err.Error()),
		)
		return
	}

	// If the slot is now occupied by a different storage, the (cluster, role) pair
	// was reassigned out-of-band. Reflect the observed value so the next plan detects
	// the drift and (because cloud_storage_id is RequiresReplace) recreates.
	setClusterCloudStorageBindingState(&data, response.Msg.GetBinding())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterCloudStorageBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Cluster cloud storage bindings cannot be updated. They must be deleted and recreated.",
	)
}

func (r *ClusterCloudStorageBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ClusterCloudStorageBindingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := parseCloudStorageRole(data.StorageRole.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("storage_role"), "Invalid storage role", err.Error())
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	// Delete targets (cluster_id, storage_role) and removes whatever occupies that slot.
	_, err = cloudComponentsClient.DeleteBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.DeleteBindingClusterCloudStorageRequest{
		ClusterId:   data.ClusterId.ValueString(),
		StorageRole: role,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return
		}
		resp.Diagnostics.AddError(
			"Error deleting cluster cloud storage binding",
			fmt.Sprintf("Could not delete cluster cloud storage binding: %s", err.Error()),
		)
		return
	}

	tflog.Trace(ctx, "deleted a chalk_cluster_cloud_storage_binding resource")
}

// ImportState imports by the real key "<cluster_id>:<storage_role>", not the binding id.
func (r *ClusterCloudStorageBindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	clusterID, role, err := splitCloudStorageBindingImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_id"), clusterID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("storage_role"), role)...)
}

func setClusterCloudStorageBindingState(data *ClusterCloudStorageBindingResourceModel, binding *serverv1.ClusterCloudStorageBinding) {
	if binding == nil {
		return
	}
	data.Id = types.StringValue(binding.GetId())
	if binding.GetClusterId() != "" {
		data.ClusterId = types.StringValue(binding.GetClusterId())
	}
	data.CloudStorageId = types.StringValue(binding.GetCloudStorageId())
	if friendly, ok := cloudStorageRoleToFriendly[binding.GetStorageRole()]; ok {
		data.StorageRole = types.StringValue(friendly)
	}
	data.CreatedAt = timestampToStringValue(binding.GetCreatedAt())
	data.UpdatedAt = timestampToStringValue(binding.GetUpdatedAt())
}
