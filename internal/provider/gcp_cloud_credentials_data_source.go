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

var _ datasource.DataSource = &GCPCloudCredentialsDataSource{}

func NewGCPCloudCredentialsDataSource() datasource.DataSource {
	return &GCPCloudCredentialsDataSource{}
}

type GCPCloudCredentialsDataSource struct {
	client *client.Manager
}

type GCPCloudCredentialsDataSourceModel struct {
	Id                          types.String `tfsdk:"id"`
	Name                        types.String `tfsdk:"name"`
	GCPProjectId                types.String `tfsdk:"gcp_project_id"`
	GCPRegion                   types.String `tfsdk:"gcp_region"`
	GCPManagementServiceAccount types.String `tfsdk:"gcp_management_service_account"`
}

func (d *GCPCloudCredentialsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gcp_cloud_credentials"
}

func (d *GCPCloudCredentialsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Chalk GCP cloud credentials object by ID. Surfaces project/region/service-account identity; deliberately omits nested docker_build_config / secretmanager_config / region_config blocks.",
		Attributes: map[string]schema.Attribute{
			"id":                             schema.StringAttribute{MarkdownDescription: "Cloud credentials identifier.", Required: true},
			"name":                           schema.StringAttribute{Computed: true},
			"gcp_project_id":                 schema.StringAttribute{Computed: true},
			"gcp_region":                     schema.StringAttribute{Computed: true},
			"gcp_management_service_account": schema.StringAttribute{Computed: true},
		},
	}
}

func (d *GCPCloudCredentialsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *GCPCloudCredentialsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data GCPCloudCredentialsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "read chalk_gcp_cloud_credentials data source", map[string]any{"id": data.Id.ValueString()})

	cc := d.client.NewCloudAccountCredentialsClient(ctx)
	creds, err := cc.GetCloudCredentials(ctx, connect.NewRequest(&serverv1.GetCloudCredentialsRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading GCP Cloud Credentials",
			fmt.Sprintf("Could not read GCP cloud credentials %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	c := creds.Msg.Credentials
	data.Name = types.StringValue(c.Name)
	if c.Spec != nil && c.Spec.Config != nil {
		if cfg, ok := c.Spec.Config.(*serverv1.CloudConfig_Gcp); ok && cfg.Gcp != nil {
			data.GCPProjectId = types.StringValue(cfg.Gcp.ProjectId)
			data.GCPRegion = types.StringValue(cfg.Gcp.Region)
			if cfg.Gcp.ManagementServiceAccount != nil {
				data.GCPManagementServiceAccount = types.StringValue(*cfg.Gcp.ManagementServiceAccount)
			} else {
				data.GCPManagementServiceAccount = types.StringNull()
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
