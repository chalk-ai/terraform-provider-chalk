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

var _ resource.Resource = &DatasourceKafkaResource{}
var _ resource.ResourceWithImportState = &DatasourceKafkaResource{}
var _ resource.ResourceWithValidateConfig = &DatasourceKafkaResource{}

// kafkaEnvVarToField maps Kafka env var names to their Terraform attribute names.
var kafkaEnvVarToField = map[string]string{
	"KAFKA_BOOTSTRAP_SERVER":      "bootstrap_server",
	"KAFKA_TOPIC":                 "topic",
	"KAFKA_CLIENT_ID_PREFIX":      "client_id_prefix",
	"KAFKA_GROUP_ID_PREFIX":       "group_id_prefix",
	"KAFKA_SECURITY_PROTOCOL":     "security_protocol",
	"KAFKA_SASL_MECHANISM":        "sasl_mechanism",
	"KAFKA_SASL_USERNAME":         "sasl_username",
	"KAFKA_SASL_PASSWORD":         "sasl_password",
	"KAFKA_SSL_CA_FILE":           "ssl_ca_file",
	"KAFKA_DLQ_TOPIC":             "dlq_topic",
	"KAFKA_MSK_IAM_AUTH":          "msk_iam_auth",
	"KAFKA_AWS_REGION":            "aws_region",
	"KAFKA_AWS_ROLE_ARN":          "aws_role_arn",
	"KAFKA_ADDITIONAL_KAFKA_ARGS": "additional_kafka_args",
}

func NewDatasourceKafkaResource() resource.Resource {
	return &DatasourceKafkaResource{}
}

type DatasourceKafkaResource struct {
	client *client.Manager
}

type DatasourceKafkaResourceModel struct {
	Id                  types.String                     `tfsdk:"id"`
	Name                types.String                     `tfsdk:"name"`
	EnvironmentId       types.String                     `tfsdk:"environment_id"`
	BootstrapServer     *DatasourceConfigValue           `tfsdk:"bootstrap_server"`
	Topic               *DatasourceConfigValue           `tfsdk:"topic"`
	ClientIdPrefix      *DatasourceConfigValue           `tfsdk:"client_id_prefix"`
	GroupIdPrefix       *DatasourceConfigValue           `tfsdk:"group_id_prefix"`
	SecurityProtocol    *DatasourceConfigValue           `tfsdk:"security_protocol"`
	SaslMechanism       *DatasourceConfigValue           `tfsdk:"sasl_mechanism"`
	SaslUsername        *DatasourceConfigValue           `tfsdk:"sasl_username"`
	SaslPassword        *DatasourceConfigValue           `tfsdk:"sasl_password"`
	SslCaFile           *DatasourceConfigValue           `tfsdk:"ssl_ca_file"`
	DlqTopic            *DatasourceConfigValue           `tfsdk:"dlq_topic"`
	MskIamAuth          *DatasourceConfigValue           `tfsdk:"msk_iam_auth"`
	AwsRegion           *DatasourceConfigValue           `tfsdk:"aws_region"`
	AwsRoleArn          *DatasourceConfigValue           `tfsdk:"aws_role_arn"`
	AdditionalKafkaArgs *DatasourceConfigValue           `tfsdk:"additional_kafka_args"`
	Config              map[string]DatasourceConfigValue `tfsdk:"config"`
}

