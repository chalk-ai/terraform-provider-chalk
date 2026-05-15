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

var _ datasource.DataSource = &AzureCloudCredentialsDataSource{}

func NewAzureCloudCredentialsDataSource() datasource.DataSource {
	return &AzureCloudCredentialsDataSource{}
}

type AzureCloudCredentialsDataSource struct {
	client *client.Manager
}

type AzureCloudCredentialsDataSourceModel struct {
	Id             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	SubscriptionId types.String `tfsdk:"subscription_id"`
	TenantId       types.String `tfsdk:"tenant_id"`
	Region         types.String `tfsdk:"region"`
	ResourceGroup  types.String `tfsdk:"resource_group"`
}

func (d *AzureCloudCredentialsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_azure_cloud_credentials"
}

func (d *AzureCloudCredentialsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Chalk Azure cloud credentials object by ID. Surfaces subscription/tenant/region/resource-group identity; deliberately omits nested container_registry_config and key_vault_config blocks (which may reference sensitive vault material).",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{MarkdownDescription: "Cloud credentials identifier.", Required: true},
			"name":            schema.StringAttribute{Computed: true},
			"subscription_id": schema.StringAttribute{Computed: true},
			"tenant_id":       schema.StringAttribute{Computed: true},
			"region":          schema.StringAttribute{Computed: true},
			"resource_group":  schema.StringAttribute{Computed: true},
		},
	}
}

func (d *AzureCloudCredentialsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AzureCloudCredentialsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AzureCloudCredentialsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "read chalk_azure_cloud_credentials data source", map[string]any{"id": data.Id.ValueString()})

	cc := d.client.NewCloudAccountCredentialsClient(ctx)
	creds, err := cc.GetCloudCredentials(ctx, connect.NewRequest(&serverv1.GetCloudCredentialsRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Azure Cloud Credentials",
			fmt.Sprintf("Could not read Azure cloud credentials %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	c := creds.Msg.Credentials
	data.Name = types.StringValue(c.Name)
	if c.Spec != nil && c.Spec.Config != nil {
		if cfg, ok := c.Spec.Config.(*serverv1.CloudConfig_Azure); ok && cfg.Azure != nil {
			data.SubscriptionId = types.StringValue(cfg.Azure.SubscriptionId)
			data.TenantId = types.StringValue(cfg.Azure.TenantId)
			data.Region = types.StringValue(cfg.Azure.Region)
			data.ResourceGroup = types.StringValue(cfg.Azure.ResourceGroup)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
