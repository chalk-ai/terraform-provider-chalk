package provider

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &ClusterTimescaleResource{}
var _ resource.ResourceWithImportState = &ClusterTimescaleResource{}

func NewClusterTimescaleResource() resource.Resource {
	return &ClusterTimescaleResource{}
}

type ClusterTimescaleResource struct {
	client *ClientManager
}

type ClusterTimescaleResourceModel struct {
	Id             types.String `tfsdk:"id"`
	EnvironmentId  types.String `tfsdk:"environment_id"`
	EnvironmentIds types.List   `tfsdk:"environment_ids"`

	// Primary Configurations
	InstanceType types.String `tfsdk:"instance_type"`
	Nodepool     types.String `tfsdk:"nodepool"`
	NodeSelector types.Map    `tfsdk:"node_selector"`
	DNSHostname  types.String `tfsdk:"dns_hostname"`
	GatewayPort  types.Int64  `tfsdk:"gateway_port"`
	GatewayId    types.String `tfsdk:"gateway_id"`

	// Optional Configurations
	BootstrapCloudResources types.Bool   `tfsdk:"bootstrap_cloud_resources"`
	BackupBucket            types.String `tfsdk:"backup_bucket"`
	BackupIamRoleArn        types.String `tfsdk:"backup_iam_role_arn"`
	BackupGcpServiceAccount types.String `tfsdk:"backup_gcp_service_account"`

	// Computed Defaults and Optional Configurations
	TimescaleImage               types.String             `tfsdk:"timescale_image"`
	DatabaseName                 types.String             `tfsdk:"database_name"`
	DatabaseReplicas             types.Int64              `tfsdk:"database_replicas"`
	Storage                      types.String             `tfsdk:"storage"`
	StorageClass                 types.String             `tfsdk:"storage_class"`
	Namespace                    types.String             `tfsdk:"namespace"`
	Request                      *KubeResourceConfigModel `tfsdk:"request"`
	ConnectionPoolReplicas       types.Int64              `tfsdk:"connection_pool_replicas"`
	ConnectionPoolMaxConnections types.String             `tfsdk:"connection_pool_max_connections"`
	ConnectionPoolSize           types.String             `tfsdk:"connection_pool_size"`
	SecretName                   types.String             `tfsdk:"secret_name"`
	Internal                     types.Bool               `tfsdk:"internal"`
	ServiceType                  types.String             `tfsdk:"service_type"`
	PostgresParameters           types.Map                `tfsdk:"postgres_parameters"`
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
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "Environment ID for the TimescaleDB cluster",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot("environment_ids")),
				},
			},
			"environment_ids": schema.ListAttribute{
				MarkdownDescription: "List of environment IDs for the TimescaleDB cluster",
				Optional:            true,
				ElementType:         types.StringType,
				Validators: []validator.List{
					listvalidator.ExactlyOneOf(path.MatchRoot("environment_id")),
				},
			},
			"timescale_image": schema.StringAttribute{
				MarkdownDescription: "TimescaleDB Docker image",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"database_name": schema.StringAttribute{
				MarkdownDescription: "Database name",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
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
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"storage_class": schema.StringAttribute{
				MarkdownDescription: "Kubernetes storage class",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"namespace": schema.StringAttribute{
				MarkdownDescription: "Kubernetes namespace",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"request": schema.SingleNestedAttribute{
				MarkdownDescription: "Resource requests",
				Optional:            true,
				Computed:            true,
				Attributes:          kubeResourceConfigSchema.Attributes,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
			},
			"connection_pool_replicas": schema.Int64Attribute{
				MarkdownDescription: "Number of connection pool replicas",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"connection_pool_max_connections": schema.StringAttribute{
				MarkdownDescription: "Maximum connections for the connection pool",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"connection_pool_size": schema.StringAttribute{
				MarkdownDescription: "Connection pool size",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"backup_bucket": schema.StringAttribute{
				MarkdownDescription: "S3/GCS bucket for backups",
				Optional:            true,
			},
			"backup_iam_role_arn": schema.StringAttribute{
				MarkdownDescription: "IAM role ARN for backups",
				Optional:            true,
			},
			"backup_gcp_service_account": schema.StringAttribute{
				MarkdownDescription: "GCP service account for backups",
				Optional:            true,
			},
			"secret_name": schema.StringAttribute{
				MarkdownDescription: "Kubernetes secret name for database credentials",
				Computed:            true,
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"internal": schema.BoolAttribute{
				MarkdownDescription: "Whether the database is internal",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"service_type": schema.StringAttribute{
				MarkdownDescription: "Kubernetes service type",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("load-balancer"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"postgres_parameters": schema.MapAttribute{
				MarkdownDescription: "PostgreSQL configuration parameters",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"bootstrap_cloud_resources": schema.BoolAttribute{
				MarkdownDescription: "Whether to bootstrap cloud resources",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"gateway_port": schema.Int64Attribute{
				MarkdownDescription: "Gateway port for the TimescaleDB",
				Optional:            true,
			},
			"gateway_id": schema.StringAttribute{
				MarkdownDescription: "Gateway ID for the TimescaleDB",
				Optional:            true,
			},
		},
	}
}

func (r *ClusterTimescaleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// upsertClusterTimescale handles the common logic for both Create and Update operations
func (r *ClusterTimescaleResource) upsertClusterTimescale(ctx context.Context, data *ClusterTimescaleResourceModel, diags *diag.Diagnostics) error {
	bc := r.client.NewBuilderClient(ctx)

	// Convert environment IDs
	var envIds []string
	if !data.EnvironmentId.IsNull() {
		// Single environment ID provided
		envIds = []string{data.EnvironmentId.ValueString()}
	} else if !data.EnvironmentIds.IsNull() && !data.EnvironmentIds.IsUnknown() {
		// List of environment IDs provided
		d := data.EnvironmentIds.ElementsAs(ctx, &envIds, false)
		diags.Append(d...)
		if diags.HasError() {
			return fmt.Errorf("failed to convert environment IDs")
		}
	}

	// Convert terraform model to proto request
	createReq := &serverv1.CreateClusterTimescaleDBRequest{
		EnvironmentIds: envIds,
		Specs: &serverv1.ClusterTimescaleSpecs{
			DatabaseReplicas:        int32(data.DatabaseReplicas.ValueInt64()),
			BootstrapCloudResources: data.BootstrapCloudResources.ValueBool(),
		},
	}

	// Handle optional timescale_image field
	if !data.TimescaleImage.IsNull() {
		createReq.Specs.TimescaleImage = data.TimescaleImage.ValueString()
	}

	// Handle optional database_name field
	if !data.DatabaseName.IsNull() {
		createReq.Specs.DatabaseName = data.DatabaseName.ValueString()
	}

	// Handle optional namespace field
	if !data.Namespace.IsNull() {
		createReq.Specs.Namespace = data.Namespace.ValueString()
	}

	// Handle optional storage field
	if !data.Storage.IsNull() {
		createReq.Specs.Storage = data.Storage.ValueString()
	}

	// Handle optional connection pool fields
	if !data.ConnectionPoolReplicas.IsNull() {
		createReq.Specs.ConnectionPoolReplicas = int32(data.ConnectionPoolReplicas.ValueInt64())
	}
	if !data.ConnectionPoolMaxConnections.IsNull() {
		createReq.Specs.ConnectionPoolMaxConnections = data.ConnectionPoolMaxConnections.ValueString()
	}
	if !data.ConnectionPoolSize.IsNull() {
		createReq.Specs.ConnectionPoolSize = data.ConnectionPoolSize.ValueString()
	}

	// Handle optional secret_name field
	if !data.SecretName.IsNull() {
		createReq.Specs.SecretName = data.SecretName.ValueString()
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

	// Handle backup config
	if !data.BackupBucket.IsNull() {
		createReq.Specs.BackupBucket = data.BackupBucket.ValueString()
	}
	if !data.BackupIamRoleArn.IsNull() {
		createReq.Specs.BackupIamRoleArn = data.BackupIamRoleArn.ValueString()
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
	if !data.GatewayPort.IsNull() {
		val := int32(data.GatewayPort.ValueInt64())
		createReq.Specs.GatewayPort = &val
	}
	if !data.GatewayId.IsNull() {
		val := data.GatewayId.ValueString()
		createReq.Specs.GatewayId = &val
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

	// Convert postgres parameters
	if !data.PostgresParameters.IsNull() && !data.PostgresParameters.IsUnknown() {
		params := make(map[string]string)
		d := data.PostgresParameters.ElementsAs(ctx, &params, false)
		diags.Append(d...)
		if diags.HasError() {
			return fmt.Errorf("failed to convert postgres parameters")
		}
		createReq.Specs.PostgresParameters = params
	}

	// Convert node selector
	if !data.NodeSelector.IsNull() && !data.NodeSelector.IsUnknown() {
		selector := make(map[string]string)
		d := data.NodeSelector.ElementsAs(ctx, &selector, false)
		diags.Append(d...)
		if diags.HasError() {
			return fmt.Errorf("failed to convert node selector")
		}
		createReq.Specs.NodeSelector = selector
	}

	metricsDb, err := bc.CreateClusterTimescaleDB(ctx, connect.NewRequest(createReq))
	if err != nil {
		return err
	}

	// Update with returned values
	data.Id = types.StringValue(metricsDb.Msg.ClusterTimescaleId)
	data.SecretName = types.StringValue(metricsDb.Msg.Specs.SecretName)
	data.TimescaleImage = types.StringValue(metricsDb.Msg.Specs.TimescaleImage)
	data.DatabaseName = types.StringValue(metricsDb.Msg.Specs.DatabaseName)
	data.Namespace = types.StringValue(metricsDb.Msg.Specs.Namespace)
	data.Storage = types.StringValue(metricsDb.Msg.Specs.Storage)

	// Set computed connection pool fields
	data.ConnectionPoolReplicas = types.Int64Value(int64(metricsDb.Msg.Specs.ConnectionPoolReplicas))
	data.ConnectionPoolMaxConnections = types.StringValue(metricsDb.Msg.Specs.ConnectionPoolMaxConnections)
	data.ConnectionPoolSize = types.StringValue(metricsDb.Msg.Specs.ConnectionPoolSize)

	// Set backup config
	if metricsDb.Msg.Specs.BackupBucket != "" {
		data.BackupBucket = types.StringValue(metricsDb.Msg.Specs.BackupBucket)
	} else {
		data.BackupBucket = types.StringNull()
	}
	if metricsDb.Msg.Specs.BackupIamRoleArn != "" {
		data.BackupIamRoleArn = types.StringValue(metricsDb.Msg.Specs.BackupIamRoleArn)
	} else {
		data.BackupIamRoleArn = types.StringNull()
	}
	if metricsDb.Msg.Specs.BackupGcpServiceAccount != "" {
		data.BackupGcpServiceAccount = types.StringValue(metricsDb.Msg.Specs.BackupGcpServiceAccount)
	} else {
		data.BackupGcpServiceAccount = types.StringNull()
	}

	// Set computed resource configs
	if metricsDb.Msg.Specs.Request != nil {
		request := &KubeResourceConfigModel{}
		if metricsDb.Msg.Specs.Request.Cpu != "" {
			request.CPU = types.StringValue(metricsDb.Msg.Specs.Request.Cpu)
		} else {
			request.CPU = types.StringNull()
		}
		if metricsDb.Msg.Specs.Request.Memory != "" {
			request.Memory = types.StringValue(metricsDb.Msg.Specs.Request.Memory)
		} else {
			request.Memory = types.StringNull()
		}
		if metricsDb.Msg.Specs.Request.EphemeralStorage != "" {
			request.EphemeralStorage = types.StringValue(metricsDb.Msg.Specs.Request.EphemeralStorage)
		} else {
			request.EphemeralStorage = types.StringNull()
		}
		if metricsDb.Msg.Specs.Request.Storage != "" {
			request.Storage = types.StringValue(metricsDb.Msg.Specs.Request.Storage)
		} else {
			request.Storage = types.StringNull()
		}
		data.Request = request
	} else {
		data.Request = nil
	}

	// Set computed postgres parameters
	if len(metricsDb.Msg.Specs.PostgresParameters) > 0 {
		params := make(map[string]attr.Value)
		for k, v := range metricsDb.Msg.Specs.PostgresParameters {
			params[k] = types.StringValue(v)
		}
		data.PostgresParameters = types.MapValueMust(types.StringType, params)
	} else {
		data.PostgresParameters = types.MapNull(types.StringType)
	}

	return nil
}

func (r *ClusterTimescaleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ClusterTimescaleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.upsertClusterTimescale(ctx, &data, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Cluster TimescaleDB",
			fmt.Sprintf("Could not create cluster TimescaleDB: %v", err),
		)
		return
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
	// Create builder client
	bc := r.client.NewBuilderClient(ctx)

	// Get the environment ID to query the TimescaleDB
	// Track which form was used so we can preserve it
	var envId string
	var usedSingular bool
	var preservedEnvIds types.List
	if !data.EnvironmentId.IsNull() {
		// Single environment ID provided
		envId = data.EnvironmentId.ValueString()
		usedSingular = true
	} else if !data.EnvironmentIds.IsNull() && !data.EnvironmentIds.IsUnknown() {
		// List of environment IDs provided
		// Preserve the exact value from state
		preservedEnvIds = data.EnvironmentIds
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
		envId = envIds[0]
		usedSingular = false
	}

	getReq := &serverv1.GetClusterTimescaleDBRequest{
		EnvironmentId: envId,
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
	// Update specs if available
	if timescale.Msg.Specs != nil {
		specs := timescale.Msg.Specs
		data.Id = types.StringValue(timescale.Msg.Id)

		// Preserve environment_id or environment_ids - keep whichever one was already set in state
		// This prevents drift during reads and imports
		if usedSingular {
			// Keep environment_id, clear environment_ids
			// environment_id is already set in data
			data.EnvironmentIds = types.ListNull(types.StringType)
		} else {
			// Keep environment_ids, clear environment_id
			// Explicitly set the preserved value to avoid semantic equality issues
			data.EnvironmentIds = preservedEnvIds
			data.EnvironmentId = types.StringNull()
		}

		data.TimescaleImage = types.StringValue(specs.TimescaleImage)
		data.DatabaseName = types.StringValue(specs.DatabaseName)
		data.DatabaseReplicas = types.Int64Value(int64(specs.DatabaseReplicas))
		data.Storage = types.StringValue(specs.Storage)
		data.Namespace = types.StringValue(specs.Namespace)
		data.ConnectionPoolReplicas = types.Int64Value(int64(specs.ConnectionPoolReplicas))
		data.ConnectionPoolMaxConnections = types.StringValue(specs.ConnectionPoolMaxConnections)
		data.ConnectionPoolSize = types.StringValue(specs.ConnectionPoolSize)

		// Set backup config
		if specs.BackupBucket != "" {
			data.BackupBucket = types.StringValue(specs.BackupBucket)
		} else {
			data.BackupBucket = types.StringNull()
		}
		if specs.BackupIamRoleArn != "" {
			data.BackupIamRoleArn = types.StringValue(specs.BackupIamRoleArn)
		} else {
			data.BackupIamRoleArn = types.StringNull()
		}
		if specs.BackupGcpServiceAccount != "" {
			data.BackupGcpServiceAccount = types.StringValue(specs.BackupGcpServiceAccount)
		} else {
			data.BackupGcpServiceAccount = types.StringNull()
		}

		data.BootstrapCloudResources = types.BoolValue(specs.BootstrapCloudResources)
		data.SecretName = types.StringValue(specs.SecretName)
		if specs.InstanceType != "" {
			data.InstanceType = types.StringValue(specs.InstanceType)
		} else {
			data.InstanceType = types.StringNull()
		}
		if specs.Nodepool != "" {
			data.Nodepool = types.StringValue(specs.Nodepool)
		} else {
			data.Nodepool = types.StringNull()
		}

		// Update optional fields
		if specs.StorageClass != nil {
			data.StorageClass = types.StringValue(*specs.StorageClass)
		} else {
			data.StorageClass = types.StringNull()
		}
		if specs.Internal != nil {
			data.Internal = types.BoolValue(*specs.Internal)
		} else {
			data.Internal = types.BoolNull()
		}
		if specs.ServiceType != nil {
			data.ServiceType = types.StringValue(*specs.ServiceType)
		} else {
			data.ServiceType = types.StringNull()
		}
		if specs.DnsHostname != nil {
			data.DNSHostname = types.StringValue(*specs.DnsHostname)
		} else {
			data.DNSHostname = types.StringNull()
		}
		if specs.GatewayPort != nil {
			data.GatewayPort = types.Int64Value(int64(*specs.GatewayPort))
		} else {
			data.GatewayPort = types.Int64Null()
		}
		if specs.GatewayId != nil {
			data.GatewayId = types.StringValue(*specs.GatewayId)
		} else {
			data.GatewayId = types.StringNull()
		}

		// Update postgres parameters
		if len(specs.PostgresParameters) > 0 {
			params := make(map[string]attr.Value)
			for k, v := range specs.PostgresParameters {
				params[k] = types.StringValue(v)
			}
			data.PostgresParameters = types.MapValueMust(types.StringType, params)
		} else {
			data.PostgresParameters = types.MapNull(types.StringType)
		}

		// Update node selector
		if len(specs.NodeSelector) > 0 {
			selector := make(map[string]attr.Value)
			for k, v := range specs.NodeSelector {
				selector[k] = types.StringValue(v)
			}
			data.NodeSelector = types.MapValueMust(types.StringType, selector)
		} else {
			data.NodeSelector = types.MapNull(types.StringType)
		}

		// Update resource configs
		if specs.Request != nil {
			request := &KubeResourceConfigModel{}
			if specs.Request.Cpu != "" {
				request.CPU = types.StringValue(specs.Request.Cpu)
			} else {
				request.CPU = types.StringNull()
			}
			if specs.Request.Memory != "" {
				request.Memory = types.StringValue(specs.Request.Memory)
			} else {
				request.Memory = types.StringNull()
			}
			if specs.Request.EphemeralStorage != "" {
				request.EphemeralStorage = types.StringValue(specs.Request.EphemeralStorage)
			} else {
				request.EphemeralStorage = types.StringNull()
			}
			if specs.Request.Storage != "" {
				request.Storage = types.StringValue(specs.Request.Storage)
			} else {
				request.Storage = types.StringNull()
			}
			data.Request = request
		} else {
			data.Request = nil
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterTimescaleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ClusterTimescaleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.upsertClusterTimescale(ctx, &data, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Chalk Cluster TimescaleDB",
			fmt.Sprintf("Could not update cluster TimescaleDB: %v", err),
		)
		return
	}

	tflog.Trace(ctx, "updated a chalk_cluster_timescale resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterTimescaleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Note: According to the proto definition, there's no DeleteClusterTimescaleDB method
	// This means the TimescaleDB lifecycle might be managed differently
	// For now, we'll just remove it from Terraform state
	data := ClusterTimescaleResourceModel{}
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	// Create builder client
	bc := r.client.NewBuilderClient(ctx)

	_, err := bc.DeleteClusterTimescaleDB(ctx, connect.NewRequest(&serverv1.DeleteClusterTimescaleDBRequest{
		ClusterTimescaleId: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Chalk Cluster TimescaleDB",
			fmt.Sprintf("Could not delete cluster TimescaleDB %s: %v", data.Id.ValueString(), err),
		)
	}
}

func (r *ClusterTimescaleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID is the environment_id
	// Set it as environment_ids (plural) as a list with single element
	// This matches the recommended config format and prevents drift
	envId := req.ID

	envIds, diags := types.ListValueFrom(ctx, types.StringType, []string{envId})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set environment_ids - Read will populate the rest including the actual resource ID
	resp.State.SetAttribute(ctx, path.Root("environment_ids"), envIds)
}