// buildConfig merges the named fields and extra config into a flat map for the API.
func (data *DatasourceKafkaResourceModel) buildConfig() map[string]DatasourceConfigValue {
	config := make(map[string]DatasourceConfigValue)

	if data.BootstrapServer != nil {
		config["KAFKA_BOOTSTRAP_SERVER"] = *data.BootstrapServer
	}
	if data.Topic != nil {
		config["KAFKA_TOPIC"] = *data.Topic
	}
	if data.ClientIdPrefix != nil {
		config["KAFKA_CLIENT_ID_PREFIX"] = *data.ClientIdPrefix
	}
	if data.GroupIdPrefix != nil {
		config["KAFKA_GROUP_ID_PREFIX"] = *data.GroupIdPrefix
	}
	if data.SecurityProtocol != nil {
		config["KAFKA_SECURITY_PROTOCOL"] = *data.SecurityProtocol
	}
	if data.SaslMechanism != nil {
		config["KAFKA_SASL_MECHANISM"] = *data.SaslMechanism
	}
	if data.SaslUsername != nil {
		config["KAFKA_SASL_USERNAME"] = *data.SaslUsername
	}
	if data.SaslPassword != nil {
		config["KAFKA_SASL_PASSWORD"] = *data.SaslPassword
	}
	if data.SslCaFile != nil {
		config["KAFKA_SSL_CA_FILE"] = *data.SslCaFile
	}
	if data.DlqTopic != nil {
		config["KAFKA_DLQ_TOPIC"] = *data.DlqTopic
	}
	if data.MskIamAuth != nil {
		config["KAFKA_MSK_IAM_AUTH"] = *data.MskIamAuth
	}
	if data.AwsRegion != nil {
		config["KAFKA_AWS_REGION"] = *data.AwsRegion
	}
	if data.AwsRoleArn != nil {
		config["KAFKA_AWS_ROLE_ARN"] = *data.AwsRoleArn
	}
	if data.AdditionalKafkaArgs != nil {
		config["KAFKA_ADDITIONAL_KAFKA_ARGS"] = *data.AdditionalKafkaArgs
	}

	maps.Copy(config, data.Config)
	return config
}

// distributeConfig populates the named fields and extra config from a refreshed flat map.
func (data *DatasourceKafkaResourceModel) distributeConfig(refreshed map[string]DatasourceConfigValue) {
	if v, ok := refreshed["KAFKA_BOOTSTRAP_SERVER"]; ok {
		data.BootstrapServer = &v
	}
	if v, ok := refreshed["KAFKA_TOPIC"]; ok {
		data.Topic = &v
	}
	if v, ok := refreshed["KAFKA_CLIENT_ID_PREFIX"]; ok {
		data.ClientIdPrefix = &v
	}
	if v, ok := refreshed["KAFKA_GROUP_ID_PREFIX"]; ok {
		data.GroupIdPrefix = &v
	}
	if v, ok := refreshed["KAFKA_SECURITY_PROTOCOL"]; ok {
		data.SecurityProtocol = &v
	}
	if v, ok := refreshed["KAFKA_SASL_MECHANISM"]; ok {
		data.SaslMechanism = &v
	}
	if v, ok := refreshed["KAFKA_SASL_USERNAME"]; ok {
		data.SaslUsername = &v
	}
	if v, ok := refreshed["KAFKA_SASL_PASSWORD"]; ok {
		data.SaslPassword = &v
	}
	if v, ok := refreshed["KAFKA_SSL_CA_FILE"]; ok {
		data.SslCaFile = &v
	}
	if v, ok := refreshed["KAFKA_DLQ_TOPIC"]; ok {
		data.DlqTopic = &v
	}
	if v, ok := refreshed["KAFKA_MSK_IAM_AUTH"]; ok {
		data.MskIamAuth = &v
	}
	if v, ok := refreshed["KAFKA_AWS_REGION"]; ok {
		data.AwsRegion = &v
	}
	if v, ok := refreshed["KAFKA_AWS_ROLE_ARN"]; ok {
		data.AwsRoleArn = &v
	}
	if v, ok := refreshed["KAFKA_ADDITIONAL_KAFKA_ARGS"]; ok {
		data.AdditionalKafkaArgs = &v
	}

	extra := make(map[string]DatasourceConfigValue)
	for k, v := range refreshed {
		if _, isNamed := kafkaEnvVarToField[k]; !isNamed {
			extra[k] = v
		}
	}
	if len(extra) > 0 {
		data.Config = extra
	} else {
		data.Config = nil
	}
}

func (r *DatasourceKafkaResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_datasource_kafka"
}

