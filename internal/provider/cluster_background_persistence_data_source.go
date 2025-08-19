package provider

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
)

var _ datasource.DataSource = &ClusterBackgroundPersistenceDataSource{}

func NewClusterBackgroundPersistenceDataSource() datasource.DataSource {
	return &ClusterBackgroundPersistenceDataSource{}
}

type ClusterBackgroundPersistenceDataSource struct {
	client *ChalkClient
}

type ClusterBackgroundPersistenceDataSourceModel struct {
	Id                                   types.String `tfsdk:"id"`
	EnvironmentId                        types.String `tfsdk:"environment_id"`
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

func (d *ClusterBackgroundPersistenceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_background_persistence"
}

func (d *ClusterBackgroundPersistenceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	kubeResourceConfigSchema := schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"cpu": schema.StringAttribute{
				MarkdownDescription: "CPU resource",
				Computed:            true,
			},
			"memory": schema.StringAttribute{
				MarkdownDescription: "Memory resource",
				Computed:            true,
			},
			"ephemeral_storage": schema.StringAttribute{
				MarkdownDescription: "Ephemeral storage",
				Computed:            true,
			},
			"storage": schema.StringAttribute{
				MarkdownDescription: "Storage",
				Computed:            true,
			},
		},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source for Chalk cluster background persistence",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Background persistence ID",
				Computed:            true,
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "Environment ID to fetch background persistence for",
				Required:            true,
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Background persistence kind",
				Computed:            true,
			},
			"namespace": schema.StringAttribute{
				MarkdownDescription: "Kubernetes namespace",
				Computed:            true,
			},
			"bus_writer_image_go": schema.StringAttribute{
				MarkdownDescription: "Bus writer Go image",
				Computed:            true,
			},
			"bus_writer_image_python": schema.StringAttribute{
				MarkdownDescription: "Bus writer Python image",
				Computed:            true,
			},
			"bus_writer_image_bswl": schema.StringAttribute{
				MarkdownDescription: "Bus writer BSWL image",
				Computed:            true,
			},
			"bus_writer_image_rust": schema.StringAttribute{
				MarkdownDescription: "Bus writer Rust image",
				Computed:            true,
			},
			"service_account_name": schema.StringAttribute{
				MarkdownDescription: "Service account name",
				Computed:            true,
			},
			"bus_backend": schema.StringAttribute{
				MarkdownDescription: "Bus backend",
				Computed:            true,
			},
			"secret_client": schema.StringAttribute{
				MarkdownDescription: "Secret client",
				Computed:            true,
			},
			"bigquery_parquet_upload_subscription_id": schema.StringAttribute{
				MarkdownDescription: "BigQuery parquet upload subscription ID",
				Computed:            true,
			},
			"bigquery_streaming_write_subscription_id": schema.StringAttribute{
				MarkdownDescription: "BigQuery streaming write subscription ID",
				Computed:            true,
			},
			"bigquery_streaming_write_topic": schema.StringAttribute{
				MarkdownDescription: "BigQuery streaming write topic",
				Computed:            true,
			},
			"bq_upload_bucket": schema.StringAttribute{
				MarkdownDescription: "BigQuery upload bucket",
				Computed:            true,
			},
			"bq_upload_topic": schema.StringAttribute{
				MarkdownDescription: "BigQuery upload topic",
				Computed:            true,
			},
			"google_cloud_project": schema.StringAttribute{
				MarkdownDescription: "Google Cloud project",
				Computed:            true,
			},
			"kafka_dlq_topic": schema.StringAttribute{
				MarkdownDescription: "Kafka DLQ topic",
				Computed:            true,
			},
			"metrics_bus_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Metrics bus subscription ID",
				Computed:            true,
			},
			"metrics_bus_topic_id": schema.StringAttribute{
				MarkdownDescription: "Metrics bus topic ID",
				Computed:            true,
			},
			"operation_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Operation subscription ID",
				Computed:            true,
			},
			"query_log_result_topic": schema.StringAttribute{
				MarkdownDescription: "Query log result topic",
				Computed:            true,
			},
			"query_log_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Query log subscription ID",
				Computed:            true,
			},
			"result_bus_metrics_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Result bus metrics subscription ID",
				Computed:            true,
			},
			"result_bus_offline_store_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Result bus offline store subscription ID",
				Computed:            true,
			},
			"result_bus_online_store_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Result bus online store subscription ID",
				Computed:            true,
			},
			"result_bus_topic_id": schema.StringAttribute{
				MarkdownDescription: "Result bus topic ID",
				Computed:            true,
			},
			"usage_bus_topic_id": schema.StringAttribute{
				MarkdownDescription: "Usage bus topic ID",
				Computed:            true,
			},
			"usage_events_subscription_id": schema.StringAttribute{
				MarkdownDescription: "Usage events subscription ID",
				Computed:            true,
			},
			"include_chalk_node_selector": schema.BoolAttribute{
				MarkdownDescription: "Include chalk node selector",
				Computed:            true,
			},
			"api_server_host": schema.StringAttribute{
				MarkdownDescription: "API server host",
				Computed:            true,
			},
			"kafka_sasl_secret": schema.StringAttribute{
				MarkdownDescription: "Kafka SASL secret",
				Computed:            true,
			},
			"metadata_provider": schema.StringAttribute{
				MarkdownDescription: "Metadata provider",
				Computed:            true,
			},
			"kafka_bootstrap_servers": schema.StringAttribute{
				MarkdownDescription: "Kafka bootstrap servers",
				Computed:            true,
			},
			"kafka_security_protocol": schema.StringAttribute{
				MarkdownDescription: "Kafka security protocol",
				Computed:            true,
			},
			"kafka_sasl_mechanism": schema.StringAttribute{
				MarkdownDescription: "Kafka SASL mechanism",
				Computed:            true,
			},
			"redis_is_clustered": schema.StringAttribute{
				MarkdownDescription: "Redis is clustered",
				Computed:            true,
			},
			"snowflake_storage_integration_name": schema.StringAttribute{
				MarkdownDescription: "Snowflake storage integration name",
				Computed:            true,
			},
			"redis_lightning_supports_has_many": schema.BoolAttribute{
				MarkdownDescription: "Redis lightning supports has many",
				Computed:            true,
			},
			"insecure": schema.BoolAttribute{
				MarkdownDescription: "Insecure mode",
				Computed:            true,
			},
			"writers": schema.ListNestedAttribute{
				MarkdownDescription: "Background persistence writers",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Writer name",
							Computed:            true,
						},
						"image_override": schema.StringAttribute{
							MarkdownDescription: "Image override",
							Computed:            true,
						},
						"hpa_specs": schema.SingleNestedAttribute{
							MarkdownDescription: "HPA specifications",
							Computed:            true,
							Attributes: map[string]schema.Attribute{
								"hpa_pubsub_subscription_id": schema.StringAttribute{
									MarkdownDescription: "HPA PubSub subscription ID",
									Computed:            true,
								},
								"hpa_min_replicas": schema.Int64Attribute{
									MarkdownDescription: "HPA minimum replicas",
									Computed:            true,
								},
								"hpa_max_replicas": schema.Int64Attribute{
									MarkdownDescription: "HPA maximum replicas",
									Computed:            true,
								},
								"hpa_target_average_value": schema.Int64Attribute{
									MarkdownDescription: "HPA target average value",
									Computed:            true,
								},
							},
						},
						"gke_spot": schema.BoolAttribute{
							MarkdownDescription: "Use GKE spot instances",
							Computed:            true,
						},
						"load_writer_configmap": schema.BoolAttribute{
							MarkdownDescription: "Load writer configmap",
							Computed:            true,
						},
						"version": schema.StringAttribute{
							MarkdownDescription: "Version",
							Computed:            true,
						},
						"request": schema.SingleNestedAttribute{
							MarkdownDescription: "Resource requests",
							Computed:            true,
							Attributes:          kubeResourceConfigSchema.Attributes,
						},
						"limit": schema.SingleNestedAttribute{
							MarkdownDescription: "Resource limits",
							Computed:            true,
							Attributes:          kubeResourceConfigSchema.Attributes,
						},
						"bus_subscriber_type": schema.StringAttribute{
							MarkdownDescription: "Bus subscriber type",
							Computed:            true,
						},
						"default_replica_count": schema.Int64Attribute{
							MarkdownDescription: "Default replica count",
							Computed:            true,
						},
						"kafka_consumer_group_override": schema.StringAttribute{
							MarkdownDescription: "Kafka consumer group override",
							Computed:            true,
						},
						"max_batch_size": schema.Int64Attribute{
							MarkdownDescription: "Max batch size",
							Computed:            true,
						},
						"message_processing_concurrency": schema.Int64Attribute{
							MarkdownDescription: "Message processing concurrency",
							Computed:            true,
						},
						"metadata_sql_ssl_ca_cert_secret": schema.StringAttribute{
							MarkdownDescription: "Metadata SQL SSL CA cert secret",
							Computed:            true,
						},
						"metadata_sql_ssl_client_cert_secret": schema.StringAttribute{
							MarkdownDescription: "Metadata SQL SSL client cert secret",
							Computed:            true,
						},
						"metadata_sql_ssl_client_key_secret": schema.StringAttribute{
							MarkdownDescription: "Metadata SQL SSL client key secret",
							Computed:            true,
						},
						"metadata_sql_uri_secret": schema.StringAttribute{
							MarkdownDescription: "Metadata SQL URI secret",
							Computed:            true,
						},
						"offline_store_inserter_db_type": schema.StringAttribute{
							MarkdownDescription: "Offline store inserter DB type",
							Computed:            true,
						},
						"storage_cache_prefix": schema.StringAttribute{
							MarkdownDescription: "Storage cache prefix",
							Computed:            true,
						},
						"usage_store_uri": schema.StringAttribute{
							MarkdownDescription: "Usage store URI",
							Computed:            true,
						},
						"results_writer_skip_producing_feature_metrics": schema.BoolAttribute{
							MarkdownDescription: "Results writer skip producing feature metrics",
							Computed:            true,
						},
						"query_table_write_drop_ratio": schema.StringAttribute{
							MarkdownDescription: "Query table write drop ratio",
							Computed:            true,
						},
					},
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Update timestamp",
				Computed:            true,
			},
		},
	}
}

func (d *ClusterBackgroundPersistenceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ChalkClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ChalkClient, got: %T", req.ProviderData),
		)
		return
	}

	d.client = client
}

func getBackgroundPersistenceWriterAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":                                           types.StringType,
		"image_override":                                 types.StringType,
		"hpa_specs":                                      types.ObjectType{AttrTypes: getBackgroundPersistenceWriterHpaAttrTypes()},
		"gke_spot":                                       types.BoolType,
		"load_writer_configmap":                          types.BoolType,
		"version":                                        types.StringType,
		"request":                                        types.ObjectType{AttrTypes: getKubeResourceConfigAttrTypes()},
		"limit":                                          types.ObjectType{AttrTypes: getKubeResourceConfigAttrTypes()},
		"bus_subscriber_type":                            types.StringType,
		"default_replica_count":                          types.Int64Type,
		"kafka_consumer_group_override":                  types.StringType,
		"max_batch_size":                                 types.Int64Type,
		"message_processing_concurrency":                 types.Int64Type,
		"metadata_sql_ssl_ca_cert_secret":                types.StringType,
		"metadata_sql_ssl_client_cert_secret":            types.StringType,
		"metadata_sql_ssl_client_key_secret":             types.StringType,
		"metadata_sql_uri_secret":                        types.StringType,
		"offline_store_inserter_db_type":                 types.StringType,
		"storage_cache_prefix":                           types.StringType,
		"usage_store_uri":                                types.StringType,
		"results_writer_skip_producing_feature_metrics":  types.BoolType,
		"query_table_write_drop_ratio":                   types.StringType,
	}
}

func getBackgroundPersistenceWriterHpaAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"hpa_pubsub_subscription_id": types.StringType,
		"hpa_min_replicas":           types.Int64Type,
		"hpa_max_replicas":           types.Int64Type,
		"hpa_target_average_value":   types.Int64Type,
	}
}

func getKubeResourceConfigAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"cpu":               types.StringType,
		"memory":            types.StringType,
		"ephemeral_storage": types.StringType,
		"storage":           types.StringType,
	}
}

func (d *ClusterBackgroundPersistenceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ClusterBackgroundPersistenceDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// First get auth token
	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         d.client.ApiServer,
			interceptors: []connect.Interceptor{},
		},
	)

	// Create builder client with token injection interceptor
	builderClient := NewBuilderClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       d.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeTokenInjectionInterceptor(authClient, d.client.ClientID, d.client.ClientSecret),
		},
	})
	
	// Get background persistence
	getReq := connect.NewRequest(&serverv1.GetClusterBackgroundPersistenceRequest{
		EnvironmentId: data.EnvironmentId.ValueString(),
	})

	getResp, err := builderClient.GetClusterBackgroundPersistence(ctx, getReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read cluster background persistence, got error: %s", err))
		return
	}

	if getResp.Msg.BackgroundPersistence == nil {
		resp.Diagnostics.AddError("Client Error", "Background persistence not found")
		return
	}

	bp := getResp.Msg.BackgroundPersistence

	// Map response to model
	data.Id = types.StringValue(bp.Id)
	data.Kind = types.StringValue(bp.Kind)
	if bp.CreatedAt != nil {
		data.CreatedAt = types.StringValue(bp.CreatedAt.AsTime().String())
	}
	if bp.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(bp.UpdatedAt.AsTime().String())
	}

	// Map specs if available
	if bp.Specs != nil {
		specs := bp.Specs
		
		// Map common persistence specs
		if specs.CommonPersistenceSpecs != nil {
			common := specs.CommonPersistenceSpecs
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
		}

		// Map deployment specs
		data.ApiServerHost = types.StringValue(specs.ApiServerHost)
		data.KafkaSaslSecret = types.StringValue(specs.KafkaSaslSecret)
		data.MetadataProvider = types.StringValue(specs.MetadataProvider)
		data.KafkaBootstrapServers = types.StringValue(specs.KafkaBootstrapServers)
		data.KafkaSecurityProtocol = types.StringValue(specs.KafkaSecurityProtocol)
		data.KafkaSaslMechanism = types.StringValue(specs.KafkaSaslMechanism)
		data.RedisIsClustered = types.StringValue(specs.RedisIsClustered)
		data.SnowflakeStorageIntegrationName = types.StringValue(specs.SnowflakeStorageIntegrationName)
		data.RedisLightningSupportsHasMany = types.BoolValue(specs.RedisLightningSupportsHasMany)
		data.Insecure = types.BoolValue(specs.Insecure)

		// Map writers
		if len(specs.Writers) > 0 {
			var writersList []BackgroundPersistenceWriterModel
			for _, writer := range specs.Writers {
				writerModel := BackgroundPersistenceWriterModel{
					Name:                                     types.StringValue(writer.Name),
					ImageOverride:                            types.StringValue(writer.ImageOverride),
					Version:                                  types.StringValue(writer.Version),
					BusSubscriberType:                        types.StringValue(writer.BusSubscriberType),
					DefaultReplicaCount:                      types.Int64Value(int64(writer.DefaultReplicaCount)),
					KafkaConsumerGroupOverride:               types.StringValue(writer.KafkaConsumerGroupOverride),
					MetadataSqlSslCaCertSecret:               types.StringValue(writer.MetadataSqlSslCaCertSecret),
					MetadataSqlSslClientCertSecret:           types.StringValue(writer.MetadataSqlSslClientCertSecret),
					MetadataSqlSslClientKeySecret:            types.StringValue(writer.MetadataSqlSslClientKeySecret),
					MetadataSqlUriSecret:                     types.StringValue(writer.MetadataSqlUriSecret),
					OfflineStoreInserterDbType:               types.StringValue(writer.OfflineStoreInserterDbType),
					StorageCachePrefix:                       types.StringValue(writer.StorageCachePrefix),
					UsageStoreUri:                            types.StringValue(writer.UsageStoreUri),
					QueryTableWriteDropRatio:                 types.StringValue(writer.QueryTableWriteDropRatio),
				}

				if writer.HpaSpecs != nil {
					writerModel.HpaSpecs = &BackgroundPersistenceWriterHpaModel{
						HpaPubsubSubscriptionId: types.StringValue(writer.HpaSpecs.HpaPubsubSubscriptionId),
					}
					if writer.HpaSpecs.HpaMinReplicas != nil {
						writerModel.HpaSpecs.HpaMinReplicas = types.Int64Value(int64(*writer.HpaSpecs.HpaMinReplicas))
					}
					if writer.HpaSpecs.HpaMaxReplicas != nil {
						writerModel.HpaSpecs.HpaMaxReplicas = types.Int64Value(int64(*writer.HpaSpecs.HpaMaxReplicas))
					}
					if writer.HpaSpecs.HpaTargetAverageValue != nil {
						writerModel.HpaSpecs.HpaTargetAverageValue = types.Int64Value(int64(*writer.HpaSpecs.HpaTargetAverageValue))
					}
				}

				if writer.GkeSpot != nil {
					writerModel.GkeSpot = types.BoolValue(*writer.GkeSpot)
				}
				if writer.LoadWriterConfigmap != nil {
					writerModel.LoadWriterConfigmap = types.BoolValue(*writer.LoadWriterConfigmap)
				}
				if writer.MaxBatchSize != nil {
					writerModel.MaxBatchSize = types.Int64Value(int64(*writer.MaxBatchSize))
				}
				if writer.MessageProcessingConcurrency != nil {
					writerModel.MessageProcessingConcurrency = types.Int64Value(int64(*writer.MessageProcessingConcurrency))
				}
				if writer.ResultsWriterSkipProducingFeatureMetrics != nil {
					writerModel.ResultsWriterSkipProducingFeatureMetrics = types.BoolValue(*writer.ResultsWriterSkipProducingFeatureMetrics)
				}

				if writer.Request != nil {
					writerModel.Request = &KubeResourceConfigModel{
						CPU:              types.StringValue(writer.Request.Cpu),
						Memory:           types.StringValue(writer.Request.Memory),
						EphemeralStorage: types.StringValue(writer.Request.EphemeralStorage),
						Storage:          types.StringValue(writer.Request.Storage),
					}
				}
				if writer.Limit != nil {
					writerModel.Limit = &KubeResourceConfigModel{
						CPU:              types.StringValue(writer.Limit.Cpu),
						Memory:           types.StringValue(writer.Limit.Memory),
						EphemeralStorage: types.StringValue(writer.Limit.EphemeralStorage),
						Storage:          types.StringValue(writer.Limit.Storage),
					}
				}

				writersList = append(writersList, writerModel)
			}
			writers, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: getBackgroundPersistenceWriterAttrTypes()}, writersList)
			resp.Diagnostics.Append(diags...)
			data.Writers = writers
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	tflog.Trace(ctx, "read chalk_cluster_background_persistence data source")
}