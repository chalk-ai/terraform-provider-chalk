package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/client"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &KubernetesClusterDataSource{}

func NewKubernetesClusterDataSource() datasource.DataSource {
	return &KubernetesClusterDataSource{}
}

type KubernetesClusterDataSource struct {
	client *client.Manager
}

func (d *KubernetesClusterDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_cluster"
}

func (d *KubernetesClusterDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing Chalk Kubernetes cluster (BYOC) by ID. Companion to `chalk_managed_cluster` but covers the unmanaged-cluster case where the customer brings their own EKS/GKE/AKS cluster.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cluster identifier.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Cluster name.",
				Computed:            true,
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Cloud provider kind (e.g., 'EKS_STANDARD', 'EKS_AUTOPILOT', 'GKE_STANDARD', 'GKE_AUTOPILOT').",
				Computed:            true,
			},
			"cloud_credential_id": schema.StringAttribute{
				MarkdownDescription: "ID of the cloud credential associated with the cluster (may be null for fully self-hosted setups).",
				Computed:            true,
			},
			"dns_zone": schema.StringAttribute{
				MarkdownDescription: "DNS zone assigned to the cluster.",
				Computed:            true,
			},
			"team_id": schema.StringAttribute{
				MarkdownDescription: "Team ID that owns the cluster.",
				Computed:            true,
			},
		},
	}
}

func (d *KubernetesClusterDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Manager)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Manager, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *KubernetesClusterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data KubernetesClusterResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "read chalk_kubernetes_cluster data source", map[string]any{"id": data.Id.ValueString()})

	cc := d.client.NewCloudComponentsClient(ctx)
	cluster, err := cc.GetCloudComponentCluster(ctx, connect.NewRequest(&serverv1.GetCloudComponentClusterRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Kubernetes Cluster",
			fmt.Sprintf("Could not read kubernetes cluster %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	r := &KubernetesClusterResource{}
	r.updateModelFromProto(&data, cluster.Msg.Cluster)
	if cluster.Msg.Cluster != nil && cluster.Msg.Cluster.Spec == nil {
		data.Name = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
