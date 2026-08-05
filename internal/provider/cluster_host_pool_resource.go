package provider

import (
	"context"
	"fmt"
	"strings"

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
	_ resource.Resource                   = &ClusterHostPoolResource{}
	_ resource.ResourceWithImportState    = &ClusterHostPoolResource{}
	_ resource.ResourceWithValidateConfig = &ClusterHostPoolResource{}
)

func NewClusterHostPoolResource() resource.Resource {
	return &ClusterHostPoolResource{}
}

type ClusterHostPoolResource struct {
	client *client.Manager
}

type ClusterHostPoolResourceModel struct {
	Id        types.String `tfsdk:"id"`
	ClusterId types.String `tfsdk:"cluster_id"`
	hostPoolSpecModel
}

func (r *ClusterHostPoolResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_host_pool"
}

func (r *ClusterHostPoolResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := hostPoolSchemaAttributes()
	attributes["cluster_id"] = schema.StringAttribute{
		MarkdownDescription: "The ID of the cluster this host pool belongs to.",
		Required:            true,
		PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a host pool scoped to a Chalk cluster. Host pools provide the hosts that Chalk Compute workloads run on.",
		Attributes:          attributes,
	}
}

func (r *ClusterHostPoolResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ClusterHostPoolResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data ClusterHostPoolResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validateHostPoolSpec(data.hostPoolSpecModel, &resp.Diagnostics)
}

func (r *ClusterHostPoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ClusterHostPoolResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	spec, diags := data.toProto()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	hostPoolClient := r.client.NewHostPoolClient(ctx)
	createResp, err := hostPoolClient.CreateClusterHostPool(ctx, connect.NewRequest(&serverv1.CreateClusterHostPoolRequest{
		ClusterId: data.ClusterId.ValueString(),
		Spec:      spec,
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating cluster host pool",
			fmt.Sprintf("Could not create cluster host pool: %s", err.Error()),
		)
		return
	}

	data.applyHostPool(createResp.Msg.GetHostPool())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterHostPoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ClusterHostPoolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hostPool, found, err := readHostPool(ctx, r.client, "", data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading cluster host pool",
			fmt.Sprintf("Could not read cluster host pool %s: %s", data.Id.ValueString(), err.Error()),
		)
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	data.applyHostPool(hostPool)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterHostPoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ClusterHostPoolResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	spec, diags := data.toProto()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	hostPoolClient := r.client.NewHostPoolClient(ctx)
	updateResp, err := hostPoolClient.UpdateClusterHostPool(ctx, connect.NewRequest(&serverv1.UpdateClusterHostPoolRequest{
		Id:         data.Id.ValueString(),
		Spec:       spec,
		UpdateMask: hostPoolUpdateMask(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating cluster host pool",
			fmt.Sprintf("Could not update cluster host pool %s: %s", data.Id.ValueString(), err.Error()),
		)
		return
	}

	data.applyHostPool(updateResp.Msg.GetHostPool())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterHostPoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ClusterHostPoolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hostPoolClient := r.client.NewHostPoolClient(ctx)
	_, err := hostPoolClient.DeleteClusterHostPool(ctx, connect.NewRequest(&serverv1.DeleteClusterHostPoolRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		resp.Diagnostics.AddError(
			"Error deleting cluster host pool",
			fmt.Sprintf("Could not delete cluster host pool %s: %s", data.Id.ValueString(), err.Error()),
		)
	}
}

func (r *ClusterHostPoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import ID in the format 'cluster_id/host_pool_id', got: %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (m *ClusterHostPoolResourceModel) applyHostPool(p *serverv1.HostPool) {
	m.Id = types.StringValue(p.GetId())
	m.hostPoolSpecModel = hostPoolSpecFromProto(p.GetSpec())
}