func (r *DatasourceKafkaResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	nestedAttrs := configValueAttributes()
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Chalk Kafka datasource integration.",

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
			"bootstrap_server": schema.SingleNestedAttribute{
				MarkdownDescription: "Comma-separated list of host/port pairs (`KAFKA_BOOTSTRAP_SERVER`).",
				Required:            true,
				Attributes:          nestedAttrs,
			},
			"topic": schema.SingleNestedAttribute{
				MarkdownDescription: "The topic to subscribe to (`KAFKA_TOPIC`).",
				Required:            true,
				Attributes:          nestedAttrs,
			},
			"client_id_prefix": schema.SingleNestedAttribute{
				MarkdownDescription: "Client ID prefix (`KAFKA_CLIENT_ID_PREFIX`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"group_id_prefix": schema.SingleNestedAttribute{
				MarkdownDescription: "Group ID prefix (`KAFKA_GROUP_ID_PREFIX`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"security_protocol": schema.SingleNestedAttribute{
				MarkdownDescription: "Protocol used to communicate with brokers. Valid values: PLAINTEXT, SSL, SASL_PLAINTEXT, SASL_SSL (`KAFKA_SECURITY_PROTOCOL`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"sasl_mechanism": schema.SingleNestedAttribute{
				MarkdownDescription: "Authentication mechanism for SASL. Valid values: PLAIN, SCRAM-SHA-256, SCRAM-SHA-512 (`KAFKA_SASL_MECHANISM`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"sasl_username": schema.SingleNestedAttribute{
				MarkdownDescription: "Username for SASL authentication (`KAFKA_SASL_USERNAME`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"sasl_password": schema.SingleNestedAttribute{
				MarkdownDescription: "Password for SASL authentication (`KAFKA_SASL_PASSWORD`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"ssl_ca_file": schema.SingleNestedAttribute{
				MarkdownDescription: "Path to SSL CA File (`KAFKA_SSL_CA_FILE`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"dlq_topic": schema.SingleNestedAttribute{
				MarkdownDescription: "Topic to send messages to when message processing fails (`KAFKA_DLQ_TOPIC`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"msk_iam_auth": schema.SingleNestedAttribute{
				MarkdownDescription: "Enable AWS MSK IAM authentication. Valid values: 'true', 'false' (`KAFKA_MSK_IAM_AUTH`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"aws_region": schema.SingleNestedAttribute{
				MarkdownDescription: "AWS region for MSK cluster, required when MSK IAM Auth is enabled (`KAFKA_AWS_REGION`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"aws_role_arn": schema.SingleNestedAttribute{
				MarkdownDescription: "IAM role ARN to assume for MSK authentication (`KAFKA_AWS_ROLE_ARN`).",
				Optional:            true,
				Attributes:          nestedAttrs,
			},
			"additional_kafka_args": schema.SingleNestedAttribute{
				MarkdownDescription: "Additional engine arguments as JSON (`KAFKA_ADDITIONAL_KAFKA_ARGS`).",
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

func (r *DatasourceKafkaResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data DatasourceKafkaResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for envVar, field := range kafkaEnvVarToField {
		if _, ok := data.Config[envVar]; ok {
			resp.Diagnostics.AddAttributeError(
				path.Root("config").AtMapKey(envVar),
				"Reserved Config Key",
				fmt.Sprintf("%q is managed by the %q attribute and must not appear in config.", envVar, field),
			)
		}
	}
}

func (r *DatasourceKafkaResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DatasourceKafkaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DatasourceKafkaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ic := r.client.NewIntegrationsClient(ctx, data.EnvironmentId.ValueString())

	response, err := ic.InsertIntegration(ctx, connect.NewRequest(&serverv1.InsertIntegrationRequest{
		Name:            data.Name.ValueString(),
		IntegrationKind: serverv1.IntegrationKind_INTEGRATION_KIND_KAFKA,
		Config:          configToProto(data.buildConfig()),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Kafka Datasource",
			fmt.Sprintf("Could not create datasource: %v", err),
		)
		return
	}

	if response.Msg.Integration == nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Kafka Datasource",
			"Server returned an empty integration response.",
		)
		return
	}
	data.Id = types.StringValue(response.Msg.Integration.Id)

	tflog.Trace(ctx, "created a chalk_datasource_kafka resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasourceKafkaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DatasourceKafkaResourceModel
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
			"Error Reading Chalk Kafka Datasource",
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
	if integration.Kind != serverv1.IntegrationKind_INTEGRATION_KIND_KAFKA {
		resp.Diagnostics.AddError(
			"Unexpected Integration Kind",
			fmt.Sprintf(
				"Expected a Kafka integration, got %q. Use chalk_datasource to manage other integration types.",
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
	if data.BootstrapServer == nil {
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

func (r *DatasourceKafkaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DatasourceKafkaResourceModel
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
			"Error Updating Chalk Kafka Datasource",
			fmt.Sprintf("Could not update datasource %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "updated a chalk_datasource_kafka resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasourceKafkaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DatasourceKafkaResourceModel
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
			"Error Deleting Chalk Kafka Datasource",
			fmt.Sprintf("Could not delete datasource %s: %v", data.Id.ValueString(), err),
		)
	}
}

func (r *DatasourceKafkaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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
