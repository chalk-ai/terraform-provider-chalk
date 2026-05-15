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

var _ datasource.DataSource = &ManagedClusterDataSource{}

func NewManagedClusterDataSource() datasource.DataSource {
	return &ManagedClusterDataSource{}
}

type ManagedClusterDataSource struct {
	client *client.Manager
}

type ManagedClusterDataSourceModel struct {
	Id                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Kind              types.String `tfsdk:"kind"`
	Designator        types.String `tfsdk:"designator"`
	CloudCredentialId types.String `tfsdk:"cloud_credential_id"`
	VpcId             types.String `tfsdk:"vpc_id"`
}

func (d *ManagedClusterDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_cluster"
}

func (d *ManagedClusterDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Chalk managed Kubernetes cluster by ID. Use this data source to reference an existing cluster (e.g. its `designator`) from a different Terraform root module.",
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
			"designator": schema.StringAttribute{
				MarkdownDescription: "Cluster designator (the suffix Chalk uses in derived resource names).",
				Computed:            true,
			},
			"cloud_credential_id": schema.StringAttribute{
				MarkdownDescription: "ID of the cloud credential used for the managed cluster.",
				Computed:            true,
			},
			"vpc_id": schema.StringAttribute{
				MarkdownDescription: "ID of the VPC the cluster lives in.",
				Computed:            true,
			},
		},
	}
}

func (d *ManagedClusterDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ManagedClusterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ManagedClusterDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "read chalk_managed_cluster data source", map[string]any{"id": data.Id.ValueString()})

	cc := d.client.NewCloudComponentsClient(ctx)
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

	c := cluster.Msg.Cluster
	data.Id = types.StringValue(c.Id)
	data.Kind = types.StringValue(c.Kind)
	if c.Spec != nil {
		data.Name = types.StringValue(c.Spec.Name)
	} else {
		data.Name = types.StringNull()
	}
	if c.Designator != nil {
		data.Designator = types.StringValue(*c.Designator)
	} else {
		data.Designator = types.StringNull()
	}
	if c.CloudCredentialId != nil {
		data.CloudCredentialId = types.StringValue(*c.CloudCredentialId)
	} else {
		data.CloudCredentialId = types.StringNull()
	}
	if c.VpcId != nil {
		data.VpcId = types.StringValue(*c.VpcId)
	} else {
		data.VpcId = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
