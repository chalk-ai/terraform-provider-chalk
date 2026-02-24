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
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &ManagedClusterResource{}
var _ resource.ResourceWithImportState = &ManagedClusterResource{}

func NewManagedClusterResource() resource.Resource {
	return &ManagedClusterResource{}
}

type ManagedClusterResource struct {
	client *ClientManager
}

type ManagedClusterResourceModel struct {
	Id                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Kind              types.String `tfsdk:"kind"`
	Designator        types.String `tfsdk:"designator"`
	CloudCredentialId types.String `tfsdk:"cloud_credential_id"`
	VpcId             types.String `tfsdk:"vpc_id"`
}

func (r *ManagedClusterResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_cluster"
}

func (r *ManagedClusterResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk managed Kubernetes cluster resource. Creates a fully managed cluster using the provided cloud credentials.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cluster identifier",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Cluster name",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Cloud provider kind (e.g., 'EKS_STANDARD', 'EKS_AUTOPILOT', 'GKE_STANDARD', 'GKE_AUTOPILOT')",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		"designator": schema.StringAttribute{
			MarkdownDescription: "Cluster designator",
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
			"cloud_credential_id": schema.StringAttribute{
				MarkdownDescription: "ID of the cloud credential to use for the managed cluster",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"vpc_id": schema.StringAttribute{
				MarkdownDescription: "ID of the VPC to use for the cluster",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *ManagedClusterResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ManagedClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ManagedClusterResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create cloud components client
	cc := r.client.NewCloudComponentsClient(ctx)

	credentialId := data.CloudCredentialId.ValueString()
	vpcId := data.VpcId.ValueString()

	createReq := &serverv1.CreateCloudComponentClusterRequest{
		Cluster: &serverv1.CloudComponentClusterRequest{
			Managed:           true,
			CloudCredentialId: &credentialId,
			VpcId:             &vpcId,
		},
	}

	cluster, err := cc.CreateCloudComponentCluster(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Managed Cluster",
			fmt.Sprintf("Could not create managed cluster: %v", err),
		)
		return
	}

	// Update with created values
	r.updateModelFromProto(&data, cluster.Msg.Cluster)

	tflog.Trace(ctx, "created a chalk_managed_cluster resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ManagedClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ManagedClusterResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create cloud components client
	cc := r.client.NewCloudComponentsClient(ctx)

	cluster, err := cc.GetCloudComponentCluster(ctx, connect.NewRequest(&serverv1.GetCloudComponentClusterRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Managed Cluster",
			fmt.Sprintf("Could not read managed cluster %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Update the model with the fetched data
	r.updateModelFromProto(&data, cluster.Msg.Cluster)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ManagedClusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Since cloud_credential_id has RequiresReplace, updates should only happen
	// for computed fields which we don't modify. Just refresh state from server.
	var data ManagedClusterResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create cloud components client
	cc := r.client.NewCloudComponentsClient(ctx)

	cluster, err := cc.GetCloudComponentCluster(ctx, connect.NewRequest(&serverv1.GetCloudComponentClusterRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Managed Cluster",
			fmt.Sprintf("Could not read managed cluster %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Update the model with the fetched data
	r.updateModelFromProto(&data, cluster.Msg.Cluster)

	tflog.Trace(ctx, "updated chalk_managed_cluster resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ManagedClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ManagedClusterResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create cloud components client
	cc := r.client.NewCloudComponentsClient(ctx)

	deleteReq := &serverv1.DeleteCloudComponentClusterRequest{
		Id: data.Id.ValueString(),
	}

	_, err := cc.DeleteCloudComponentCluster(ctx, connect.NewRequest(deleteReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Managed Cluster",
			fmt.Sprintf("Could not delete managed cluster %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "deleted chalk_managed_cluster resource")
}

func (r *ManagedClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ManagedClusterResource) updateModelFromProto(model *ManagedClusterResourceModel, cluster *serverv1.CloudComponentClusterResponse) {
	model.Id = types.StringValue(cluster.Id)
	model.Name = types.StringValue(cluster.Spec.Name)
	model.Kind = types.StringValue(cluster.Kind)

	if cluster.Designator != nil {
		model.Designator = types.StringValue(*cluster.Designator)
	} else {
		model.Designator = types.StringNull()
	}

	if cluster.CloudCredentialId != nil {
		model.CloudCredentialId = types.StringValue(*cluster.CloudCredentialId)
	}

	if cluster.VpcId != nil {
		model.VpcId = types.StringValue(*cluster.VpcId)
	} else {
		model.VpcId = types.StringNull()
	}
}
