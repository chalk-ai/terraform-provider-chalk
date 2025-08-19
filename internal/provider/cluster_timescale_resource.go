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
	"net/http"
)

var _ resource.Resource = &ClusterTimescaleResource{}
var _ resource.ResourceWithImportState = &ClusterTimescaleResource{}

func NewClusterTimescaleResource() resource.Resource {
	return &ClusterTimescaleResource{}
}

type ClusterTimescaleResource struct {
	client *ChalkClient
}

type ClusterTimescaleResourceModel struct {
	Id                            types.String `tfsdk:"id"`
	EnvironmentIds                types.List   `tfsdk:"environment_ids"`
	TimescaleImage                types.String `tfsdk:"timescale_image"`
	DatabaseName                  types.String `tfsdk:"database_name"`
	DatabaseReplicas              types.Int64  `tfsdk:"database_replicas"`
	Storage                       types.String `tfsdk:"storage"`
	StorageClass                  types.String `tfsdk:"storage_class"`
	Namespace                     types.String `tfsdk:"namespace"`
	Request                       *KubeResourceConfigModel `tfsdk:"request"`
	Limit                         *KubeResourceConfigModel `tfsdk:"limit"`
	ConnectionPoolReplicas        types.Int64  `tfsdk:"connection_pool_replicas"`
	ConnectionPoolMaxConnections  types.String `tfsdk:"connection_pool_max_connections"`
	ConnectionPoolSize            types.String `tfsdk:"connection_pool_size"`
	ConnectionPoolMode            types.String `tfsdk:"connection_pool_mode"`
	BackupBucket                  types.String `tfsdk:"backup_bucket"`
	BackupIamRoleArn              types.String `tfsdk:"backup_iam_role_arn"`
	SecretName                    types.String `tfsdk:"secret_name"`
	Internal                      types.Bool   `tfsdk:"internal"`
	ServiceType                   types.String `tfsdk:"service_type"`
	PostgresParameters            types.Map    `tfsdk:"postgres_parameters"`
	IncludeChalkNodeSelector      types.Bool   `tfsdk:"include_chalk_node_selector"`
	BackupGcpServiceAccount       types.String `tfsdk:"backup_gcp_service_account"`
	InstanceType                  types.String `tfsdk:"instance_type"`
	Nodepool                      types.String `tfsdk:"nodepool"`
	NodeSelector                  types.Map    `tfsdk:"node_selector"`
	DNSHostname                   types.String `tfsdk:"dns_hostname"`
	CreatedAt                     types.String `tfsdk:"created_at"`
	UpdatedAt                     types.String `tfsdk:"updated_at"`
}

func (r *ClusterTimescaleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_timescale"
}

