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

var _ datasource.DataSource = &AWSCloudCredentialsDataSource{}

func NewAWSCloudCredentialsDataSource() datasource.DataSource {
	return &AWSCloudCredentialsDataSource{}
}

type AWSCloudCredentialsDataSource struct {
	client *client.Manager
}

type AWSCloudCredentialsDataSourceModel struct {
	Id                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	AWSAccountId         types.String `tfsdk:"aws_account_id"`
	AWSManagementRoleArn types.String `tfsdk:"aws_management_role_arn"`
	AWSRegion            types.String `tfsdk:"aws_region"`
	AWSExternalId        types.String `tfsdk:"aws_external_id"`
}

func (d *AWSCloudCredentialsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aws_cloud_credentials"
}

func (d *AWSCloudCredentialsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Chalk AWS cloud credentials object by ID. Returns the identifying account/region/role fields; deliberately omits docker_build_config and gcp_workload_identity (use the resource block if you need those nested configs).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud credentials identifier.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Cloud credentials display name.",
				Computed:            true,
			},
			"aws_account_id": schema.StringAttribute{
				MarkdownDescription: "AWS account ID.",
				Computed:            true,
			},
			"aws_management_role_arn": schema.StringAttribute{
				MarkdownDescription: "AWS management role ARN that Chalk assumes into the account.",
				Computed:            true,
			},
			"aws_region": schema.StringAttribute{
				MarkdownDescription: "AWS region.",
				Computed:            true,
			},
			"aws_external_id": schema.StringAttribute{
				MarkdownDescription: "AWS external ID used for STS role assumption (commonly the team ID).",
				Computed:            true,
			},
		},
	}
}

func (d *AWSCloudCredentialsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AWSCloudCredentialsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AWSCloudCredentialsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "read chalk_aws_cloud_credentials data source", map[string]any{"id": data.Id.ValueString()})

	cc := d.client.NewCloudAccountCredentialsClient(ctx)
	creds, err := cc.GetCloudCredentials(ctx, connect.NewRequest(&serverv1.GetCloudCredentialsRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading AWS Cloud Credentials",
			fmt.Sprintf("Could not read AWS cloud credentials %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	c := creds.Msg.Credentials
	data.Name = types.StringValue(c.Name)
	if c.Spec != nil && c.Spec.Config != nil {
		if cfg, ok := c.Spec.Config.(*serverv1.CloudConfig_Aws); ok && cfg.Aws != nil {
			data.AWSAccountId = types.StringValue(cfg.Aws.AccountId)
			data.AWSManagementRoleArn = types.StringValue(cfg.Aws.ManagementRoleArn)
			data.AWSRegion = types.StringValue(cfg.Aws.Region)
			if cfg.Aws.ExternalId != nil {
				data.AWSExternalId = types.StringValue(*cfg.Aws.ExternalId)
			} else {
				data.AWSExternalId = types.StringNull()
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
