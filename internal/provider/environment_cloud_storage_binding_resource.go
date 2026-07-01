package provider

import (
	"context"
	"fmt"
	"strings"

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
	_ resource.Resource                = &EnvironmentCloudStorageBindingResource{}
	_ resource.ResourceWithImportState = &EnvironmentCloudStorageBindingResource{}
)

func NewEnvironmentCloudStorageBindingResource() resource.Resource {
	return &EnvironmentCloudStorageBindingResource{}
}

type EnvironmentCloudStorageBindingResource struct {
	client *client.Manager
}

type EnvironmentCloudStorageBindingResourceModel struct {
	Id             types.String `tfsdk:"id"`
	EnvironmentId  types.String `tfsdk:"environment_id"`
	CloudStorageId types.String `tfsdk:"cloud_storage_id"`
	StorageRole    types.String `tfsdk:"storage_role"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func (r *EnvironmentCloudStorageBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_cloud_storage_binding"
}

func (r *EnvironmentCloudStorageBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Binds a `chalk_cloud_storage` to an environment for a given storage role.\n\n" +
			"Bindings are keyed by `(environment_id, storage_role)` — an environment has at most one storage per role, and the binding is identified by that pair rather than by `cloud_storage_id`. " +
			"Every attribute is replace-only (there is no update RPC).\n\n" +
			"~> **Delete removes whatever occupies the (environment, role) slot.** Because deletes target `(environment_id, storage_role)` (not a binding id), a stale plan can delete a binding that was created out-of-band for the same environment and role. " +
			"If the storage occupying a slot is reassigned outside Terraform, the next plan reconciles `cloud_storage_id` and schedules a recreate.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Binding identifier.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the environment to bind the cloud storage to. Changing this forces a new resource.",
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

func (r *EnvironmentCloudStorageBindingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EnvironmentCloudStorageBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EnvironmentCloudStorageBindingResourceModel
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

	response, err := cloudComponentsClient.CreateBindingEnvironmentCloudStorage(ctx, connect.NewRequest(&serverv1.CreateBindingEnvironmentCloudStorageRequest{
		EnvironmentId:  data.EnvironmentId.ValueString(),
		CloudStorageId: data.CloudStorageId.ValueString(),
		StorageRole:    role,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeAlreadyExists {
			resp.Diagnostics.AddError(
				"Environment already has a binding for this role",
				fmt.Sprintf("Environment %q already has a cloud storage binding for role %q. Remove the existing binding first, or import it. (%s)",
					data.EnvironmentId.ValueString(), data.StorageRole.ValueString(), err.Error()),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error creating environment cloud storage binding",
			fmt.Sprintf("Could not create environment cloud storage binding: %s", err.Error()),
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

	setEnvironmentCloudStorageBindingState(&data, response.Msg.GetBinding())

	tflog.Trace(ctx, "created a chalk_environment_cloud_storage_binding resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentCloudStorageBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EnvironmentCloudStorageBindingResourceModel
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

	// Get is keyed by (environment_id, storage_role) and IGNORES cloud_storage_id,
	// so it returns whatever storage currently occupies this slot.
	response, err := cloudComponentsClient.GetBindingEnvironmentCloudStorage(ctx, connect.NewRequest(&serverv1.GetBindingEnvironmentCloudStorageRequest{
		EnvironmentId: data.EnvironmentId.ValueString(),
		StorageRole:   role,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading environment cloud storage binding",
			fmt.Sprintf("Could not read environment cloud storage binding: %s", err.Error()),
		)
		return
	}

	// If the slot is now occupied by a different storage, the (environment, role)
	// pair was reassigned out-of-band. Reflect the observed value so the next plan
	// detects the drift and (because cloud_storage_id is RequiresReplace) recreates.
	setEnvironmentCloudStorageBindingState(&data, response.Msg.GetBinding())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentCloudStorageBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Environment cloud storage bindings cannot be updated. They must be deleted and recreated.",
	)
}

func (r *EnvironmentCloudStorageBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EnvironmentCloudStorageBindingResourceModel
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

	// Delete targets (environment_id, storage_role) and removes whatever occupies
	// that slot.
	_, err = cloudComponentsClient.DeleteBindingEnvironmentCloudStorage(ctx, connect.NewRequest(&serverv1.DeleteBindingEnvironmentCloudStorageRequest{
		EnvironmentId: data.EnvironmentId.ValueString(),
		StorageRole:   role,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return
		}
		resp.Diagnostics.AddError(
			"Error deleting environment cloud storage binding",
			fmt.Sprintf("Could not delete environment cloud storage binding: %s", err.Error()),
		)
		return
	}

	tflog.Trace(ctx, "deleted a chalk_environment_cloud_storage_binding resource")
}

// ImportState imports by the real key "<environment_id>:<storage_role>", not the binding id.
func (r *EnvironmentCloudStorageBindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	envID, role, err := splitCloudStorageBindingImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), envID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("storage_role"), role)...)
}

func setEnvironmentCloudStorageBindingState(data *EnvironmentCloudStorageBindingResourceModel, binding *serverv1.EnvironmentCloudStorageBinding) {
	if binding == nil {
		return
	}
	data.Id = types.StringValue(binding.GetId())
	if binding.GetEnvironmentId() != "" {
		data.EnvironmentId = types.StringValue(binding.GetEnvironmentId())
	}
	data.CloudStorageId = types.StringValue(binding.GetCloudStorageId())
	if friendly, ok := cloudStorageRoleToFriendly[binding.GetStorageRole()]; ok {
		data.StorageRole = types.StringValue(friendly)
	}
	data.CreatedAt = timestampToStringValue(binding.GetCreatedAt())
	data.UpdatedAt = timestampToStringValue(binding.GetUpdatedAt())
}

// splitCloudStorageBindingImportID parses "<target_id>:<role>" and validates the role.
func splitCloudStorageBindingImportID(id string) (targetID, role string, err error) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("import ID must be in the form \"<target_id>:<role>\", got %q", id)
	}
	if _, perr := parseCloudStorageRole(parts[1]); perr != nil {
		return "", "", perr
	}
	return parts[0], parts[1], nil
}
