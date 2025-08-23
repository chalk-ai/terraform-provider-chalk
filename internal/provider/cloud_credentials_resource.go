package provider

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"net/http"
)

var _ resource.Resource = &CloudCredentialsResource{}
var _ resource.ResourceWithImportState = &CloudCredentialsResource{}

func NewCloudCredentialsResource() resource.Resource {
	return &CloudCredentialsResource{}
}

type CloudCredentialsResource struct {
	client *ChalkClient
}

type DockerBuildConfigModel struct {
	Builder                    types.String `tfsdk:"builder"`
	PushRegistryType          types.String `tfsdk:"push_registry_type"`
	PushRegistryTagPrefix     types.String `tfsdk:"push_registry_tag_prefix"`
	RegistryCredentialsSecret types.String `tfsdk:"registry_credentials_secret_id"`
	NotificationTopic         types.String `tfsdk:"notification_topic"`
}

type GCPWorkloadIdentityModel struct {
	ProjectNumber    types.String `tfsdk:"project_number"`
	ServiceAccount   types.String `tfsdk:"service_account"`
	PoolId           types.String `tfsdk:"pool_id"`
	ProviderId       types.String `tfsdk:"provider_id"`
}

type CloudCredentialsResourceModel struct {
	Id   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Kind types.String `tfsdk:"kind"`
	
	// AWS Configuration
	AWSAccountId          types.String `tfsdk:"aws_account_id"`
	AWSManagementRoleArn  types.String `tfsdk:"aws_management_role_arn"`
	AWSRegion             types.String `tfsdk:"aws_region"`
	AWSExternalId         types.String `tfsdk:"aws_external_id"`
	
	// GCP Configuration  
	GCPProjectId                 types.String `tfsdk:"gcp_project_id"`
	GCPRegion                    types.String `tfsdk:"gcp_region"`
	GCPManagementServiceAccount  types.String `tfsdk:"gcp_management_service_account"`
	
	// Block Configuration
	DockerBuildConfig    []DockerBuildConfigModel    `tfsdk:"docker_build_config"`
	GCPWorkloadIdentity  []GCPWorkloadIdentityModel  `tfsdk:"gcp_workload_identity"`
}

func (r *CloudCredentialsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_credentials"
}

