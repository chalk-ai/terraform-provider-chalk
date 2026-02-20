package provider

import (
	"context"
	"fmt"
	"strings"

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

var _ resource.Resource = &DatasourcePostgresqlResource{}
var _ resource.ResourceWithImportState = &DatasourcePostgresqlResource{}
var _ resource.ResourceWithValidateConfig = &DatasourcePostgresqlResource{}

// pgEnvVarToField maps PG env var names to their Terraform attribute names.
var pgEnvVarToField = map[string]string{
	"PGHOST":     "host",
	"PGPORT":     "port",
	"PGDATABASE": "database",
	"PGUSER":     "user",
	"PGPASSWORD": "password",
}

func NewDatasourcePostgresqlResource() resource.Resource {
	return &DatasourcePostgresqlResource{}
}

type DatasourcePostgresqlResource struct {
	client *ClientManager
}

type DatasourcePostgresqlResourceModel struct {
	Id            types.String                     `tfsdk:"id"`
	Name          types.String                     `tfsdk:"name"`
	EnvironmentId types.String                     `tfsdk:"environment_id"`
	Host          *DatasourceConfigValue           `tfsdk:"host"`
	Port          *DatasourceConfigValue           `tfsdk:"port"`
	Database      *DatasourceConfigValue           `tfsdk:"database"`
	User          *DatasourceConfigValue           `tfsdk:"user"`
	Password      *DatasourceConfigValue           `tfsdk:"password"`
	Config        map[string]DatasourceConfigValue `tfsdk:"config"`
}

var nullConfigValue = DatasourceConfigValue{
	Literal:  types.StringNull(),
	SecretId: types.StringNull(),
}

// buildConfig merges the named fields and extra config into a flat map for the API.
// Required fields (host, port, database, user) are always included — using a null placeholder
// when their pointer is nil — so refreshConfigKeys fetches them from the server on import.
// The optional password field is only included when set.
func (data *DatasourcePostgresqlResourceModel) buildConfig() map[string]DatasourceConfigValue {
	deref := func(p *DatasourceConfigValue) DatasourceConfigValue {
		if p != nil {
			return *p
		}
		return nullConfigValue
	}
	config := map[string]DatasourceConfigValue{
		"PGHOST":     deref(data.Host),
		"PGPORT":     deref(data.Port),
		"PGDATABASE": deref(data.Database),
		"PGUSER":     deref(data.User),
	}
	if data.Password != nil {
		config["PGPASSWORD"] = *data.Password
	}
	for k, v := range data.Config {
		config[k] = v
	}
	return config
}

// distributeConfig populates the named fields and extra config from a refreshed flat map.
func (data *DatasourcePostgresqlResourceModel) distributeConfig(refreshed map[string]DatasourceConfigValue) {
	if v, ok := refreshed["PGHOST"]; ok {
		data.Host = &v
	}
	if v, ok := refreshed["PGPORT"]; ok {
		data.Port = &v
	}
	if v, ok := refreshed["PGDATABASE"]; ok {
		data.Database = &v
	}
	if v, ok := refreshed["PGUSER"]; ok {
		data.User = &v
	}
	if v, ok := refreshed["PGPASSWORD"]; ok {
		data.Password = &v
	}
	extra := make(map[string]DatasourceConfigValue)
	for k, v := range refreshed {
		if _, isNamed := pgEnvVarToField[k]; !isNamed {
			extra[k] = v
		}
	}
	if len(extra) > 0 {
		data.Config = extra
	} else {
		data.Config = nil
	}
}

func (r *DatasourcePostgresqlResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_datasource_postgresql"
}

func (r *DatasourcePostgresqlResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	nestedAttrs := configValueAttributes()
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Chalk PostgreSQL datasource integration.",

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
				MarkdownDescription: "The PostgreSQL server hostname or IP address (`PGHOST`).",
				Required:            true,
				Attributes:          nestedAttrs,
			},
			"port": schema.SingleNestedAttribute{
				MarkdownDescription: "The PostgreSQL server port (`PGPORT`).",
				Required:            true,
				Attributes:          nestedAttrs,
			},
			"database": schema.SingleNestedAttribute{
				MarkdownDescription: "The PostgreSQL database name (`PGDATABASE`).",
				Required:            true,
				Attributes:          nestedAttrs,
			},
			"user": schema.SingleNestedAttribute{
				MarkdownDescription: "The PostgreSQL user (`PGUSER`).",
				Required:            true,
				Attributes:          nestedAttrs,
			},
			"password": schema.SingleNestedAttribute{
				MarkdownDescription: "The PostgreSQL password (`PGPASSWORD`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"config": schema.MapNestedAttribute{
				MarkdownDescription: "Additional configuration for the datasource. " +
					"Must not contain keys managed by the named attributes (PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD).",
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: nestedAttrs,
				},
			},
		},
	}
}

func (r *DatasourcePostgresqlResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data DatasourcePostgresqlResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for envVar, field := range pgEnvVarToField {
		if _, ok := data.Config[envVar]; ok {
			resp.Diagnostics.AddAttributeError(
				path.Root("config").AtMapKey(envVar),
				"Reserved Config Key",
				fmt.Sprintf("%q is managed by the %q attribute and must not appear in config.", envVar, field),
			)
		}
	}
}

func (r *DatasourcePostgresqlResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ClientManager)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ClientManager, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *DatasourcePostgresqlResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DatasourcePostgresqlResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ic := r.client.NewIntegrationsClient(ctx, data.EnvironmentId.ValueString())

	response, err := ic.InsertIntegration(ctx, connect.NewRequest(&serverv1.InsertIntegrationRequest{
		Name:            data.Name.ValueString(),
		IntegrationKind: serverv1.IntegrationKind_INTEGRATION_KIND_POSTGRESQL,
		Config:          configToProto(data.buildConfig()),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk PostgreSQL Datasource",
			fmt.Sprintf("Could not create datasource: %v", err),
		)
		return
	}

	if response.Msg.Integration == nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk PostgreSQL Datasource",
			"Server returned an empty integration response.",
		)
		return
	}
	data.Id = types.StringValue(response.Msg.Integration.Id)

	tflog.Trace(ctx, "created a chalk_datasource_postgresql resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasourcePostgresqlResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DatasourcePostgresqlResourceModel
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
			"Error Reading Chalk PostgreSQL Datasource",
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
	if integration.Kind != serverv1.IntegrationKind_INTEGRATION_KIND_POSTGRESQL {
		resp.Diagnostics.AddError(
			"Unexpected Integration Kind",
			fmt.Sprintf(
				"Expected a PostgreSQL integration, got %q. Use chalk_datasource to manage other integration types.",
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
	if data.Host == nil {
		for _, secret := range iws.Secrets {
			if _, isNamed := pgEnvVarToField[secret.Name]; !isNamed {
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

func (r *DatasourcePostgresqlResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DatasourcePostgresqlResourceModel
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
			"Error Updating Chalk PostgreSQL Datasource",
			fmt.Sprintf("Could not update datasource %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "updated a chalk_datasource_postgresql resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasourcePostgresqlResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DatasourcePostgresqlResourceModel
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
			"Error Deleting Chalk PostgreSQL Datasource",
			fmt.Sprintf("Could not delete datasource %s: %v", data.Id.ValueString(), err),
		)
	}
}

func (r *DatasourcePostgresqlResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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
