package provider

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/internal/client"
	"github.com/cockroachdb/errors"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &EnvironmentDataSource{}

func NewEnvironmentDataSource() datasource.DataSource {
	return &EnvironmentDataSource{}
}

type EnvironmentDataSource struct {
	client *client.Manager
}

type EnvironmentDataSourceModel struct {
	Id                                       types.String `tfsdk:"id"`
	Name                                     types.String `tfsdk:"name"`
	ProjectId                                types.String `tfsdk:"project_id"`
	TeamId                                   types.String `tfsdk:"team_id"`
	ActiveDeploymentId                       types.String `tfsdk:"active_deployment_id"`
	WorkerUrl                                types.String `tfsdk:"worker_url"`
	ServiceUrl                               types.String `tfsdk:"service_url"`
	BranchUrl                                types.String `tfsdk:"branch_url"`
	OfflineStoreSecret                       types.String `tfsdk:"offline_store_secret"`
	OnlineStoreSecret                        types.String `tfsdk:"online_store_secret"`
	FeatureStoreSecret                       types.String `tfsdk:"feature_store_secret"`
	PostgresSecret                           types.String `tfsdk:"postgres_secret"`
	OnlineStoreKind                          types.String `tfsdk:"online_store_kind"`
	EmqUri                                   types.String `tfsdk:"emq_uri"`
	VpcConnectorName                         types.String `tfsdk:"vpc_connector_name"`
	KubeClusterName                          types.String `tfsdk:"kube_cluster_name"`
	BranchKubeClusterName                    types.String `tfsdk:"branch_kube_cluster_name"`
	EngineKubeClusterName                    types.String `tfsdk:"engine_kube_cluster_name"`
	ShadowEngineKubeClusterName              types.String `tfsdk:"shadow_engine_kube_cluster_name"`
	KubeJobNamespace                         types.String `tfsdk:"kube_job_namespace"`
	KubePreviewNamespace                     types.String `tfsdk:"kube_preview_namespace"`
	KubeServiceAccountName                   types.String `tfsdk:"kube_service_account_name"`
	StreamingQueryServiceUri                 types.String `tfsdk:"streaming_query_service_uri"`
	SkipOfflineWritesForOnlineCachedFeatures types.Bool   `tfsdk:"skip_offline_writes_for_online_cached_features"`
	ResultBusTopic                           types.String `tfsdk:"result_bus_topic"`
	OnlinePersistenceMode                    types.String `tfsdk:"online_persistence_mode"`
	MetricsBusTopic                          types.String `tfsdk:"metrics_bus_topic"`
	BigtableInstanceName                     types.String `tfsdk:"bigtable_instance_name"`
	BigtableTableName                        types.String `tfsdk:"bigtable_table_name"`
	CloudAccountLocator                      types.String `tfsdk:"cloud_account_locator"`
	CloudRegion                              types.String `tfsdk:"cloud_region"`
	CloudTenancyId                           types.String `tfsdk:"cloud_tenancy_id"`
	SourceBundleBucket                       types.String `tfsdk:"source_bundle_bucket"`
	EngineDockerRegistryPath                 types.String `tfsdk:"engine_docker_registry_path"`
	DefaultPlanner                           types.String `tfsdk:"default_planner"`
	AdditionalEnvVars                        types.Map    `tfsdk:"additional_env_vars"`
	AdditionalCronEnvVars                    types.Map    `tfsdk:"additional_cron_env_vars"`
	PrivatePipRepositories                   types.String `tfsdk:"private_pip_repositories"`
	IsSandbox                                types.Bool   `tfsdk:"is_sandbox"`
	CloudProvider                            types.String `tfsdk:"cloud_provider"`
	CloudConfig                              types.Object `tfsdk:"cloud_config"`
	SpecConfigJson                           types.Map    `tfsdk:"spec_config_json"`
	ArchivedAt                               types.String `tfsdk:"archived_at"`
	MetadataServerMetricsStoreSecret         types.String `tfsdk:"metadata_server_metrics_store_secret"`
	QueryServerMetricsStoreSecret            types.String `tfsdk:"query_server_metrics_store_secret"`
	PinnedBaseImage                          types.String `tfsdk:"pinned_base_image"`
	ClusterGatewayId                         types.String `tfsdk:"cluster_gateway_id"`
	ClusterTimescaledbId                     types.String `tfsdk:"cluster_timescaledb_id"`
	BackgroundPersistenceDeploymentId        types.String `tfsdk:"background_persistence_deployment_id"`
	EnvironmentBuckets                       types.Object `tfsdk:"environment_buckets"`
	ClusterTimescaledbSecret                 types.String `tfsdk:"cluster_timescaledb_secret"`
	GrpcEngineUrl                            types.String `tfsdk:"grpc_engine_url"`
	KubeClusterMode                          types.String `tfsdk:"kube_cluster_mode"`
	DashboardUrl                             types.String `tfsdk:"dashboard_url"`
}