func (r *CloudCredentialsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk cloud credentials resource for configuring cloud provider authentication",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud credentials identifier",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Cloud credentials name",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Cloud provider kind (aws or gcp)",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			
			// AWS Configuration
			"aws_account_id": schema.StringAttribute{
				MarkdownDescription: "AWS account ID (required for AWS kind)",
				Optional:            true,
			},
			"aws_management_role_arn": schema.StringAttribute{
				MarkdownDescription: "AWS management role ARN (required for AWS kind)",
				Optional:            true,
			},
			"aws_region": schema.StringAttribute{
				MarkdownDescription: "AWS region (required for AWS kind)",
				Optional:            true,
			},
			"aws_external_id": schema.StringAttribute{
				MarkdownDescription: "AWS external ID for role assumption",
				Optional:            true,
			},
			
			// GCP Configuration
			"gcp_project_id": schema.StringAttribute{
				MarkdownDescription: "GCP project ID (required for GCP kind)",
				Optional:            true,
			},
			"gcp_region": schema.StringAttribute{
				MarkdownDescription: "GCP region (required for GCP kind)",
				Optional:            true,
			},
			"gcp_management_service_account": schema.StringAttribute{
				MarkdownDescription: "GCP management service account",
				Optional:            true,
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
			"gcp_workload_identity": schema.ListNestedBlock{
				MarkdownDescription: "GCP workload identity configuration for AWS (optional, max 1)",
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

func (r *CloudCredentialsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CloudCredentialsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CloudCredentialsResourceModel

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

	// Build the cloud config based on kind
	var cloudConfig *serverv1.CloudConfig
	kind := data.Kind.ValueString()

	if kind == "aws" {
		// Validate AWS required fields
		if data.AWSAccountId.IsNull() || data.AWSManagementRoleArn.IsNull() || data.AWSRegion.IsNull() {
			resp.Diagnostics.AddError(
				"Missing AWS Configuration",
				"For AWS cloud credentials, aws_account_id, aws_management_role_arn, and aws_region are required",
			)
			return
		}

		awsConfig := &serverv1.AWSCloudConfig{
			AccountId:         data.AWSAccountId.ValueString(),
			ManagementRoleArn: data.AWSManagementRoleArn.ValueString(),
			Region:           data.AWSRegion.ValueString(),
		}

		if !data.AWSExternalId.IsNull() {
			externalId := data.AWSExternalId.ValueString()
			awsConfig.ExternalId = &externalId
		}
		
		// Add Docker build config if provided
		if dockerConfig := buildDockerConfig(&data); dockerConfig != nil {
			awsConfig.DockerBuildConfig = dockerConfig
		}
		
		// Add GCP workload identity if provided
		if workloadIdentity := buildGCPWorkloadIdentity(&data); workloadIdentity != nil {
			awsConfig.GcpWorkloadIdentity = workloadIdentity
		}

		cloudConfig = &serverv1.CloudConfig{
			Config: &serverv1.CloudConfig_Aws{
				Aws: awsConfig,
			},
		}
	} else if kind == "gcp" {
		// Validate GCP required fields
		if data.GCPProjectId.IsNull() || data.GCPRegion.IsNull() {
			resp.Diagnostics.AddError(
				"Missing GCP Configuration",
				"For GCP cloud credentials, gcp_project_id and gcp_region are required",
			)
			return
		}

		gcpConfig := &serverv1.GCPCloudConfig{
			ProjectId: data.GCPProjectId.ValueString(),
			Region:    data.GCPRegion.ValueString(),
		}

		if !data.GCPManagementServiceAccount.IsNull() {
			serviceAccount := data.GCPManagementServiceAccount.ValueString()
			gcpConfig.ManagementServiceAccount = &serviceAccount
		}
		
		// Add Docker build config if provided
		if dockerConfig := buildDockerConfig(&data); dockerConfig != nil {
			gcpConfig.DockerBuildConfig = dockerConfig
		}

		cloudConfig = &serverv1.CloudConfig{
			Config: &serverv1.CloudConfig_Gcp{
				Gcp: gcpConfig,
			},
		}
	} else {
		resp.Diagnostics.AddError(
			"Invalid Cloud Kind",
			fmt.Sprintf("Cloud kind must be 'aws' or 'gcp', got: %s", kind),
		)
		return
	}

	createReq := &serverv1.CreateCloudCredentialsRequest{
		Credentials: &serverv1.CloudCredentialsRequest{
			Name:   data.Name.ValueString(),
			Kind:   kind,
			Config: cloudConfig,
		},
	}

	creds, err := credClient.CreateCloudCredentials(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Cloud Credentials",
			fmt.Sprintf("Could not create cloud credentials: %v", err),
		)
		return
	}

	// Update with created values
	data.Id = types.StringValue(creds.Msg.Credentials.Id)

	tflog.Trace(ctx, "created a chalk_cloud_credentials resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudCredentialsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CloudCredentialsResourceModel

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
			"Error Reading Cloud Credentials",
			fmt.Sprintf("Could not read cloud credentials %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Update the model with the fetched data
	c := creds.Msg.Credentials
	data.Name = types.StringValue(c.Name)
	data.Kind = types.StringValue(c.Kind)

	// Extract configuration based on kind
	if c.Spec != nil && c.Spec.Config != nil {
		switch config := c.Spec.Config.(type) {
		case *serverv1.CloudConfig_Aws:
			aws := config.Aws
			data.AWSAccountId = types.StringValue(aws.AccountId)
			data.AWSManagementRoleArn = types.StringValue(aws.ManagementRoleArn)
			data.AWSRegion = types.StringValue(aws.Region)
			if aws.ExternalId != nil {
				data.AWSExternalId = types.StringValue(*aws.ExternalId)
			}
			
			// Extract Docker build config if present
			if aws.DockerBuildConfig != nil {
				extractDockerConfig(aws.DockerBuildConfig, &data)
			}
			
			// Extract GCP workload identity if present
			if aws.GcpWorkloadIdentity != nil {
				extractGCPWorkloadIdentity(aws.GcpWorkloadIdentity, &data)
			}
		case *serverv1.CloudConfig_Gcp:
			gcp := config.Gcp
			data.GCPProjectId = types.StringValue(gcp.ProjectId)
			data.GCPRegion = types.StringValue(gcp.Region)
			if gcp.ManagementServiceAccount != nil {
				data.GCPManagementServiceAccount = types.StringValue(*gcp.ManagementServiceAccount)
			}
			
			// Extract Docker build config if present
			if gcp.DockerBuildConfig != nil {
				extractDockerConfig(gcp.DockerBuildConfig, &data)
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudCredentialsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Cloud credentials don't support update operations based on the proto definition
	// The name and kind require replacement, and config changes would typically require recreation
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Cloud credentials cannot be updated. Changes require resource replacement.",
	)
}

func (r *CloudCredentialsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CloudCredentialsResourceModel

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
			"Error Deleting Cloud Credentials",
			fmt.Sprintf("Could not delete cloud credentials %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "deleted chalk_cloud_credentials resource")
}

func (r *CloudCredentialsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helper function to build Docker build config if block is provided
func buildDockerConfig(data *CloudCredentialsResourceModel) *serverv1.DockerBuildConfig {
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

// Helper function to build GCP workload identity if block is provided
func buildGCPWorkloadIdentity(data *CloudCredentialsResourceModel) *serverv1.GCPWorkloadIdentity {
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

// Helper function to extract Docker build config from proto to model
func extractDockerConfig(dockerConfig *serverv1.DockerBuildConfig, data *CloudCredentialsResourceModel) {
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

// Helper function to extract GCP workload identity from proto to model
func extractGCPWorkloadIdentity(workloadIdentity *serverv1.GCPWorkloadIdentity, data *CloudCredentialsResourceModel) {
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