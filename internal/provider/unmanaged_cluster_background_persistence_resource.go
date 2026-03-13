package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	serverv1connect "github.com/chalk-ai/chalk-go/gen/chalk/server/v1/serverv1connect"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &UnmanagedClusterBackgroundPersistenceResource{}
var _ resource.ResourceWithImportState = &UnmanagedClusterBackgroundPersistenceResource{}
var _ resource.ResourceWithConfigValidators = &UnmanagedClusterBackgroundPersistenceResource{}

func NewUnmanagedClusterBackgroundPersistenceResource() resource.Resource {
	return &UnmanagedClusterBackgroundPersistenceResource{}
}

type UnmanagedClusterBackgroundPersistenceResource struct {
	client *ClientManager
}

type UnmanagedClusterBGPersistenceModel struct {
	Id                                          types.String          `tfsdk:"id"`
	KubeClusterId                               types.String          `tfsdk:"kube_cluster_id"`
	ServiceAccountName                          types.String          `tfsdk:"service_account_name"`
	Namespace                                   types.String          `tfsdk:"namespace"`
	ApiServerHost                               types.String          `tfsdk:"api_server_host"`
	OfflineStoreSnowflakeStorageIntegrationName types.String          `tfsdk:"offline_store_snowflake_storage_integration_name"`
	OfflineStoreUploadBucketName                types.String          `tfsdk:"offline_store_upload_bucket_name"`
	BusWriterImageGo                            types.String          `tfsdk:"bus_writer_image_go"`
	BusWriterImagePython                        types.String          `tfsdk:"bus_writer_image_python"`
	BusWriterImageBswl                          types.String          `tfsdk:"bus_writer_image_bswl"`
	BusWriterImageRust                          types.String          `tfsdk:"bus_writer_image_rust"`
	Writers                                     types.List            `tfsdk:"writers"`
	GooglePubSub                                *BGPGooglePubSubModel `tfsdk:"google_pubsub"`
	Kafka                                       *BGPKafkaModel        `tfsdk:"kafka"`
}

type BGPGooglePubSubModel struct {
	OfflineStoreUploadBus         *BGPOfflineStoreUploadModel         `tfsdk:"offline_store_upload_bus"`
	OfflineStoreStreamingWriteBus *BGPOfflineStoreStreamingWriteModel `tfsdk:"offline_store_streaming_write_bus"`
	MetricsBus                    *BGPGooglePubSubMetricsBusModel     `tfsdk:"metrics_bus"`
	ResultBus                     *BGPGooglePubSubResultBusModel      `tfsdk:"result_bus"`
}

type BGPOfflineStoreUploadModel struct {
	SubscriptionId types.String `tfsdk:"subscription_id"`
	TopicId        types.String `tfsdk:"topic_id"`
}

type BGPOfflineStoreStreamingWriteModel struct {
	SubscriptionId types.String `tfsdk:"subscription_id"`
	TopicId        types.String `tfsdk:"topic_id"`
}

type BGPGooglePubSubMetricsBusModel struct {
	SubscriptionId types.String `tfsdk:"subscription_id"`
	TopicId        types.String `tfsdk:"topic_id"`
}

type BGPGooglePubSubResultBusModel struct {
	OfflineStoreSubscriptionId types.String `tfsdk:"offline_store_subscription_id"`
	OnlineStoreSubscriptionId  types.String `tfsdk:"online_store_subscription_id"`
	TopicId                    types.String `tfsdk:"topic_id"`
}

type BGPKafkaModel struct {
	SaslSecret                           types.String `tfsdk:"sasl_secret"`
	BootstrapServers                     types.String `tfsdk:"bootstrap_servers"`
	SaslMechanism                        types.String `tfsdk:"sasl_mechanism"`
	SecurityProtocol                     types.String `tfsdk:"security_protocol"`
	DlqTopic                             types.String `tfsdk:"dlq_topic"`
	OfflineStoreBusUploadTopicId         types.String `tfsdk:"offline_store_bus_upload_topic_id"`
	OfflineStoreBusStreamingWriteTopicId types.String `tfsdk:"offline_store_bus_streaming_write_topic_id"`
	MetricsBusTopicId                    types.String `tfsdk:"metrics_bus_topic_id"`
	ResultBusTopicId                     types.String `tfsdk:"result_bus_topic_id"`
}

