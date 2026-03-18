package provider

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/chalk-ai/terraform-provider-chalk/client"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &DatasourceDatabricksResource{}
var _ resource.ResourceWithImportState = &DatasourceDatabricksResource{}
var _ resource.ResourceWithValidateConfig = &DatasourceDatabricksResource{}

// databricksEnvVarToField maps Databricks env var names to their Terraform attribute names.
var databricksEnvVarToField = map[string]string{
	"DATABRICKS_HOST":          "host",
	"DATABRICKS_HTTP_PATH":     "http_path",
	"DATABRICKS_TOKEN":         "access_token",
	"DATABRICKS_CLIENT_ID":     "client_id",
	"DATABRICKS_CLIENT_SECRET": "client_secret",
	"DATABRICKS_DATABASE":      "database",
	"DATABRICKS_PORT":          "port",
}

func NewDatasourceDatabricksResource() resource.Resource {
	return &DatasourceDatabricksResource{}
}

type DatasourceDatabricksResource struct {
	client *client.Manager
}

type DatasourceDatabricksResourceModel struct {
	Id            types.String                     `tfsdk:"id"`
	Name          types.String                     `tfsdk:"name"`
	EnvironmentId types.String                     `tfsdk:"environment_id"`
	Host          *DatasourceConfigValue           `tfsdk:"host"`
	HttpPath      *DatasourceConfigValue           `tfsdk:"http_path"`
	AccessToken   *DatasourceConfigValue           `tfsdk:"access_token"`
	ClientId      *DatasourceConfigValue           `tfsdk:"client_id"`
	ClientSecret  *DatasourceConfigValue           `tfsdk:"client_secret"`
	Database      *DatasourceConfigValue           `tfsdk:"database"`
	Port          *DatasourceConfigValue           `tfsdk:"port"`
	Config        map[string]DatasourceConfigValue `tfsdk:"config"`
}

// buildConfig merges the named fields and extra config into a flat map for the API.
// Only non-nil fields are included in the config.
func (data *DatasourceDatabricksResourceModel) buildConfig() map[string]DatasourceConfigValue {
	config := make(map[string]DatasourceConfigValue)

	if data.Host != nil {
		config["DATABRICKS_HOST"] = *data.Host
	}
	if data.HttpPath != nil {
		config["DATABRICKS_HTTP_PATH"] = *data.HttpPath
	}
	if data.AccessToken != nil {
		config["DATABRICKS_TOKEN"] = *data.AccessToken
	}
	if data.ClientId != nil {
		config["DATABRICKS_CLIENT_ID"] = *data.ClientId
	}
	if data.ClientSecret != nil {
		config["DATABRICKS_CLIENT_SECRET"] = *data.ClientSecret
	}
	if data.Database != nil {
		config["DATABRICKS_DATABASE"] = *data.Database
	}
	if data.Port != nil {
		config["DATABRICKS_PORT"] = *data.Port
	}

	maps.Copy(config, data.Config)
	return config
}

// distributeConfig populates the named fields and extra config from a refreshed flat map.
func (data *DatasourceDatabricksResourceModel) distributeConfig(refreshed map[string]DatasourceConfigValue) {
	if v, ok := refreshed["DATABRICKS_HOST"]; ok {
		data.Host = &v
	}
	if v, ok := refreshed["DATABRICKS_HTTP_PATH"]; ok {
		data.HttpPath = &v
	}
	if v, ok := refreshed["DATABRICKS_TOKEN"]; ok {
		data.AccessToken = &v
	}
	if v, ok := refreshed["DATABRICKS_CLIENT_ID"]; ok {
		data.ClientId = &v
	}
	if v, ok := refreshed["DATABRICKS_CLIENT_SECRET"]; ok {
		data.ClientSecret = &v
	}
	if v, ok := refreshed["DATABRICKS_DATABASE"]; ok {
		data.Database = &v
	}
	if v, ok := refreshed["DATABRICKS_PORT"]; ok {
		data.Port = &v
	}

	extra := make(map[string]DatasourceConfigValue)
	for k, v := range refreshed {
		if _, isNamed := databricksEnvVarToField[k]; !isNamed {
			extra[k] = v
		}
	}
	if len(extra) > 0 {
		data.Config = extra
	} else {
		data.Config = nil
	}
}

func (r *DatasourceDatabricksResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_datasource_databricks"
}

func (r *DatasourceDatabricksResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	nestedAttrs := configValueAttributes()
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Chalk Databricks datasource integration.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the datasource integration.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the datasource integration.",
				Required:            true,
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The environment ID that this datasource is scoped to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"host": schema.SingleNestedAttribute{
				MarkdownDescription: "The Databricks server hostname (`DATABRICKS_HOST`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"http_path": schema.SingleNestedAttribute{
				MarkdownDescription: "The HTTP path for Databricks SQL endpoint (`DATABRICKS_HTTP_PATH`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"access_token": schema.SingleNestedAttribute{
				MarkdownDescription: "The Databricks access token (`DATABRICKS_TOKEN`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"client_id": schema.SingleNestedAttribute{
				MarkdownDescription: "The OAuth client ID (`DATABRICKS_CLIENT_ID`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"client_secret": schema.SingleNestedAttribute{
				MarkdownDescription: "The OAuth client secret (`DATABRICKS_CLIENT_SECRET`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"database": schema.SingleNestedAttribute{
				MarkdownDescription: "The database to use (`DATABRICKS_DATABASE`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"port": schema.SingleNestedAttribute{
				MarkdownDescription: "The Databricks port (`DATABRICKS_PORT`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"config": schema.MapNestedAttribute{
				MarkdownDescription: "Additional configuration for the datasource. " +
					"Must not contain keys managed by the named attributes.",
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: nestedAttrs,
				},
			},
		},
	}
}