func (r *ClusterTimescaleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
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
		MarkdownDescription: "Chalk cluster TimescaleDB resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "TimescaleDB identifier",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_ids": schema.ListAttribute{
				MarkdownDescription: "List of environment IDs for the TimescaleDB cluster",
				Required:            true,
				ElementType:         types.StringType,
			},
			"timescale_image": schema.StringAttribute{
				MarkdownDescription: "TimescaleDB Docker image",
				Required:            true,
			},
			"database_name": schema.StringAttribute{
				MarkdownDescription: "Database name",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"database_replicas": schema.Int64Attribute{
				MarkdownDescription: "Number of database replicas",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
			},
			"storage": schema.StringAttribute{
				MarkdownDescription: "Storage size (e.g., '100Gi')",
				Required:            true,
			},
			"storage_class": schema.StringAttribute{
				MarkdownDescription: "Kubernetes storage class",
				Optional:            true,
			},
			"namespace": schema.StringAttribute{
				MarkdownDescription: "Kubernetes namespace",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
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
			"connection_pool_replicas": schema.Int64Attribute{
				MarkdownDescription: "Number of connection pool replicas",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
			},
			"connection_pool_max_connections": schema.StringAttribute{
				MarkdownDescription: "Maximum connections for the connection pool",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("100"),
			},
			"connection_pool_size": schema.StringAttribute{
				MarkdownDescription: "Connection pool size",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("25"),
			},
			"connection_pool_mode": schema.StringAttribute{
				MarkdownDescription: "Connection pool mode (e.g., 'transaction', 'session')",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("transaction"),
			},
			"backup_bucket": schema.StringAttribute{
				MarkdownDescription: "S3/GCS bucket for backups",
				Required:            true,
			},
			"backup_iam_role_arn": schema.StringAttribute{
				MarkdownDescription: "IAM role ARN for backups",
				Required:            true,
			},
			"secret_name": schema.StringAttribute{
				MarkdownDescription: "Kubernetes secret name for database credentials",
				Required:            true,
			},
			"internal": schema.BoolAttribute{
				MarkdownDescription: "Whether the database is internal",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"service_type": schema.StringAttribute{
				MarkdownDescription: "Kubernetes service type",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("ClusterIP"),
			},
			"postgres_parameters": schema.MapAttribute{
				MarkdownDescription: "PostgreSQL configuration parameters",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"include_chalk_node_selector": schema.BoolAttribute{
				MarkdownDescription: "Whether to include chalk node selector",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"backup_gcp_service_account": schema.StringAttribute{
				MarkdownDescription: "GCP service account for backups",
				Optional:            true,
			},
			"instance_type": schema.StringAttribute{
				MarkdownDescription: "Instance type",
				Optional:            true,
			},
			"nodepool": schema.StringAttribute{
				MarkdownDescription: "Nodepool name",
				Optional:            true,
			},
			"node_selector": schema.MapAttribute{
				MarkdownDescription: "Node selector labels",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"dns_hostname": schema.StringAttribute{
				MarkdownDescription: "DNS hostname",
				Optional:            true,
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

func (r *ClusterTimescaleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ClusterTimescaleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ClusterTimescaleResourceModel

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

	// Convert terraform model to proto request
	createReq := &serverv1.CreateClusterTimescaleDBRequest{
		EnvironmentIds: envIds,
		Specs: &serverv1.ClusterTimescaleSpecs{
			TimescaleImage:               data.TimescaleImage.ValueString(),
			DatabaseName:                 data.DatabaseName.ValueString(),
			DatabaseReplicas:             int32(data.DatabaseReplicas.ValueInt64()),
			Storage:                      data.Storage.ValueString(),
			Namespace:                    data.Namespace.ValueString(),
			ConnectionPoolReplicas:       int32(data.ConnectionPoolReplicas.ValueInt64()),
			ConnectionPoolMaxConnections: data.ConnectionPoolMaxConnections.ValueString(),
			ConnectionPoolSize:           data.ConnectionPoolSize.ValueString(),
			ConnectionPoolMode:           data.ConnectionPoolMode.ValueString(),
			BackupBucket:                 data.BackupBucket.ValueString(),
			BackupIamRoleArn:             data.BackupIamRoleArn.ValueString(),
			SecretName:                   data.SecretName.ValueString(),
			IncludeChalkNodeSelector:     data.IncludeChalkNodeSelector.ValueBool(),
		},
	}

	// Set optional fields
	if !data.StorageClass.IsNull() {
		val := data.StorageClass.ValueString()
		createReq.Specs.StorageClass = &val
	}
	if !data.Internal.IsNull() {
		val := data.Internal.ValueBool()
		createReq.Specs.Internal = &val
	}
	if !data.ServiceType.IsNull() {
		val := data.ServiceType.ValueString()
		createReq.Specs.ServiceType = &val
	}
	if !data.BackupGcpServiceAccount.IsNull() {
		createReq.Specs.BackupGcpServiceAccount = data.BackupGcpServiceAccount.ValueString()
	}
	if !data.InstanceType.IsNull() {
		createReq.Specs.InstanceType = data.InstanceType.ValueString()
	}
	if !data.Nodepool.IsNull() {
		createReq.Specs.Nodepool = data.Nodepool.ValueString()
	}
	if !data.DNSHostname.IsNull() {
		val := data.DNSHostname.ValueString()
		createReq.Specs.DnsHostname = &val
	}

	// Convert resource configs
	if data.Request != nil {
		createReq.Specs.Request = &serverv1.KubeResourceConfig{
			Cpu:              data.Request.CPU.ValueString(),
			Memory:           data.Request.Memory.ValueString(),
			EphemeralStorage: data.Request.EphemeralStorage.ValueString(),
			Storage:          data.Request.Storage.ValueString(),
		}
	}
	if data.Limit != nil {
		createReq.Specs.Limit = &serverv1.KubeResourceConfig{
			Cpu:              data.Limit.CPU.ValueString(),
			Memory:           data.Limit.Memory.ValueString(),
			EphemeralStorage: data.Limit.EphemeralStorage.ValueString(),
			Storage:          data.Limit.Storage.ValueString(),
		}
	}

	// Convert postgres parameters
	if !data.PostgresParameters.IsNull() {
		params := make(map[string]string)
		diags = data.PostgresParameters.ElementsAs(ctx, &params, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Specs.PostgresParameters = params
	}

	// Convert node selector
	if !data.NodeSelector.IsNull() {
		selector := make(map[string]string)
		diags = data.NodeSelector.ElementsAs(ctx, &selector, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Specs.NodeSelector = selector
	}

	_, err := bc.CreateClusterTimescaleDB(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Cluster TimescaleDB",
			fmt.Sprintf("Could not create cluster TimescaleDB: %v", err),
		)
		return
	}

	// Since CreateClusterTimescaleDB doesn't return the created resource, we need to get it
	// We'll use the first environment ID to query for the TimescaleDB
	if len(envIds) > 0 {
		getReq := &serverv1.GetClusterTimescaleDBRequest{
			EnvironmentId: envIds[0],
		}

		timescale, err := bc.GetClusterTimescaleDB(ctx, connect.NewRequest(getReq))
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Created Chalk Cluster TimescaleDB",
				fmt.Sprintf("TimescaleDB was created but could not be read: %v", err),
			)
			return
		}

		// Update with created values
		data.Id = types.StringValue(timescale.Msg.Id)
		if timescale.Msg.CreatedAt != nil {
			data.CreatedAt = types.StringValue(timescale.Msg.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"))
		}
		if timescale.Msg.UpdatedAt != nil {
			data.UpdatedAt = types.StringValue(timescale.Msg.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z"))
		}
	}

	tflog.Trace(ctx, "created a chalk_cluster_timescale resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterTimescaleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ClusterTimescaleResourceModel

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

	// Get the first environment ID to query the TimescaleDB
	var envIds []string
	diags := data.EnvironmentIds.ElementsAs(ctx, &envIds, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(envIds) == 0 {
		resp.Diagnostics.AddError(
			"No Environment IDs",
			"No environment IDs found in state for reading cluster TimescaleDB",
		)
		return
	}

	getReq := &serverv1.GetClusterTimescaleDBRequest{
		EnvironmentId: envIds[0],
	}

	timescale, err := bc.GetClusterTimescaleDB(ctx, connect.NewRequest(getReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Chalk Cluster TimescaleDB",
			fmt.Sprintf("Could not read cluster TimescaleDB %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Update the model with the fetched data
	if timescale.Msg.CreatedAt != nil {
		data.CreatedAt = types.StringValue(timescale.Msg.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"))
	}
	if timescale.Msg.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(timescale.Msg.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z"))
	}

	// Update specs if available
	if timescale.Msg.Specs != nil {
		specs := timescale.Msg.Specs
		data.TimescaleImage = types.StringValue(specs.TimescaleImage)
		data.DatabaseName = types.StringValue(specs.DatabaseName)
		data.DatabaseReplicas = types.Int64Value(int64(specs.DatabaseReplicas))
		data.Storage = types.StringValue(specs.Storage)
		data.Namespace = types.StringValue(specs.Namespace)
		data.ConnectionPoolReplicas = types.Int64Value(int64(specs.ConnectionPoolReplicas))
		data.ConnectionPoolMaxConnections = types.StringValue(specs.ConnectionPoolMaxConnections)
		data.ConnectionPoolSize = types.StringValue(specs.ConnectionPoolSize)
		data.ConnectionPoolMode = types.StringValue(specs.ConnectionPoolMode)
		data.BackupBucket = types.StringValue(specs.BackupBucket)
		data.BackupIamRoleArn = types.StringValue(specs.BackupIamRoleArn)
		data.SecretName = types.StringValue(specs.SecretName)
		data.IncludeChalkNodeSelector = types.BoolValue(specs.IncludeChalkNodeSelector)
		data.BackupGcpServiceAccount = types.StringValue(specs.BackupGcpServiceAccount)
		data.InstanceType = types.StringValue(specs.InstanceType)
		data.Nodepool = types.StringValue(specs.Nodepool)

		// Update optional fields
		if specs.StorageClass != nil {
			data.StorageClass = types.StringValue(*specs.StorageClass)
		}
		if specs.Internal != nil {
			data.Internal = types.BoolValue(*specs.Internal)
		}
		if specs.ServiceType != nil {
			data.ServiceType = types.StringValue(*specs.ServiceType)
		}
		if specs.DnsHostname != nil {
			data.DNSHostname = types.StringValue(*specs.DnsHostname)
		}

		// Update postgres parameters
		if len(specs.PostgresParameters) > 0 {
			params := make(map[string]attr.Value)
			for k, v := range specs.PostgresParameters {
				params[k] = types.StringValue(v)
			}
			data.PostgresParameters = types.MapValueMust(types.StringType, params)
		}

		// Update node selector
		if len(specs.NodeSelector) > 0 {
			selector := make(map[string]attr.Value)
			for k, v := range specs.NodeSelector {
				selector[k] = types.StringValue(v)
			}
			data.NodeSelector = types.MapValueMust(types.StringType, selector)
		}

		// Update resource configs - simplified for now
		// A full implementation would properly convert from proto back to terraform models
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterTimescaleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Note: According to the proto definition, there's no UpdateClusterTimescaleDB method
	// There is a MigrateClusterTimescaleDB method, but that's for migrations, not general updates
	// For now, we'll return an error indicating updates are not supported
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Cluster TimescaleDB updates are not supported by the Chalk API. Please recreate the resource if changes are needed.",
	)
}

func (r *ClusterTimescaleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Note: According to the proto definition, there's no DeleteClusterTimescaleDB method
	// This means the TimescaleDB lifecycle might be managed differently
	// For now, we'll just remove it from Terraform state
	tflog.Trace(ctx, "cluster TimescaleDB deletion - removing from terraform state only (no API delete available)")
}

func (r *ClusterTimescaleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}