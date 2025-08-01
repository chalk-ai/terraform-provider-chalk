package provider

import (
	"context"
	"fmt"
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
	ApiToken types.String `tfsdk:"api_token"`
	ApiUrl   types.String `tfsdk:"api_url"`
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
			"api_token": schema.StringAttribute{
				MarkdownDescription: "Chalk API token for authentication",
				Optional:            true,
				Sensitive:           true,
			},
			"api_url": schema.StringAttribute{
				MarkdownDescription: "Chalk API URL",
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

	if data.ApiToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Unknown Chalk API Token",
			"The provider cannot create the Chalk API client as there is an unknown configuration value for the Chalk API token. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the CHALK_API_TOKEN environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	apiToken := data.ApiToken.ValueString()
	apiUrl := data.ApiUrl.ValueString()

	if apiUrl == "" {
		apiUrl = "https://api.chalk.ai"
	}

	client := &ChalkClient{
		ApiToken: apiToken,
		ApiUrl:   apiUrl,
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *ChalkProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

func (p *ChalkProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewEnvironmentDataSource,
	}
}

type ChalkClient struct {
	ApiToken string
	ApiUrl   string
}

func (c *ChalkClient) String() string {
	return fmt.Sprintf("ChalkClient{ApiUrl: %s}", c.ApiUrl)
}
