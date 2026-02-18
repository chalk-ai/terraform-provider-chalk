package provider

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

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
	ClientID          types.String `tfsdk:"client_id"`
	ClientSecret      types.String `tfsdk:"client_secret"`
	JWT               types.String `tfsdk:"jwt"`
	JWTCommandProcess types.String `tfsdk:"jwt_command_process"`
	ApiServer         types.String `tfsdk:"api_server"`
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
			"jwt": schema.StringAttribute{
				MarkdownDescription: "JWT token for authentication. Can also be set via CHALK_JWT environment variable. If provided, this takes precedence over client_id/client_secret.",
				Optional:            true,
				Sensitive:           true,
			},
			"jwt_command_process": schema.StringAttribute{
				MarkdownDescription: "Shell command to execute to retrieve JWT token. Can also be set via CHALK_JWT_COMMAND_PROCESS environment variable. The command's stdout will be used as the JWT token.",
				Optional:            true,
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

	if data.JWT.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("jwt"),
			"Unknown Chalk JWT",
			"The provider cannot create the Chalk API client as there is an unknown configuration value for the Chalk JWT. "+
				"Either target apply the source of the value first or set the value statically in the configuration.",
		)
	}

	if data.JWTCommandProcess.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("jwt_command_process"),
			"Unknown Chalk JWT Command Process",
			"The provider cannot create the Chalk API client as there is an unknown configuration value for the Chalk JWT command process. "+
				"Either target apply the source of the value first or set the value statically in the configuration.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	clientID := data.ClientID.ValueString()
	clientSecret := data.ClientSecret.ValueString()
	jwt := data.JWT.ValueString()
	jwtCommandProcess := data.JWTCommandProcess.ValueString()
	apiServer := data.ApiServer.ValueString()

	// Check environment variables if not set in config
	if jwt == "" {
		jwt = os.Getenv("CHALK_JWT")
	}
	if jwtCommandProcess == "" {
		jwtCommandProcess = os.Getenv("CHALK_JWT_COMMAND_PROCESS")
	}
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

	// Execute JWT command if provided and JWT is not directly set
	if jwt == "" && jwtCommandProcess != "" {
		cmd := exec.CommandContext(ctx, "sh", "-c", jwtCommandProcess)
		output, err := cmd.Output()
		if err != nil {
			resp.Diagnostics.AddError(
				"JWT Command Execution Failed",
				fmt.Sprintf("Failed to execute JWT command process: %v", err),
			)
			return
		}
		jwt = strings.TrimSpace(string(output))
	}

	// Validate authentication: either JWT or client_id/client_secret must be provided
	hasJWT := jwt != ""
	hasClientCredentials := clientID != "" && clientSecret != ""

	if !hasJWT && !hasClientCredentials {
		resp.Diagnostics.AddError(
			"Missing Authentication",
			"Either JWT (jwt or jwt_command_process) or Client Credentials (client_id and client_secret) must be configured. "+
				"JWT can be set via the 'jwt' field, CHALK_JWT environment variable, or by providing a shell command via 'jwt_command_process' or CHALK_JWT_COMMAND_PROCESS. "+
				"Client Credentials can be set via 'client_id'/'client_secret' fields or CHALK_CLIENT_ID/CHALK_CLIENT_SECRET environment variables.",
		)
		return
	}

	// Warn if both authentication methods are provided (JWT takes precedence)
	if hasJWT && hasClientCredentials {
		resp.Diagnostics.AddWarning(
			"Multiple Authentication Methods",
			"Both JWT and Client Credentials are configured. JWT authentication will be used and Client Credentials will be ignored.",
		)
	}

	client := &ChalkClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		JWT:          jwt,
		ApiServer:    apiServer,
	}

	// Create ClientManager to handle all gRPC client creation
	clientManager := NewClientManager(client)

	resp.DataSourceData = clientManager
	resp.ResourceData = clientManager
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
		NewManagedClusterResource,
		NewAWSCloudCredentialsResource,
		NewGCPCloudCredentialsResource,
		NewAzureCloudCredentialsResource,
		NewClusterGatewayBindingResource,
		NewPrivateGatewayBindingResource,
		NewClusterBackgroundPersistenceDeploymentBindingResource,
		NewTelemetryResource,
		NewTelemetryBindingResource,
		NewManagedAWSVPCResource,
		NewManagedGCPVPCResource,
		NewManagedAzureVPCResource,
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
	JWT          string
	ApiServer    string
}

func (c *ChalkClient) String() string {
	return fmt.Sprintf("ChalkClient{ApiServer: %s}", c.ApiServer)
}