func (r *DatasourceDatabricksResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data DatasourceDatabricksResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for envVar, field := range databricksEnvVarToField {
		if _, ok := data.Config[envVar]; ok {
			resp.Diagnostics.AddAttributeError(
				path.Root("config").AtMapKey(envVar),
				"Reserved Config Key",
				fmt.Sprintf("%q is managed by the %q attribute and must not appear in config.", envVar, field),
			)
		}
	}
}

func (r *DatasourceDatabricksResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Manager)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Manager, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *DatasourceDatabricksResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DatasourceDatabricksResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ic := r.client.NewIntegrationsClient(ctx, data.EnvironmentId.ValueString())

	response, err := ic.InsertIntegration(ctx, connect.NewRequest(&serverv1.InsertIntegrationRequest{
		Name:            data.Name.ValueString(),
		IntegrationKind: serverv1.IntegrationKind_INTEGRATION_KIND_DATABRICKS,
		Config:          configToProto(data.buildConfig()),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Databricks Datasource",
			fmt.Sprintf("Could not create datasource: %v", err),
		)
		return
	}

	if response.Msg.Integration == nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Databricks Datasource",
			"Server returned an empty integration response.",
		)
		return
	}
	data.Id = types.StringValue(response.Msg.Integration.Id)

	tflog.Trace(ctx, "created a chalk_datasource_databricks resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasourceDatabricksResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DatasourceDatabricksResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ic := r.client.NewIntegrationsClient(ctx, data.EnvironmentId.ValueString())

	getResp, err := ic.GetIntegration(ctx, connect.NewRequest(&serverv1.GetIntegrationRequest{
		IntegrationId: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Chalk Databricks Datasource",
			fmt.Sprintf("Could not read datasource %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	iws := getResp.Msg.IntegrationWithSecrets
	if iws == nil || iws.Integration == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	integration := iws.Integration
	if integration.Kind != serverv1.IntegrationKind_INTEGRATION_KIND_DATABRICKS {
		resp.Diagnostics.AddError(
			"Unexpected Integration Kind",
			fmt.Sprintf(
				"Expected a Databricks integration, got %q. Use chalk_datasource to manage other integration types.",
				integrationKindToString(integration.Kind),
			),
		)
		return
	}

	data.Id = types.StringValue(integration.Id)
	if integration.Name != nil {
		data.Name = types.StringValue(*integration.Name)
	}
	data.EnvironmentId = types.StringValue(integration.EnvironmentId)

	returnedSecrets := make(map[string]string, len(iws.Secrets))
	for _, secret := range iws.Secrets {
		if secret.Value != nil {
			returnedSecrets[secret.Name] = *secret.Value
		} else {
			returnedSecrets[secret.Name] = ""
		}
	}

	stateConfig := data.buildConfig()

	// On import, Host is nil because ImportState only sets id and environment_id.
	// Discover any extra config keys from the server's secret list so they get
	// populated into data.Config automatically, giving users a complete starting state.
	if data.Host == nil && data.HttpPath == nil && data.AccessToken == nil {
		for _, secret := range iws.Secrets {
			if _, isNamed := databricksEnvVarToField[secret.Name]; !isNamed {
				stateConfig[secret.Name] = nullConfigValue
			} else {
				stateConfig[secret.Name] = nullConfigValue
			}
		}
	}

	refreshed := refreshConfigKeys(ctx, ic, data.Id.ValueString(), returnedSecrets, stateConfig, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	data.distributeConfig(refreshed)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasourceDatabricksResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DatasourceDatabricksResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ic := r.client.NewIntegrationsClient(ctx, data.EnvironmentId.ValueString())

	_, err := ic.UpdateIntegration(ctx, connect.NewRequest(&serverv1.UpdateIntegrationRequest{
		IntegrationId: data.Id.ValueString(),
		Name:          data.Name.ValueString(),
		Config:        configToProto(data.buildConfig()),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Chalk Databricks Datasource",
			fmt.Sprintf("Could not update datasource %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "updated a chalk_datasource_databricks resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasourceDatabricksResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DatasourceDatabricksResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ic := r.client.NewIntegrationsClient(ctx, data.EnvironmentId.ValueString())

	_, err := ic.DeleteIntegration(ctx, connect.NewRequest(&serverv1.DeleteIntegrationRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Chalk Databricks Datasource",
			fmt.Sprintf("Could not delete datasource %s: %v", data.Id.ValueString(), err),
		)
	}
}

func (r *DatasourceDatabricksResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import ID in the format 'environment_id/integration_id', got: %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}
