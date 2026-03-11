// Deprecated: This resource is deprecated. You likely want to modify unmanaged_cluster_background_persistence_resource.go instead.
package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &ClusterBackgroundPersistenceResource{}
var _ resource.ResourceWithImportState = &ClusterBackgroundPersistenceResource{}

func NewClusterBackgroundPersistenceResource() resource.Resource {
	return &ClusterBackgroundPersistenceResource{}
}

type ClusterBackgroundPersistenceResource struct {
	client *ClientManager
}

type KubeResourceConfigModel struct {
	CPU              types.String `tfsdk:"cpu"`
	Memory           types.String `tfsdk:"memory"`
	EphemeralStorage types.String `tfsdk:"ephemeral_storage"`
	Storage          types.String `tfsdk:"storage"`
}

type BackgroundPersistenceWriterHpaModel struct {
	HpaPubsubSubscriptionId types.String `tfsdk:"hpa_pubsub_subscription_id"`
	HpaMinReplicas          types.Int64  `tfsdk:"hpa_min_replicas"`
	HpaMaxReplicas          types.Int64  `tfsdk:"hpa_max_replicas"`
	HpaTargetAverageValue   types.Int64  `tfsdk:"hpa_target_average_value"`
}

type BackgroundPersistenceWriterModel struct {
	Name                                     types.String                         `tfsdk:"name"`
	ImageOverride                            types.String                         `tfsdk:"image_override"`
	HpaSpecs                                 *BackgroundPersistenceWriterHpaModel `tfsdk:"hpa_specs"`
	GkeSpot                                  types.Bool                           `tfsdk:"gke_spot"`
	LoadWriterConfigmap                      types.Bool                           `tfsdk:"load_writer_configmap"`
	Version                                  types.String                         `tfsdk:"version"`
	Request                                  *KubeResourceConfigModel             `tfsdk:"request"`
	Limit                                    *KubeResourceConfigModel             `tfsdk:"limit"`
	BusSubscriberType                        types.String                         `tfsdk:"bus_subscriber_type"`
	DefaultReplicaCount                      types.Int64                          `tfsdk:"default_replica_count"`
	KafkaConsumerGroupOverride               types.String                         `tfsdk:"kafka_consumer_group_override"`
	MaxBatchSize                             types.Int64                          `tfsdk:"max_batch_size"`
	MessageProcessingConcurrency             types.Int64                          `tfsdk:"message_processing_concurrency"`
	MetadataSqlSslCaCertSecret               types.String                         `tfsdk:"metadata_sql_ssl_ca_cert_secret"`
	MetadataSqlSslClientCertSecret           types.String                         `tfsdk:"metadata_sql_ssl_client_cert_secret"`
	MetadataSqlSslClientKeySecret            types.String                         `tfsdk:"metadata_sql_ssl_client_key_secret"`
	MetadataSqlUriSecret                     types.String                         `tfsdk:"metadata_sql_uri_secret"`
	OfflineStoreInserterDbType               types.String                         `tfsdk:"offline_store_inserter_db_type"`
	StorageCachePrefix                       types.String                         `tfsdk:"storage_cache_prefix"`
	ResultsWriterSkipProducingFeatureMetrics types.Bool                           `tfsdk:"results_writer_skip_producing_feature_metrics"`
	QueryTableWriteDropRatio                 types.String                         `tfsdk:"query_table_write_drop_ratio"`
}

