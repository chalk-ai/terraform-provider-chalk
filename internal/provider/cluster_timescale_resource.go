package provider

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
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
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

var _ resource.Resource = &ClusterTimescaleResource{}
var _ resource.ResourceWithImportState = &ClusterTimescaleResource{}

// kubeResourceConfigAttrTypes defines the attribute types for KubeResourceConfig objects
var kubeResourceConfigAttrTypes = map[string]attr.Type{
	"cpu":               types.StringType,
	"memory":            types.StringType,
	"ephemeral_storage": types.StringType,
	"storage":           types.StringType,
}

func NewClusterTimescaleResource() resource.Resource {
	return &ClusterTimescaleResource{}
}

type ClusterTimescaleResource struct {
	client *ClientManager
}

type ClusterTimescaleResourceModel struct {
	Id            types.String `tfsdk:"id"`
	EnvironmentId types.String `tfsdk:"environment_id"`

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
	TimescaleImage               types.String `tfsdk:"timescale_image"`
	DatabaseName                 types.String `tfsdk:"database_name"`
	DatabaseReplicas             types.Int64  `tfsdk:"database_replicas"`
	Storage                      types.String `tfsdk:"storage"`
	StorageClass                 types.String `tfsdk:"storage_class"`
	Namespace                    types.String `tfsdk:"namespace"`
	Request                      types.Object `tfsdk:"request"`
	ConnectionPoolReplicas       types.Int64  `tfsdk:"connection_pool_replicas"`
	ConnectionPoolMaxConnections types.String `tfsdk:"connection_pool_max_connections"`
	ConnectionPoolSize           types.String `tfsdk:"connection_pool_size"`
	SecretName                   types.String `tfsdk:"secret_name"`
	Internal                     types.Bool   `tfsdk:"internal"`
	ServiceType                  types.String `tfsdk:"service_type"`
	PostgresParameters           types.Map    `tfsdk:"postgres_parameters"`
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
				Required:            true,
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"backup_iam_role_arn": schema.StringAttribute{
				MarkdownDescription: "IAM role ARN for backups",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"backup_gcp_service_account": schema.StringAttribute{
				MarkdownDescription: "GCP service account for backups",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
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

// buildClusterTimescaleSpecs builds ClusterTimescaleSpecs proto from Terraform model
func buildClusterTimescaleSpecs(ctx context.Context, data *ClusterTimescaleResourceModel, diags *diag.Diagnostics) (*serverv1.ClusterTimescaleSpecs, error) {
	specs := &serverv1.ClusterTimescaleSpecs{
		DatabaseReplicas:        int32(data.DatabaseReplicas.ValueInt64()),
		BootstrapCloudResources: data.BootstrapCloudResources.ValueBool(),
	}

	// Simple strings
	if !data.TimescaleImage.IsNull() {
		specs.TimescaleImage = data.TimescaleImage.ValueString()
	}
	if !data.DatabaseName.IsNull() {
		specs.DatabaseName = data.DatabaseName.ValueString()
	}
	if !data.Storage.IsNull() {
		specs.Storage = data.Storage.ValueString()
	}
	if !data.Namespace.IsNull() {
		specs.Namespace = data.Namespace.ValueString()
	}
	if !data.ConnectionPoolMaxConnections.IsNull() {
		specs.ConnectionPoolMaxConnections = data.ConnectionPoolMaxConnections.ValueString()
	}
	if !data.ConnectionPoolSize.IsNull() {
		specs.ConnectionPoolSize = data.ConnectionPoolSize.ValueString()
	}
	if !data.BackupBucket.IsNull() {
		specs.BackupBucket = data.BackupBucket.ValueString()
	}
	if !data.BackupIamRoleArn.IsNull() {
		specs.BackupIamRoleArn = data.BackupIamRoleArn.ValueString()
	}
	if !data.BackupGcpServiceAccount.IsNull() {
		specs.BackupGcpServiceAccount = data.BackupGcpServiceAccount.ValueString()
	}
	if !data.InstanceType.IsNull() {
		specs.InstanceType = data.InstanceType.ValueString()
	}
	if !data.Nodepool.IsNull() {
		specs.Nodepool = data.Nodepool.ValueString()
	}
	if !data.SecretName.IsNull() {
		specs.SecretName = data.SecretName.ValueString()
	}

	// Integers
	if !data.ConnectionPoolReplicas.IsNull() {
		specs.ConnectionPoolReplicas = int32(data.ConnectionPoolReplicas.ValueInt64())
	}

	// Optional pointers
	if !data.StorageClass.IsNull() {
		val := data.StorageClass.ValueString()
		specs.StorageClass = &val
	}
	if !data.Internal.IsNull() {
		val := data.Internal.ValueBool()
		specs.Internal = &val
	}
	if !data.ServiceType.IsNull() {
		val := data.ServiceType.ValueString()
		specs.ServiceType = &val
	}
	if !data.DNSHostname.IsNull() {
		val := data.DNSHostname.ValueString()
		specs.DnsHostname = &val
	}
	if !data.GatewayPort.IsNull() {
		val := int32(data.GatewayPort.ValueInt64())
		specs.GatewayPort = &val
	}
	if !data.GatewayId.IsNull() {
		val := data.GatewayId.ValueString()
		specs.GatewayId = &val
	}

	// Complex object - Request
	if !data.Request.IsNull() && !data.Request.IsUnknown() {
		var request KubeResourceConfigModel
		d := data.Request.As(ctx, &request, basetypes.ObjectAsOptions{})
		diags.Append(d...)
		if diags.HasError() {
			return nil, fmt.Errorf("failed to convert request config")
		}
		specs.Request = &serverv1.KubeResourceConfig{
			Cpu:              request.CPU.ValueString(),
			Memory:           request.Memory.ValueString(),
			EphemeralStorage: request.EphemeralStorage.ValueString(),
			Storage:          request.Storage.ValueString(),
		}
	}

	// Maps
	if !data.PostgresParameters.IsNull() && !data.PostgresParameters.IsUnknown() {
		params := make(map[string]string)
		d := data.PostgresParameters.ElementsAs(ctx, &params, false)
		diags.Append(d...)
		if diags.HasError() {
			return nil, fmt.Errorf("failed to convert postgres parameters")
		}
		// Only set if the map is not empty
		if len(params) > 0 {
			specs.PostgresParameters = params
		}
	}

	if !data.NodeSelector.IsNull() && !data.NodeSelector.IsUnknown() {
		selector := make(map[string]string)
		d := data.NodeSelector.ElementsAs(ctx, &selector, false)
		diags.Append(d...)
		if diags.HasError() {
			return nil, fmt.Errorf("failed to convert node selector")
		}
		// Only set if the map is not empty
		if len(selector) > 0 {
			specs.NodeSelector = selector
		}
	}

	return specs, nil
}

// updateStateFromSpecs updates Terraform model from ClusterTimescaleSpecs proto response
func updateStateFromSpecs(data *ClusterTimescaleResourceModel, specs *serverv1.ClusterTimescaleSpecs) error {
	// Simple strings
	data.TimescaleImage = types.StringValue(specs.TimescaleImage)
	data.DatabaseName = types.StringValue(specs.DatabaseName)
	data.Storage = types.StringValue(specs.Storage)
	data.Namespace = types.StringValue(specs.Namespace)
	data.ConnectionPoolMaxConnections = types.StringValue(specs.ConnectionPoolMaxConnections)
	data.ConnectionPoolSize = types.StringValue(specs.ConnectionPoolSize)
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

	// Integers
	data.DatabaseReplicas = types.Int64Value(int64(specs.DatabaseReplicas))
	data.ConnectionPoolReplicas = types.Int64Value(int64(specs.ConnectionPoolReplicas))

	// Boolean
	data.BootstrapCloudResources = types.BoolValue(specs.BootstrapCloudResources)

	// Backup fields
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

	// Optional pointers
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

	// Request object
	if specs.Request != nil {
		requestAttrs := map[string]attr.Value{
			"cpu":               types.StringNull(),
			"memory":            types.StringNull(),
			"ephemeral_storage": types.StringNull(),
			"storage":           types.StringNull(),
		}
		if specs.Request.Cpu != "" {
			requestAttrs["cpu"] = types.StringValue(specs.Request.Cpu)
		}
		if specs.Request.Memory != "" {
			requestAttrs["memory"] = types.StringValue(specs.Request.Memory)
		}
		if specs.Request.EphemeralStorage != "" {
			requestAttrs["ephemeral_storage"] = types.StringValue(specs.Request.EphemeralStorage)
		}
		if specs.Request.Storage != "" {
			requestAttrs["storage"] = types.StringValue(specs.Request.Storage)
		}
		data.Request, _ = types.ObjectValue(kubeResourceConfigAttrTypes, requestAttrs)
	} else {
		data.Request = types.ObjectNull(kubeResourceConfigAttrTypes)
	}

	// Maps
	if len(specs.PostgresParameters) > 0 {
		params := make(map[string]attr.Value)
		for k, v := range specs.PostgresParameters {
			params[k] = types.StringValue(v)
		}
		data.PostgresParameters = types.MapValueMust(types.StringType, params)
	} else {
		data.PostgresParameters = types.MapNull(types.StringType)
	}

	if len(specs.NodeSelector) > 0 {
		selector := make(map[string]attr.Value)
		for k, v := range specs.NodeSelector {
			selector[k] = types.StringValue(v)
		}
		data.NodeSelector = types.MapValueMust(types.StringType, selector)
	} else {
		data.NodeSelector = types.MapNull(types.StringType)
	}

	return nil
}

// buildTimescaleUpdateMask compares plan and state to determine which fields changed
func buildTimescaleUpdateMask(data, state *ClusterTimescaleResourceModel) []string {
	var updateMaskPaths []string

	// Core Database fields
	if !data.TimescaleImage.Equal(state.TimescaleImage) {
		updateMaskPaths = append(updateMaskPaths, "timescale_image")
	}
	if !data.Storage.Equal(state.Storage) {
		updateMaskPaths = append(updateMaskPaths, "storage")
	}
	if !data.DatabaseReplicas.Equal(state.DatabaseReplicas) {
		updateMaskPaths = append(updateMaskPaths, "database_replicas")
	}

	// Connection Pool fields
	if !data.ConnectionPoolReplicas.Equal(state.ConnectionPoolReplicas) {
		updateMaskPaths = append(updateMaskPaths, "connection_pool_replicas")
	}
	if !data.ConnectionPoolMaxConnections.Equal(state.ConnectionPoolMaxConnections) {
		updateMaskPaths = append(updateMaskPaths, "connection_pool_max_connections")
	}
	if !data.ConnectionPoolSize.Equal(state.ConnectionPoolSize) {
		updateMaskPaths = append(updateMaskPaths, "connection_pool_size")
	}

	// Infrastructure fields
	if !data.InstanceType.Equal(state.InstanceType) {
		updateMaskPaths = append(updateMaskPaths, "instance_type")
	}
	if !data.Nodepool.Equal(state.Nodepool) {
		updateMaskPaths = append(updateMaskPaths, "nodepool")
	}
	if !data.NodeSelector.Equal(state.NodeSelector) {
		updateMaskPaths = append(updateMaskPaths, "node_selector")
	}

	// Gateway fields
	if !data.GatewayPort.Equal(state.GatewayPort) {
		updateMaskPaths = append(updateMaskPaths, "gateway_port")
	}
	if !data.GatewayId.Equal(state.GatewayId) {
		updateMaskPaths = append(updateMaskPaths, "gateway_id")
	}

	// Resources
	// Skip if unknown (Terraform hasn't determined the value yet)
	if !data.Request.IsUnknown() && !data.Request.Equal(state.Request) {
		updateMaskPaths = append(updateMaskPaths, "request")
	}

	// Database Config
	// Skip if unknown (Terraform hasn't determined the value yet)
	if !data.PostgresParameters.IsUnknown() && !data.PostgresParameters.Equal(state.PostgresParameters) {
		updateMaskPaths = append(updateMaskPaths, "postgres_parameters")
	}

	return updateMaskPaths
}

func (r *ClusterTimescaleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ClusterTimescaleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bc := r.client.NewBuilderClient(ctx)

	// Build specs from plan
	specs, err := buildClusterTimescaleSpecs(ctx, &data, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError(
			"Configuration Error",
			fmt.Sprintf("Failed to build TimescaleDB specification: %v", err),
		)
		return
	}

	// Create request
	createReq := &serverv1.CreateClusterTimescaleDBRequest{
		EnvironmentIds: []string{data.EnvironmentId.ValueString()},
		Specs:          specs,
	}

	// Call Create RPC
	response, err := bc.CreateClusterTimescaleDB(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Cluster TimescaleDB",
			fmt.Sprintf("Could not create cluster TimescaleDB: %v", err),
		)
		return
	}

	// Update model with response
	data.Id = types.StringValue(response.Msg.ClusterTimescaleId)
	if err := updateStateFromSpecs(&data, response.Msg.Specs); err != nil {
		resp.Diagnostics.AddError(
			"Error Processing Response",
			fmt.Sprintf("Could not process create response: %v", err),
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

	bc := r.client.NewBuilderClient(ctx)

	// Get the environment ID to query the TimescaleDB
	envId := data.EnvironmentId.ValueString()

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
	if timescale.Msg.Specs != nil {
		data.Id = types.StringValue(timescale.Msg.Id)
		if err := updateStateFromSpecs(&data, timescale.Msg.Specs); err != nil {
			resp.Diagnostics.AddError(
				"Error Processing Response",
				fmt.Sprintf("Could not process read response: %v", err),
			)
			return
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

	// Get current state to compare
	var state ClusterTimescaleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bc := r.client.NewBuilderClient(ctx)

	// Build specs from plan
	specs, err := buildClusterTimescaleSpecs(ctx, &data, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError(
			"Configuration Error",
			fmt.Sprintf("Failed to build TimescaleDB specification: %v", err),
		)
		return
	}

	// Build field mask by comparing plan and state
	updateMaskPaths := buildTimescaleUpdateMask(&data, &state)

	// Only make RPC call if there are changes
	if len(updateMaskPaths) > 0 {
		updateReq := &serverv1.UpdateClusterTimescaleDBRequest{
			ClusterTimescaleId: data.Id.ValueString(),
			Specs:              specs,
			UpdateMask: &fieldmaskpb.FieldMask{
				Paths: updateMaskPaths,
			},
		}

		response, err := bc.UpdateClusterTimescaleDB(ctx, connect.NewRequest(updateReq))
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Chalk Cluster TimescaleDB",
				fmt.Sprintf("Could not update cluster TimescaleDB: %v", err),
			)
			return
		}

		// Update model with response
		if err := updateStateFromSpecs(&data, response.Msg.Specs); err != nil {
			resp.Diagnostics.AddError(
				"Error Processing Response",
				fmt.Sprintf("Could not process update response: %v", err),
			)
			return
		}
	}

	tflog.Trace(ctx, "updated a chalk_cluster_timescale resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterTimescaleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	data := ClusterTimescaleResourceModel{}
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

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
	envId := req.ID

	// Set environment_id - Read will populate the rest including the actual resource ID
	resp.State.SetAttribute(ctx, path.Root("environment_id"), envId)
}
