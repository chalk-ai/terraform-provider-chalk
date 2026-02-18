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
)

var _ resource.Resource = &AWSCloudCredentialsResource{}
var _ resource.ResourceWithImportState = &AWSCloudCredentialsResource{}

func NewAWSCloudCredentialsResource() resource.Resource {
	return &AWSCloudCredentialsResource{}
}

type AWSCloudCredentialsResource struct {
	client *ClientManager
}

type AWSCloudCredentialsResourceModel struct {
	Id   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`

	// AWS Configuration
	AWSAccountId         types.String               `tfsdk:"aws_account_id"`
	AWSManagementRoleArn types.String               `tfsdk:"aws_management_role_arn"`
	AWSRegion            types.String               `tfsdk:"aws_region"`
	AWSExternalId        types.String               `tfsdk:"aws_external_id"`
	GCPWorkloadIdentity  []GCPWorkloadIdentityModel `tfsdk:"gcp_workload_identity"`

	// Block Configuration
	DockerBuildConfig []DockerBuildConfigModel `tfsdk:"docker_build_config"`
}

func (r *AWSCloudCredentialsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aws_cloud_credentials"
}

func (r *AWSCloudCredentialsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk AWS cloud credentials resource for configuring AWS authentication",

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
			"aws_account_id": schema.StringAttribute{
				MarkdownDescription: "AWS account ID",
				Required:            true,
			},
			"aws_management_role_arn": schema.StringAttribute{
				MarkdownDescription: "AWS management role ARN",
				Required:            true,
			},
			"aws_region": schema.StringAttribute{
				MarkdownDescription: "AWS region",
				Required:            true,
			},
			"aws_external_id": schema.StringAttribute{
				MarkdownDescription: "AWS external ID for role assumption",
				Optional:            true,
				Computed:            true,
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

func (r *AWSCloudCredentialsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AWSCloudCredentialsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AWSCloudCredentialsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create cloud credentials client
	credClient := r.client.NewCloudAccountCredentialsClient(ctx)

	awsConfig := &serverv1.AWSCloudConfig{
		AccountId:         data.AWSAccountId.ValueString(),
		ManagementRoleArn: data.AWSManagementRoleArn.ValueString(),
		Region:            data.AWSRegion.ValueString(),
	}

	if !data.AWSExternalId.IsNull() {
		externalId := data.AWSExternalId.ValueString()
		awsConfig.ExternalId = &externalId
	}

	// Add Docker build config if provided
	if dockerConfig := buildDockerConfig(&data.DockerBuildConfig); dockerConfig != nil {
		awsConfig.DockerBuildConfig = dockerConfig
	}

	// Add GCP workload identity if provided
	if workloadIdentity := buildGCPWorkloadIdentity(&data.GCPWorkloadIdentity); workloadIdentity != nil {
		awsConfig.GcpWorkloadIdentity = workloadIdentity
	}

	cloudConfig := &serverv1.CloudConfig{
		Config: &serverv1.CloudConfig_Aws{
			Aws: awsConfig,
		},
	}

	createReq := &serverv1.CreateCloudCredentialsRequest{
		Credentials: &serverv1.CloudCredentialsRequest{
			Name:   data.Name.ValueString(),
			Kind:   "aws",
			Config: cloudConfig,
		},
	}

	creds, err := credClient.CreateCloudCredentials(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating AWS Cloud Credentials",
			fmt.Sprintf("Could not create AWS cloud credentials: %v", err),
		)
		return
	}

	// Update with created values
	c := creds.Msg.Credentials
	data.Id = types.StringValue(c.Id)

	// Extract configuration from response to ensure all computed fields are populated
	if c.Spec != nil && c.Spec.Config != nil {
		if config, ok := c.Spec.Config.(*serverv1.CloudConfig_Aws); ok {
			aws := config.Aws
			if aws.ExternalId != nil {
				data.AWSExternalId = types.StringValue(*aws.ExternalId)
			} else {
				data.AWSExternalId = types.StringNull()
			}
		}
	}

	tflog.Trace(ctx, "created a chalk_aws_cloud_credentials resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AWSCloudCredentialsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AWSCloudCredentialsResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create cloud credentials client
	credClient := r.client.NewCloudAccountCredentialsClient(ctx)

	creds, err := credClient.GetCloudCredentials(ctx, connect.NewRequest(&serverv1.GetCloudCredentialsRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading AWS Cloud Credentials",
			fmt.Sprintf("Could not read AWS cloud credentials %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Update the model with the fetched data
	c := creds.Msg.Credentials
	data.Name = types.StringValue(c.Name)

	// Extract configuration
	if c.Spec != nil && c.Spec.Config != nil {
		if config, ok := c.Spec.Config.(*serverv1.CloudConfig_Aws); ok {
			aws := config.Aws
			data.AWSAccountId = types.StringValue(aws.AccountId)
			data.AWSManagementRoleArn = types.StringValue(aws.ManagementRoleArn)
			data.AWSRegion = types.StringValue(aws.Region)
			if aws.ExternalId != nil {
				data.AWSExternalId = types.StringValue(*aws.ExternalId)
			} else {
				data.AWSExternalId = types.StringNull()
			}

			// Extract Docker build config if present
			if aws.DockerBuildConfig != nil {
				extractDockerConfig(aws.DockerBuildConfig, &data.DockerBuildConfig)
			}

			// Extract GCP workload identity if present
			if aws.GcpWorkloadIdentity != nil {
				extractGCPWorkloadIdentity(aws.GcpWorkloadIdentity, &data.GCPWorkloadIdentity)
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AWSCloudCredentialsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AWSCloudCredentialsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create cloud credentials client
	credClient := r.client.NewCloudAccountCredentialsClient(ctx)

	awsConfig := &serverv1.AWSCloudConfig{
		AccountId:         data.AWSAccountId.ValueString(),
		ManagementRoleArn: data.AWSManagementRoleArn.ValueString(),
		Region:            data.AWSRegion.ValueString(),
	}

	if !data.AWSExternalId.IsNull() {
		externalId := data.AWSExternalId.ValueString()
		awsConfig.ExternalId = &externalId
	}

	// Add Docker build config if provided
	if dockerConfig := buildDockerConfig(&data.DockerBuildConfig); dockerConfig != nil {
		awsConfig.DockerBuildConfig = dockerConfig
	}

	// Add GCP workload identity if provided
	if workloadIdentity := buildGCPWorkloadIdentity(&data.GCPWorkloadIdentity); workloadIdentity != nil {
		awsConfig.GcpWorkloadIdentity = workloadIdentity
	}

	cloudConfig := &serverv1.CloudConfig{
		Config: &serverv1.CloudConfig_Aws{
			Aws: awsConfig,
		},
	}

	updateReq := &serverv1.UpdateCloudCredentialsRequest{
		Id: data.Id.ValueString(),
		Credentials: &serverv1.CloudCredentialsRequest{
			Name:   data.Name.ValueString(),
			Kind:   "aws",
			Config: cloudConfig,
		},
	}

	creds, err := credClient.UpdateCloudCredentials(ctx, connect.NewRequest(updateReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating AWS Cloud Credentials",
			fmt.Sprintf("Could not update AWS cloud credentials: %v", err),
		)
		return
	}

	// Update with returned values
	c := creds.Msg.Credentials
	data.Name = types.StringValue(c.Name)

	// Extract configuration from response
	if c.Spec != nil && c.Spec.Config != nil {
		if config, ok := c.Spec.Config.(*serverv1.CloudConfig_Aws); ok {
			aws := config.Aws
			data.AWSAccountId = types.StringValue(aws.AccountId)
			data.AWSManagementRoleArn = types.StringValue(aws.ManagementRoleArn)
			data.AWSRegion = types.StringValue(aws.Region)
			if aws.ExternalId != nil {
				data.AWSExternalId = types.StringValue(*aws.ExternalId)
			} else {
				data.AWSExternalId = types.StringNull()
			}

			// Extract Docker build config if present
			if aws.DockerBuildConfig != nil {
				extractDockerConfig(aws.DockerBuildConfig, &data.DockerBuildConfig)
			} else {
				data.DockerBuildConfig = []DockerBuildConfigModel{}
			}

			// Extract GCP workload identity if present
			if aws.GcpWorkloadIdentity != nil {
				extractGCPWorkloadIdentity(aws.GcpWorkloadIdentity, &data.GCPWorkloadIdentity)
			} else {
				data.GCPWorkloadIdentity = []GCPWorkloadIdentityModel{}
			}
		}
	}

	tflog.Trace(ctx, "updated chalk_aws_cloud_credentials resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AWSCloudCredentialsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AWSCloudCredentialsResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create cloud credentials client
	credClient := r.client.NewCloudAccountCredentialsClient(ctx)

	deleteReq := &serverv1.DeleteCloudCredentialsRequest{
		Id: data.Id.ValueString(),
	}

	_, err := credClient.DeleteCloudCredentials(ctx, connect.NewRequest(deleteReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting AWS Cloud Credentials",
			fmt.Sprintf("Could not delete AWS cloud credentials %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "deleted chalk_aws_cloud_credentials resource")
}

func (r *AWSCloudCredentialsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helper function to build Docker build config if block is provided
func buildDockerConfig(dockerConfigSlice *[]DockerBuildConfigModel) *serverv1.DockerBuildConfig {
	if dockerConfigSlice == nil || len(*dockerConfigSlice) == 0 {
		return nil
	}

	dockerBlock := (*dockerConfigSlice)[0]
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
func buildGCPWorkloadIdentity(workloadIdentitySlice *[]GCPWorkloadIdentityModel) *serverv1.GCPWorkloadIdentity {
	if workloadIdentitySlice == nil || len(*workloadIdentitySlice) == 0 {
		return nil
	}

	workloadBlock := (*workloadIdentitySlice)[0]
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
func extractDockerConfig(dockerConfig *serverv1.DockerBuildConfig, dockerConfigSlice *[]DockerBuildConfigModel) {
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

	*dockerConfigSlice = []DockerBuildConfigModel{dockerBlock}
}

// Helper function to extract GCP workload identity from proto to model
func extractGCPWorkloadIdentity(workloadIdentity *serverv1.GCPWorkloadIdentity, workloadIdentitySlice *[]GCPWorkloadIdentityModel) {
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

	*workloadIdentitySlice = []GCPWorkloadIdentityModel{workloadBlock}
}
