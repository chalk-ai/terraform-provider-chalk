package provider

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &DatasourceResource{}
var _ resource.ResourceWithImportState = &DatasourceResource{}

func NewDatasourceResource() resource.Resource {
	return &DatasourceResource{}
}

type DatasourceResource struct {
	client *ClientManager
}

type DatasourceResourceModel struct {
	Id                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	Kind                 types.String `tfsdk:"kind"`
	EnvironmentId        types.String `tfsdk:"environment_id"`
	EnvironmentVariables types.Map    `tfsdk:"environment_variables"`
}

func (r *DatasourceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_datasource"
}

func (r *DatasourceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Chalk datasource integration (e.g., PostgreSQL, Snowflake, Kafka).",

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
			"kind": schema.StringAttribute{
				MarkdownDescription: "The type of datasource (e.g., `postgresql`, `snowflake`, `kafka`, `mysql`, `bigquery`, `redshift`, `clickhouse`, `databricks`, `dynamodb`, `spanner`, `trino`, `mssql`, `pubsub`, `kinesis`, `athena`, `aws`, `gcp`, `openai`, `cohere`).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The environment ID that this datasource is scoped to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"environment_variables": schema.MapAttribute{
				MarkdownDescription: "A map of environment variable names to values for configuring the datasource connection.",
				Required:            true,
				Sensitive:           true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *DatasourceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// parseIntegrationKind converts a user-friendly string like "postgresql" to the proto enum.
func parseIntegrationKind(kindStr string) (serverv1.IntegrationKind, error) {
	enumName := "INTEGRATION_KIND_" + strings.ToUpper(kindStr)
	val, ok := serverv1.IntegrationKind_value[enumName]
	if !ok {
		return serverv1.IntegrationKind_INTEGRATION_KIND_UNSPECIFIED,
			fmt.Errorf("unknown integration kind %q", kindStr)
	}
	return serverv1.IntegrationKind(val), nil
}

// integrationKindToString converts a proto enum to a user-friendly string like "postgresql".
func integrationKindToString(kind serverv1.IntegrationKind) string {
	name := kind.String()
	return strings.ToLower(strings.TrimPrefix(name, "INTEGRATION_KIND_"))
}

func (r *DatasourceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DatasourceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	kind, err := parseIntegrationKind(data.Kind.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Kind", err.Error())
		return
	}

	envVars := make(map[string]string)
	resp.Diagnostics.Append(data.EnvironmentVariables.ElementsAs(ctx, &envVars, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ic := r.client.NewIntegrationsClient(ctx, data.EnvironmentId.ValueString())

	insertReq := &serverv1.InsertIntegrationRequest{
		Name:                 data.Name.ValueString(),
		IntegrationKind:      kind,
		EnvironmentVariables: envVars,
	}

	response, err := ic.InsertIntegration(ctx, connect.NewRequest(insertReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Datasource",
			fmt.Sprintf("Could not create datasource: %v", err),
		)
		return
	}

	data.Id = types.StringValue(response.Msg.Integration.Id)

	tflog.Trace(ctx, "created a chalk_datasource resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasourceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DatasourceResourceModel
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
			"Error Reading Chalk Datasource",
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
	data.Id = types.StringValue(integration.Id)
	if integration.Name != nil {
		data.Name = types.StringValue(*integration.Name)
	}
	data.Kind = types.StringValue(integrationKindToString(integration.Kind))
	data.EnvironmentId = types.StringValue(integration.EnvironmentId)

	// Reconstruct environment_variables from secrets
	if len(iws.Secrets) > 0 {
		envVarAttrs := make(map[string]attr.Value, len(iws.Secrets))
		for _, secret := range iws.Secrets {
			if secret.Value != nil {
				envVarAttrs[secret.Name] = types.StringValue(*secret.Value)
			} else {
				envVarAttrs[secret.Name] = types.StringValue("")
			}
		}
		data.EnvironmentVariables = types.MapValueMust(types.StringType, envVarAttrs)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasourceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DatasourceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envVars := make(map[string]string)
	resp.Diagnostics.Append(data.EnvironmentVariables.ElementsAs(ctx, &envVars, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ic := r.client.NewIntegrationsClient(ctx, data.EnvironmentId.ValueString())

	updateReq := &serverv1.UpdateIntegrationRequest{
		IntegrationId:        data.Id.ValueString(),
		Name:                 data.Name.ValueString(),
		EnvironmentVariables: envVars,
	}

	_, err := ic.UpdateIntegration(ctx, connect.NewRequest(updateReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Chalk Datasource",
			fmt.Sprintf("Could not update datasource %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "updated a chalk_datasource resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasourceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DatasourceResourceModel
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
			"Error Deleting Chalk Datasource",
			fmt.Sprintf("Could not delete datasource %s: %v", data.Id.ValueString(), err),
		)
	}
}

func (r *DatasourceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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