func (r *UnmanagedClusterBackgroundPersistenceResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("google_pubsub"),
			path.MatchRoot("kafka"),
		),
	}
}

func (r *UnmanagedClusterBackgroundPersistenceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_unmanaged_cluster_background_persistence"
}

func (r *UnmanagedClusterBackgroundPersistenceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	googlePubSubSchema := schema.SingleNestedAttribute{
		MarkdownDescription: "Google PubSub bus configuration. Exactly one of `google_pubsub` or `kafka` must be provided.",
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"offline_store_upload_bus": schema.SingleNestedAttribute{
				MarkdownDescription: "Offline store upload bus configuration",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"subscription_id": schema.StringAttribute{
						MarkdownDescription: "Offline store upload bus subscription ID",
						Required:            true,
					},
					"topic_id": schema.StringAttribute{
						MarkdownDescription: "Offline store upload bus topic ID",
						Required:            true,
					},
				},
			},
			"offline_store_streaming_write_bus": schema.SingleNestedAttribute{
				MarkdownDescription: "Offline store streaming write bus configuration",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"subscription_id": schema.StringAttribute{
						MarkdownDescription: "Offline store streaming write bus subscription ID",
						Required:            true,
					},
					"topic_id": schema.StringAttribute{
						MarkdownDescription: "Offline store streaming write bus topic ID",
						Required:            true,
					},
				},
			},
			"metrics_bus": schema.SingleNestedAttribute{
				MarkdownDescription: "Metrics bus configuration",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"subscription_id": schema.StringAttribute{
						MarkdownDescription: "Metrics bus subscription ID",
						Required:            true,
					},
					"topic_id": schema.StringAttribute{
						MarkdownDescription: "Metrics bus topic ID",
						Required:            true,
					},
				},
			},
			"result_bus": schema.SingleNestedAttribute{
				MarkdownDescription: "Result bus configuration",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"offline_store_subscription_id": schema.StringAttribute{
						MarkdownDescription: "Result bus offline store subscription ID",
						Required:            true,
					},
					"online_store_subscription_id": schema.StringAttribute{
						MarkdownDescription: "Result bus online store subscription ID",
						Required:            true,
					},
					"topic_id": schema.StringAttribute{
						MarkdownDescription: "Result bus topic ID",
						Required:            true,
					},
				},
			},
		},
	}

	kafkaSchema := schema.SingleNestedAttribute{
		MarkdownDescription: "Kafka bus configuration. Exactly one of `google_pubsub` or `kafka` must be provided.",
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"sasl_secret": schema.StringAttribute{
				MarkdownDescription: "Kafka SASL secret",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"bootstrap_servers": schema.StringAttribute{
				MarkdownDescription: "Kafka bootstrap servers",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"sasl_mechanism": schema.StringAttribute{
				MarkdownDescription: "Kafka SASL mechanism",
				Optional:            true,
			},
			"security_protocol": schema.StringAttribute{
				MarkdownDescription: "Kafka security protocol",
				Optional:            true,
			},
			"dlq_topic": schema.StringAttribute{
				MarkdownDescription: "Kafka DLQ topic",
				Required:            true,
			},
			"offline_store_bus_upload_topic_id": schema.StringAttribute{
				MarkdownDescription: "Offline store bus upload topic ID",
				Required:            true,
			},
			"offline_store_bus_streaming_write_topic_id": schema.StringAttribute{
				MarkdownDescription: "Offline store bus streaming write topic ID",
				Required:            true,
			},
			"metrics_bus_topic_id": schema.StringAttribute{
				MarkdownDescription: "Metrics bus topic ID",
				Required:            true,
			},
			"result_bus_topic_id": schema.StringAttribute{
				MarkdownDescription: "Result bus topic ID",
				Required:            true,
			},
		},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk unmanaged cluster background persistence resource.",

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
			"service_account_name": schema.StringAttribute{
				MarkdownDescription: "Service account name",
				Required:            true,
			},
			"namespace": schema.StringAttribute{
				MarkdownDescription: "Kubernetes namespace",
				Required:            true,
			},
			"api_server_host": schema.StringAttribute{
				MarkdownDescription: "API server host. Defaults to the provider's `api_server` if not set.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"offline_store_snowflake_storage_integration_name": schema.StringAttribute{
				MarkdownDescription: "Snowflake storage integration name",
				Optional:            true,
			},
			"offline_store_upload_bucket_name": schema.StringAttribute{
				MarkdownDescription: "Bucket name used for offline store uploads",
				Optional:            true,
			},
			"bus_writer_image_go": schema.StringAttribute{
				MarkdownDescription: "Go bus writer image. Only set this if instructed to by Chalk.",
				Optional:            true,
			},
			"bus_writer_image_python": schema.StringAttribute{
				MarkdownDescription: "Python bus writer image. Only set this if instructed to by Chalk.",
				Optional:            true,
			},
			"bus_writer_image_bswl": schema.StringAttribute{
				MarkdownDescription: "BSWL bus writer image. Only set this if instructed to by Chalk.",
				Optional:            true,
			},
			"bus_writer_image_rust": schema.StringAttribute{
				MarkdownDescription: "Rust bus writer image. Only set this if instructed to by Chalk.",
				Optional:            true,
			},
			"writers":       bgpUnmanagedWritersSchemaAttribute,
			"google_pubsub": googlePubSubSchema,
			"kafka":         kafkaSchema,
		},
	}
}

