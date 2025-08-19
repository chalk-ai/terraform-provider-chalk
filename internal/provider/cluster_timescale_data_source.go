package provider

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
)

var _ datasource.DataSource = &ClusterTimescaleDataSource{}

func NewClusterTimescaleDataSource() datasource.DataSource {
	return &ClusterTimescaleDataSource{}
}

type ClusterTimescaleDataSource struct {
	client *ChalkClient
}

type ClusterTimescaleDataSourceModel struct {
	Id                           types.String             `tfsdk:"id"`
	EnvironmentId                types.String            `tfsdk:"environment_id"`
	TimescaleImage               types.String             `tfsdk:"timescale_image"`
	DatabaseName                 types.String             `tfsdk:"database_name"`
	DatabaseReplicas             types.Int64              `tfsdk:"database_replicas"`
	Storage                      types.String             `tfsdk:"storage"`
	StorageClass                 types.String             `tfsdk:"storage_class"`
	Namespace                    types.String             `tfsdk:"namespace"`
	Request                      *KubeResourceConfigModel `tfsdk:"request"`
	Limit                        *KubeResourceConfigModel `tfsdk:"limit"`
	ConnectionPoolReplicas       types.Int64              `tfsdk:"connection_pool_replicas"`
	ConnectionPoolMaxConnections types.String             `tfsdk:"connection_pool_max_connections"`
	ConnectionPoolSize           types.String             `tfsdk:"connection_pool_size"`
	ConnectionPoolMode           types.String             `tfsdk:"connection_pool_mode"`
	BackupBucket                 types.String             `tfsdk:"backup_bucket"`
	BackupIamRoleArn             types.String             `tfsdk:"backup_iam_role_arn"`
	SecretName                   types.String             `tfsdk:"secret_name"`
	Internal                     types.Bool               `tfsdk:"internal"`
	ServiceType                  types.String             `tfsdk:"service_type"`
	PostgresParameters           types.Map                `tfsdk:"postgres_parameters"`
	IncludeChalkNodeSelector     types.Bool               `tfsdk:"include_chalk_node_selector"`
	BackupGcpServiceAccount      types.String             `tfsdk:"backup_gcp_service_account"`
	InstanceType                 types.String             `tfsdk:"instance_type"`
	Nodepool                     types.String             `tfsdk:"nodepool"`
	NodeSelector                 types.Map                `tfsdk:"node_selector"`
	DNSHostname                  types.String             `tfsdk:"dns_hostname"`
	CreatedAt                    types.String             `tfsdk:"created_at"`
	UpdatedAt                    types.String             `tfsdk:"updated_at"`
}

func (d *ClusterTimescaleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_timescale"
}

