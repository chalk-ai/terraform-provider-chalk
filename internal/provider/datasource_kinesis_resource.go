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

var _ resource.Resource = &DatasourceKinesisResource{}
var _ resource.ResourceWithImportState = &DatasourceKinesisResource{}
var _ resource.ResourceWithValidateConfig = &DatasourceKinesisResource{}

// kinesisEnvVarToField maps Kinesis env var names to their Terraform attribute names.
var kinesisEnvVarToField = map[string]string{
	"KINESIS_REGION_NAME":                   "region_name",
	"KINESIS_STREAM_NAME":                   "stream_name",
	"KINESIS_STREAM_ARN":                    "stream_arn",
	"KINESIS_LATE_ARRIVAL_DEADLINE":         "late_arrival_deadline",
	"KINESIS_DEAD_LETTER_QUEUE_STREAM_NAME": "dead_letter_queue_stream_name",
	"KINESIS_AWS_ACCESS_KEY_ID":             "aws_access_key_id",
	"KINESIS_CONSUMER_ROLE_ARN":             "consumer_role_arn",
	"KINESIS_AWS_SECRET_ACCESS_KEY":         "aws_secret_access_key",
	"KINESIS_AWS_SESSION_TOKEN":             "aws_session_token",
	"KINESIS_ENDPOINT_URL":                  "endpoint_url",
	"KINESIS_ENHANCED_FANOUT_CONSUMER_NAME": "enhanced_fanout_consumer_name",
}

func NewDatasourceKinesisResource() resource.Resource {
	return &DatasourceKinesisResource{}
}

type DatasourceKinesisResource struct {
	client *client.Manager
}

type DatasourceKinesisResourceModel struct {
	Id                         types.String                     `tfsdk:"id"`
	Name                       types.String                     `tfsdk:"name"`
	EnvironmentId              types.String                     `tfsdk:"environment_id"`
	RegionName                 *DatasourceConfigValue           `tfsdk:"region_name"`
	StreamName                 *DatasourceConfigValue           `tfsdk:"stream_name"`
	StreamArn                  *DatasourceConfigValue           `tfsdk:"stream_arn"`
	LateArrivalDeadline        *DatasourceConfigValue           `tfsdk:"late_arrival_deadline"`
	DeadLetterQueueStreamName  *DatasourceConfigValue           `tfsdk:"dead_letter_queue_stream_name"`
	AwsAccessKeyId             *DatasourceConfigValue           `tfsdk:"aws_access_key_id"`
	ConsumerRoleArn            *DatasourceConfigValue           `tfsdk:"consumer_role_arn"`
	AwsSecretAccessKey         *DatasourceConfigValue           `tfsdk:"aws_secret_access_key"`
	AwsSessionToken            *DatasourceConfigValue           `tfsdk:"aws_session_token"`
	EndpointUrl                *DatasourceConfigValue           `tfsdk:"endpoint_url"`
	EnhancedFanoutConsumerName *DatasourceConfigValue           `tfsdk:"enhanced_fanout_consumer_name"`
	Config                     map[string]DatasourceConfigValue `tfsdk:"config"`
}

// buildConfig merges the named fields and extra config into a flat map for the API.
func (data *DatasourceKinesisResourceModel) buildConfig() map[string]DatasourceConfigValue {
	config := make(map[string]DatasourceConfigValue)

	if data.RegionName != nil {
		config["KINESIS_REGION_NAME"] = *data.RegionName
	}
	if data.StreamName != nil {
		config["KINESIS_STREAM_NAME"] = *data.StreamName
	}
	if data.StreamArn != nil {
		config["KINESIS_STREAM_ARN"] = *data.StreamArn
	}
	if data.LateArrivalDeadline != nil {
		config["KINESIS_LATE_ARRIVAL_DEADLINE"] = *data.LateArrivalDeadline
	}
	if data.DeadLetterQueueStreamName != nil {
		config["KINESIS_DEAD_LETTER_QUEUE_STREAM_NAME"] = *data.DeadLetterQueueStreamName
	}
	if data.AwsAccessKeyId != nil {
		config["KINESIS_AWS_ACCESS_KEY_ID"] = *data.AwsAccessKeyId
	}
	if data.ConsumerRoleArn != nil {
		config["KINESIS_CONSUMER_ROLE_ARN"] = *data.ConsumerRoleArn
	}
	if data.AwsSecretAccessKey != nil {
		config["KINESIS_AWS_SECRET_ACCESS_KEY"] = *data.AwsSecretAccessKey
	}
	if data.AwsSessionToken != nil {
		config["KINESIS_AWS_SESSION_TOKEN"] = *data.AwsSessionToken
	}
	if data.EndpointUrl != nil {
		config["KINESIS_ENDPOINT_URL"] = *data.EndpointUrl
	}
	if data.EnhancedFanoutConsumerName != nil {
		config["KINESIS_ENHANCED_FANOUT_CONSUMER_NAME"] = *data.EnhancedFanoutConsumerName
	}

	maps.Copy(config, data.Config)
	return config
}