func (r *UnmanagedClusterBackgroundPersistenceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// applyApiServerHostDefault sets ApiServerHost from the provider's api_server when the
// user has not provided a value. Must be called before buildUnmanagedBGPProtoRequest.
func applyApiServerHostDefault(data *UnmanagedClusterBGPersistenceModel, defaultApiServer string) {
	if (data.ApiServerHost.IsNull() || data.ApiServerHost.IsUnknown()) && defaultApiServer != "" {
		data.ApiServerHost = types.StringValue(defaultApiServer)
	}
}

func buildUnmanagedBGPProtoRequest(ctx context.Context, data *UnmanagedClusterBGPersistenceModel) (*serverv1.CreateClusterBackgroundPersistenceRequest, error) {
	protoWriters, diags := bgpWritersTFToProto(ctx, data.Writers)
	if diags.HasError() {
		return nil, fmt.Errorf("error converting writers")
	}

	commonSpecs := &serverv1.BackgroundPersistenceCommonSpecs{
		ServiceAccountName: data.ServiceAccountName.ValueString(),
	}

	commonSpecs.Namespace = data.Namespace.ValueString()

	if !data.OfflineStoreUploadBucketName.IsNull() {
		commonSpecs.BqUploadBucket = data.OfflineStoreUploadBucketName.ValueString()
	}
	if !data.BusWriterImageGo.IsNull() {
		commonSpecs.BusWriterImageGo = data.BusWriterImageGo.ValueString()
	}
	if !data.BusWriterImagePython.IsNull() {
		commonSpecs.BusWriterImagePython = data.BusWriterImagePython.ValueString()
	}
	if !data.BusWriterImageBswl.IsNull() {
		commonSpecs.BusWriterImageBswl = data.BusWriterImageBswl.ValueString()
	}
	if !data.BusWriterImageRust.IsNull() {
		commonSpecs.BusWriterImageRust = data.BusWriterImageRust.ValueString()
	}

	// Apply google_pubsub fields to commonSpecs
	if data.GooglePubSub != nil {
		ps := data.GooglePubSub
		commonSpecs.BigqueryParquetUploadSubscriptionId = ps.OfflineStoreUploadBus.SubscriptionId.ValueString()
		commonSpecs.BqUploadTopic = ps.OfflineStoreUploadBus.TopicId.ValueString()
		commonSpecs.BigqueryStreamingWriteSubscriptionId = ps.OfflineStoreStreamingWriteBus.SubscriptionId.ValueString()
		commonSpecs.BigqueryStreamingWriteTopic = ps.OfflineStoreStreamingWriteBus.TopicId.ValueString()
		commonSpecs.MetricsBusSubscriptionId = ps.MetricsBus.SubscriptionId.ValueString()
		commonSpecs.MetricsBusTopicId = ps.MetricsBus.TopicId.ValueString()
		commonSpecs.ResultBusOfflineStoreSubscriptionId = ps.ResultBus.OfflineStoreSubscriptionId.ValueString()
		commonSpecs.ResultBusOnlineStoreSubscriptionId = ps.ResultBus.OnlineStoreSubscriptionId.ValueString()
		commonSpecs.ResultBusTopicId = ps.ResultBus.TopicId.ValueString()
	}

	// Apply kafka fields to commonSpecs and deploymentSpecs
	deploymentSpecs := &serverv1.BackgroundPersistenceDeploymentSpecs{
		CommonPersistenceSpecs: commonSpecs,
		Writers:                protoWriters,
	}

	if !data.ApiServerHost.IsNull() && !data.ApiServerHost.IsUnknown() {
		deploymentSpecs.ApiServerHost = data.ApiServerHost.ValueString()
	}
	if !data.OfflineStoreSnowflakeStorageIntegrationName.IsNull() {
		deploymentSpecs.SnowflakeStorageIntegrationName = data.OfflineStoreSnowflakeStorageIntegrationName.ValueString()
	}

	if data.Kafka != nil {
		k := data.Kafka
		if !k.SaslSecret.IsNull() {
			deploymentSpecs.KafkaSaslSecret = k.SaslSecret.ValueString()
		}
		if !k.BootstrapServers.IsNull() {
			deploymentSpecs.KafkaBootstrapServers = k.BootstrapServers.ValueString()
		}
		if !k.SaslMechanism.IsNull() {
			deploymentSpecs.KafkaSaslMechanism = k.SaslMechanism.ValueString()
		}
		if !k.SecurityProtocol.IsNull() {
			deploymentSpecs.KafkaSecurityProtocol = k.SecurityProtocol.ValueString()
		}
		commonSpecs.KafkaDlqTopic = k.DlqTopic.ValueString()
		commonSpecs.BqUploadTopic = k.OfflineStoreBusUploadTopicId.ValueString()
		commonSpecs.BigqueryStreamingWriteTopic = k.OfflineStoreBusStreamingWriteTopicId.ValueString()
		commonSpecs.MetricsBusTopicId = k.MetricsBusTopicId.ValueString()
		commonSpecs.ResultBusTopicId = k.ResultBusTopicId.ValueString()
	}

	createReq := &serverv1.CreateClusterBackgroundPersistenceRequest{
		Specs:         deploymentSpecs,
		KubeClusterId: data.KubeClusterId.ValueStringPointer(),
	}

	if !data.Id.IsNull() {
		createReq.Id = data.Id.ValueStringPointer()
	}

	return createReq, nil
}

// refreshWritersAfterWrite fetches the current specs from the server and returns the
// writers list with server-populated computed fields (e.g. name).
func (r *UnmanagedClusterBackgroundPersistenceResource) refreshWritersAfterWrite(
	ctx context.Context,
	bc serverv1connect.BuilderServiceClient,
	id string,
) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	getResp, err := bc.GetClusterBackgroundPersistence(ctx, connect.NewRequest(&serverv1.GetClusterBackgroundPersistenceRequest{
		Id: &id,
	}))
	if err != nil {
		diags.AddError(
			"Error Reading Chalk Unmanaged Cluster Background Persistence After Write",
			fmt.Sprintf("Could not read unmanaged cluster background persistence %s: %v", id, err),
		)
		return types.ListNull(bgpWriterObjectType), diags
	}
	if getResp.Msg.BackgroundPersistence == nil || getResp.Msg.BackgroundPersistence.Specs == nil {
		diags.AddError(
			"Unexpected Response After Write",
			fmt.Sprintf("Server returned no specs for background persistence %s after write", id),
		)
		return types.ListNull(bgpWriterObjectType), diags
	}
	return bgpWritersProtoToTF(ctx, getResp.Msg.BackgroundPersistence.Specs.Writers)
}