func (d *ClusterTimescaleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
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
		MarkdownDescription: "Data source for Chalk cluster TimescaleDB",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "TimescaleDB ID",
				Computed:            true,
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "Environment ID to fetch TimescaleDB for",
				Required:            true,
			},
			"timescale_image": schema.StringAttribute{
				MarkdownDescription: "TimescaleDB image",
				Computed:            true,
			},
			"database_name": schema.StringAttribute{
				MarkdownDescription: "Database name",
				Computed:            true,
			},
			"database_replicas": schema.Int64Attribute{
				MarkdownDescription: "Number of database replicas",
				Computed:            true,
			},
			"storage": schema.StringAttribute{
				MarkdownDescription: "Storage size",
				Computed:            true,
			},
			"storage_class": schema.StringAttribute{
				MarkdownDescription: "Storage class",
				Computed:            true,
			},
			"namespace": schema.StringAttribute{
				MarkdownDescription: "Kubernetes namespace",
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
			"connection_pool_replicas": schema.Int64Attribute{
				MarkdownDescription: "Connection pool replicas",
				Computed:            true,
			},
			"connection_pool_max_connections": schema.StringAttribute{
				MarkdownDescription: "Connection pool max connections",
				Computed:            true,
			},
			"connection_pool_size": schema.StringAttribute{
				MarkdownDescription: "Connection pool size",
				Computed:            true,
			},
			"connection_pool_mode": schema.StringAttribute{
				MarkdownDescription: "Connection pool mode",
				Computed:            true,
			},
			"backup_bucket": schema.StringAttribute{
				MarkdownDescription: "Backup bucket",
				Computed:            true,
			},
			"backup_iam_role_arn": schema.StringAttribute{
				MarkdownDescription: "Backup IAM role ARN",
				Computed:            true,
			},
			"secret_name": schema.StringAttribute{
				MarkdownDescription: "Secret name",
				Computed:            true,
			},
			"internal": schema.BoolAttribute{
				MarkdownDescription: "Internal flag",
				Computed:            true,
			},
			"service_type": schema.StringAttribute{
				MarkdownDescription: "Service type",
				Computed:            true,
			},
			"postgres_parameters": schema.MapAttribute{
				MarkdownDescription: "Postgres parameters",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"include_chalk_node_selector": schema.BoolAttribute{
				MarkdownDescription: "Include chalk node selector",
				Computed:            true,
			},
			"backup_gcp_service_account": schema.StringAttribute{
				MarkdownDescription: "Backup GCP service account",
				Computed:            true,
			},
			"instance_type": schema.StringAttribute{
				MarkdownDescription: "Instance type",
				Computed:            true,
			},
			"nodepool": schema.StringAttribute{
				MarkdownDescription: "Nodepool",
				Computed:            true,
			},
			"node_selector": schema.MapAttribute{
				MarkdownDescription: "Node selector",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"dns_hostname": schema.StringAttribute{
				MarkdownDescription: "DNS hostname",
				Computed:            true,
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

func (d *ClusterTimescaleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ClusterTimescaleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ClusterTimescaleDataSourceModel

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
	
	// Get TimescaleDB
	getReq := connect.NewRequest(&serverv1.GetClusterTimescaleDBRequest{
		EnvironmentId: data.EnvironmentId.ValueString(),
	})

	getResp, err := builderClient.GetClusterTimescaleDB(ctx, getReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read cluster TimescaleDB, got error: %s", err))
		return
	}

	// Map response to model
	data.Id = types.StringValue(getResp.Msg.Id)
	if getResp.Msg.CreatedAt != nil {
		data.CreatedAt = types.StringValue(getResp.Msg.CreatedAt.AsTime().String())
	}
	if getResp.Msg.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(getResp.Msg.UpdatedAt.AsTime().String())
	}

	// Map specs if available
	if getResp.Msg.Specs != nil {
		specs := getResp.Msg.Specs
		data.TimescaleImage = types.StringValue(specs.TimescaleImage)
		data.DatabaseName = types.StringValue(specs.DatabaseName)
		data.DatabaseReplicas = types.Int64Value(int64(specs.DatabaseReplicas))
		data.Storage = types.StringValue(specs.Storage)
		if specs.StorageClass != nil {
			data.StorageClass = types.StringValue(*specs.StorageClass)
		}
		data.Namespace = types.StringValue(specs.Namespace)
		
		// Map resource configs
		if specs.Request != nil {
			data.Request = &KubeResourceConfigModel{
				CPU:              types.StringValue(specs.Request.Cpu),
				Memory:           types.StringValue(specs.Request.Memory),
				EphemeralStorage: types.StringValue(specs.Request.EphemeralStorage),
				Storage:          types.StringValue(specs.Request.Storage),
			}
		}
		if specs.Limit != nil {
			data.Limit = &KubeResourceConfigModel{
				CPU:              types.StringValue(specs.Limit.Cpu),
				Memory:           types.StringValue(specs.Limit.Memory),
				EphemeralStorage: types.StringValue(specs.Limit.EphemeralStorage),
				Storage:          types.StringValue(specs.Limit.Storage),
			}
		}

		// Map connection pool settings
		data.ConnectionPoolReplicas = types.Int64Value(int64(specs.ConnectionPoolReplicas))
		data.ConnectionPoolMaxConnections = types.StringValue(specs.ConnectionPoolMaxConnections)
		data.ConnectionPoolSize = types.StringValue(specs.ConnectionPoolSize)
		data.ConnectionPoolMode = types.StringValue(specs.ConnectionPoolMode)

		// Map backup settings
		data.BackupBucket = types.StringValue(specs.BackupBucket)
		data.BackupIamRoleArn = types.StringValue(specs.BackupIamRoleArn)
		data.BackupGcpServiceAccount = types.StringValue(specs.BackupGcpServiceAccount)

		data.SecretName = types.StringValue(specs.SecretName)
		if specs.Internal != nil {
			data.Internal = types.BoolValue(*specs.Internal)
		}
		if specs.ServiceType != nil {
			data.ServiceType = types.StringValue(*specs.ServiceType)
		}

		// Map postgres parameters
		if len(specs.PostgresParameters) > 0 {
			params, diags := types.MapValueFrom(ctx, types.StringType, specs.PostgresParameters)
			resp.Diagnostics.Append(diags...)
			data.PostgresParameters = params
		}

		data.IncludeChalkNodeSelector = types.BoolValue(specs.IncludeChalkNodeSelector)
		data.InstanceType = types.StringValue(specs.InstanceType)
		data.Nodepool = types.StringValue(specs.Nodepool)

		// Map node selector
		if len(specs.NodeSelector) > 0 {
			selector, diags := types.MapValueFrom(ctx, types.StringType, specs.NodeSelector)
			resp.Diagnostics.Append(diags...)
			data.NodeSelector = selector
		}

		if specs.DnsHostname != nil {
			data.DNSHostname = types.StringValue(*specs.DnsHostname)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	tflog.Trace(ctx, "read chalk_cluster_timescale data source")
}