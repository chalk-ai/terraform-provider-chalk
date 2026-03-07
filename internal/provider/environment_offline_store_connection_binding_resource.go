package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &EnvironmentOfflineStoreConnectionBindingResource{}
	_ resource.ResourceWithImportState = &EnvironmentOfflineStoreConnectionBindingResource{}
)

func NewEnvironmentOfflineStoreConnectionBindingResource() resource.Resource {
	return &EnvironmentOfflineStoreConnectionBindingResource{}
}

type EnvironmentOfflineStoreConnectionBindingResource struct {
	client *ClientManager
}

type EnvironmentOfflineStoreConnectionBindingResourceModel struct {
	EnvironmentId            types.String `tfsdk:"environment_id"`
	OfflineStoreConnectionId types.String `tfsdk:"offline_store_connection_id"`
}

func (r *EnvironmentOfflineStoreConnectionBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_offline_store_connection_binding"
}

func (r *EnvironmentOfflineStoreConnectionBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a binding between a Chalk environment and an offline store connection.",
		Attributes: map[string]schema.Attribute{
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the environment to bind to the offline store connection.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"offline_store_connection_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the offline store connection to bind to the environment.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *EnvironmentOfflineStoreConnectionBindingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EnvironmentOfflineStoreConnectionBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EnvironmentOfflineStoreConnectionBindingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	osc := r.client.NewOfflineStoreConnectionClient(ctx, data.EnvironmentId.ValueString())

	_, err := osc.CreateBindingEnvironmentOfflineStoreConnection(ctx, connect.NewRequest(&serverv1.CreateBindingEnvironmentOfflineStoreConnectionRequest{
		EnvironmentId:            data.EnvironmentId.ValueString(),
		OfflineStoreConnectionId: data.OfflineStoreConnectionId.ValueString(),
		Name:                     "default",
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating environment offline store connection binding",
			fmt.Sprintf("Could not create environment offline store connection binding: %s", err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentOfflineStoreConnectionBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EnvironmentOfflineStoreConnectionBindingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	osc := r.client.NewOfflineStoreConnectionClient(ctx, data.EnvironmentId.ValueString())

	getResp, err := osc.GetBindingEnvironmentOfflineStoreConnection(ctx, connect.NewRequest(&serverv1.GetBindingEnvironmentOfflineStoreConnectionRequest{
		EnvironmentId: data.EnvironmentId.ValueString(),
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading environment offline store connection binding",
			fmt.Sprintf("Could not read environment offline store connection binding: %s", err.Error()),
		)
		return
	}

	data.EnvironmentId = types.StringValue(getResp.Msg.EnvironmentId)
	data.OfflineStoreConnectionId = types.StringValue(getResp.Msg.OfflineStoreConnectionId)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentOfflineStoreConnectionBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Environment offline store connection bindings cannot be updated. They must be deleted and recreated.",
	)
}

func (r *EnvironmentOfflineStoreConnectionBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EnvironmentOfflineStoreConnectionBindingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	osc := r.client.NewOfflineStoreConnectionClient(ctx, data.EnvironmentId.ValueString())

	_, err := osc.DeleteBindingEnvironmentOfflineStoreConnection(ctx, connect.NewRequest(&serverv1.DeleteBindingEnvironmentOfflineStoreConnectionRequest{
		EnvironmentId:            data.EnvironmentId.ValueString(),
		OfflineStoreConnectionId: data.OfflineStoreConnectionId.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting environment offline store connection binding",
			fmt.Sprintf("Could not delete environment offline store connection binding: %s", err.Error()),
		)
	}
}

func (r *EnvironmentOfflineStoreConnectionBindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("environment_id"), req, resp)
}
