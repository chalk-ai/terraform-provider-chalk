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

var _ resource.Resource = &KubernetesClusterResource{}
var _ resource.ResourceWithImportState = &KubernetesClusterResource{}

func NewKubernetesClusterResource() resource.Resource {
	return &KubernetesClusterResource{}
}

type KubernetesClusterResource struct {
	client *ClientManager
}

type KubernetesClusterResourceModel struct {
	Id                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Kind              types.String `tfsdk:"kind"`
	CloudCredentialId types.String `tfsdk:"cloud_credential_id"`
	DnsZone           types.String `tfsdk:"dns_zone"`
	TeamId            types.String `tfsdk:"team_id"`
}

func (r *KubernetesClusterResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_cluster"
}

func (r *KubernetesClusterResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk Kubernetes cluster resource (unmanaged)",

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
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Cloud provider kind (e.g., 'EKS_STANDARD', 'EKS_AUTOPILOT', 'GKE_STANDARD', 'GKE_AUTOPILOT')",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cloud_credential_id": schema.StringAttribute{
				MarkdownDescription: "ID of the cloud credential to use for the cluster",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"dns_zone": schema.StringAttribute{
				MarkdownDescription: "DNS zone for the cluster",
				Optional:            true,
			},
			"team_id": schema.StringAttribute{
				MarkdownDescription: "Team ID",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *KubernetesClusterResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *KubernetesClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data KubernetesClusterResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create cloud components client
	cc := r.client.NewCloudComponentsClient(ctx)

	createReq := &serverv1.CreateCloudComponentClusterRequest{
		Cluster: &serverv1.CloudComponentClusterRequest{
			Kind: data.Kind.ValueString(),
			Spec: &serverv1.CloudComponentCluster{
				Name: data.Name.ValueString(),
			},
			Managed: false, // Always unmanaged
		},
	}

	if !data.CloudCredentialId.IsNull() {
		credentialId := data.CloudCredentialId.ValueString()
		createReq.Cluster.CloudCredentialId = &credentialId
	}

	if !data.DnsZone.IsNull() {
		dnsZone := data.DnsZone.ValueString()
		createReq.Cluster.Spec.DnsZone = &dnsZone
	}

	cluster, err := cc.CreateCloudComponentCluster(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Kubernetes Cluster",
			fmt.Sprintf("Could not create cluster: %v", err),
		)
		return
	}

	// Update with created values
	r.updateModelFromProto(&data, cluster.Msg.Cluster)

	tflog.Trace(ctx, "created a chalk_kubernetes_cluster resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KubernetesClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data KubernetesClusterResourceModel

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
			"Error Reading Kubernetes Cluster",
			fmt.Sprintf("Could not read cluster %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Update the model with the fetched data
	r.updateModelFromProto(&data, cluster.Msg.Cluster)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KubernetesClusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data KubernetesClusterResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create cloud components client
	cc := r.client.NewCloudComponentsClient(ctx)

	updateReq := &serverv1.UpdateCloudComponentClusterRequest{
		Id: data.Id.ValueString(),
		Cluster: &serverv1.CloudComponentClusterRequest{
			Kind: data.Kind.ValueString(),
			Spec: &serverv1.CloudComponentCluster{
				Name: data.Name.ValueString(),
			},
			Managed: false, // Always unmanaged
		},
	}

	if !data.CloudCredentialId.IsNull() {
		credentialId := data.CloudCredentialId.ValueString()
		updateReq.Cluster.CloudCredentialId = &credentialId
	}

	if !data.DnsZone.IsNull() {
		dnsZone := data.DnsZone.ValueString()
		updateReq.Cluster.Spec.DnsZone = &dnsZone
	}

	cluster, err := cc.UpdateCloudComponentCluster(ctx, connect.NewRequest(updateReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Kubernetes Cluster",
			fmt.Sprintf("Could not update cluster: %v", err),
		)
		return
	}

	// Update the model with the updated data
	r.updateModelFromProto(&data, cluster.Msg.Cluster)

	tflog.Trace(ctx, "updated a chalk_kubernetes_cluster resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KubernetesClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No-op: This resource is unmanaged, so we only remove it from Terraform state
	// The actual Kubernetes cluster is not deleted
	tflog.Trace(ctx, "removing chalk_kubernetes_cluster from terraform state (unmanaged resource - cluster not deleted)")
}

func (r *KubernetesClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *KubernetesClusterResource) updateModelFromProto(model *KubernetesClusterResourceModel, cluster *serverv1.CloudComponentClusterResponse) {
	model.Id = types.StringValue(cluster.Id)
	model.Name = types.StringValue(cluster.Spec.Name)
	model.Kind = types.StringValue(cluster.Kind)
	model.TeamId = types.StringValue(cluster.TeamId)

	if cluster.CloudCredentialId != nil {
		model.CloudCredentialId = types.StringValue(*cluster.CloudCredentialId)
	} else {
		model.CloudCredentialId = types.StringNull()
	}

	if cluster.Spec.DnsZone != nil {
		model.DnsZone = types.StringValue(*cluster.Spec.DnsZone)
	} else {
		model.DnsZone = types.StringNull()
	}
}
