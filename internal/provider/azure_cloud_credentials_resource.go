package provider

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"net/http"
)

var _ resource.Resource = &AzureCloudCredentialsResource{}
var _ resource.ResourceWithImportState = &AzureCloudCredentialsResource{}

func NewAzureCloudCredentialsResource() resource.Resource {
	return &AzureCloudCredentialsResource{}
}

type AzureCloudCredentialsResource struct {
	client *ChalkClient
}

type AzureContainerRegistryConfigModel struct {
	RegistryName types.String `tfsdk:"registry_name"`
}

type AzureKeyVaultConfigModel struct {
	VaultName types.String `tfsdk:"vault_name"`
}

type AzureCloudCredentialsResourceModel struct {
	Id   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`

	// Azure Configuration
	SubscriptionId types.String `tfsdk:"subscription_id"`
	TenantId       types.String `tfsdk:"tenant_id"`
	Region         types.String `tfsdk:"region"`
	ResourceGroup  types.String `tfsdk:"resource_group"`

	// Block Configuration
	DockerBuildConfig       []DockerBuildConfigModel            `tfsdk:"docker_build_config"`
	ContainerRegistryConfig []AzureContainerRegistryConfigModel `tfsdk:"container_registry_config"`
	KeyVaultConfig          []AzureKeyVaultConfigModel          `tfsdk:"key_vault_config"`
	GCPWorkloadIdentity     []GCPWorkloadIdentityModel          `tfsdk:"gcp_workload_identity"`
}

func (r *AzureCloudCredentialsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_azure_cloud_credentials"
}

func (r *AzureCloudCredentialsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk Azure cloud credentials resource for configuring Azure authentication",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Azure cloud credentials identifier",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Azure cloud credentials name",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			// Azure Configuration
			"subscription_id": schema.StringAttribute{
				MarkdownDescription: "Azure subscription ID",
				Required:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Azure tenant ID",
				Required:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Azure region",
				Required:            true,
			},
			"resource_group": schema.StringAttribute{
				MarkdownDescription: "Azure resource group",
				Required:            true,
			},
		},

		Blocks: map[string]schema.Block{
			"docker_build_config": schema.ListNestedBlock{
				MarkdownDescription: "Docker build configuration (optional, max 1)",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"builder": schema.StringAttribute{
							MarkdownDescription: "Docker builder configuration",
							Optional:            true,
						},
						"push_registry_type": schema.StringAttribute{
							MarkdownDescription: "Docker push registry type",
							Optional:            true,
						},
						"push_registry_tag_prefix": schema.StringAttribute{
							MarkdownDescription: "Docker push registry tag prefix",
							Optional:            true,
						},
						"registry_credentials_secret_id": schema.StringAttribute{
							MarkdownDescription: "Docker registry credentials secret ID",
							Optional:            true,
							Sensitive:           true,
						},
						"notification_topic": schema.StringAttribute{
							MarkdownDescription: "Docker build notification topic",
							Optional:            true,
						},
					},
				},
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
			},
			"container_registry_config": schema.ListNestedBlock{
				MarkdownDescription: "Azure Container Registry configuration (optional, max 1)",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"registry_name": schema.StringAttribute{
							MarkdownDescription: "Azure Container Registry name",
							Optional:            true,
						},
					},
				},
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
			},
			"key_vault_config": schema.ListNestedBlock{
				MarkdownDescription: "Azure Key Vault configuration (optional, max 1)",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"vault_name": schema.StringAttribute{
							MarkdownDescription: "Azure Key Vault name",
							Optional:            true,
						},
					},
				},
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
			},
			"gcp_workload_identity": schema.ListNestedBlock{
				MarkdownDescription: "GCP workload identity configuration for Azure (optional, max 1)",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"project_number": schema.StringAttribute{
							MarkdownDescription: "GCP project number for workload identity federation",
							Optional:            true,
						},
						"service_account": schema.StringAttribute{
							MarkdownDescription: "GCP service account email for workload identity",
							Optional:            true,
						},
						"pool_id": schema.StringAttribute{
							MarkdownDescription: "GCP workload identity pool ID",
							Optional:            true,
						},
						"provider_id": schema.StringAttribute{
							MarkdownDescription: "GCP workload identity provider ID",
							Optional:            true,
						},
					},
				},
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
			},
		},
	}
}

func (r *AzureCloudCredentialsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AzureCloudCredentialsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AzureCloudCredentialsResourceModel

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

	// Create cloud credentials client with token injection interceptor
	credClient := NewCloudAccountCredentialsClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	})

	// Build the Azure cloud config
	azureConfig := &serverv1.AzureCloudConfig{
		SubscriptionId: data.SubscriptionId.ValueString(),
		TenantId:       data.TenantId.ValueString(),
		Region:         data.Region.ValueString(),
		ResourceGroup:  data.ResourceGroup.ValueString(),
	}

	// Add Docker build config if provided
	if dockerConfig := buildDockerConfigForAzure(&data); dockerConfig != nil {
		azureConfig.DockerBuildConfig = dockerConfig
	}

	// Add container registry config if provided
	if containerRegistryConfig := buildAzureContainerRegistryConfig(&data); containerRegistryConfig != nil {
		azureConfig.ContainerRegistryConfig = containerRegistryConfig
	}

	// Add key vault config if provided
	if keyVaultConfig := buildAzureKeyVaultConfig(&data); keyVaultConfig != nil {
		azureConfig.KeyVaultConfig = keyVaultConfig
	}

	// Add GCP workload identity if provided
	if workloadIdentity := buildGCPWorkloadIdentityForAzure(&data); workloadIdentity != nil {
		azureConfig.GcpWorkloadIdentity = workloadIdentity
	}

	cloudConfig := &serverv1.CloudConfig{
		Config: &serverv1.CloudConfig_Azure{
			Azure: azureConfig,
		},
	}

	createReq := &serverv1.CreateCloudCredentialsRequest{
		Credentials: &serverv1.CloudCredentialsRequest{
			Name:   data.Name.ValueString(),
			Kind:   "azure",
			Config: cloudConfig,
		},
	}

	creds, err := credClient.CreateCloudCredentials(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Azure Cloud Credentials",
			fmt.Sprintf("Could not create Azure cloud credentials: %v", err),
		)
		return
	}

	// Update with created values
	data.Id = types.StringValue(creds.Msg.Credentials.Id)

	tflog.Trace(ctx, "created a chalk_azure_cloud_credentials resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AzureCloudCredentialsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AzureCloudCredentialsResourceModel

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

	// Create cloud credentials client with token injection interceptor
	credClient := NewCloudAccountCredentialsClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	})

	creds, err := credClient.GetCloudCredentials(ctx, connect.NewRequest(&serverv1.GetCloudCredentialsRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Azure Cloud Credentials",
			fmt.Sprintf("Could not read Azure cloud credentials %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Update the model with the fetched data
	c := creds.Msg.Credentials
	data.Name = types.StringValue(c.Name)

	// Extract Azure configuration
	if c.Spec != nil && c.Spec.Config != nil {
		if config, ok := c.Spec.Config.(*serverv1.CloudConfig_Azure); ok {
			azure := config.Azure
			data.SubscriptionId = types.StringValue(azure.SubscriptionId)
			data.TenantId = types.StringValue(azure.TenantId)
			data.Region = types.StringValue(azure.Region)
			data.ResourceGroup = types.StringValue(azure.ResourceGroup)

			// Extract Docker build config if present
			if azure.DockerBuildConfig != nil {
				extractDockerConfigForAzure(azure.DockerBuildConfig, &data)
			}

			// Extract container registry config if present
			if azure.ContainerRegistryConfig != nil {
				extractAzureContainerRegistryConfig(azure.ContainerRegistryConfig, &data)
			}

			// Extract key vault config if present
			if azure.KeyVaultConfig != nil {
				extractAzureKeyVaultConfig(azure.KeyVaultConfig, &data)
			}

			// Extract GCP workload identity if present
			if azure.GcpWorkloadIdentity != nil {
				extractGCPWorkloadIdentityForAzure(azure.GcpWorkloadIdentity, &data)
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AzureCloudCredentialsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AzureCloudCredentialsResourceModel

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

	// Create cloud credentials client with token injection interceptor
	credClient := NewCloudAccountCredentialsClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	})

	// Build the Azure cloud config
	azureConfig := &serverv1.AzureCloudConfig{
		SubscriptionId: data.SubscriptionId.ValueString(),
		TenantId:       data.TenantId.ValueString(),
		Region:         data.Region.ValueString(),
		ResourceGroup:  data.ResourceGroup.ValueString(),
	}

	// Add Docker build config if provided
	if dockerConfig := buildDockerConfigForAzure(&data); dockerConfig != nil {
		azureConfig.DockerBuildConfig = dockerConfig
	}

	// Add container registry config if provided
	if containerRegistryConfig := buildAzureContainerRegistryConfig(&data); containerRegistryConfig != nil {
		azureConfig.ContainerRegistryConfig = containerRegistryConfig
	}

	// Add key vault config if provided
	if keyVaultConfig := buildAzureKeyVaultConfig(&data); keyVaultConfig != nil {
		azureConfig.KeyVaultConfig = keyVaultConfig
	}

	// Add GCP workload identity if provided
	if workloadIdentity := buildGCPWorkloadIdentityForAzure(&data); workloadIdentity != nil {
		azureConfig.GcpWorkloadIdentity = workloadIdentity
	}

	cloudConfig := &serverv1.CloudConfig{
		Config: &serverv1.CloudConfig_Azure{
			Azure: azureConfig,
		},
	}

	updateReq := &serverv1.UpdateCloudCredentialsRequest{
		Id: data.Id.ValueString(),
		Credentials: &serverv1.CloudCredentialsRequest{
			Name:   data.Name.ValueString(),
			Kind:   "azure",
			Config: cloudConfig,
		},
	}

	creds, err := credClient.UpdateCloudCredentials(ctx, connect.NewRequest(updateReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Azure Cloud Credentials",
			fmt.Sprintf("Could not update Azure cloud credentials: %v", err),
		)
		return
	}

	// Update with returned values
	c := creds.Msg.Credentials
	data.Name = types.StringValue(c.Name)

	// Extract Azure configuration
	if c.Spec != nil && c.Spec.Config != nil {
		if config, ok := c.Spec.Config.(*serverv1.CloudConfig_Azure); ok {
			azure := config.Azure
			data.SubscriptionId = types.StringValue(azure.SubscriptionId)
			data.TenantId = types.StringValue(azure.TenantId)
			data.Region = types.StringValue(azure.Region)
			data.ResourceGroup = types.StringValue(azure.ResourceGroup)

			// Extract Docker build config if present
			if azure.DockerBuildConfig != nil {
				extractDockerConfigForAzure(azure.DockerBuildConfig, &data)
			} else {
				data.DockerBuildConfig = []DockerBuildConfigModel{}
			}

			// Extract container registry config if present
			if azure.ContainerRegistryConfig != nil {
				extractAzureContainerRegistryConfig(azure.ContainerRegistryConfig, &data)
			} else {
				data.ContainerRegistryConfig = []AzureContainerRegistryConfigModel{}
			}

			// Extract key vault config if present
			if azure.KeyVaultConfig != nil {
				extractAzureKeyVaultConfig(azure.KeyVaultConfig, &data)
			} else {
				data.KeyVaultConfig = []AzureKeyVaultConfigModel{}
			}

			// Extract GCP workload identity if present
			if azure.GcpWorkloadIdentity != nil {
				extractGCPWorkloadIdentityForAzure(azure.GcpWorkloadIdentity, &data)
			} else {
				data.GCPWorkloadIdentity = []GCPWorkloadIdentityModel{}
			}
		}
	}

	tflog.Trace(ctx, "updated chalk_azure_cloud_credentials resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AzureCloudCredentialsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AzureCloudCredentialsResourceModel

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

	// Create cloud credentials client with token injection interceptor
	credClient := NewCloudAccountCredentialsClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	})

	deleteReq := &serverv1.DeleteCloudCredentialsRequest{
		Id: data.Id.ValueString(),
	}

	_, err := credClient.DeleteCloudCredentials(ctx, connect.NewRequest(deleteReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Azure Cloud Credentials",
			fmt.Sprintf("Could not delete Azure cloud credentials %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "deleted chalk_azure_cloud_credentials resource")
}

func (r *AzureCloudCredentialsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helper function to build Docker build config if block is provided (for Azure)
func buildDockerConfigForAzure(data *AzureCloudCredentialsResourceModel) *serverv1.DockerBuildConfig {
	if len(data.DockerBuildConfig) == 0 {
		return nil
	}

	dockerBlock := data.DockerBuildConfig[0]
	dockerConfig := &serverv1.DockerBuildConfig{}

	if !dockerBlock.Builder.IsNull() {
		dockerConfig.Builder = dockerBlock.Builder.ValueString()
	}
	if !dockerBlock.PushRegistryType.IsNull() {
		dockerConfig.PushRegistryType = dockerBlock.PushRegistryType.ValueString()
	}
	if !dockerBlock.PushRegistryTagPrefix.IsNull() {
		dockerConfig.PushRegistryTagPrefix = dockerBlock.PushRegistryTagPrefix.ValueString()
	}
	if !dockerBlock.RegistryCredentialsSecret.IsNull() {
		dockerConfig.RegistryCredentialsSecretId = dockerBlock.RegistryCredentialsSecret.ValueString()
	}
	if !dockerBlock.NotificationTopic.IsNull() {
		dockerConfig.NotificationTopic = dockerBlock.NotificationTopic.ValueString()
	}

	return dockerConfig
}

// Helper function to build Azure Container Registry config if block is provided
func buildAzureContainerRegistryConfig(data *AzureCloudCredentialsResourceModel) *serverv1.AzureContainerRegistryConfig {
	if len(data.ContainerRegistryConfig) == 0 {
		return nil
	}

	registryBlock := data.ContainerRegistryConfig[0]
	registryConfig := &serverv1.AzureContainerRegistryConfig{}

	if !registryBlock.RegistryName.IsNull() {
		registryName := registryBlock.RegistryName.ValueString()
		registryConfig.RegistryName = &registryName
	}

	return registryConfig
}

// Helper function to build Azure Key Vault config if block is provided
func buildAzureKeyVaultConfig(data *AzureCloudCredentialsResourceModel) *serverv1.AzureKeyVaultConfig {
	if len(data.KeyVaultConfig) == 0 {
		return nil
	}

	vaultBlock := data.KeyVaultConfig[0]
	vaultConfig := &serverv1.AzureKeyVaultConfig{}

	if !vaultBlock.VaultName.IsNull() {
		vaultName := vaultBlock.VaultName.ValueString()
		vaultConfig.VaultName = &vaultName
	}

	return vaultConfig
}

// Helper function to extract Docker build config from proto to model (for Azure)
func extractDockerConfigForAzure(dockerConfig *serverv1.DockerBuildConfig, data *AzureCloudCredentialsResourceModel) {
	dockerBlock := DockerBuildConfigModel{}

	if dockerConfig.Builder != "" {
		dockerBlock.Builder = types.StringValue(dockerConfig.Builder)
	}
	if dockerConfig.PushRegistryType != "" {
		dockerBlock.PushRegistryType = types.StringValue(dockerConfig.PushRegistryType)
	}
	if dockerConfig.PushRegistryTagPrefix != "" {
		dockerBlock.PushRegistryTagPrefix = types.StringValue(dockerConfig.PushRegistryTagPrefix)
	}
	if dockerConfig.RegistryCredentialsSecretId != "" {
		dockerBlock.RegistryCredentialsSecret = types.StringValue(dockerConfig.RegistryCredentialsSecretId)
	}
	if dockerConfig.NotificationTopic != "" {
		dockerBlock.NotificationTopic = types.StringValue(dockerConfig.NotificationTopic)
	}

	data.DockerBuildConfig = []DockerBuildConfigModel{dockerBlock}
}

// Helper function to extract Azure Container Registry config from proto to model
func extractAzureContainerRegistryConfig(registryConfig *serverv1.AzureContainerRegistryConfig, data *AzureCloudCredentialsResourceModel) {
	registryBlock := AzureContainerRegistryConfigModel{}

	if registryConfig.RegistryName != nil && *registryConfig.RegistryName != "" {
		registryBlock.RegistryName = types.StringValue(*registryConfig.RegistryName)
	}

	data.ContainerRegistryConfig = []AzureContainerRegistryConfigModel{registryBlock}
}

// Helper function to extract Azure Key Vault config from proto to model
func extractAzureKeyVaultConfig(vaultConfig *serverv1.AzureKeyVaultConfig, data *AzureCloudCredentialsResourceModel) {
	vaultBlock := AzureKeyVaultConfigModel{}

	if vaultConfig.VaultName != nil && *vaultConfig.VaultName != "" {
		vaultBlock.VaultName = types.StringValue(*vaultConfig.VaultName)
	}

	data.KeyVaultConfig = []AzureKeyVaultConfigModel{vaultBlock}
}

// Helper function to build GCP workload identity if block is provided (for Azure)
func buildGCPWorkloadIdentityForAzure(data *AzureCloudCredentialsResourceModel) *serverv1.GCPWorkloadIdentity {
	if len(data.GCPWorkloadIdentity) == 0 {
		return nil
	}

	workloadBlock := data.GCPWorkloadIdentity[0]
	workloadIdentity := &serverv1.GCPWorkloadIdentity{}

	if !workloadBlock.ProjectNumber.IsNull() {
		workloadIdentity.GcpProjectNumber = workloadBlock.ProjectNumber.ValueString()
	}
	if !workloadBlock.ServiceAccount.IsNull() {
		workloadIdentity.GcpServiceAccount = workloadBlock.ServiceAccount.ValueString()
	}
	if !workloadBlock.PoolId.IsNull() {
		workloadIdentity.PoolId = workloadBlock.PoolId.ValueString()
	}
	if !workloadBlock.ProviderId.IsNull() {
		workloadIdentity.ProviderId = workloadBlock.ProviderId.ValueString()
	}

	return workloadIdentity
}

// Helper function to extract GCP workload identity from proto to model (for Azure)
func extractGCPWorkloadIdentityForAzure(workloadIdentity *serverv1.GCPWorkloadIdentity, data *AzureCloudCredentialsResourceModel) {
	workloadBlock := GCPWorkloadIdentityModel{}

	if workloadIdentity.GcpProjectNumber != "" {
		workloadBlock.ProjectNumber = types.StringValue(workloadIdentity.GcpProjectNumber)
	}
	if workloadIdentity.GcpServiceAccount != "" {
		workloadBlock.ServiceAccount = types.StringValue(workloadIdentity.GcpServiceAccount)
	}
	if workloadIdentity.PoolId != "" {
		workloadBlock.PoolId = types.StringValue(workloadIdentity.PoolId)
	}
	if workloadIdentity.ProviderId != "" {
		workloadBlock.ProviderId = types.StringValue(workloadIdentity.ProviderId)
	}

	data.GCPWorkloadIdentity = []GCPWorkloadIdentityModel{workloadBlock}
}