// distributeConfig populates the named fields and extra config from a refreshed flat map.
func (data *DatasourceKinesisResourceModel) distributeConfig(refreshed map[string]DatasourceConfigValue) {
	if v, ok := refreshed["KINESIS_REGION_NAME"]; ok {
		data.RegionName = &v
	}
	if v, ok := refreshed["KINESIS_STREAM_NAME"]; ok {
		data.StreamName = &v
	}
	if v, ok := refreshed["KINESIS_STREAM_ARN"]; ok {
		data.StreamArn = &v
	}
	if v, ok := refreshed["KINESIS_LATE_ARRIVAL_DEADLINE"]; ok {
		data.LateArrivalDeadline = &v
	}
	if v, ok := refreshed["KINESIS_DEAD_LETTER_QUEUE_STREAM_NAME"]; ok {
		data.DeadLetterQueueStreamName = &v
	}
	if v, ok := refreshed["KINESIS_AWS_ACCESS_KEY_ID"]; ok {
		data.AwsAccessKeyId = &v
	}
	if v, ok := refreshed["KINESIS_CONSUMER_ROLE_ARN"]; ok {
		data.ConsumerRoleArn = &v
	}
	if v, ok := refreshed["KINESIS_AWS_SECRET_ACCESS_KEY"]; ok {
		data.AwsSecretAccessKey = &v
	}
	if v, ok := refreshed["KINESIS_AWS_SESSION_TOKEN"]; ok {
		data.AwsSessionToken = &v
	}
	if v, ok := refreshed["KINESIS_ENDPOINT_URL"]; ok {
		data.EndpointUrl = &v
	}
	if v, ok := refreshed["KINESIS_ENHANCED_FANOUT_CONSUMER_NAME"]; ok {
		data.EnhancedFanoutConsumerName = &v
	}

	extra := make(map[string]DatasourceConfigValue)
	for k, v := range refreshed {
		if _, isNamed := kinesisEnvVarToField[k]; !isNamed {
			extra[k] = v
		}
	}
	if len(extra) > 0 {
		data.Config = extra
	} else {
		data.Config = nil
	}
}

func (r *DatasourceKinesisResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_datasource_kinesis"
}

