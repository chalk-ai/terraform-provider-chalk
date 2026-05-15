package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/client"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &ManagedAWSVPCDataSource{}

func NewManagedAWSVPCDataSource() datasource.DataSource {
	return &ManagedAWSVPCDataSource{}
}

type ManagedAWSVPCDataSource struct {
	client *client.Manager
}

func (d *ManagedAWSVPCDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_aws_vpc"
}

func (d *ManagedAWSVPCDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Chalk managed AWS VPC by ID. Use this data source to reference an existing VPC (e.g. its `designator`, subnets, or CIDR block) from a different Terraform root module.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "VPC identifier.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "VPC name.",
				Computed:            true,
			},
			"designator": schema.StringAttribute{
				MarkdownDescription: "VPC designator (suffix used by chalkadmin push commands).",
				Computed:            true,
			},
			"cloud_credential_id": schema.StringAttribute{
				MarkdownDescription: "ID of the cloud credential used for the managed VPC.",
				Computed:            true,
			},
			"cidr_block": schema.StringAttribute{
				MarkdownDescription: "The primary IPv4 CIDR block of the VPC.",
				Computed:            true,
			},
			"additional_cidr_blocks": schema.ListAttribute{
				MarkdownDescription: "Additional IPv4 CIDR blocks of the VPC.",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"subnets": schema.ListNestedAttribute{
				MarkdownDescription: "Subnets configured on the VPC.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":               schema.StringAttribute{Computed: true},
						"private_cidr_block": schema.StringAttribute{Computed: true},
						"public_cidr_block":  schema.StringAttribute{Computed: true},
						"availability_zone":  schema.StringAttribute{Computed: true},
					},
				},
			},
			"additional_public_routes": schema.ListNestedAttribute{
				MarkdownDescription: "Additional public-subnet routes.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":                   schema.StringAttribute{Computed: true},
						"destination_cidr_block": schema.StringAttribute{Computed: true},
						"peer_id":                schema.StringAttribute{Computed: true},
					},
				},
			},
			"additional_private_routes": schema.ListNestedAttribute{
				MarkdownDescription: "Additional private-subnet routes.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":                   schema.StringAttribute{Computed: true},
						"destination_cidr_block": schema.StringAttribute{Computed: true},
						"peer_id":                schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *ManagedAWSVPCDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ManagedAWSVPCDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ManagedAWSVPCResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "read chalk_managed_aws_vpc data source", map[string]any{"id": data.Id.ValueString()})

	cc := d.client.NewCloudComponentsClient(ctx)
	vpc, err := cc.GetCloudComponentVpc(ctx, connect.NewRequest(&serverv1.GetCloudComponentVpcRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Managed VPC",
			fmt.Sprintf("Could not read managed VPC %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	resp.Diagnostics.Append(d.populateModel(ctx, &data, vpc.Msg.Vpc)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *ManagedAWSVPCDataSource) populateModel(ctx context.Context, model *ManagedAWSVPCResourceModel, vpc *serverv1.CloudComponentVpcResponse) diag.Diagnostics {
	r := &ManagedAWSVPCResource{}
	return r.updateModelFromProto(ctx, model, vpc)
}
