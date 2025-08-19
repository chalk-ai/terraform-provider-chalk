package provider

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"net/http"
)

var _ resource.Resource = &ClusterBackgroundPersistenceResource{}
var _ resource.ResourceWithImportState = &ClusterBackgroundPersistenceResource{}

func NewClusterBackgroundPersistenceResource() resource.Resource {
	return &ClusterBackgroundPersistenceResource{}
}

type ClusterBackgroundPersistenceResource struct {
	client *ChalkClient
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
	UsageStoreUri                            types.String                         `tfsdk:"usage_store_uri"`
	ResultsWriterSkipProducingFeatureMetrics types.Bool                           `tfsdk:"results_writer_skip_producing_feature_metrics"`
	QueryTableWriteDropRatio                 types.String                         `tfsdk:"query_table_write_drop_ratio"`
}

type ClusterBackgroundPersistenceResourceModel struct {
	Id                                   types.String `tfsdk:"id"`
	EnvironmentIds                       types.List   `tfsdk:"environment_ids"`
	Kind                                 types.String `tfsdk:"kind"`
	Namespace                            types.String `tfsdk:"namespace"`
	BusWriterImageGo                     types.String `tfsdk:"bus_writer_image_go"`
	BusWriterImagePython                 types.String `tfsdk:"bus_writer_image_python"`
	BusWriterImageBswl                   types.String `tfsdk:"bus_writer_image_bswl"`
	BusWriterImageRust                   types.String `tfsdk:"bus_writer_image_rust"`
	ServiceAccountName                   types.String `tfsdk:"service_account_name"`
	BusBackend                           types.String `tfsdk:"bus_backend"`
	SecretClient                         types.String `tfsdk:"secret_client"`
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
	ResultBusMetricsSubscriptionId       types.String `tfsdk:"result_bus_metrics_subscription_id"`
	ResultBusOfflineStoreSubscriptionId  types.String `tfsdk:"result_bus_offline_store_subscription_id"`
	ResultBusOnlineStoreSubscriptionId   types.String `tfsdk:"result_bus_online_store_subscription_id"`
	ResultBusTopicId                     types.String `tfsdk:"result_bus_topic_id"`
	UsageBusTopicId                      types.String `tfsdk:"usage_bus_topic_id"`
	UsageEventsSubscriptionId            types.String `tfsdk:"usage_events_subscription_id"`
	IncludeChalkNodeSelector             types.Bool   `tfsdk:"include_chalk_node_selector"`
	ApiServerHost                        types.String `tfsdk:"api_server_host"`
	KafkaSaslSecret                      types.String `tfsdk:"kafka_sasl_secret"`
	MetadataProvider                     types.String `tfsdk:"metadata_provider"`
	KafkaBootstrapServers                types.String `tfsdk:"kafka_bootstrap_servers"`
	KafkaSecurityProtocol                types.String `tfsdk:"kafka_security_protocol"`
	KafkaSaslMechanism                   types.String `tfsdk:"kafka_sasl_mechanism"`
	RedisIsClustered                     types.String `tfsdk:"redis_is_clustered"`
	SnowflakeStorageIntegrationName      types.String `tfsdk:"snowflake_storage_integration_name"`
	RedisLightningSupportsHasMany        types.Bool   `tfsdk:"redis_lightning_supports_has_many"`
	Insecure                             types.Bool   `tfsdk:"insecure"`
	Writers                              types.List   `tfsdk:"writers"`
	CreatedAt                            types.String `tfsdk:"created_at"`
	UpdatedAt                            types.String `tfsdk:"updated_at"`
}

func (r *ClusterBackgroundPersistenceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_background_persistence"
}

func (r *ClusterBackgroundPersistenceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	kubeResourceConfigSchema := schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"cpu": schema.StringAttribute{
				MarkdownDescription: "CPU resource specification",
				Optional:            true,
			},
			"memory": schema.StringAttribute{
				MarkdownDescription: "Memory resource specification",
				Optional:            true,
			},
			"ephemeral_storage": schema.StringAttribute{
				MarkdownDescription: "Ephemeral storage resource specification",
				Optional:            true,
			},
			"storage": schema.StringAttribute{
				MarkdownDescription: "Storage resource specification",
				Optional:            true,
			},
		},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk cluster background persistence resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Background persistence identifier",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_ids": schema.ListAttribute{
				MarkdownDescription: "List of environment IDs for the background persistence",
				Required:            true,
				ElementType:         types.StringType,
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Background persistence kind",
				Computed:            true,
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
				Required:            true,
			},
			"bus_writer_image_python": schema.StringAttribute{
				MarkdownDescription: "Python bus writer image",
				Required:            true,
			},
			"bus_writer_image_bswl": schema.StringAttribute{
				MarkdownDescription: "BSWL bus writer image",
				Required:            true,
			},
			"bus_writer_image_rust": schema.StringAttribute{
				MarkdownDescription: "Rust bus writer image",
				Optional:            true,
			},
			"service_account_name": schema.StringAttribute{
				MarkdownDescription: "Service account name",
				Required:            true,
			},
			"bus_backend": schema.StringAttribute{
				MarkdownDescription: "Bus backend",
				Required:            true,
			},
			"secret_client": schema.StringAttribute{
				MarkdownDescription: "Secret client",
				Required:            true,
			},
			"bigquery_parquet_upload_subscription_id": schema.StringAttribute{
				MarkdownDescription: "BigQuery parquet upload subscription ID",
				Required:            true,
			},
			"bigquery_streaming_write_subscription_id": schema.StringAttribute{
				MarkdownDescription: "BigQuery streaming write subscription ID",
				Required:            true,
			},
			"bigquery_streaming_write_topic": schema.StringAttribute{
				MarkdownDescription: "BigQuery streaming write topic",
				Required:            true,
			},
			"bq_upload_bucket": schema.StringAttribute{
				MarkdownDescription: "BigQuery upload bucket",
				Required:            true,
			},
			"bq_upload_topic": schema.StringAttribute{
				MarkdownDescription: "BigQuery upload topic",
				Required:            true,
			},
			"google_cloud_project": schema.StringAttribute{
				MarkdownDescription: "Google Cloud project",
				Required:            true,
			},
			"kafka_dlq_topic": schema.StringAttribute{
				MarkdownDescription: "Kafka DLQ topic",
				Required:            true,
			},
			"metrics_bus_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Metrics bus subscription ID",
				Required:            true,
			},
			"metrics_bus_topic_id": schema.StringAttribute{
				MarkdownDescription: "Metrics bus topic ID",
				Required:            true,
			},
			"operation_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Operation subscription ID",
				Required:            true,
			},
			"query_log_result_topic": schema.StringAttribute{
				MarkdownDescription: "Query log result topic",
				Required:            true,
			},
			"query_log_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Query log subscription ID",
				Required:            true,
			},
			"result_bus_metrics_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Result bus metrics subscription ID",
				Required:            true,
			},
			"result_bus_offline_store_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Result bus offline store subscription ID",
				Required:            true,
			},
			"result_bus_online_store_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Result bus online store subscription ID",
				Required:            true,
			},
			"result_bus_topic_id": schema.StringAttribute{
				MarkdownDescription: "Result bus topic ID",
				Required:            true,
			},
			"usage_bus_topic_id": schema.StringAttribute{
				MarkdownDescription: "Usage bus topic ID",
				Required:            true,
			},
			"usage_events_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Usage events subscription ID",
				Required:            true,
			},
			"include_chalk_node_selector": schema.BoolAttribute{
				MarkdownDescription: "Whether to include chalk node selector",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"api_server_host": schema.StringAttribute{
				MarkdownDescription: "API server host",
				Required:            true,
			},
			"kafka_sasl_secret": schema.StringAttribute{
				MarkdownDescription: "Kafka SASL secret",
				Required:            true,
			},
			"metadata_provider": schema.StringAttribute{
				MarkdownDescription: "Metadata provider",
				Required:            true,
			},
			"kafka_bootstrap_servers": schema.StringAttribute{
				MarkdownDescription: "Kafka bootstrap servers",
				Required:            true,
			},
			"kafka_security_protocol": schema.StringAttribute{
				MarkdownDescription: "Kafka security protocol",
				Required:            true,
			},
			"kafka_sasl_mechanism": schema.StringAttribute{
				MarkdownDescription: "Kafka SASL mechanism",
				Required:            true,
			},
			"redis_is_clustered": schema.StringAttribute{
				MarkdownDescription: "Redis is clustered",
				Required:            true,
			},
			"snowflake_storage_integration_name": schema.StringAttribute{
				MarkdownDescription: "Snowflake storage integration name",
				Required:            true,
			},
			"redis_lightning_supports_has_many": schema.BoolAttribute{
				MarkdownDescription: "Redis lightning supports has many",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"insecure": schema.BoolAttribute{
				MarkdownDescription: "Insecure mode",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"writers": schema.ListNestedAttribute{
				MarkdownDescription: "Background persistence writers",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Writer name",
							Required:            true,
						},
						"image_override": schema.StringAttribute{
							MarkdownDescription: "Image override",
							Required:            true,
						},
						"hpa_specs": schema.SingleNestedAttribute{
							MarkdownDescription: "HPA specifications",
							Optional:            true,
							Attributes: map[string]schema.Attribute{
								"hpa_pubsub_subscription_id": schema.StringAttribute{
									MarkdownDescription: "HPA pubsub subscription ID",
									Required:            true,
								},
								"hpa_min_replicas": schema.Int64Attribute{
									MarkdownDescription: "HPA minimum replicas",
									Optional:            true,
									Computed:            true,
									Default:             int64default.StaticInt64(1),
								},
								"hpa_max_replicas": schema.Int64Attribute{
									MarkdownDescription: "HPA maximum replicas",
									Optional:            true,
									Computed:            true,
									Default:             int64default.StaticInt64(10),
								},
								"hpa_target_average_value": schema.Int64Attribute{
									MarkdownDescription: "HPA target average value",
									Optional:            true,
									Computed:            true,
									Default:             int64default.StaticInt64(5),
								},
							},
						},
						"gke_spot": schema.BoolAttribute{
							MarkdownDescription: "GKE spot instances",
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(false),
						},
						"load_writer_configmap": schema.BoolAttribute{
							MarkdownDescription: "Load writer configmap",
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(false),
						},
						"version": schema.StringAttribute{
							MarkdownDescription: "Writer version",
							Required:            true,
						},
						"request": schema.SingleNestedAttribute{
							MarkdownDescription: "Resource requests",
							Optional:            true,
							Attributes:          kubeResourceConfigSchema.Attributes,
						},
						"limit": schema.SingleNestedAttribute{
							MarkdownDescription: "Resource limits",
							Optional:            true,
							Attributes:          kubeResourceConfigSchema.Attributes,
						},
						"bus_subscriber_type": schema.StringAttribute{
							MarkdownDescription: "Bus subscriber type",
							Required:            true,
						},
						"default_replica_count": schema.Int64Attribute{
							MarkdownDescription: "Default replica count",
							Optional:            true,
							Computed:            true,
							Default:             int64default.StaticInt64(1),
						},
						"kafka_consumer_group_override": schema.StringAttribute{
							MarkdownDescription: "Kafka consumer group override",
							Optional:            true,
						},
						"max_batch_size": schema.Int64Attribute{
							MarkdownDescription: "Maximum batch size",
							Optional:            true,
						},
						"message_processing_concurrency": schema.Int64Attribute{
							MarkdownDescription: "Message processing concurrency",
							Optional:            true,
						},
						"metadata_sql_ssl_ca_cert_secret": schema.StringAttribute{
							MarkdownDescription: "Metadata SQL SSL CA cert secret",
							Required:            true,
						},
						"metadata_sql_ssl_client_cert_secret": schema.StringAttribute{
							MarkdownDescription: "Metadata SQL SSL client cert secret",
							Required:            true,
						},
						"metadata_sql_ssl_client_key_secret": schema.StringAttribute{
							MarkdownDescription: "Metadata SQL SSL client key secret",
							Required:            true,
						},
						"metadata_sql_uri_secret": schema.StringAttribute{
							MarkdownDescription: "Metadata SQL URI secret",
							Required:            true,
						},
						"offline_store_inserter_db_type": schema.StringAttribute{
							MarkdownDescription: "Offline store inserter DB type",
							Required:            true,
						},
						"storage_cache_prefix": schema.StringAttribute{
							MarkdownDescription: "Storage cache prefix",
							Required:            true,
						},
						"usage_store_uri": schema.StringAttribute{
							MarkdownDescription: "Usage store URI",
							Required:            true,
						},
						"results_writer_skip_producing_feature_metrics": schema.BoolAttribute{
							MarkdownDescription: "Results writer skip producing feature metrics",
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(false),
						},
						"query_table_write_drop_ratio": schema.StringAttribute{
							MarkdownDescription: "Query table write drop ratio",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("0.0"),
						},
					},
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp",
				Computed:            true,
			},
		},
	}
}

func (r *ClusterBackgroundPersistenceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ChalkClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ChalkClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
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
	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         r.client.ApiServer,
			interceptors: []connect.Interceptor{MakeApiServerHeaderInterceptor("x-chalk-server", "go-api")},
		},
	)

	// Create builder client with token injection interceptor
	bc := NewBuilderClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	})

	// Convert environment IDs
	var envIds []string
	diags := data.EnvironmentIds.ElementsAs(ctx, &envIds, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert writers
	var writers []BackgroundPersistenceWriterModel
	diags = data.Writers.ElementsAs(ctx, &writers, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var protoWriters []*serverv1.BackgroundPersistenceWriterSpecs
	for _, writer := range writers {
		protoWriter := &serverv1.BackgroundPersistenceWriterSpecs{
			Name:                           writer.Name.ValueString(),
			ImageOverride:                  writer.ImageOverride.ValueString(),
			Version:                        writer.Version.ValueString(),
			BusSubscriberType:              writer.BusSubscriberType.ValueString(),
			DefaultReplicaCount:            int32(writer.DefaultReplicaCount.ValueInt64()),
			MetadataSqlSslCaCertSecret:     writer.MetadataSqlSslCaCertSecret.ValueString(),
			MetadataSqlSslClientCertSecret: writer.MetadataSqlSslClientCertSecret.ValueString(),
			MetadataSqlSslClientKeySecret:  writer.MetadataSqlSslClientKeySecret.ValueString(),
			MetadataSqlUriSecret:           writer.MetadataSqlUriSecret.ValueString(),
			OfflineStoreInserterDbType:     writer.OfflineStoreInserterDbType.ValueString(),
			StorageCachePrefix:             writer.StorageCachePrefix.ValueString(),
			UsageStoreUri:                  writer.UsageStoreUri.ValueString(),
			QueryTableWriteDropRatio:       writer.QueryTableWriteDropRatio.ValueString(),
		}

		// Convert optional fields
		if !writer.GkeSpot.IsNull() {
			val := writer.GkeSpot.ValueBool()
			protoWriter.GkeSpot = &val
		}
		if !writer.LoadWriterConfigmap.IsNull() {
			val := writer.LoadWriterConfigmap.ValueBool()
			protoWriter.LoadWriterConfigmap = &val
		}
		if !writer.KafkaConsumerGroupOverride.IsNull() {
			protoWriter.KafkaConsumerGroupOverride = writer.KafkaConsumerGroupOverride.ValueString()
		}
		if !writer.MaxBatchSize.IsNull() {
			val := int32(writer.MaxBatchSize.ValueInt64())
			protoWriter.MaxBatchSize = &val
		}
		if !writer.MessageProcessingConcurrency.IsNull() {
			val := int32(writer.MessageProcessingConcurrency.ValueInt64())
			protoWriter.MessageProcessingConcurrency = &val
		}
		if !writer.ResultsWriterSkipProducingFeatureMetrics.IsNull() {
			val := writer.ResultsWriterSkipProducingFeatureMetrics.ValueBool()
			protoWriter.ResultsWriterSkipProducingFeatureMetrics = &val
		}

		// Convert HPA specs
		if writer.HpaSpecs != nil {
			protoWriter.HpaSpecs = &serverv1.BackgroundPersistenceWriterHpaSpecs{
				HpaPubsubSubscriptionId: writer.HpaSpecs.HpaPubsubSubscriptionId.ValueString(),
			}
			if !writer.HpaSpecs.HpaMinReplicas.IsNull() {
				val := int32(writer.HpaSpecs.HpaMinReplicas.ValueInt64())
				protoWriter.HpaSpecs.HpaMinReplicas = &val
			}
			if !writer.HpaSpecs.HpaMaxReplicas.IsNull() {
				val := int32(writer.HpaSpecs.HpaMaxReplicas.ValueInt64())
				protoWriter.HpaSpecs.HpaMaxReplicas = &val
			}
			if !writer.HpaSpecs.HpaTargetAverageValue.IsNull() {
				val := int32(writer.HpaSpecs.HpaTargetAverageValue.ValueInt64())
				protoWriter.HpaSpecs.HpaTargetAverageValue = &val
			}
		}

		// Convert resource configs
		if writer.Request != nil {
			protoWriter.Request = &serverv1.KubeResourceConfig{
				Cpu:              writer.Request.CPU.ValueString(),
				Memory:           writer.Request.Memory.ValueString(),
				EphemeralStorage: writer.Request.EphemeralStorage.ValueString(),
				Storage:          writer.Request.Storage.ValueString(),
			}
		}
		if writer.Limit != nil {
			protoWriter.Limit = &serverv1.KubeResourceConfig{
				Cpu:              writer.Limit.CPU.ValueString(),
				Memory:           writer.Limit.Memory.ValueString(),
				EphemeralStorage: writer.Limit.EphemeralStorage.ValueString(),
				Storage:          writer.Limit.Storage.ValueString(),
			}
		}

		protoWriters = append(protoWriters, protoWriter)
	}

	// Convert terraform model to proto request
	createReq := &serverv1.CreateClusterBackgroundPersistenceRequest{
		EnvironmentIds: envIds,
		Specs: &serverv1.BackgroundPersistenceDeploymentSpecs{
			CommonPersistenceSpecs: &serverv1.BackgroundPersistenceCommonSpecs{
				Namespace:                            data.Namespace.ValueString(),
				BusWriterImageGo:                     data.BusWriterImageGo.ValueString(),
				BusWriterImagePython:                 data.BusWriterImagePython.ValueString(),
				BusWriterImageBswl:                   data.BusWriterImageBswl.ValueString(),
				ServiceAccountName:                   data.ServiceAccountName.ValueString(),
				BusBackend:                           data.BusBackend.ValueString(),
				SecretClient:                         data.SecretClient.ValueString(),
				BigqueryParquetUploadSubscriptionId:  data.BigqueryParquetUploadSubscriptionId.ValueString(),
				BigqueryStreamingWriteSubscriptionId: data.BigqueryStreamingWriteSubscriptionId.ValueString(),
				BigqueryStreamingWriteTopic:          data.BigqueryStreamingWriteTopic.ValueString(),
				BqUploadBucket:                       data.BqUploadBucket.ValueString(),
				BqUploadTopic:                        data.BqUploadTopic.ValueString(),
				GoogleCloudProject:                   data.GoogleCloudProject.ValueString(),
				KafkaDlqTopic:                        data.KafkaDlqTopic.ValueString(),
				MetricsBusSubscriptionId:             data.MetricsBusSubscriptionId.ValueString(),
				MetricsBusTopicId:                    data.MetricsBusTopicId.ValueString(),
				OperationSubscriptionId:              data.OperationSubscriptionId.ValueString(),
				QueryLogResultTopic:                  data.QueryLogResultTopic.ValueString(),
				QueryLogSubscriptionId:               data.QueryLogSubscriptionId.ValueString(),
				ResultBusMetricsSubscriptionId:       data.ResultBusMetricsSubscriptionId.ValueString(),
				ResultBusOfflineStoreSubscriptionId:  data.ResultBusOfflineStoreSubscriptionId.ValueString(),
				ResultBusOnlineStoreSubscriptionId:   data.ResultBusOnlineStoreSubscriptionId.ValueString(),
				ResultBusTopicId:                     data.ResultBusTopicId.ValueString(),
				UsageBusTopicId:                      data.UsageBusTopicId.ValueString(),
				UsageEventsSubscriptionId:            data.UsageEventsSubscriptionId.ValueString(),
				IncludeChalkNodeSelector:             data.IncludeChalkNodeSelector.ValueBool(),
			},
			ApiServerHost:                   data.ApiServerHost.ValueString(),
			KafkaSaslSecret:                 data.KafkaSaslSecret.ValueString(),
			MetadataProvider:                data.MetadataProvider.ValueString(),
			KafkaBootstrapServers:           data.KafkaBootstrapServers.ValueString(),
			KafkaSecurityProtocol:           data.KafkaSecurityProtocol.ValueString(),
			KafkaSaslMechanism:              data.KafkaSaslMechanism.ValueString(),
			RedisIsClustered:                data.RedisIsClustered.ValueString(),
			SnowflakeStorageIntegrationName: data.SnowflakeStorageIntegrationName.ValueString(),
			RedisLightningSupportsHasMany:   data.RedisLightningSupportsHasMany.ValueBool(),
			Insecure:                        data.Insecure.ValueBool(),
			Writers:                         protoWriters,
		},
	}

	// Set optional rust image
	if !data.BusWriterImageRust.IsNull() {
		createReq.Specs.CommonPersistenceSpecs.BusWriterImageRust = data.BusWriterImageRust.ValueString()
	}

	_, err := bc.CreateClusterBackgroundPersistence(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Cluster Background Persistence",
			fmt.Sprintf("Could not create cluster background persistence: %v", err),
		)
		return
	}

	// Since CreateClusterBackgroundPersistence doesn't return the created resource, we need to get it
	// We'll use the first environment ID to query for the background persistence
	if len(envIds) > 0 {
		getReq := &serverv1.GetClusterBackgroundPersistenceRequest{
			EnvironmentId: envIds[0],
		}

		bgPersistence, err := bc.GetClusterBackgroundPersistence(ctx, connect.NewRequest(getReq))
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Created Chalk Cluster Background Persistence",
				fmt.Sprintf("Background persistence was created but could not be read: %v", err),
			)
			return
		}

		// Update with created values
		data.Id = types.StringValue(bgPersistence.Msg.BackgroundPersistence.Id)
		data.Kind = types.StringValue(bgPersistence.Msg.BackgroundPersistence.Kind)
		if bgPersistence.Msg.BackgroundPersistence.CreatedAt != nil {
			data.CreatedAt = types.StringValue(bgPersistence.Msg.BackgroundPersistence.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"))
		}
		if bgPersistence.Msg.BackgroundPersistence.UpdatedAt != nil {
			data.UpdatedAt = types.StringValue(bgPersistence.Msg.BackgroundPersistence.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z"))
		}
	}

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
	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         r.client.ApiServer,
			interceptors: []connect.Interceptor{MakeApiServerHeaderInterceptor("x-chalk-server", "go-api")},
		},
	)

	// Create builder client with token injection interceptor
	bc := NewBuilderClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	})

	// Get the first environment ID to query the background persistence
	var envIds []string
	diags := data.EnvironmentIds.ElementsAs(ctx, &envIds, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(envIds) == 0 {
		resp.Diagnostics.AddError(
			"No Environment IDs",
			"No environment IDs found in state for reading cluster background persistence",
		)
		return
	}

	getReq := &serverv1.GetClusterBackgroundPersistenceRequest{
		EnvironmentId: envIds[0],
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
	data.Kind = types.StringValue(bg.Kind)

	if bg.CreatedAt != nil {
		data.CreatedAt = types.StringValue(bg.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"))
	}
	if bg.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(bg.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z"))
	}

	// Update specs if available
	if bg.Specs != nil && bg.Specs.CommonPersistenceSpecs != nil {
		common := bg.Specs.CommonPersistenceSpecs
		data.Namespace = types.StringValue(common.Namespace)
		data.BusWriterImageGo = types.StringValue(common.BusWriterImageGo)
		data.BusWriterImagePython = types.StringValue(common.BusWriterImagePython)
		data.BusWriterImageBswl = types.StringValue(common.BusWriterImageBswl)
		data.BusWriterImageRust = types.StringValue(common.BusWriterImageRust)
		data.ServiceAccountName = types.StringValue(common.ServiceAccountName)
		data.BusBackend = types.StringValue(common.BusBackend)
		data.SecretClient = types.StringValue(common.SecretClient)
		data.BigqueryParquetUploadSubscriptionId = types.StringValue(common.BigqueryParquetUploadSubscriptionId)
		data.BigqueryStreamingWriteSubscriptionId = types.StringValue(common.BigqueryStreamingWriteSubscriptionId)
		data.BigqueryStreamingWriteTopic = types.StringValue(common.BigqueryStreamingWriteTopic)
		data.BqUploadBucket = types.StringValue(common.BqUploadBucket)
		data.BqUploadTopic = types.StringValue(common.BqUploadTopic)
		data.GoogleCloudProject = types.StringValue(common.GoogleCloudProject)
		data.KafkaDlqTopic = types.StringValue(common.KafkaDlqTopic)
		data.MetricsBusSubscriptionId = types.StringValue(common.MetricsBusSubscriptionId)
		data.MetricsBusTopicId = types.StringValue(common.MetricsBusTopicId)
		data.OperationSubscriptionId = types.StringValue(common.OperationSubscriptionId)
		data.QueryLogResultTopic = types.StringValue(common.QueryLogResultTopic)
		data.QueryLogSubscriptionId = types.StringValue(common.QueryLogSubscriptionId)
		data.ResultBusMetricsSubscriptionId = types.StringValue(common.ResultBusMetricsSubscriptionId)
		data.ResultBusOfflineStoreSubscriptionId = types.StringValue(common.ResultBusOfflineStoreSubscriptionId)
		data.ResultBusOnlineStoreSubscriptionId = types.StringValue(common.ResultBusOnlineStoreSubscriptionId)
		data.ResultBusTopicId = types.StringValue(common.ResultBusTopicId)
		data.UsageBusTopicId = types.StringValue(common.UsageBusTopicId)
		data.UsageEventsSubscriptionId = types.StringValue(common.UsageEventsSubscriptionId)
		data.IncludeChalkNodeSelector = types.BoolValue(common.IncludeChalkNodeSelector)

		// Update deployment-level specs
		data.ApiServerHost = types.StringValue(bg.Specs.ApiServerHost)
		data.KafkaSaslSecret = types.StringValue(bg.Specs.KafkaSaslSecret)
		data.MetadataProvider = types.StringValue(bg.Specs.MetadataProvider)
		data.KafkaBootstrapServers = types.StringValue(bg.Specs.KafkaBootstrapServers)
		data.KafkaSecurityProtocol = types.StringValue(bg.Specs.KafkaSecurityProtocol)
		data.KafkaSaslMechanism = types.StringValue(bg.Specs.KafkaSaslMechanism)
		data.RedisIsClustered = types.StringValue(bg.Specs.RedisIsClustered)
		data.SnowflakeStorageIntegrationName = types.StringValue(bg.Specs.SnowflakeStorageIntegrationName)
		data.RedisLightningSupportsHasMany = types.BoolValue(bg.Specs.RedisLightningSupportsHasMany)
		data.Insecure = types.BoolValue(bg.Specs.Insecure)

		// Update writers - this is complex, so for now we'll keep the existing state
		// A full implementation would convert from proto back to terraform models
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterBackgroundPersistenceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Note: According to the proto definition, there's no UpdateClusterBackgroundPersistence method
	// So we'll need to delete and recreate, but this would be disruptive
	// For now, we'll return an error indicating updates are not supported
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Cluster background persistence updates are not supported by the Chalk API. Please recreate the resource if changes are needed.",
	)
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