func (r *UnmanagedClusterBackgroundPersistenceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UnmanagedClusterBGPersistenceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	bc := r.client.NewBuilderClient(ctx)

	applyApiServerHostDefault(&data, r.client.GetChalkClient().ApiServer)

	createReq, err := buildUnmanagedBGPProtoRequest(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Error Building Request", err.Error())
		return
	}

	response, err := bc.CreateClusterBackgroundPersistence(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Unmanaged Cluster Background Persistence",
			fmt.Sprintf("Could not create unmanaged cluster background persistence: %v", err),
		)
		return
	}

	data.Id = types.StringValue(response.Msg.Id)

	// Refresh writers from the server to populate computed fields (e.g. name).
	writersList, writerDiags := r.refreshWritersAfterWrite(ctx, bc, data.Id.ValueString())
	resp.Diagnostics.Append(writerDiags...)
	if !resp.Diagnostics.HasError() {
		data.Writers = writersList
	}

	tflog.Trace(ctx, "created a chalk_unmanaged_cluster_background_persistence resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UnmanagedClusterBackgroundPersistenceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UnmanagedClusterBGPersistenceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	bc := r.client.NewBuilderClient(ctx)

	getReq := &serverv1.GetClusterBackgroundPersistenceRequest{
		Id: data.Id.ValueStringPointer(),
	}

	bgPersistence, err := bc.GetClusterBackgroundPersistence(ctx, connect.NewRequest(getReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Chalk Unmanaged Cluster Background Persistence",
			fmt.Sprintf("Could not read unmanaged cluster background persistence %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	bg := bgPersistence.Msg.BackgroundPersistence

	if bg == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if bg.GetKubeClusterId() != "" {
		data.KubeClusterId = types.StringValue(bg.GetKubeClusterId())
	} else {
		data.KubeClusterId = types.StringNull()
	}

	if bg.Specs != nil && bg.Specs.CommonPersistenceSpecs != nil {
		common := bg.Specs.CommonPersistenceSpecs

		data.ServiceAccountName = types.StringValue(common.ServiceAccountName)

		if common.Namespace != "" {
			data.Namespace = types.StringValue(common.Namespace)
		} else {
			data.Namespace = types.StringNull()
		}

		if bg.Specs.ApiServerHost != "" {
			data.ApiServerHost = types.StringValue(bg.Specs.ApiServerHost)
		} else {
			data.ApiServerHost = types.StringNull()
		}
		if bg.Specs.SnowflakeStorageIntegrationName != "" {
			data.OfflineStoreSnowflakeStorageIntegrationName = types.StringValue(bg.Specs.SnowflakeStorageIntegrationName)
		} else {
			data.OfflineStoreSnowflakeStorageIntegrationName = types.StringNull()
		}

		if common.BqUploadBucket != "" {
			data.OfflineStoreUploadBucketName = types.StringValue(common.BqUploadBucket)
		} else {
			data.OfflineStoreUploadBucketName = types.StringNull()
		}

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

		// Reconstruct google_pubsub or kafka block based on which bus type is configured.
		// Kafka is identified by the presence of kafka-specific deployment fields.
		if bg.Specs.KafkaSaslSecret != "" || bg.Specs.KafkaBootstrapServers != "" {
			data.GooglePubSub = nil
		} else {
			// PubSub mode: all four sub-blocks are Required in the schema, so always populate them.
			data.GooglePubSub = &BGPGooglePubSubModel{
				OfflineStoreUploadBus: &BGPOfflineStoreUploadModel{
					SubscriptionId: types.StringValue(common.BigqueryParquetUploadSubscriptionId),
					TopicId:        types.StringValue(common.BqUploadTopic),
				},
				OfflineStoreStreamingWriteBus: &BGPOfflineStoreStreamingWriteModel{
					SubscriptionId: types.StringValue(common.BigqueryStreamingWriteSubscriptionId),
					TopicId:        types.StringValue(common.BigqueryStreamingWriteTopic),
				},
				MetricsBus: &BGPGooglePubSubMetricsBusModel{
					SubscriptionId: types.StringValue(common.MetricsBusSubscriptionId),
					TopicId:        types.StringValue(common.MetricsBusTopicId),
				},
				ResultBus: &BGPGooglePubSubResultBusModel{
					OfflineStoreSubscriptionId: types.StringValue(common.ResultBusOfflineStoreSubscriptionId),
					OnlineStoreSubscriptionId:  types.StringValue(common.ResultBusOnlineStoreSubscriptionId),
					TopicId:                    types.StringValue(common.ResultBusTopicId),
				},
			}
		}

		// Reconstruct kafka block if kafka fields are set
		if bg.Specs.KafkaSaslSecret != "" || bg.Specs.KafkaBootstrapServers != "" {
			data.Kafka = &BGPKafkaModel{}
			if bg.Specs.KafkaSaslSecret != "" {
				data.Kafka.SaslSecret = types.StringValue(bg.Specs.KafkaSaslSecret)
			} else {
				data.Kafka.SaslSecret = types.StringNull()
			}
			if bg.Specs.KafkaBootstrapServers != "" {
				data.Kafka.BootstrapServers = types.StringValue(bg.Specs.KafkaBootstrapServers)
			} else {
				data.Kafka.BootstrapServers = types.StringNull()
			}
			if bg.Specs.KafkaSaslMechanism != "" {
				data.Kafka.SaslMechanism = types.StringValue(bg.Specs.KafkaSaslMechanism)
			} else {
				data.Kafka.SaslMechanism = types.StringNull()
			}
			if bg.Specs.KafkaSecurityProtocol != "" {
				data.Kafka.SecurityProtocol = types.StringValue(bg.Specs.KafkaSecurityProtocol)
			} else {
				data.Kafka.SecurityProtocol = types.StringNull()
			}
			data.Kafka.DlqTopic = types.StringValue(common.KafkaDlqTopic)
			data.Kafka.OfflineStoreBusUploadTopicId = types.StringValue(common.BqUploadTopic)
			data.Kafka.OfflineStoreBusStreamingWriteTopicId = types.StringValue(common.BigqueryStreamingWriteTopic)
			data.Kafka.MetricsBusTopicId = types.StringValue(common.MetricsBusTopicId)
			data.Kafka.ResultBusTopicId = types.StringValue(common.ResultBusTopicId)
		} else {
			data.Kafka = nil
		}

		// Update writers
		writersList, writerDiags := bgpWritersProtoToTF(ctx, bg.Specs.Writers)
		resp.Diagnostics.Append(writerDiags...)
		if !resp.Diagnostics.HasError() {
			data.Writers = writersList
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UnmanagedClusterBackgroundPersistenceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UnmanagedClusterBGPersistenceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read ID from current state for upsert
	var state UnmanagedClusterBGPersistenceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Id = state.Id

	bc := r.client.NewBuilderClient(ctx)

	applyApiServerHostDefault(&data, r.client.GetChalkClient().ApiServer)

	createReq, err := buildUnmanagedBGPProtoRequest(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Error Building Request", err.Error())
		return
	}

	response, err := bc.CreateClusterBackgroundPersistence(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Chalk Unmanaged Cluster Background Persistence",
			fmt.Sprintf("Could not update unmanaged cluster background persistence: %v", err),
		)
		return
	}

	data.Id = types.StringValue(response.Msg.Id)

	// Refresh writers from the server to populate computed fields (e.g. name).
	writersList, writerDiags := r.refreshWritersAfterWrite(ctx, bc, data.Id.ValueString())
	resp.Diagnostics.Append(writerDiags...)
	if !resp.Diagnostics.HasError() {
		data.Writers = writersList
	}

	tflog.Trace(ctx, "updated a chalk_unmanaged_cluster_background_persistence resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UnmanagedClusterBackgroundPersistenceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Note: According to the proto definition, there's no DeleteClusterBackgroundPersistence method
	// This means the background persistence lifecycle might be managed differently
	// For now, we'll just remove it from Terraform state
	tflog.Trace(ctx, "cluster background persistence deletion - removing from terraform state only (no API delete available)")
}

func (r *UnmanagedClusterBackgroundPersistenceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
