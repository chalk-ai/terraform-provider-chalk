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
	_ resource.Resource                   = &CloudContainerRegistryResource{}
	_ resource.ResourceWithImportState    = &CloudContainerRegistryResource{}
	_ resource.ResourceWithValidateConfig = &CloudContainerRegistryResource{}
)

func NewCloudContainerRegistryResource() resource.Resource {
	return &CloudContainerRegistryResource{}
}

// CloudContainerRegistryResource registers a reference to a cloud container
// registry (GAR/ECR/ACR) plus the cloud credential used to reach it. Chalk does
// not provision the registry. Every attribute is replace-only: there is no update
// RPC, so any change forces recreation.
type CloudContainerRegistryResource struct {
	client *client.Manager
}

func (r *CloudContainerRegistryResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_container_registry"
}

func (r *CloudContainerRegistryResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = cloudContainerRegistrySchema()
}

// ValidateConfig enforces at plan time that exactly one config block is set and
// that the registry `name` path matches the kind that block implies, mirroring
// the server-side checks so a doomed config fails fast.
func (r *CloudContainerRegistryResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data cloudContainerRegistryResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Count the config blocks that are set. `config` is Required, so a nil model
	// only happens for an unknown (interpolated) value we cannot validate yet.
	if data.Config == nil {
		return
	}
	var set int
	if data.Config.Gar != nil {
		set++
	}
	if data.Config.Ecr != nil {
		set++
	}
	if data.Config.Acr != nil {
		set++
	}
	if set == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("config"),
			"Missing container registry config",
			"Exactly one of config.gar, config.ecr, or config.acr must be set.",
		)
		return
	}
	if set > 1 {
		resp.Diagnostics.AddAttributeError(
			path.Root("config"),
			"Ambiguous container registry config",
			"Only one of config.gar, config.ecr, or config.acr may be set.",
		)
		return
	}

	// The registry path is only checkable once name is known.
	if data.Name.IsNull() || data.Name.IsUnknown() {
		return
	}
	kind := registryKindFromConfig(data.Config)
	if ok, reason := validateRegistryPathForKind(kind, data.Name.ValueString()); !ok {
		resp.Diagnostics.AddAttributeError(
			path.Root("name"),
			"Invalid registry name for kind",
			fmt.Sprintf("%s, got %q", reason, data.Name.ValueString()),
		)
	}
}

func (r *CloudContainerRegistryResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureCloudManager(req, resp)
}

func (r *CloudContainerRegistryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data cloudContainerRegistryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	kind := registryKindFromConfig(data.Config)
	if kind == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("config"),
			"Missing container registry config",
			"Exactly one of config.gar, config.ecr, or config.acr must be set.",
		)
		return
	}

	spec := &serverv1.CloudComponentContainerRegistry{
		Name:   data.Name.ValueString(),
		Config: registryConfigToProto(data.Config),
	}
	if !data.Designator.IsNull() && !data.Designator.IsUnknown() {
		designator := data.Designator.ValueString()
		spec.Designator = &designator
	}

	credId := data.CloudCredentialId.ValueString()
	response, err := r.client.NewCloudComponentsClient(ctx).CreateCloudComponentContainerRegistry(ctx, connect.NewRequest(&serverv1.CreateCloudComponentContainerRegistryRequest{
		ContainerRegistry: &serverv1.CloudComponentContainerRegistryRequest{
			Kind:              kind,
			Spec:              spec,
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
	tflog.Trace(ctx, "created a chalk_cloud_container_registry resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudContainerRegistryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
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
			"Error reading cloud container registry",
			"Could not read cloud container registry "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	setCloudContainerRegistryState(&data, response.Msg.GetContainerRegistry())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudContainerRegistryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Cloud container registries cannot be updated. They must be deleted and recreated.",
	)
}

func (r *CloudContainerRegistryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
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
			"Error deleting cloud container registry",
			"Could not delete cloud container registry "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}
	tflog.Trace(ctx, "deleted a chalk_cloud_container_registry resource")
}

func (r *CloudContainerRegistryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