type ClusterBackgroundPersistenceResourceModel struct {
	Id types.String `tfsdk:"id"`

	Namespace                            types.String `tfsdk:"namespace"`
	BusWriterImageGo                     types.String `tfsdk:"bus_writer_image_go"`
	BusWriterImagePython                 types.String `tfsdk:"bus_writer_image_python"`
	BusWriterImageBswl                   types.String `tfsdk:"bus_writer_image_bswl"`
	BusWriterImageRust                   types.String `tfsdk:"bus_writer_image_rust"`
	ServiceAccountName                   types.String `tfsdk:"service_account_name"`
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

	ApiServerHost                   types.String `tfsdk:"api_server_host"`
	KafkaSaslSecret                 types.String `tfsdk:"kafka_sasl_secret"`
	KafkaBootstrapServers           types.String `tfsdk:"kafka_bootstrap_servers"`
	SnowflakeStorageIntegrationName types.String `tfsdk:"snowflake_storage_integration_name"`
	RedisLightningSupportsHasMany   types.Bool   `tfsdk:"redis_lightning_supports_has_many"`
	Writers                         types.List   `tfsdk:"writers"`
	KubeClusterId                   types.String `tfsdk:"kube_cluster_id"`
}

func (r *ClusterBackgroundPersistenceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_background_persistence"
}

func (r *ClusterBackgroundPersistenceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		DeprecationMessage:  "Use chalk_unmanaged_cluster_background_persistence instead.",
		MarkdownDescription: "~> **Deprecated** Use `chalk_unmanaged_cluster_background_persistence` instead.\n\nChalk cluster background persistence resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Background persistence identifier",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"kube_cluster_id": schema.StringAttribute{
				MarkdownDescription: "Kubernetes cluster ID",
				Required:            true,
			},
			"namespace": schema.StringAttribute{
				MarkdownDescription: "Kubernetes namespace",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"bus_writer_image_go": schema.StringAttribute{
				MarkdownDescription: "Go bus writer image",
				Optional:            true,
			},
			"bus_writer_image_python": schema.StringAttribute{
				MarkdownDescription: "Python bus writer image",
				Optional:            true,
			},
			"bus_writer_image_bswl": schema.StringAttribute{
				MarkdownDescription: "BSWL bus writer image",
				Optional:            true,
			},
			"bus_writer_image_rust": schema.StringAttribute{
				MarkdownDescription: "Rust bus writer image",
				Optional:            true,
			},
			"service_account_name": schema.StringAttribute{
				MarkdownDescription: "Service account name",
				Required:            true,
			},
			"bigquery_parquet_upload_subscription_id": schema.StringAttribute{
				MarkdownDescription: "BigQuery parquet upload subscription ID",
				Optional:            true,
			},
			"bigquery_streaming_write_subscription_id": schema.StringAttribute{
				MarkdownDescription: "BigQuery streaming write subscription ID",
				Optional:            true,
			},
			"bigquery_streaming_write_topic": schema.StringAttribute{
				MarkdownDescription: "BigQuery streaming write topic",
				Optional:            true,
			},
			"bq_upload_bucket": schema.StringAttribute{
				MarkdownDescription: "BigQuery upload bucket",
				Optional:            true,
			},
			"bq_upload_topic": schema.StringAttribute{
				MarkdownDescription: "BigQuery upload topic",
				Optional:            true,
			},
			"google_cloud_project": schema.StringAttribute{
				MarkdownDescription: "Google Cloud project",
				Optional:            true,
			},
			"kafka_dlq_topic": schema.StringAttribute{
				MarkdownDescription: "Kafka DLQ topic",
				Optional:            true,
			},
			"metrics_bus_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Metrics bus subscription ID",
				Optional:            true,
			},
			"metrics_bus_topic_id": schema.StringAttribute{
				MarkdownDescription: "Metrics bus topic ID",
				Optional:            true,
			},
			"operation_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Operation subscription ID",
				Optional:            true,
			},
			"query_log_result_topic": schema.StringAttribute{
				MarkdownDescription: "Query log result topic",
				Optional:            true,
			},
			"query_log_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Query log subscription ID",
				Optional:            true,
			},
			"result_bus_offline_store_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Result bus offline store subscription ID",
				Optional:            true,
			},
			"result_bus_online_store_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Result bus online store subscription ID",
				Optional:            true,
			},
			"result_bus_topic_id": schema.StringAttribute{
				MarkdownDescription: "Result bus topic ID",
				Optional:            true,
			},
			"api_server_host": schema.StringAttribute{
				MarkdownDescription: "API server host",
				Optional:            true,
			},
			"kafka_sasl_secret": schema.StringAttribute{
				MarkdownDescription: "Kafka SASL secret",
				Optional:            true,
			},
			"kafka_bootstrap_servers": schema.StringAttribute{
				MarkdownDescription: "Kafka bootstrap servers",
				Optional:            true,
			},
			"snowflake_storage_integration_name": schema.StringAttribute{
				MarkdownDescription: "Snowflake storage integration name",
				Optional:            true,
			},
			"redis_lightning_supports_has_many": schema.BoolAttribute{
				MarkdownDescription: "Redis lightning supports has many",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"writers": bgpWritersSchemaAttribute,
		},
	}
}

func (r *ClusterBackgroundPersistenceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ClusterBackgroundPersistenceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ClusterBackgroundPersistenceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	// Create builder client
	bc := r.client.NewBuilderClient(ctx)

	// Convert writers
	protoWriters, writerDiags := bgpWritersTFToProto(ctx, data.Writers)
	resp.Diagnostics.Append(writerDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert terraform model to proto request
	commonSpecs := &serverv1.BackgroundPersistenceCommonSpecs{
		Namespace:          data.Namespace.ValueString(),
		ServiceAccountName: data.ServiceAccountName.ValueString(),
	}

	// Handle optional subscription and topic fields
	if !data.BigqueryParquetUploadSubscriptionId.IsNull() {
		commonSpecs.BigqueryParquetUploadSubscriptionId = data.BigqueryParquetUploadSubscriptionId.ValueString()
	}
	if !data.BigqueryStreamingWriteSubscriptionId.IsNull() {
		commonSpecs.BigqueryStreamingWriteSubscriptionId = data.BigqueryStreamingWriteSubscriptionId.ValueString()
	}
	if !data.BigqueryStreamingWriteTopic.IsNull() {
		commonSpecs.BigqueryStreamingWriteTopic = data.BigqueryStreamingWriteTopic.ValueString()
	}
	if !data.BqUploadBucket.IsNull() {
		commonSpecs.BqUploadBucket = data.BqUploadBucket.ValueString()
	}
	if !data.BqUploadTopic.IsNull() {
		commonSpecs.BqUploadTopic = data.BqUploadTopic.ValueString()
	}
	if !data.MetricsBusSubscriptionId.IsNull() {
		commonSpecs.MetricsBusSubscriptionId = data.MetricsBusSubscriptionId.ValueString()
	}
	if !data.MetricsBusTopicId.IsNull() {
		commonSpecs.MetricsBusTopicId = data.MetricsBusTopicId.ValueString()
	}
	if !data.OperationSubscriptionId.IsNull() {
		commonSpecs.OperationSubscriptionId = data.OperationSubscriptionId.ValueString()
	}
	if !data.QueryLogResultTopic.IsNull() {
		commonSpecs.QueryLogResultTopic = data.QueryLogResultTopic.ValueString()
	}
	if !data.QueryLogSubscriptionId.IsNull() {
		commonSpecs.QueryLogSubscriptionId = data.QueryLogSubscriptionId.ValueString()
	}
	if !data.ResultBusOfflineStoreSubscriptionId.IsNull() {
		commonSpecs.ResultBusOfflineStoreSubscriptionId = data.ResultBusOfflineStoreSubscriptionId.ValueString()
	}
	if !data.ResultBusOnlineStoreSubscriptionId.IsNull() {
		commonSpecs.ResultBusOnlineStoreSubscriptionId = data.ResultBusOnlineStoreSubscriptionId.ValueString()
	}
	if !data.ResultBusTopicId.IsNull() {
		commonSpecs.ResultBusTopicId = data.ResultBusTopicId.ValueString()
	}

	// Handle optional google_cloud_project field
	if !data.GoogleCloudProject.IsNull() {
		commonSpecs.GoogleCloudProject = data.GoogleCloudProject.ValueString()
	}

	if !data.KafkaDlqTopic.IsNull() {
		commonSpecs.KafkaDlqTopic = data.KafkaDlqTopic.ValueString()
	}

	// Handle optional fields
	if !data.BusWriterImageGo.IsNull() {
		commonSpecs.BusWriterImageGo = data.BusWriterImageGo.ValueString()
	}
	if !data.BusWriterImagePython.IsNull() {
		commonSpecs.BusWriterImagePython = data.BusWriterImagePython.ValueString()
	}
	if !data.BusWriterImageBswl.IsNull() {
		commonSpecs.BusWriterImageBswl = data.BusWriterImageBswl.ValueString()
	}

	deploymentSpecs := &serverv1.BackgroundPersistenceDeploymentSpecs{
		CommonPersistenceSpecs: commonSpecs,
		Writers:                protoWriters,
	}

	// Handle optional deployment-level fields
	if !data.ApiServerHost.IsNull() {
		deploymentSpecs.ApiServerHost = data.ApiServerHost.ValueString()
	}
	if !data.KafkaSaslSecret.IsNull() {
		deploymentSpecs.KafkaSaslSecret = data.KafkaSaslSecret.ValueString()
	}
	if !data.KafkaBootstrapServers.IsNull() {
		deploymentSpecs.KafkaBootstrapServers = data.KafkaBootstrapServers.ValueString()
	}
	if !data.SnowflakeStorageIntegrationName.IsNull() {
		deploymentSpecs.SnowflakeStorageIntegrationName = data.SnowflakeStorageIntegrationName.ValueString()
	}
	if !data.RedisLightningSupportsHasMany.IsNull() {
		deploymentSpecs.RedisLightningSupportsHasMany = data.RedisLightningSupportsHasMany.ValueBool()
	}

	createReq := &serverv1.CreateClusterBackgroundPersistenceRequest{
		Specs:         deploymentSpecs,
		KubeClusterId: data.KubeClusterId.ValueStringPointer(),
	}

	// Support upsert by setting ID if it exists
	if !data.Id.IsNull() {
		createReq.Id = data.Id.ValueStringPointer()
	}

	// Set optional rust image
	if !data.BusWriterImageRust.IsNull() {
		createReq.Specs.CommonPersistenceSpecs.BusWriterImageRust = data.BusWriterImageRust.ValueString()
	}

	response, err := bc.CreateClusterBackgroundPersistence(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Cluster Background Persistence",
			fmt.Sprintf("Could not create cluster background persistence: %v", err),
		)
		return
	}

	data.Id = types.StringValue(response.Msg.Id)

	tflog.Trace(ctx, "created a chalk_cluster_background_persistence resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterBackgroundPersistenceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ClusterBackgroundPersistenceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	// Create builder client
	bc := r.client.NewBuilderClient(ctx)

	getReq := &serverv1.GetClusterBackgroundPersistenceRequest{
		Id: data.Id.ValueStringPointer(),
	}

	bgPersistence, err := bc.GetClusterBackgroundPersistence(ctx, connect.NewRequest(getReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Chalk Cluster Background Persistence",
			fmt.Sprintf("Could not read cluster background persistence %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Update the model with the fetched data
	bg := bgPersistence.Msg.BackgroundPersistence

	// Set kube_cluster_id
	if bg.GetKubeClusterId() != "" {
		data.KubeClusterId = types.StringValue(bg.GetKubeClusterId())
	} else {
		data.KubeClusterId = types.StringNull()
	}

	// Update specs if available
	if bg.Specs != nil && bg.Specs.CommonPersistenceSpecs != nil {
		common := bg.Specs.CommonPersistenceSpecs
		data.Namespace = types.StringValue(common.Namespace)
		data.ServiceAccountName = types.StringValue(common.ServiceAccountName)

		// Handle optional subscription and topic fields
		if common.BigqueryParquetUploadSubscriptionId != "" {
			data.BigqueryParquetUploadSubscriptionId = types.StringValue(common.BigqueryParquetUploadSubscriptionId)
		} else {
			data.BigqueryParquetUploadSubscriptionId = types.StringNull()
		}
		if common.BigqueryStreamingWriteSubscriptionId != "" {
			data.BigqueryStreamingWriteSubscriptionId = types.StringValue(common.BigqueryStreamingWriteSubscriptionId)
		} else {
			data.BigqueryStreamingWriteSubscriptionId = types.StringNull()
		}
		if common.BigqueryStreamingWriteTopic != "" {
			data.BigqueryStreamingWriteTopic = types.StringValue(common.BigqueryStreamingWriteTopic)
		} else {
			data.BigqueryStreamingWriteTopic = types.StringNull()
		}
		if common.BqUploadBucket != "" {
			data.BqUploadBucket = types.StringValue(common.BqUploadBucket)
		} else {
			data.BqUploadBucket = types.StringNull()
		}
		if common.BqUploadTopic != "" {
			data.BqUploadTopic = types.StringValue(common.BqUploadTopic)
		} else {
			data.BqUploadTopic = types.StringNull()
		}
		if common.MetricsBusSubscriptionId != "" {
			data.MetricsBusSubscriptionId = types.StringValue(common.MetricsBusSubscriptionId)
		} else {
			data.MetricsBusSubscriptionId = types.StringNull()
		}
		if common.MetricsBusTopicId != "" {
			data.MetricsBusTopicId = types.StringValue(common.MetricsBusTopicId)
		} else {
			data.MetricsBusTopicId = types.StringNull()
		}
		if common.OperationSubscriptionId != "" {
			data.OperationSubscriptionId = types.StringValue(common.OperationSubscriptionId)
		} else {
			data.OperationSubscriptionId = types.StringNull()
		}
		if common.QueryLogResultTopic != "" {
			data.QueryLogResultTopic = types.StringValue(common.QueryLogResultTopic)
		} else {
			data.QueryLogResultTopic = types.StringNull()
		}
		if common.QueryLogSubscriptionId != "" {
			data.QueryLogSubscriptionId = types.StringValue(common.QueryLogSubscriptionId)
		} else {
			data.QueryLogSubscriptionId = types.StringNull()
		}
		if common.ResultBusOfflineStoreSubscriptionId != "" {
			data.ResultBusOfflineStoreSubscriptionId = types.StringValue(common.ResultBusOfflineStoreSubscriptionId)
		} else {
			data.ResultBusOfflineStoreSubscriptionId = types.StringNull()
		}
		if common.ResultBusOnlineStoreSubscriptionId != "" {
			data.ResultBusOnlineStoreSubscriptionId = types.StringValue(common.ResultBusOnlineStoreSubscriptionId)
		} else {
			data.ResultBusOnlineStoreSubscriptionId = types.StringNull()
		}
		if common.ResultBusTopicId != "" {
			data.ResultBusTopicId = types.StringValue(common.ResultBusTopicId)
		} else {
			data.ResultBusTopicId = types.StringNull()
		}

		if common.KafkaDlqTopic != "" {
			data.KafkaDlqTopic = types.StringValue(common.KafkaDlqTopic)
		} else {
			data.KafkaDlqTopic = types.StringNull()
		}

		// Handle optional google_cloud_project field
		if common.GoogleCloudProject != "" {
			data.GoogleCloudProject = types.StringValue(common.GoogleCloudProject)
		} else {
			data.GoogleCloudProject = types.StringNull()
		}

		// Handle optional fields - set to null if empty, otherwise set the value
		if common.BusWriterImageGo != "" {
			data.BusWriterImageGo = types.StringValue(common.BusWriterImageGo)
		} else {
			data.BusWriterImageGo = types.StringNull()
		}
		if common.BusWriterImagePython != "" {
			data.BusWriterImagePython = types.StringValue(common.BusWriterImagePython)
		} else {
			data.BusWriterImagePython = types.StringNull()
		}
		if common.BusWriterImageBswl != "" {
			data.BusWriterImageBswl = types.StringValue(common.BusWriterImageBswl)
		} else {
			data.BusWriterImageBswl = types.StringNull()
		}
		if common.BusWriterImageRust != "" {
			data.BusWriterImageRust = types.StringValue(common.BusWriterImageRust)
		} else {
			data.BusWriterImageRust = types.StringNull()
		}

		// Handle optional deployment fields
		if bg.Specs.ApiServerHost != "" {
			data.ApiServerHost = types.StringValue(bg.Specs.ApiServerHost)
		} else {
			data.ApiServerHost = types.StringNull()
		}
		if bg.Specs.KafkaSaslSecret != "" {
			data.KafkaSaslSecret = types.StringValue(bg.Specs.KafkaSaslSecret)
		} else {
			data.KafkaSaslSecret = types.StringNull()
		}
		if bg.Specs.KafkaBootstrapServers != "" {
			data.KafkaBootstrapServers = types.StringValue(bg.Specs.KafkaBootstrapServers)
		} else {
			data.KafkaBootstrapServers = types.StringNull()
		}
		if bg.Specs.SnowflakeStorageIntegrationName != "" {
			data.SnowflakeStorageIntegrationName = types.StringValue(bg.Specs.SnowflakeStorageIntegrationName)
		} else {
			data.SnowflakeStorageIntegrationName = types.StringNull()
		}
		data.RedisLightningSupportsHasMany = types.BoolValue(bg.Specs.RedisLightningSupportsHasMany)

		// Update writers
		writersList, writerDiags := bgpWritersProtoToTF(ctx, bg.Specs.Writers)
		resp.Diagnostics.Append(writerDiags...)
		if !resp.Diagnostics.HasError() {
			data.Writers = writersList
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterBackgroundPersistenceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ClusterBackgroundPersistenceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	// Create builder client
	bc := r.client.NewBuilderClient(ctx)

	// Convert writers
	protoWriters, writerDiags := bgpWritersTFToProto(ctx, data.Writers)
	resp.Diagnostics.Append(writerDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert terraform model to proto request - reuse create logic since it's an upsert
	commonSpecs := &serverv1.BackgroundPersistenceCommonSpecs{
		Namespace:          data.Namespace.ValueString(),
		ServiceAccountName: data.ServiceAccountName.ValueString(),
	}

	// Handle optional subscription and topic fields
	if !data.BigqueryParquetUploadSubscriptionId.IsNull() {
		commonSpecs.BigqueryParquetUploadSubscriptionId = data.BigqueryParquetUploadSubscriptionId.ValueString()
	}
	if !data.BigqueryStreamingWriteSubscriptionId.IsNull() {
		commonSpecs.BigqueryStreamingWriteSubscriptionId = data.BigqueryStreamingWriteSubscriptionId.ValueString()
	}
	if !data.BigqueryStreamingWriteTopic.IsNull() {
		commonSpecs.BigqueryStreamingWriteTopic = data.BigqueryStreamingWriteTopic.ValueString()
	}
	if !data.BqUploadBucket.IsNull() {
		commonSpecs.BqUploadBucket = data.BqUploadBucket.ValueString()
	}
	if !data.BqUploadTopic.IsNull() {
		commonSpecs.BqUploadTopic = data.BqUploadTopic.ValueString()
	}
	if !data.MetricsBusSubscriptionId.IsNull() {
		commonSpecs.MetricsBusSubscriptionId = data.MetricsBusSubscriptionId.ValueString()
	}
	if !data.MetricsBusTopicId.IsNull() {
		commonSpecs.MetricsBusTopicId = data.MetricsBusTopicId.ValueString()
	}
	if !data.OperationSubscriptionId.IsNull() {
		commonSpecs.OperationSubscriptionId = data.OperationSubscriptionId.ValueString()
	}
	if !data.QueryLogResultTopic.IsNull() {
		commonSpecs.QueryLogResultTopic = data.QueryLogResultTopic.ValueString()
	}
	if !data.QueryLogSubscriptionId.IsNull() {
		commonSpecs.QueryLogSubscriptionId = data.QueryLogSubscriptionId.ValueString()
	}
	if !data.ResultBusOfflineStoreSubscriptionId.IsNull() {
		commonSpecs.ResultBusOfflineStoreSubscriptionId = data.ResultBusOfflineStoreSubscriptionId.ValueString()
	}
	if !data.ResultBusOnlineStoreSubscriptionId.IsNull() {
		commonSpecs.ResultBusOnlineStoreSubscriptionId = data.ResultBusOnlineStoreSubscriptionId.ValueString()
	}
	if !data.ResultBusTopicId.IsNull() {
		commonSpecs.ResultBusTopicId = data.ResultBusTopicId.ValueString()
	}

	// Handle optional google_cloud_project field
	if !data.GoogleCloudProject.IsNull() {
		commonSpecs.GoogleCloudProject = data.GoogleCloudProject.ValueString()
	}

	if !data.KafkaDlqTopic.IsNull() {
		commonSpecs.KafkaDlqTopic = data.KafkaDlqTopic.ValueString()
	}

	// Handle optional fields
	if !data.BusWriterImageGo.IsNull() {
		commonSpecs.BusWriterImageGo = data.BusWriterImageGo.ValueString()
	}
	if !data.BusWriterImagePython.IsNull() {
		commonSpecs.BusWriterImagePython = data.BusWriterImagePython.ValueString()
	}
	if !data.BusWriterImageBswl.IsNull() {
		commonSpecs.BusWriterImageBswl = data.BusWriterImageBswl.ValueString()
	}

	deploymentSpecs := &serverv1.BackgroundPersistenceDeploymentSpecs{
		CommonPersistenceSpecs: commonSpecs,
		Writers:                protoWriters,
	}

	// Handle optional deployment-level fields
	if !data.ApiServerHost.IsNull() {
		deploymentSpecs.ApiServerHost = data.ApiServerHost.ValueString()
	}
	if !data.KafkaSaslSecret.IsNull() {
		deploymentSpecs.KafkaSaslSecret = data.KafkaSaslSecret.ValueString()
	}
	if !data.KafkaBootstrapServers.IsNull() {
		deploymentSpecs.KafkaBootstrapServers = data.KafkaBootstrapServers.ValueString()
	}
	if !data.SnowflakeStorageIntegrationName.IsNull() {
		deploymentSpecs.SnowflakeStorageIntegrationName = data.SnowflakeStorageIntegrationName.ValueString()
	}
	if !data.RedisLightningSupportsHasMany.IsNull() {
		deploymentSpecs.RedisLightningSupportsHasMany = data.RedisLightningSupportsHasMany.ValueBool()
	}

	createReq := &serverv1.CreateClusterBackgroundPersistenceRequest{
		Specs:         deploymentSpecs,
		KubeClusterId: data.KubeClusterId.ValueStringPointer(),
	}

	// Use the known ID from current state for the upsert
	createReq.Id = data.Id.ValueStringPointer()

	// Set optional rust image
	if !data.BusWriterImageRust.IsNull() {
		createReq.Specs.CommonPersistenceSpecs.BusWriterImageRust = data.BusWriterImageRust.ValueString()
	}

	response, err := bc.CreateClusterBackgroundPersistence(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Chalk Cluster Background Persistence",
			fmt.Sprintf("Could not update cluster background persistence: %v", err),
		)
		return
	}

	data.Id = types.StringValue(response.Msg.Id)

	tflog.Trace(ctx, "updated a chalk_cluster_background_persistence resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterBackgroundPersistenceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Note: According to the proto definition, there's no DeleteClusterBackgroundPersistence method
	// This means the background persistence lifecycle might be managed differently
	// For now, we'll just remove it from Terraform state
	tflog.Trace(ctx, "cluster background persistence deletion - removing from terraform state only (no API delete available)")
}

func (r *ClusterBackgroundPersistenceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