func (r *DatasourceKinesisResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	nestedAttrs := configValueAttributes()
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Chalk Kinesis datasource integration.",

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
			"region_name": schema.SingleNestedAttribute{
				MarkdownDescription: "The AWS region name, e.g. 'us-east-2' (`KINESIS_REGION_NAME`).",
				Required:            true,
				Attributes:          nestedAttrs,
			},
			"stream_name": schema.SingleNestedAttribute{
				MarkdownDescription: "The name of the Kinesis stream. Either this or stream_arn must be specified (`KINESIS_STREAM_NAME`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"stream_arn": schema.SingleNestedAttribute{
				MarkdownDescription: "The ARN of the Kinesis stream. Either this or stream_name must be specified (`KINESIS_STREAM_ARN`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"late_arrival_deadline": schema.SingleNestedAttribute{
				MarkdownDescription: "Messages older than this deadline will not be processed. Format: '1hr30m40s', '30m', 'infinity' (`KINESIS_LATE_ARRIVAL_DEADLINE`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"dead_letter_queue_stream_name": schema.SingleNestedAttribute{
				MarkdownDescription: "Kinesis stream name to send messages when message processing fails (`KINESIS_DEAD_LETTER_QUEUE_STREAM_NAME`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"aws_access_key_id": schema.SingleNestedAttribute{
				MarkdownDescription: "AWS access key ID credential (`KINESIS_AWS_ACCESS_KEY_ID`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"consumer_role_arn": schema.SingleNestedAttribute{
				MarkdownDescription: "Consumer role ARN to assume (`KINESIS_CONSUMER_ROLE_ARN`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"aws_secret_access_key": schema.SingleNestedAttribute{
				MarkdownDescription: "AWS secret access key credential (`KINESIS_AWS_SECRET_ACCESS_KEY`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"aws_session_token": schema.SingleNestedAttribute{
				MarkdownDescription: "AWS session token credential (`KINESIS_AWS_SESSION_TOKEN`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"endpoint_url": schema.SingleNestedAttribute{
				MarkdownDescription: "Endpoint URL to hit Kinesis server (`KINESIS_ENDPOINT_URL`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"enhanced_fanout_consumer_name": schema.SingleNestedAttribute{
				MarkdownDescription: "Consumer name if using Enhanced Fan-Out consumption (`KINESIS_ENHANCED_FANOUT_CONSUMER_NAME`).",
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

func (r *DatasourceKinesisResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data DatasourceKinesisResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for envVar, field := range kinesisEnvVarToField {
		if _, ok := data.Config[envVar]; ok {
			resp.Diagnostics.AddAttributeError(
				path.Root("config").AtMapKey(envVar),
				"Reserved Config Key",
				fmt.Sprintf("%q is managed by the %q attribute and must not appear in config.", envVar, field),
			)
		}
	}
}

func (r *DatasourceKinesisResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DatasourceKinesisResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DatasourceKinesisResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ic := r.client.NewIntegrationsClient(ctx, data.EnvironmentId.ValueString())

	response, err := ic.InsertIntegration(ctx, connect.NewRequest(&serverv1.InsertIntegrationRequest{
		Name:            data.Name.ValueString(),
		IntegrationKind: serverv1.IntegrationKind_INTEGRATION_KIND_KINESIS,
		Config:          configToProto(data.buildConfig()),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Kinesis Datasource",
			fmt.Sprintf("Could not create datasource: %v", err),
		)
		return
	}

	if response.Msg.Integration == nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Kinesis Datasource",
			"Server returned an empty integration response.",
		)
		return
	}
	data.Id = types.StringValue(response.Msg.Integration.Id)

	tflog.Trace(ctx, "created a chalk_datasource_kinesis resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasourceKinesisResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DatasourceKinesisResourceModel
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
			"Error Reading Chalk Kinesis Datasource",
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
	if integration.Kind != serverv1.IntegrationKind_INTEGRATION_KIND_KINESIS {
		resp.Diagnostics.AddError(
			"Unexpected Integration Kind",
			fmt.Sprintf(
				"Expected a Kinesis integration, got %q. Use chalk_datasource to manage other integration types.",
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

	// On import, named fields are nil because ImportState only sets id and environment_id.
	// Discover config keys from the server's secret list so they get populated automatically.
	if data.RegionName == nil {
		for _, secret := range iws.Secrets {
			stateConfig[secret.Name] = nullConfigValue
		}
	}

	refreshed := refreshConfigKeys(ctx, ic, data.Id.ValueString(), returnedSecrets, stateConfig, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	data.distributeConfig(refreshed)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasourceKinesisResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DatasourceKinesisResourceModel
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
			"Error Updating Chalk Kinesis Datasource",
			fmt.Sprintf("Could not update datasource %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "updated a chalk_datasource_kinesis resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasourceKinesisResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DatasourceKinesisResourceModel
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
			"Error Deleting Chalk Kinesis Datasource",
			fmt.Sprintf("Could not delete datasource %s: %v", data.Id.ValueString(), err),
		)
	}
}

func (r *DatasourceKinesisResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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
