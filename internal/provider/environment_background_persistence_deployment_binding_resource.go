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
	_ resource.Resource                = &EnvironmentBackgroundPersistenceDeploymentBindingResource{}
	_ resource.ResourceWithImportState = &EnvironmentBackgroundPersistenceDeploymentBindingResource{}
)

func NewEnvironmentBackgroundPersistenceDeploymentBindingResource() resource.Resource {
	return &EnvironmentBackgroundPersistenceDeploymentBindingResource{}
}

type EnvironmentBackgroundPersistenceDeploymentBindingResource struct {
	client *ClientManager
}

type EnvironmentBackgroundPersistenceDeploymentBindingResourceModel struct {
	EnvironmentID                     types.String `tfsdk:"environment_id"`
	BackgroundPersistenceDeploymentID types.String `tfsdk:"background_persistence_deployment_id"`
}

func (r *EnvironmentBackgroundPersistenceDeploymentBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_background_persistence_deployment_binding"
}

func (r *EnvironmentBackgroundPersistenceDeploymentBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a binding between a Chalk environment and a background persistence deployment.",
		Attributes: map[string]schema.Attribute{
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the environment to bind to the background persistence deployment.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"background_persistence_deployment_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the background persistence deployment to bind to the environment.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *EnvironmentBackgroundPersistenceDeploymentBindingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EnvironmentBackgroundPersistenceDeploymentBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EnvironmentBackgroundPersistenceDeploymentBindingResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	createRequest := &serverv1.CreateBindingEnvironmentBackgroundPersistenceDeploymentRequest{
		EnvironmentId:                     data.EnvironmentID.ValueString(),
		BackgroundPersistenceDeploymentId: data.BackgroundPersistenceDeploymentID.ValueString(),
	}

	_, err := cloudComponentsClient.CreateBindingEnvironmentBackgroundPersistenceDeployment(ctx, connect.NewRequest(createRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating environment background persistence deployment binding",
			fmt.Sprintf("Could not create environment background persistence deployment binding: %s", err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentBackgroundPersistenceDeploymentBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EnvironmentBackgroundPersistenceDeploymentBindingResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	getRequest := &serverv1.GetBindingEnvironmentBackgroundPersistenceDeploymentRequest{
		EnvironmentId: data.EnvironmentID.ValueString(),
	}

	response, err := cloudComponentsClient.GetBindingEnvironmentBackgroundPersistenceDeployment(ctx, connect.NewRequest(getRequest))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading environment background persistence deployment binding",
			fmt.Sprintf("Could not read environment background persistence deployment binding: %s", err.Error()),
		)
		return
	}

	data.EnvironmentID = types.StringValue(response.Msg.EnvironmentId)
	data.BackgroundPersistenceDeploymentID = types.StringValue(response.Msg.BackgroundPersistenceDeploymentId)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentBackgroundPersistenceDeploymentBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Environment background persistence deployment bindings cannot be updated. They must be deleted and recreated.",
	)
}

func (r *EnvironmentBackgroundPersistenceDeploymentBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EnvironmentBackgroundPersistenceDeploymentBindingResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	deleteRequest := &serverv1.DeleteBindingEnvironmentBackgroundPersistenceDeploymentRequest{
		EnvironmentId: data.EnvironmentID.ValueString(),
	}

	_, err := cloudComponentsClient.DeleteBindingEnvironmentBackgroundPersistenceDeployment(ctx, connect.NewRequest(deleteRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting environment background persistence deployment binding",
			fmt.Sprintf("Could not delete environment background persistence deployment binding: %s", err.Error()),
		)
		return
	}
}

func (r *EnvironmentBackgroundPersistenceDeploymentBindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("environment_id"), req, resp)
}
