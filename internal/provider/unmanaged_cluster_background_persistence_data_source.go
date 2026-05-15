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

var _ datasource.DataSource = &UnmanagedClusterBackgroundPersistenceDataSource{}

func NewUnmanagedClusterBackgroundPersistenceDataSource() datasource.DataSource {
	return &UnmanagedClusterBackgroundPersistenceDataSource{}
}

type UnmanagedClusterBackgroundPersistenceDataSource struct {
	client *client.Manager
}

// UnmanagedClusterBGPersistenceDataSourceModel mirrors the resource model
// minus autodiscover_key (deliberately omitted — sensitive on the resource).
type UnmanagedClusterBGPersistenceDataSourceModel struct {
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

func (d *UnmanagedClusterBackgroundPersistenceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_unmanaged_cluster_background_persistence"
}

func (d *UnmanagedClusterBackgroundPersistenceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	googlePubSubSchema := schema.SingleNestedAttribute{
		MarkdownDescription: "Google PubSub bus configuration (populated if the deployment uses PubSub).",
		Computed:            true,
		Attributes: map[string]schema.Attribute{
			"offline_store_upload_bus": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"subscription_id": schema.StringAttribute{Computed: true},
					"topic_id":        schema.StringAttribute{Computed: true},
				},
			},
			"offline_store_streaming_write_bus": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"subscription_id": schema.StringAttribute{Computed: true},
					"topic_id":        schema.StringAttribute{Computed: true},
				},
			},
			"metrics_bus": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"subscription_id": schema.StringAttribute{Computed: true},
					"topic_id":        schema.StringAttribute{Computed: true},
				},
			},
			"result_bus": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"offline_store_subscription_id": schema.StringAttribute{Computed: true},
					"online_store_subscription_id":  schema.StringAttribute{Computed: true},
					"topic_id":                      schema.StringAttribute{Computed: true},
				},
			},
		},
	}

	kafkaSchema := schema.SingleNestedAttribute{
		MarkdownDescription: "Kafka bus configuration (populated if the deployment uses Kafka).",
		Computed:            true,
		Attributes: map[string]schema.Attribute{
			"sasl_secret":                       schema.StringAttribute{Computed: true},
			"bootstrap_servers":                 schema.StringAttribute{Computed: true},
			"sasl_mechanism":                    schema.StringAttribute{Computed: true},
			"security_protocol":                 schema.StringAttribute{Computed: true},
			"dlq_topic":                         schema.StringAttribute{Computed: true},
			"offline_store_bus_upload_topic_id": schema.StringAttribute{Computed: true},
			"offline_store_bus_streaming_write_topic_id": schema.StringAttribute{Computed: true},
			"metrics_bus_topic_id":                       schema.StringAttribute{Computed: true},
			"result_bus_topic_id":                        schema.StringAttribute{Computed: true},
		},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Chalk unmanaged cluster background persistence deployment by ID. **Does not** surface `autodiscover_key` (Sensitive on the resource).",
		Attributes: map[string]schema.Attribute{
			"id":                   schema.StringAttribute{MarkdownDescription: "Background persistence identifier.", Required: true},
			"kube_cluster_id":      schema.StringAttribute{Computed: true},
			"service_account_name": schema.StringAttribute{Computed: true},
			"namespace":            schema.StringAttribute{Computed: true},
			"api_server_host":      schema.StringAttribute{Computed: true},
			"offline_store_snowflake_storage_integration_name": schema.StringAttribute{Computed: true},
			"offline_store_upload_bucket_name":                 schema.StringAttribute{Computed: true},
			"bus_writer_image_go":                              schema.StringAttribute{Computed: true},
			"bus_writer_image_python":                          schema.StringAttribute{Computed: true},
			"bus_writer_image_bswl":                            schema.StringAttribute{Computed: true},
			"bus_writer_image_rust":                            schema.StringAttribute{Computed: true},
			"writers":                                          bgpUnmanagedWritersDataSourceSchemaAttribute(),
			"google_pubsub":                                    googlePubSubSchema,
			"kafka":                                            kafkaSchema,
		},
	}
}

func (d *UnmanagedClusterBackgroundPersistenceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *UnmanagedClusterBackgroundPersistenceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UnmanagedClusterBGPersistenceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "read chalk_unmanaged_cluster_background_persistence data source", map[string]any{"id": data.Id.ValueString()})

	bc := d.client.NewBuilderClient(ctx)
	getReq := &serverv1.GetClusterBackgroundPersistenceRequest{
		Id: data.Id.ValueStringPointer(),
	}
	bgResp, err := bc.GetClusterBackgroundPersistence(ctx, connect.NewRequest(getReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Chalk Unmanaged Cluster Background Persistence",
			fmt.Sprintf("Could not read unmanaged cluster background persistence %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	bg := bgResp.Msg.BackgroundPersistence
	if bg == nil {
		resp.Diagnostics.AddError(
			"Empty Background Persistence Response",
			fmt.Sprintf("Server returned no background persistence for %s", data.Id.ValueString()),
		)
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
	data.ServiceAccountName = types.StringValue(common.ServiceAccountName)
	data.Namespace = stringOrNull(common.Namespace)
	data.ApiServerHost = stringOrNull(bg.Specs.ApiServerHost)
	data.OfflineStoreSnowflakeStorageIntegrationName = stringOrNull(bg.Specs.SnowflakeStorageIntegrationName)
	data.OfflineStoreUploadBucketName = stringOrNull(common.BqUploadBucket)
	data.BusWriterImageGo = stringOrNull(common.BusWriterImageGo)
	data.BusWriterImagePython = stringOrNull(common.BusWriterImagePython)
	data.BusWriterImageBswl = stringOrNull(common.BusWriterImageBswl)
	data.BusWriterImageRust = stringOrNull(common.BusWriterImageRust)

	isKafka := bg.Specs.KafkaSaslSecret != "" || bg.Specs.KafkaBootstrapServers != ""
	if isKafka {
		data.Kafka = &BGPKafkaModel{
			SaslSecret:                           stringOrNull(bg.Specs.KafkaSaslSecret),
			BootstrapServers:                     stringOrNull(bg.Specs.KafkaBootstrapServers),
			SaslMechanism:                        stringOrNull(bg.Specs.KafkaSaslMechanism),
			SecurityProtocol:                     stringOrNull(bg.Specs.KafkaSecurityProtocol),
			DlqTopic:                             types.StringValue(common.KafkaDlqTopic),
			OfflineStoreBusUploadTopicId:         types.StringValue(common.BqUploadTopic),
			OfflineStoreBusStreamingWriteTopicId: types.StringValue(common.BigqueryStreamingWriteTopic),
			MetricsBusTopicId:                    types.StringValue(common.MetricsBusTopicId),
			ResultBusTopicId:                     types.StringValue(common.ResultBusTopicId),
		}
		data.GooglePubSub = nil
	} else {
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
		data.Kafka = nil
	}

	writersList, writerDiags := bgpWritersProtoToTF(ctx, bg.Specs.Writers)
	resp.Diagnostics.Append(writerDiags...)
	if !resp.Diagnostics.HasError() {
		data.Writers = writersList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
