// Deprecated: This data source mirrors the deprecated chalk_cluster_background_persistence
// resource. Prefer chalk_unmanaged_cluster_background_persistence for new work.

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

var _ datasource.DataSource = &ClusterBackgroundPersistenceDataSource{}

func NewClusterBackgroundPersistenceDataSource() datasource.DataSource {
	return &ClusterBackgroundPersistenceDataSource{}
}

type ClusterBackgroundPersistenceDataSource struct {
	client *client.Manager
}

// ClusterBackgroundPersistenceDataSourceModel mirrors the legacy resource model
// minus kafka_sasl_secret (deliberately omitted — sensitive on the resource).
type ClusterBackgroundPersistenceDataSourceModel struct {
	Id                                   types.String `tfsdk:"id"`
	KubeClusterId                        types.String `tfsdk:"kube_cluster_id"`
	Namespace                            types.String `tfsdk:"namespace"`
	ServiceAccountName                   types.String `tfsdk:"service_account_name"`
	BusWriterImageGo                     types.String `tfsdk:"bus_writer_image_go"`
	BusWriterImagePython                 types.String `tfsdk:"bus_writer_image_python"`
	BusWriterImageBswl                   types.String `tfsdk:"bus_writer_image_bswl"`
	BusWriterImageRust                   types.String `tfsdk:"bus_writer_image_rust"`
	BigqueryParquetUploadSubscriptionId  types.String `tfsdk:"bigquery_parquet_upload_subscription_id"`
	BigqueryStreamingWriteSubscriptionId types.String `tfsdk:"bigquery_streaming_write_subscription_id"`
	BigqueryStreamingWriteTopic          types.String `tfsdk:"bigquery_streaming_write_topic"`
	BqUploadBucket                       types.String `tfsdk:"bq_upload_bucket"`
	BqUploadTopic                        types.String `tfsdk:"bq_upload_topic"`
	GoogleCloudProject                   types.String `tfsdk:"google_cloud_project"`
	KafkaDlqTopic                        types.String `tfsdk:"kafka_dlq_topic"`
	MetricsBusSubscriptionId             types.String `tfsdk:"metrics_bus_subscription_id"`
	MetricsBusTopicId                    types.String `tfsdk:"metrics_bus_topic_id"`
	OperationSubscriptionId              types.String `tfsdk:"operation_subscription_id"`
	QueryLogResultTopic                  types.String `tfsdk:"query_log_result_topic"`
	QueryLogSubscriptionId               types.String `tfsdk:"query_log_subscription_id"`
	ResultBusOfflineStoreSubscriptionId  types.String `tfsdk:"result_bus_offline_store_subscription_id"`
	ResultBusOnlineStoreSubscriptionId   types.String `tfsdk:"result_bus_online_store_subscription_id"`
	ResultBusTopicId                     types.String `tfsdk:"result_bus_topic_id"`
	ApiServerHost                        types.String `tfsdk:"api_server_host"`
	KafkaBootstrapServers                types.String `tfsdk:"kafka_bootstrap_servers"`
	SnowflakeStorageIntegrationName      types.String `tfsdk:"snowflake_storage_integration_name"`
	RedisLightningSupportsHasMany        types.Bool   `tfsdk:"redis_lightning_supports_has_many"`
	Writers                              types.List   `tfsdk:"writers"`
}

func (d *ClusterBackgroundPersistenceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_background_persistence"
}

func (d *ClusterBackgroundPersistenceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		DeprecationMessage:  "Use chalk_unmanaged_cluster_background_persistence instead.",
		MarkdownDescription: "~> **Deprecated** Use `chalk_unmanaged_cluster_background_persistence` instead.\n\nReads a legacy Chalk cluster background persistence deployment by ID. **Does not** surface `kafka_sasl_secret` (Sensitive on the resource).",
		Attributes: map[string]schema.Attribute{
			"id":                      schema.StringAttribute{MarkdownDescription: "Background persistence identifier.", Required: true},
			"kube_cluster_id":         schema.StringAttribute{Computed: true},
			"namespace":               schema.StringAttribute{Computed: true},
			"service_account_name":    schema.StringAttribute{Computed: true},
			"bus_writer_image_go":     schema.StringAttribute{Computed: true},
			"bus_writer_image_python": schema.StringAttribute{Computed: true},
			"bus_writer_image_bswl":   schema.StringAttribute{Computed: true},
			"bus_writer_image_rust":   schema.StringAttribute{Computed: true},
			"bigquery_parquet_upload_subscription_id":  schema.StringAttribute{Computed: true},
			"bigquery_streaming_write_subscription_id": schema.StringAttribute{Computed: true},
			"bigquery_streaming_write_topic":           schema.StringAttribute{Computed: true},
			"bq_upload_bucket":                         schema.StringAttribute{Computed: true},
			"bq_upload_topic":                          schema.StringAttribute{Computed: true},
			"google_cloud_project":                     schema.StringAttribute{Computed: true},
			"kafka_dlq_topic":                          schema.StringAttribute{Computed: true},
			"metrics_bus_subscription_id":              schema.StringAttribute{Computed: true},
			"metrics_bus_topic_id":                     schema.StringAttribute{Computed: true},
			"operation_subscription_id":                schema.StringAttribute{Computed: true},
			"query_log_result_topic":                   schema.StringAttribute{Computed: true},
			"query_log_subscription_id":                schema.StringAttribute{Computed: true},
			"result_bus_offline_store_subscription_id": schema.StringAttribute{Computed: true},
			"result_bus_online_store_subscription_id":  schema.StringAttribute{Computed: true},
			"result_bus_topic_id":                      schema.StringAttribute{Computed: true},
			"api_server_host":                          schema.StringAttribute{Computed: true},
			"kafka_bootstrap_servers":                  schema.StringAttribute{Computed: true},
			"snowflake_storage_integration_name":       schema.StringAttribute{Computed: true},
			"redis_lightning_supports_has_many":        schema.BoolAttribute{Computed: true},
			"writers":                                  bgpUnmanagedWritersDataSourceSchemaAttribute(),
		},
	}
}

func (d *ClusterBackgroundPersistenceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ClusterBackgroundPersistenceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ClusterBackgroundPersistenceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "read chalk_cluster_background_persistence data source", map[string]any{"id": data.Id.ValueString()})

	bc := d.client.NewBuilderClient(ctx)
	bgResp, err := bc.GetClusterBackgroundPersistence(ctx, connect.NewRequest(&serverv1.GetClusterBackgroundPersistenceRequest{
		Id: data.Id.ValueStringPointer(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Chalk Cluster Background Persistence",
			fmt.Sprintf("Could not read cluster background persistence %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	bg := bgResp.Msg.BackgroundPersistence
	if bg == nil {
		resp.Diagnostics.AddError("Empty Background Persistence Response", fmt.Sprintf("Server returned no background persistence for %s", data.Id.ValueString()))
		return
	}

	if bg.GetKubeClusterId() != "" {
		data.KubeClusterId = types.StringValue(bg.GetKubeClusterId())
	} else {
		data.KubeClusterId = types.StringNull()
	}

	if bg.Specs == nil || bg.Specs.CommonPersistenceSpecs == nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	common := bg.Specs.CommonPersistenceSpecs
	data.Namespace = types.StringValue(common.Namespace)
	data.ServiceAccountName = types.StringValue(common.ServiceAccountName)
	data.BusWriterImageGo = stringOrNull(common.BusWriterImageGo)
	data.BusWriterImagePython = stringOrNull(common.BusWriterImagePython)
	data.BusWriterImageBswl = stringOrNull(common.BusWriterImageBswl)
	data.BusWriterImageRust = stringOrNull(common.BusWriterImageRust)
	data.BigqueryParquetUploadSubscriptionId = stringOrNull(common.BigqueryParquetUploadSubscriptionId)
	data.BigqueryStreamingWriteSubscriptionId = stringOrNull(common.BigqueryStreamingWriteSubscriptionId)
	data.BigqueryStreamingWriteTopic = stringOrNull(common.BigqueryStreamingWriteTopic)
	data.BqUploadBucket = stringOrNull(common.BqUploadBucket)
	data.BqUploadTopic = stringOrNull(common.BqUploadTopic)
	data.GoogleCloudProject = stringOrNull(common.GoogleCloudProject)
	data.KafkaDlqTopic = stringOrNull(common.KafkaDlqTopic)
	data.MetricsBusSubscriptionId = stringOrNull(common.MetricsBusSubscriptionId)
	data.MetricsBusTopicId = stringOrNull(common.MetricsBusTopicId)
	data.OperationSubscriptionId = stringOrNull(common.OperationSubscriptionId)
	data.QueryLogResultTopic = stringOrNull(common.QueryLogResultTopic)
	data.QueryLogSubscriptionId = stringOrNull(common.QueryLogSubscriptionId)
	data.ResultBusOfflineStoreSubscriptionId = stringOrNull(common.ResultBusOfflineStoreSubscriptionId)
	data.ResultBusOnlineStoreSubscriptionId = stringOrNull(common.ResultBusOnlineStoreSubscriptionId)
	data.ResultBusTopicId = stringOrNull(common.ResultBusTopicId)
	data.ApiServerHost = stringOrNull(bg.Specs.ApiServerHost)
	data.KafkaBootstrapServers = stringOrNull(bg.Specs.KafkaBootstrapServers)
	data.SnowflakeStorageIntegrationName = stringOrNull(bg.Specs.SnowflakeStorageIntegrationName)
	data.RedisLightningSupportsHasMany = types.BoolValue(bg.Specs.RedisLightningSupportsHasMany)

	writersList, writerDiags := bgpWritersProtoToTF(ctx, bg.Specs.Writers)
	resp.Diagnostics.Append(writerDiags...)
	if !resp.Diagnostics.HasError() {
		data.Writers = writersList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