func (d *EnvironmentDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (d *EnvironmentDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk environment data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Environment identifier",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Environment name",
				Computed:            true,
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Project ID",
				Computed:            true,
			},
			"team_id": schema.StringAttribute{
				MarkdownDescription: "Team ID",
				Computed:            true,
			},
			"active_deployment_id": schema.StringAttribute{
				MarkdownDescription: "Active deployment ID",
				Computed:            true,
			},
			"worker_url": schema.StringAttribute{
				MarkdownDescription: "Worker URL",
				Computed:            true,
			},
			"service_url": schema.StringAttribute{
				MarkdownDescription: "Service URL",
				Computed:            true,
			},
			"branch_url": schema.StringAttribute{
				MarkdownDescription: "Branch URL",
				Computed:            true,
			},
			"offline_store_secret": schema.StringAttribute{
				MarkdownDescription: "Offline store secret",
				Computed:            true,
			},
			"online_store_secret": schema.StringAttribute{
				MarkdownDescription: "Online store secret",
				Computed:            true,
			},
			"feature_store_secret": schema.StringAttribute{
				MarkdownDescription: "Feature store secret",
				Computed:            true,
			},
			"postgres_secret": schema.StringAttribute{
				MarkdownDescription: "Postgres secret",
				Computed:            true,
			},
			"online_store_kind": schema.StringAttribute{
				MarkdownDescription: "Online store kind for the environment",
				Computed:            true,
			},
			"emq_uri": schema.StringAttribute{
				MarkdownDescription: "EMQ URI",
				Computed:            true,
			},
			"vpc_connector_name": schema.StringAttribute{
				MarkdownDescription: "VPC connector name",
				Computed:            true,
			},
			"kube_cluster_name": schema.StringAttribute{
				MarkdownDescription: "Kubernetes cluster name",
				Computed:            true,
			},
			"branch_kube_cluster_name": schema.StringAttribute{
				MarkdownDescription: "Branch Kubernetes cluster name",
				Computed:            true,
			},
			"engine_kube_cluster_name": schema.StringAttribute{
				MarkdownDescription: "Engine Kubernetes cluster name",
				Computed:            true,
			},
			"shadow_engine_kube_cluster_name": schema.StringAttribute{
				MarkdownDescription: "Shadow engine Kubernetes cluster name",
				Computed:            true,
			},
			"kube_job_namespace": schema.StringAttribute{
				MarkdownDescription: "Kubernetes job namespace",
				Computed:            true,
			},
			"kube_preview_namespace": schema.StringAttribute{
				MarkdownDescription: "Kubernetes preview namespace",
				Computed:            true,
			},
			"kube_service_account_name": schema.StringAttribute{
				MarkdownDescription: "Kubernetes service account name",
				Computed:            true,
			},
			"streaming_query_service_uri": schema.StringAttribute{
				MarkdownDescription: "Streaming query service URI",
				Computed:            true,
			},
			"skip_offline_writes_for_online_cached_features": schema.BoolAttribute{
				MarkdownDescription: "Skip offline writes for online cached features",
				Computed:            true,
			},
			"result_bus_topic": schema.StringAttribute{
				MarkdownDescription: "Result bus topic",
				Computed:            true,
			},
			"online_persistence_mode": schema.StringAttribute{
				MarkdownDescription: "Online persistence mode",
				Computed:            true,
			},
			"metrics_bus_topic": schema.StringAttribute{
				MarkdownDescription: "Metrics bus topic",
				Computed:            true,
			},
			"bigtable_instance_name": schema.StringAttribute{
				MarkdownDescription: "Bigtable instance name",
				Computed:            true,
			},
			"bigtable_table_name": schema.StringAttribute{
				MarkdownDescription: "Bigtable table name",
				Computed:            true,
			},
			"cloud_account_locator": schema.StringAttribute{
				MarkdownDescription: "Cloud Account Locator",
				Computed:            true,
			},
			"cloud_region": schema.StringAttribute{
				MarkdownDescription: "Cloud region",
				Computed:            true,
			},
			"cloud_tenancy_id": schema.StringAttribute{
				MarkdownDescription: "Cloud tenancy ID",
				Computed:            true,
			},
			"source_bundle_bucket": schema.StringAttribute{
				MarkdownDescription: "Source bundle bucket",
				Computed:            true,
			},
			"engine_docker_registry_path": schema.StringAttribute{
				MarkdownDescription: "Engine Docker registry path",
				Computed:            true,
			},
			"default_planner": schema.StringAttribute{
				MarkdownDescription: "Default planner",
				Computed:            true,
			},
			"additional_env_vars": schema.MapAttribute{
				MarkdownDescription: "Additional environment variables",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"additional_cron_env_vars": schema.MapAttribute{
				MarkdownDescription: "Additional cron environment variables",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"private_pip_repositories": schema.StringAttribute{
				MarkdownDescription: "Private pip repositories",
				Computed:            true,
			},
			"is_sandbox": schema.BoolAttribute{
				MarkdownDescription: "Is sandbox environment",
				Computed:            true,
			},
			"cloud_provider": schema.StringAttribute{
				MarkdownDescription: "Cloud provider",
				Computed:            true,
			},
			"cloud_config": schema.ObjectAttribute{
				MarkdownDescription: "Cloud configuration",
				Computed:            true,
				AttributeTypes: map[string]attr.Type{
					"aws": types.ObjectType{AttrTypes: map[string]attr.Type{
						"account_id":          types.StringType,
						"management_role_arn": types.StringType,
						"region":              types.StringType,
						"external_id":         types.StringType,
					}},
					"gcp": types.ObjectType{AttrTypes: map[string]attr.Type{
						"project_id":                 types.StringType,
						"region":                     types.StringType,
						"management_service_account": types.StringType,
					}},
				},
			},
			"spec_config_json": schema.MapAttribute{
				MarkdownDescription: "Spec config JSON",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"archived_at": schema.StringAttribute{
				MarkdownDescription: "Archived at timestamp",
				Computed:            true,
			},
			"metadata_server_metrics_store_secret": schema.StringAttribute{
				MarkdownDescription: "Metadata server metrics store secret",
				Computed:            true,
			},
			"query_server_metrics_store_secret": schema.StringAttribute{
				MarkdownDescription: "Query server metrics store secret",
				Computed:            true,
			},
			"pinned_base_image": schema.StringAttribute{
				MarkdownDescription: "Pinned base image",
				Computed:            true,
			},
			"cluster_gateway_id": schema.StringAttribute{
				MarkdownDescription: "Cluster gateway ID",
				Computed:            true,
			},
			"cluster_timescaledb_id": schema.StringAttribute{
				MarkdownDescription: "Cluster TimescaleDB ID",
				Computed:            true,
			},
			"background_persistence_deployment_id": schema.StringAttribute{
				MarkdownDescription: "Background persistence deployment ID",
				Computed:            true,
			},
			"environment_buckets": schema.ObjectAttribute{
				MarkdownDescription: "Environment object storage buckets",
				Computed:            true,
				AttributeTypes: map[string]attr.Type{
					"dataset_bucket":        types.StringType,
					"plan_stages_bucket":    types.StringType,
					"source_bundle_bucket":  types.StringType,
					"model_registry_bucket": types.StringType,
				},
			},
			"cluster_timescaledb_secret": schema.StringAttribute{
				MarkdownDescription: "Cluster TimescaleDB secret",
				Computed:            true,
			},
			"grpc_engine_url": schema.StringAttribute{
				MarkdownDescription: "gRPC engine URL",
				Computed:            true,
			},
			"kube_cluster_mode": schema.StringAttribute{
				MarkdownDescription: "Kubernetes cluster mode",
				Computed:            true,
			},
			"dashboard_url": schema.StringAttribute{
				MarkdownDescription: "Dashboard URL",
				Computed:            true,
			},
		},
	}
}

func (d *EnvironmentDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Manager)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Manager, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

func (d *EnvironmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EnvironmentDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "read chalk_environment data source", map[string]interface{}{
		"id": data.Id.ValueString(),
	})

	// Create team client
	tc, err := d.client.NewTeamClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("client error", errors.Wrap(err, "get team client").Error())
		return
	}

	env, err := tc.GetEnv(ctx, connect.NewRequest(&serverv1.GetEnvRequest{}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Chalk Environment",
			fmt.Sprintf("Could not read environment %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Map all fields from the Environment protobuf message
	e := env.Msg.Environment
	data.Name = types.StringValue(e.Name)
	data.ProjectId = types.StringValue(e.ProjectId)
	data.TeamId = types.StringValue(e.TeamId)

	// Optional string fields
	if e.ActiveDeploymentId != nil {
		data.ActiveDeploymentId = types.StringValue(*e.ActiveDeploymentId)
	} else {
		data.ActiveDeploymentId = types.StringNull()
	}

	if e.WorkerUrl != nil {
		data.WorkerUrl = types.StringValue(*e.WorkerUrl)
	} else {
		data.WorkerUrl = types.StringNull()
	}

	if e.ServiceUrl != nil {
		data.ServiceUrl = types.StringValue(*e.ServiceUrl)
	} else {
		data.ServiceUrl = types.StringNull()
	}

	if e.BranchUrl != nil {
		data.BranchUrl = types.StringValue(*e.BranchUrl)
	} else {
		data.BranchUrl = types.StringNull()
	}

	if e.OfflineStoreSecret != nil {
		data.OfflineStoreSecret = types.StringValue(*e.OfflineStoreSecret)
	} else {
		data.OfflineStoreSecret = types.StringNull()
	}

	if e.OnlineStoreSecret != nil {
		data.OnlineStoreSecret = types.StringValue(*e.OnlineStoreSecret)
	} else {
		data.OnlineStoreSecret = types.StringNull()
	}

	if e.FeatureStoreSecret != nil {
		data.FeatureStoreSecret = types.StringValue(*e.FeatureStoreSecret)
	} else {
		data.FeatureStoreSecret = types.StringNull()
	}

	if e.PostgresSecret != nil {
		data.PostgresSecret = types.StringValue(*e.PostgresSecret)
	} else {
		data.PostgresSecret = types.StringNull()
	}

	if e.OnlineStoreKind != nil {
		data.OnlineStoreKind = types.StringValue(*e.OnlineStoreKind)
	} else {
		data.OnlineStoreKind = types.StringNull()
	}

	if e.EmqUri != nil {
		data.EmqUri = types.StringValue(*e.EmqUri)
	} else {
		data.EmqUri = types.StringNull()
	}

	if e.VpcConnectorName != nil {
		data.VpcConnectorName = types.StringValue(*e.VpcConnectorName)
	} else {
		data.VpcConnectorName = types.StringNull()
	}

	if e.KubeClusterName != nil {
		data.KubeClusterName = types.StringValue(*e.KubeClusterName)
	} else {
		data.KubeClusterName = types.StringNull()
	}

	if e.BranchKubeClusterName != nil {
		data.BranchKubeClusterName = types.StringValue(*e.BranchKubeClusterName)
	} else {
		data.BranchKubeClusterName = types.StringNull()
	}

	if e.EngineKubeClusterName != nil {
		data.EngineKubeClusterName = types.StringValue(*e.EngineKubeClusterName)
	} else {
		data.EngineKubeClusterName = types.StringNull()
	}

	if e.ShadowEngineKubeClusterName != nil {
		data.ShadowEngineKubeClusterName = types.StringValue(*e.ShadowEngineKubeClusterName)
	} else {
		data.ShadowEngineKubeClusterName = types.StringNull()
	}

	if e.KubeJobNamespace != nil {
		data.KubeJobNamespace = types.StringValue(*e.KubeJobNamespace)
	} else {
		data.KubeJobNamespace = types.StringNull()
	}

	if e.KubePreviewNamespace != nil {
		data.KubePreviewNamespace = types.StringValue(*e.KubePreviewNamespace)
	} else {
		data.KubePreviewNamespace = types.StringNull()
	}

	if e.KubeServiceAccountName != nil {
		data.KubeServiceAccountName = types.StringValue(*e.KubeServiceAccountName)
	} else {
		data.KubeServiceAccountName = types.StringNull()
	}

	if e.StreamingQueryServiceUri != nil {
		data.StreamingQueryServiceUri = types.StringValue(*e.StreamingQueryServiceUri)
	} else {
		data.StreamingQueryServiceUri = types.StringNull()
	}

	data.SkipOfflineWritesForOnlineCachedFeatures = types.BoolValue(e.SkipOfflineWritesForOnlineCachedFeatures)

	if e.ResultBusTopic != nil {
		data.ResultBusTopic = types.StringValue(*e.ResultBusTopic)
	} else {
		data.ResultBusTopic = types.StringNull()
	}

	if e.OnlinePersistenceMode != nil {
		data.OnlinePersistenceMode = types.StringValue(*e.OnlinePersistenceMode)
	} else {
		data.OnlinePersistenceMode = types.StringNull()
	}

	if e.MetricsBusTopic != nil {
		data.MetricsBusTopic = types.StringValue(*e.MetricsBusTopic)
	} else {
		data.MetricsBusTopic = types.StringNull()
	}

	if e.BigtableInstanceName != nil {
		data.BigtableInstanceName = types.StringValue(*e.BigtableInstanceName)
	} else {
		data.BigtableInstanceName = types.StringNull()
	}

	if e.BigtableTableName != nil {
		data.BigtableTableName = types.StringValue(*e.BigtableTableName)
	} else {
		data.BigtableTableName = types.StringNull()
	}

	if e.CloudAccountLocator != nil {
		data.CloudAccountLocator = types.StringValue(*e.CloudAccountLocator)
	} else {
		data.CloudAccountLocator = types.StringNull()
	}

	if e.CloudRegion != nil {
		data.CloudRegion = types.StringValue(*e.CloudRegion)
	} else {
		data.CloudRegion = types.StringNull()
	}

	if e.CloudTenancyId != nil {
		data.CloudTenancyId = types.StringValue(*e.CloudTenancyId)
	} else {
		data.CloudTenancyId = types.StringNull()
	}

	if e.SourceBundleBucket != nil {
		data.SourceBundleBucket = types.StringValue(*e.SourceBundleBucket)
	} else {
		data.SourceBundleBucket = types.StringNull()
	}

	if e.EngineDockerRegistryPath != nil {
		data.EngineDockerRegistryPath = types.StringValue(*e.EngineDockerRegistryPath)
	} else {
		data.EngineDockerRegistryPath = types.StringNull()
	}

	if e.DefaultPlanner != nil {
		data.DefaultPlanner = types.StringValue(*e.DefaultPlanner)
	} else {
		data.DefaultPlanner = types.StringNull()
	}

	// Map fields
	if e.AdditionalEnvVars != nil {
		elements := make(map[string]attr.Value)
		for k, v := range e.AdditionalEnvVars {
			elements[k] = types.StringValue(v)
		}
		data.AdditionalEnvVars = types.MapValueMust(types.StringType, elements)
	} else {
		data.AdditionalEnvVars = types.MapNull(types.StringType)
	}

	if e.AdditionalCronEnvVars != nil {
		elements := make(map[string]attr.Value)
		for k, v := range e.AdditionalCronEnvVars {
			elements[k] = types.StringValue(v)
		}
		data.AdditionalCronEnvVars = types.MapValueMust(types.StringType, elements)
	} else {
		data.AdditionalCronEnvVars = types.MapNull(types.StringType)
	}

	if e.PrivatePipRepositories != nil {
		data.PrivatePipRepositories = types.StringValue(*e.PrivatePipRepositories)
	} else {
		data.PrivatePipRepositories = types.StringNull()
	}

	data.IsSandbox = types.BoolValue(e.IsSandbox)

	// Cloud provider enum
	switch e.CloudProvider {
	case serverv1.CloudProviderKind_CLOUD_PROVIDER_KIND_GCP:
		data.CloudProvider = types.StringValue("GCP")
	case serverv1.CloudProviderKind_CLOUD_PROVIDER_KIND_AWS:
		data.CloudProvider = types.StringValue("AWS")
	default:
		data.CloudProvider = types.StringValue("UNSPECIFIED")
	}

	// TODO: Implement CloudConfig, SpecConfigJson, EnvironmentBuckets, and ArchivedAt mapping
	data.CloudConfig = types.ObjectNull(map[string]attr.Type{
		"aws": types.ObjectType{AttrTypes: map[string]attr.Type{
			"account_id":          types.StringType,
			"management_role_arn": types.StringType,
			"region":              types.StringType,
			"external_id":         types.StringType,
		}},
		"gcp": types.ObjectType{AttrTypes: map[string]attr.Type{
			"project_id":                 types.StringType,
			"region":                     types.StringType,
			"management_service_account": types.StringType,
		}},
	})

	data.SpecConfigJson = types.MapNull(types.StringType)

	if e.ArchivedAt != nil {
		data.ArchivedAt = types.StringValue(e.ArchivedAt.AsTime().Format(time.RFC3339))
	} else {
		data.ArchivedAt = types.StringNull()
	}

	if e.MetadataServerMetricsStoreSecret != nil {
		data.MetadataServerMetricsStoreSecret = types.StringValue(*e.MetadataServerMetricsStoreSecret)
	} else {
		data.MetadataServerMetricsStoreSecret = types.StringNull()
	}

	if e.QueryServerMetricsStoreSecret != nil {
		data.QueryServerMetricsStoreSecret = types.StringValue(*e.QueryServerMetricsStoreSecret)
	} else {
		data.QueryServerMetricsStoreSecret = types.StringNull()
	}

	if e.PinnedBaseImage != nil {
		data.PinnedBaseImage = types.StringValue(*e.PinnedBaseImage)
	} else {
		data.PinnedBaseImage = types.StringNull()
	}

	if e.ClusterGatewayId != nil {
		data.ClusterGatewayId = types.StringValue(*e.ClusterGatewayId)
	} else {
		data.ClusterGatewayId = types.StringNull()
	}

	if e.ClusterTimescaledbId != nil {
		data.ClusterTimescaledbId = types.StringValue(*e.ClusterTimescaledbId)
	} else {
		data.ClusterTimescaledbId = types.StringNull()
	}

	if e.BackgroundPersistenceDeploymentId != nil {
		data.BackgroundPersistenceDeploymentId = types.StringValue(*e.BackgroundPersistenceDeploymentId)
	} else {
		data.BackgroundPersistenceDeploymentId = types.StringNull()
	}

	data.EnvironmentBuckets = types.ObjectNull(map[string]attr.Type{
		"dataset_bucket":        types.StringType,
		"plan_stages_bucket":    types.StringType,
		"source_bundle_bucket":  types.StringType,
		"model_registry_bucket": types.StringType,
	})

	if e.ClusterTimescaledbSecret != nil {
		data.ClusterTimescaledbSecret = types.StringValue(*e.ClusterTimescaledbSecret)
	} else {
		data.ClusterTimescaledbSecret = types.StringNull()
	}

	if e.GrpcEngineUrl != nil {
		data.GrpcEngineUrl = types.StringValue(*e.GrpcEngineUrl)
	} else {
		data.GrpcEngineUrl = types.StringNull()
	}

	if e.KubeClusterMode != nil {
		data.KubeClusterMode = types.StringValue(*e.KubeClusterMode)
	} else {
		data.KubeClusterMode = types.StringNull()
	}

	if e.DashboardUrl != nil {
		data.DashboardUrl = types.StringValue(*e.DashboardUrl)
	} else {
		data.DashboardUrl = types.StringNull()
	}

	tflog.Trace(ctx, "read chalk_environment data source complete", map[string]interface{}{
		"id": data.Id.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
