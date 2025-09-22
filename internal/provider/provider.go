package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/path"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &ChalkProvider{}

type ChalkProvider struct {
	version string
}

type ChalkProviderModel struct {
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	ApiServer    types.String `tfsdk:"api_server"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ChalkProvider{
			version: version,
		}
	}
}

func (p *ChalkProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "chalk"
	resp.Version = p.version
}

func (p *ChalkProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"client_id": schema.StringAttribute{
				MarkdownDescription: "Chalk client ID for authentication. Can also be set via CHALK_CLIENT_ID environment variable.",
				Optional:            true,
			},
			"client_secret": schema.StringAttribute{
				MarkdownDescription: "Chalk client secret for authentication. Can also be set via CHALK_CLIENT_SECRET environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"api_server": schema.StringAttribute{
				MarkdownDescription: "Chalk API server URL. Can also be set via CHALK_API_SERVER environment variable. Defaults to https://api.chalk.ai",
				Optional:            true,
			},
		},
	}
}

func (p *ChalkProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data ChalkProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.ClientID.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_id"),
			"Unknown Chalk Client ID",
			"The provider cannot create the Chalk API client as there is an unknown configuration value for the Chalk client ID. "+
				"Either target apply the source of the value first or set the value statically in the configuration.",
		)
	}

	if data.ClientSecret.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_secret"),
			"Unknown Chalk Client Secret",
			"The provider cannot create the Chalk API client as there is an unknown configuration value for the Chalk client secret. "+
				"Either target apply the source of the value first or set the value statically in the configuration.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	clientID := data.ClientID.ValueString()
	clientSecret := data.ClientSecret.ValueString()
	apiServer := data.ApiServer.ValueString()

	// Check environment variables if not set in config
	if clientID == "" {
		clientID = os.Getenv("CHALK_CLIENT_ID")
	}
	if clientSecret == "" {
		clientSecret = os.Getenv("CHALK_CLIENT_SECRET")
	}
	if apiServer == "" {
		apiServer = os.Getenv("CHALK_API_SERVER")
		if apiServer == "" {
			apiServer = "https://api.chalk.ai"
		}
	}

	// Validate required fields
	if clientID == "" {
		resp.Diagnostics.AddError(
			"Missing Client ID",
			"Client ID must be configured either via the provider block or the CHALK_CLIENT_ID environment variable.",
		)
	}
	if clientSecret == "" {
		resp.Diagnostics.AddError(
			"Missing Client Secret",
			"Client Secret must be configured either via the provider block or the CHALK_CLIENT_SECRET environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	client := &ChalkClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		ApiServer:    apiServer,
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *ChalkProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewEnvironmentResource,
		NewProjectResource,
		NewServiceTokenResource,
		NewClusterGatewayResource,
		NewClusterBackgroundPersistenceResource,
		NewClusterTimescaleResource,
		NewKubernetesClusterResource,
		NewCloudCredentialsResource,
		NewClusterGatewayBindingResource,
		NewClusterBackgroundPersistenceDeploymentBindingResource,
		NewTelemetryResource,
		NewTelemetryBindingResource,
	}
}

func (p *ChalkProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewEnvironmentDataSource,
	}
}

type ChalkClient struct {
	ClientID     string
	ClientSecret string
	ApiServer    string
}

func (c *ChalkClient) String() string {
	return fmt.Sprintf("ChalkClient{ApiServer: %s}", c.ApiServer)
}
