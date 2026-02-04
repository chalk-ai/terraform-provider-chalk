package provider

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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

	//Todo remove EnvironmentIds
	EnvironmentIds                       types.List   `tfsdk:"environment_ids"`
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

	//TODO deprecate
	IncludeChalkNodeSelector types.Bool `tfsdk:"include_chalk_node_selector"`

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
				Optional:            true,
				ElementType:         types.StringType,
			},
			"kube_cluster_id": schema.StringAttribute{
				MarkdownDescription: "Kubernetes cluster ID",
				Optional:            true,
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
				Optional:            true,
			},
			"kafka_dlq_topic": schema.StringAttribute{
				MarkdownDescription: "Kafka DLQ topic",
				Optional:            true,
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
			"include_chalk_node_selector": schema.BoolAttribute{
				MarkdownDescription: "Whether to include chalk node selector",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
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
							Optional:            true,
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
							Optional:            true,
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
							Optional:            true,
						},
						"metadata_sql_ssl_client_cert_secret": schema.StringAttribute{
							MarkdownDescription: "Metadata SQL SSL client cert secret",
							Optional:            true,
						},
						"metadata_sql_ssl_client_key_secret": schema.StringAttribute{
							MarkdownDescription: "Metadata SQL SSL client key secret",
							Optional:            true,
						},
						"metadata_sql_uri_secret": schema.StringAttribute{
							MarkdownDescription: "Metadata SQL URI secret",
							Optional:            true,
						},
						"offline_store_inserter_db_type": schema.StringAttribute{
							MarkdownDescription: "Offline store inserter DB type",
							Optional:            true,
						},
						"storage_cache_prefix": schema.StringAttribute{
							MarkdownDescription: "Storage cache prefix",
							Optional:            true,
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

	var envIds []string
	// Convert environment IDs
	if !data.EnvironmentIds.IsNull() {
		diags := data.EnvironmentIds.ElementsAs(ctx, &envIds, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Convert writers
	var writers []BackgroundPersistenceWriterModel
	diags := data.Writers.ElementsAs(ctx, &writers, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var protoWriters []*serverv1.BackgroundPersistenceWriterSpecs
	for _, writer := range writers {
		protoWriter := &serverv1.BackgroundPersistenceWriterSpecs{
			Name:              writer.Name.ValueString(),
			BusSubscriberType: writer.BusSubscriberType.ValueString(),
		}

		// Handle optional string fields
		if !writer.ImageOverride.IsNull() {
			protoWriter.ImageOverride = writer.ImageOverride.ValueString()
		}
		if !writer.Version.IsNull() {
			protoWriter.Version = writer.Version.ValueString()
		}
		if !writer.DefaultReplicaCount.IsNull() {
			protoWriter.DefaultReplicaCount = int32(writer.DefaultReplicaCount.ValueInt64())
		}
		if !writer.MetadataSqlSslCaCertSecret.IsNull() {
			protoWriter.MetadataSqlSslCaCertSecret = writer.MetadataSqlSslCaCertSecret.ValueString()
		}
		if !writer.MetadataSqlSslClientCertSecret.IsNull() {
			protoWriter.MetadataSqlSslClientCertSecret = writer.MetadataSqlSslClientCertSecret.ValueString()
		}
		if !writer.MetadataSqlSslClientKeySecret.IsNull() {
			protoWriter.MetadataSqlSslClientKeySecret = writer.MetadataSqlSslClientKeySecret.ValueString()
		}
		if !writer.MetadataSqlUriSecret.IsNull() {
			protoWriter.MetadataSqlUriSecret = writer.MetadataSqlUriSecret.ValueString()
		}
		if !writer.OfflineStoreInserterDbType.IsNull() {
			protoWriter.OfflineStoreInserterDbType = writer.OfflineStoreInserterDbType.ValueString()
		}
		if !writer.StorageCachePrefix.IsNull() {
			protoWriter.StorageCachePrefix = writer.StorageCachePrefix.ValueString()
		}
		if !writer.QueryTableWriteDropRatio.IsNull() {
			protoWriter.QueryTableWriteDropRatio = writer.QueryTableWriteDropRatio.ValueString()
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
			protoWriter.Request = &serverv1.KubeResourceConfig{}
			if !writer.Request.CPU.IsNull() {
				protoWriter.Request.Cpu = writer.Request.CPU.ValueString()
			}
			if !writer.Request.Memory.IsNull() {
				protoWriter.Request.Memory = writer.Request.Memory.ValueString()
			}
			if !writer.Request.EphemeralStorage.IsNull() {
				protoWriter.Request.EphemeralStorage = writer.Request.EphemeralStorage.ValueString()
			}
			if !writer.Request.Storage.IsNull() {
				protoWriter.Request.Storage = writer.Request.Storage.ValueString()
			}
		}
		if writer.Limit != nil {
			protoWriter.Limit = &serverv1.KubeResourceConfig{}
			if !writer.Limit.CPU.IsNull() {
				protoWriter.Limit.Cpu = writer.Limit.CPU.ValueString()
			}
			if !writer.Limit.Memory.IsNull() {
				protoWriter.Limit.Memory = writer.Limit.Memory.ValueString()
			}
			if !writer.Limit.EphemeralStorage.IsNull() {
				protoWriter.Limit.EphemeralStorage = writer.Limit.EphemeralStorage.ValueString()
			}
			if !writer.Limit.Storage.IsNull() {
				protoWriter.Limit.Storage = writer.Limit.Storage.ValueString()
			}
		}

		protoWriters = append(protoWriters, protoWriter)
	}

	// Convert terraform model to proto request
	commonSpecs := &serverv1.BackgroundPersistenceCommonSpecs{
		Namespace:                            data.Namespace.ValueString(),
		ServiceAccountName:                   data.ServiceAccountName.ValueString(),
		BigqueryParquetUploadSubscriptionId:  data.BigqueryParquetUploadSubscriptionId.ValueString(),
		BigqueryStreamingWriteSubscriptionId: data.BigqueryStreamingWriteSubscriptionId.ValueString(),
		BigqueryStreamingWriteTopic:          data.BigqueryStreamingWriteTopic.ValueString(),
		BqUploadBucket:                       data.BqUploadBucket.ValueString(),
		BqUploadTopic:                        data.BqUploadTopic.ValueString(),
		MetricsBusSubscriptionId:             data.MetricsBusSubscriptionId.ValueString(),
		MetricsBusTopicId:                    data.MetricsBusTopicId.ValueString(),
		OperationSubscriptionId:              data.OperationSubscriptionId.ValueString(),
		QueryLogResultTopic:                  data.QueryLogResultTopic.ValueString(),
		QueryLogSubscriptionId:               data.QueryLogSubscriptionId.ValueString(),
		ResultBusOfflineStoreSubscriptionId:  data.ResultBusOfflineStoreSubscriptionId.ValueString(),
		ResultBusOnlineStoreSubscriptionId:   data.ResultBusOnlineStoreSubscriptionId.ValueString(),
		ResultBusTopicId:                     data.ResultBusTopicId.ValueString(),
		IncludeChalkNodeSelector:             data.IncludeChalkNodeSelector.ValueBool(),
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

	//create a deployment based on whether kube_cluster_id or environment_ids is provided
	var createReq *serverv1.CreateClusterBackgroundPersistenceRequest
	if !data.KubeClusterId.IsNull() {
		createReq = &serverv1.CreateClusterBackgroundPersistenceRequest{
			Specs:         deploymentSpecs,
			KubeClusterId: data.KubeClusterId.ValueStringPointer(),
		}
	} else {
		createReq = &serverv1.CreateClusterBackgroundPersistenceRequest{
			EnvironmentIds: envIds,
			Specs:          deploymentSpecs,
		}
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
		data.BigqueryParquetUploadSubscriptionId = types.StringValue(common.BigqueryParquetUploadSubscriptionId)
		data.BigqueryStreamingWriteSubscriptionId = types.StringValue(common.BigqueryStreamingWriteSubscriptionId)
		data.BigqueryStreamingWriteTopic = types.StringValue(common.BigqueryStreamingWriteTopic)
		data.BqUploadBucket = types.StringValue(common.BqUploadBucket)
		data.BqUploadTopic = types.StringValue(common.BqUploadTopic)
		data.MetricsBusSubscriptionId = types.StringValue(common.MetricsBusSubscriptionId)
		data.MetricsBusTopicId = types.StringValue(common.MetricsBusTopicId)
		data.OperationSubscriptionId = types.StringValue(common.OperationSubscriptionId)
		data.QueryLogResultTopic = types.StringValue(common.QueryLogResultTopic)
		data.QueryLogSubscriptionId = types.StringValue(common.QueryLogSubscriptionId)
		data.ResultBusOfflineStoreSubscriptionId = types.StringValue(common.ResultBusOfflineStoreSubscriptionId)
		data.ResultBusOnlineStoreSubscriptionId = types.StringValue(common.ResultBusOnlineStoreSubscriptionId)
		data.ResultBusTopicId = types.StringValue(common.ResultBusTopicId)
		data.IncludeChalkNodeSelector = types.BoolValue(common.IncludeChalkNodeSelector)

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

		// Update writers - convert from proto back to terraform models
		if len(bg.Specs.Writers) > 0 {
			var tfWriters []BackgroundPersistenceWriterModel
			for _, protoWriter := range bg.Specs.Writers {
				tfWriter := BackgroundPersistenceWriterModel{
					Name:              types.StringValue(protoWriter.Name),
					BusSubscriberType: types.StringValue(protoWriter.BusSubscriberType),
				}

				// Handle optional string fields
				if protoWriter.ImageOverride != "" {
					tfWriter.ImageOverride = types.StringValue(protoWriter.ImageOverride)
				} else {
					tfWriter.ImageOverride = types.StringNull()
				}
				if protoWriter.Version != "" {
					tfWriter.Version = types.StringValue(protoWriter.Version)
				} else {
					tfWriter.Version = types.StringNull()
				}
				if protoWriter.DefaultReplicaCount != 0 {
					tfWriter.DefaultReplicaCount = types.Int64Value(int64(protoWriter.DefaultReplicaCount))
				} else {
					tfWriter.DefaultReplicaCount = types.Int64Null()
				}
				if protoWriter.MetadataSqlSslCaCertSecret != "" {
					tfWriter.MetadataSqlSslCaCertSecret = types.StringValue(protoWriter.MetadataSqlSslCaCertSecret)
				} else {
					tfWriter.MetadataSqlSslCaCertSecret = types.StringNull()
				}
				if protoWriter.MetadataSqlSslClientCertSecret != "" {
					tfWriter.MetadataSqlSslClientCertSecret = types.StringValue(protoWriter.MetadataSqlSslClientCertSecret)
				} else {
					tfWriter.MetadataSqlSslClientCertSecret = types.StringNull()
				}
				if protoWriter.MetadataSqlSslClientKeySecret != "" {
					tfWriter.MetadataSqlSslClientKeySecret = types.StringValue(protoWriter.MetadataSqlSslClientKeySecret)
				} else {
					tfWriter.MetadataSqlSslClientKeySecret = types.StringNull()
				}
				if protoWriter.MetadataSqlUriSecret != "" {
					tfWriter.MetadataSqlUriSecret = types.StringValue(protoWriter.MetadataSqlUriSecret)
				} else {
					tfWriter.MetadataSqlUriSecret = types.StringNull()
				}
				if protoWriter.OfflineStoreInserterDbType != "" {
					tfWriter.OfflineStoreInserterDbType = types.StringValue(protoWriter.OfflineStoreInserterDbType)
				} else {
					tfWriter.OfflineStoreInserterDbType = types.StringNull()
				}
				if protoWriter.StorageCachePrefix != "" {
					tfWriter.StorageCachePrefix = types.StringValue(protoWriter.StorageCachePrefix)
				} else {
					tfWriter.StorageCachePrefix = types.StringNull()
				}
				if protoWriter.QueryTableWriteDropRatio != "" {
					tfWriter.QueryTableWriteDropRatio = types.StringValue(protoWriter.QueryTableWriteDropRatio)
				} else {
					tfWriter.QueryTableWriteDropRatio = types.StringNull()
				}
				if protoWriter.KafkaConsumerGroupOverride != "" {
					tfWriter.KafkaConsumerGroupOverride = types.StringValue(protoWriter.KafkaConsumerGroupOverride)
				} else {
					tfWriter.KafkaConsumerGroupOverride = types.StringNull()
				}

				// Handle optional pointer fields
				if protoWriter.GkeSpot != nil {
					tfWriter.GkeSpot = types.BoolValue(*protoWriter.GkeSpot)
				} else {
					tfWriter.GkeSpot = types.BoolNull()
				}
				if protoWriter.LoadWriterConfigmap != nil {
					tfWriter.LoadWriterConfigmap = types.BoolValue(*protoWriter.LoadWriterConfigmap)
				} else {
					tfWriter.LoadWriterConfigmap = types.BoolNull()
				}
				if protoWriter.MaxBatchSize != nil {
					tfWriter.MaxBatchSize = types.Int64Value(int64(*protoWriter.MaxBatchSize))
				} else {
					tfWriter.MaxBatchSize = types.Int64Null()
				}
				if protoWriter.MessageProcessingConcurrency != nil {
					tfWriter.MessageProcessingConcurrency = types.Int64Value(int64(*protoWriter.MessageProcessingConcurrency))
				} else {
					tfWriter.MessageProcessingConcurrency = types.Int64Null()
				}
				if protoWriter.ResultsWriterSkipProducingFeatureMetrics != nil {
					tfWriter.ResultsWriterSkipProducingFeatureMetrics = types.BoolValue(*protoWriter.ResultsWriterSkipProducingFeatureMetrics)
				} else {
					tfWriter.ResultsWriterSkipProducingFeatureMetrics = types.BoolNull()
				}

				// Convert HPA specs
				if protoWriter.HpaSpecs != nil {
					tfWriter.HpaSpecs = &BackgroundPersistenceWriterHpaModel{
						HpaPubsubSubscriptionId: types.StringValue(protoWriter.HpaSpecs.HpaPubsubSubscriptionId),
					}
					if protoWriter.HpaSpecs.HpaMinReplicas != nil {
						tfWriter.HpaSpecs.HpaMinReplicas = types.Int64Value(int64(*protoWriter.HpaSpecs.HpaMinReplicas))
					} else {
						tfWriter.HpaSpecs.HpaMinReplicas = types.Int64Null()
					}
					if protoWriter.HpaSpecs.HpaMaxReplicas != nil {
						tfWriter.HpaSpecs.HpaMaxReplicas = types.Int64Value(int64(*protoWriter.HpaSpecs.HpaMaxReplicas))
					} else {
						tfWriter.HpaSpecs.HpaMaxReplicas = types.Int64Null()
					}
					if protoWriter.HpaSpecs.HpaTargetAverageValue != nil {
						tfWriter.HpaSpecs.HpaTargetAverageValue = types.Int64Value(int64(*protoWriter.HpaSpecs.HpaTargetAverageValue))
					} else {
						tfWriter.HpaSpecs.HpaTargetAverageValue = types.Int64Null()
					}
				}

				// Convert resource configs
				if protoWriter.Request != nil {
					tfWriter.Request = &KubeResourceConfigModel{}
					if protoWriter.Request.Cpu != "" {
						tfWriter.Request.CPU = types.StringValue(protoWriter.Request.Cpu)
					} else {
						tfWriter.Request.CPU = types.StringNull()
					}
					if protoWriter.Request.Memory != "" {
						tfWriter.Request.Memory = types.StringValue(protoWriter.Request.Memory)
					} else {
						tfWriter.Request.Memory = types.StringNull()
					}
					if protoWriter.Request.EphemeralStorage != "" {
						tfWriter.Request.EphemeralStorage = types.StringValue(protoWriter.Request.EphemeralStorage)
					} else {
						tfWriter.Request.EphemeralStorage = types.StringNull()
					}
					if protoWriter.Request.Storage != "" {
						tfWriter.Request.Storage = types.StringValue(protoWriter.Request.Storage)
					} else {
						tfWriter.Request.Storage = types.StringNull()
					}
				}

				if protoWriter.Limit != nil {
					tfWriter.Limit = &KubeResourceConfigModel{}
					if protoWriter.Limit.Cpu != "" {
						tfWriter.Limit.CPU = types.StringValue(protoWriter.Limit.Cpu)
					} else {
						tfWriter.Limit.CPU = types.StringNull()
					}
					if protoWriter.Limit.Memory != "" {
						tfWriter.Limit.Memory = types.StringValue(protoWriter.Limit.Memory)
					} else {
						tfWriter.Limit.Memory = types.StringNull()
					}
					if protoWriter.Limit.EphemeralStorage != "" {
						tfWriter.Limit.EphemeralStorage = types.StringValue(protoWriter.Limit.EphemeralStorage)
					} else {
						tfWriter.Limit.EphemeralStorage = types.StringNull()
					}
					if protoWriter.Limit.Storage != "" {
						tfWriter.Limit.Storage = types.StringValue(protoWriter.Limit.Storage)
					} else {
						tfWriter.Limit.Storage = types.StringNull()
					}
				}

				tfWriters = append(tfWriters, tfWriter)
			}

			// Convert to types.List
			writersList, diags := types.ListValueFrom(ctx, types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"name":                                types.StringType,
					"image_override":                      types.StringType,
					"hpa_specs":                           types.ObjectType{AttrTypes: map[string]attr.Type{"hpa_pubsub_subscription_id": types.StringType, "hpa_min_replicas": types.Int64Type, "hpa_max_replicas": types.Int64Type, "hpa_target_average_value": types.Int64Type}},
					"gke_spot":                            types.BoolType,
					"load_writer_configmap":               types.BoolType,
					"version":                             types.StringType,
					"request":                             types.ObjectType{AttrTypes: map[string]attr.Type{"cpu": types.StringType, "memory": types.StringType, "ephemeral_storage": types.StringType, "storage": types.StringType}},
					"limit":                               types.ObjectType{AttrTypes: map[string]attr.Type{"cpu": types.StringType, "memory": types.StringType, "ephemeral_storage": types.StringType, "storage": types.StringType}},
					"bus_subscriber_type":                 types.StringType,
					"default_replica_count":               types.Int64Type,
					"kafka_consumer_group_override":       types.StringType,
					"max_batch_size":                      types.Int64Type,
					"message_processing_concurrency":      types.Int64Type,
					"metadata_sql_ssl_ca_cert_secret":     types.StringType,
					"metadata_sql_ssl_client_cert_secret": types.StringType,
					"metadata_sql_ssl_client_key_secret":  types.StringType,
					"metadata_sql_uri_secret":             types.StringType,
					"offline_store_inserter_db_type":      types.StringType,
					"storage_cache_prefix":                types.StringType,
					"results_writer_skip_producing_feature_metrics": types.BoolType,
					"query_table_write_drop_ratio":                  types.StringType,
				},
			}, tfWriters)
			resp.Diagnostics.Append(diags...)
			if !resp.Diagnostics.HasError() {
				data.Writers = writersList
			}
		} else {
			data.Writers = types.ListNull(types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"name":                                types.StringType,
					"image_override":                      types.StringType,
					"hpa_specs":                           types.ObjectType{AttrTypes: map[string]attr.Type{"hpa_pubsub_subscription_id": types.StringType, "hpa_min_replicas": types.Int64Type, "hpa_max_replicas": types.Int64Type, "hpa_target_average_value": types.Int64Type}},
					"gke_spot":                            types.BoolType,
					"load_writer_configmap":               types.BoolType,
					"version":                             types.StringType,
					"request":                             types.ObjectType{AttrTypes: map[string]attr.Type{"cpu": types.StringType, "memory": types.StringType, "ephemeral_storage": types.StringType, "storage": types.StringType}},
					"limit":                               types.ObjectType{AttrTypes: map[string]attr.Type{"cpu": types.StringType, "memory": types.StringType, "ephemeral_storage": types.StringType, "storage": types.StringType}},
					"bus_subscriber_type":                 types.StringType,
					"default_replica_count":               types.Int64Type,
					"kafka_consumer_group_override":       types.StringType,
					"max_batch_size":                      types.Int64Type,
					"message_processing_concurrency":      types.Int64Type,
					"metadata_sql_ssl_ca_cert_secret":     types.StringType,
					"metadata_sql_ssl_client_cert_secret": types.StringType,
					"metadata_sql_ssl_client_key_secret":  types.StringType,
					"metadata_sql_uri_secret":             types.StringType,
					"offline_store_inserter_db_type":      types.StringType,
					"storage_cache_prefix":                types.StringType,
					"results_writer_skip_producing_feature_metrics": types.BoolType,
					"query_table_write_drop_ratio":                  types.StringType,
				},
			})
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

	var envIds []string
	// Convert environment IDs
	if !data.EnvironmentIds.IsNull() {
		diags := data.EnvironmentIds.ElementsAs(ctx, &envIds, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Convert writers
	var writers []BackgroundPersistenceWriterModel
	diags := data.Writers.ElementsAs(ctx, &writers, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var protoWriters []*serverv1.BackgroundPersistenceWriterSpecs
	for _, writer := range writers {
		protoWriter := &serverv1.BackgroundPersistenceWriterSpecs{
			Name:              writer.Name.ValueString(),
			BusSubscriberType: writer.BusSubscriberType.ValueString(),
		}

		// Handle optional string fields
		if !writer.ImageOverride.IsNull() {
			protoWriter.ImageOverride = writer.ImageOverride.ValueString()
		}
		if !writer.Version.IsNull() {
			protoWriter.Version = writer.Version.ValueString()
		}
		if !writer.DefaultReplicaCount.IsNull() {
			protoWriter.DefaultReplicaCount = int32(writer.DefaultReplicaCount.ValueInt64())
		}
		if !writer.MetadataSqlSslCaCertSecret.IsNull() {
			protoWriter.MetadataSqlSslCaCertSecret = writer.MetadataSqlSslCaCertSecret.ValueString()
		}
		if !writer.MetadataSqlSslClientCertSecret.IsNull() {
			protoWriter.MetadataSqlSslClientCertSecret = writer.MetadataSqlSslClientCertSecret.ValueString()
		}
		if !writer.MetadataSqlSslClientKeySecret.IsNull() {
			protoWriter.MetadataSqlSslClientKeySecret = writer.MetadataSqlSslClientKeySecret.ValueString()
		}
		if !writer.MetadataSqlUriSecret.IsNull() {
			protoWriter.MetadataSqlUriSecret = writer.MetadataSqlUriSecret.ValueString()
		}
		if !writer.OfflineStoreInserterDbType.IsNull() {
			protoWriter.OfflineStoreInserterDbType = writer.OfflineStoreInserterDbType.ValueString()
		}
		if !writer.StorageCachePrefix.IsNull() {
			protoWriter.StorageCachePrefix = writer.StorageCachePrefix.ValueString()
		}
		if !writer.QueryTableWriteDropRatio.IsNull() {
			protoWriter.QueryTableWriteDropRatio = writer.QueryTableWriteDropRatio.ValueString()
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
			protoWriter.Request = &serverv1.KubeResourceConfig{}
			if !writer.Request.CPU.IsNull() {
				protoWriter.Request.Cpu = writer.Request.CPU.ValueString()
			}
			if !writer.Request.Memory.IsNull() {
				protoWriter.Request.Memory = writer.Request.Memory.ValueString()
			}
			if !writer.Request.EphemeralStorage.IsNull() {
				protoWriter.Request.EphemeralStorage = writer.Request.EphemeralStorage.ValueString()
			}
			if !writer.Request.Storage.IsNull() {
				protoWriter.Request.Storage = writer.Request.Storage.ValueString()
			}
		}
		if writer.Limit != nil {
			protoWriter.Limit = &serverv1.KubeResourceConfig{}
			if !writer.Limit.CPU.IsNull() {
				protoWriter.Limit.Cpu = writer.Limit.CPU.ValueString()
			}
			if !writer.Limit.Memory.IsNull() {
				protoWriter.Limit.Memory = writer.Limit.Memory.ValueString()
			}
			if !writer.Limit.EphemeralStorage.IsNull() {
				protoWriter.Limit.EphemeralStorage = writer.Limit.EphemeralStorage.ValueString()
			}
			if !writer.Limit.Storage.IsNull() {
				protoWriter.Limit.Storage = writer.Limit.Storage.ValueString()
			}
		}

		protoWriters = append(protoWriters, protoWriter)
	}

	// Convert terraform model to proto request - reuse create logic since it's an upsert
	commonSpecs := &serverv1.BackgroundPersistenceCommonSpecs{
		Namespace:                            data.Namespace.ValueString(),
		ServiceAccountName:                   data.ServiceAccountName.ValueString(),
		BigqueryParquetUploadSubscriptionId:  data.BigqueryParquetUploadSubscriptionId.ValueString(),
		BigqueryStreamingWriteSubscriptionId: data.BigqueryStreamingWriteSubscriptionId.ValueString(),
		BigqueryStreamingWriteTopic:          data.BigqueryStreamingWriteTopic.ValueString(),
		BqUploadBucket:                       data.BqUploadBucket.ValueString(),
		BqUploadTopic:                        data.BqUploadTopic.ValueString(),
		MetricsBusSubscriptionId:             data.MetricsBusSubscriptionId.ValueString(),
		MetricsBusTopicId:                    data.MetricsBusTopicId.ValueString(),
		OperationSubscriptionId:              data.OperationSubscriptionId.ValueString(),
		QueryLogResultTopic:                  data.QueryLogResultTopic.ValueString(),
		QueryLogSubscriptionId:               data.QueryLogSubscriptionId.ValueString(),
		ResultBusOfflineStoreSubscriptionId:  data.ResultBusOfflineStoreSubscriptionId.ValueString(),
		ResultBusOnlineStoreSubscriptionId:   data.ResultBusOnlineStoreSubscriptionId.ValueString(),
		ResultBusTopicId:                     data.ResultBusTopicId.ValueString(),
		IncludeChalkNodeSelector:             data.IncludeChalkNodeSelector.ValueBool(),
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

	//create a deployment based on whether kube_cluster_id or environment_ids is provided
	var createReq *serverv1.CreateClusterBackgroundPersistenceRequest
	if !data.KubeClusterId.IsNull() {
		createReq = &serverv1.CreateClusterBackgroundPersistenceRequest{
			Specs:         deploymentSpecs,
			KubeClusterId: data.KubeClusterId.ValueStringPointer(),
		}
	} else {
		createReq = &serverv1.CreateClusterBackgroundPersistenceRequest{
			EnvironmentIds: envIds,
			Specs:          deploymentSpecs,
		}
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
