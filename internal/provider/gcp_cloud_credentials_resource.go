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

var _ resource.Resource = &GCPCloudCredentialsResource{}
var _ resource.ResourceWithImportState = &GCPCloudCredentialsResource{}

func NewGCPCloudCredentialsResource() resource.Resource {
	return &GCPCloudCredentialsResource{}
}

type GCPCloudCredentialsResource struct {
	client *ClientManager
}

type GCPCloudCredentialsResourceModel struct {
	Id   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`

	// GCP Configuration
	GCPProjectId                types.String `tfsdk:"gcp_project_id"`
	GCPRegion                   types.String `tfsdk:"gcp_region"`
	GCPManagementServiceAccount types.String `tfsdk:"gcp_management_service_account"`

	// Block Configuration
	DockerBuildConfig []DockerBuildConfigModel `tfsdk:"docker_build_config"`
}

func (r *GCPCloudCredentialsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gcp_cloud_credentials"
}

func (r *GCPCloudCredentialsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk GCP cloud credentials resource for configuring GCP authentication",

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
			"gcp_project_id": schema.StringAttribute{
				MarkdownDescription: "GCP project ID",
				Required:            true,
			},
			"gcp_region": schema.StringAttribute{
				MarkdownDescription: "GCP region",
				Required:            true,
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
		},
	}
}

func (r *GCPCloudCredentialsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *GCPCloudCredentialsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GCPCloudCredentialsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create cloud credentials client
	credClient := r.client.NewCloudAccountCredentialsClient(ctx)

	gcpConfig := &serverv1.GCPCloudConfig{
		ProjectId: data.GCPProjectId.ValueString(),
		Region:    data.GCPRegion.ValueString(),
	}

	if !data.GCPManagementServiceAccount.IsNull() {
		serviceAccount := data.GCPManagementServiceAccount.ValueString()
		gcpConfig.ManagementServiceAccount = &serviceAccount
	}

	// Add Docker build config if provided
	if dockerConfig := buildDockerConfigGCP(&data.DockerBuildConfig); dockerConfig != nil {
		gcpConfig.DockerBuildConfig = dockerConfig
	}

	cloudConfig := &serverv1.CloudConfig{
		Config: &serverv1.CloudConfig_Gcp{
			Gcp: gcpConfig,
		},
	}

	createReq := &serverv1.CreateCloudCredentialsRequest{
		Credentials: &serverv1.CloudCredentialsRequest{
			Name:   data.Name.ValueString(),
			Kind:   "gcp",
			Config: cloudConfig,
		},
	}

	creds, err := credClient.CreateCloudCredentials(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating GCP Cloud Credentials",
			fmt.Sprintf("Could not create GCP cloud credentials: %v", err),
		)
		return
	}

	// Update with created values
	data.Id = types.StringValue(creds.Msg.Credentials.Id)

	tflog.Trace(ctx, "created a chalk_gcp_cloud_credentials resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GCPCloudCredentialsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GCPCloudCredentialsResourceModel

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
			"Error Reading GCP Cloud Credentials",
			fmt.Sprintf("Could not read GCP cloud credentials %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Update the model with the fetched data
	c := creds.Msg.Credentials
	data.Name = types.StringValue(c.Name)

	// Extract configuration
	if c.Spec != nil && c.Spec.Config != nil {
		if config, ok := c.Spec.Config.(*serverv1.CloudConfig_Gcp); ok {
			gcp := config.Gcp
			data.GCPProjectId = types.StringValue(gcp.ProjectId)
			data.GCPRegion = types.StringValue(gcp.Region)
			if gcp.ManagementServiceAccount != nil {
				data.GCPManagementServiceAccount = types.StringValue(*gcp.ManagementServiceAccount)
			} else {
				data.GCPManagementServiceAccount = types.StringNull()
			}

			// Extract Docker build config if present
			if gcp.DockerBuildConfig != nil {
				extractDockerConfigGCP(gcp.DockerBuildConfig, &data.DockerBuildConfig)
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GCPCloudCredentialsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data GCPCloudCredentialsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create cloud credentials client
	credClient := r.client.NewCloudAccountCredentialsClient(ctx)

	gcpConfig := &serverv1.GCPCloudConfig{
		ProjectId: data.GCPProjectId.ValueString(),
		Region:    data.GCPRegion.ValueString(),
	}

	if !data.GCPManagementServiceAccount.IsNull() {
		serviceAccount := data.GCPManagementServiceAccount.ValueString()
		gcpConfig.ManagementServiceAccount = &serviceAccount
	}

	// Add Docker build config if provided
	if dockerConfig := buildDockerConfigGCP(&data.DockerBuildConfig); dockerConfig != nil {
		gcpConfig.DockerBuildConfig = dockerConfig
	}

	cloudConfig := &serverv1.CloudConfig{
		Config: &serverv1.CloudConfig_Gcp{
			Gcp: gcpConfig,
		},
	}

	updateReq := &serverv1.UpdateCloudCredentialsRequest{
		Id: data.Id.ValueString(),
		Credentials: &serverv1.CloudCredentialsRequest{
			Name:   data.Name.ValueString(),
			Kind:   "gcp",
			Config: cloudConfig,
		},
	}

	creds, err := credClient.UpdateCloudCredentials(ctx, connect.NewRequest(updateReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating GCP Cloud Credentials",
			fmt.Sprintf("Could not update GCP cloud credentials: %v", err),
		)
		return
	}

	// Update with returned values
	c := creds.Msg.Credentials
	data.Name = types.StringValue(c.Name)

	// Extract configuration from response
	if c.Spec != nil && c.Spec.Config != nil {
		if config, ok := c.Spec.Config.(*serverv1.CloudConfig_Gcp); ok {
			gcp := config.Gcp
			data.GCPProjectId = types.StringValue(gcp.ProjectId)
			data.GCPRegion = types.StringValue(gcp.Region)
			if gcp.ManagementServiceAccount != nil {
				data.GCPManagementServiceAccount = types.StringValue(*gcp.ManagementServiceAccount)
			} else {
				data.GCPManagementServiceAccount = types.StringNull()
			}

			// Extract Docker build config if present
			if gcp.DockerBuildConfig != nil {
				extractDockerConfigGCP(gcp.DockerBuildConfig, &data.DockerBuildConfig)
			} else {
				data.DockerBuildConfig = []DockerBuildConfigModel{}
			}
		}
	}

	tflog.Trace(ctx, "updated chalk_gcp_cloud_credentials resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GCPCloudCredentialsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GCPCloudCredentialsResourceModel

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
			"Error Deleting GCP Cloud Credentials",
			fmt.Sprintf("Could not delete GCP cloud credentials %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "deleted chalk_gcp_cloud_credentials resource")
}

func (r *GCPCloudCredentialsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helper function to build Docker build config if block is provided
func buildDockerConfigGCP(dockerConfigSlice *[]DockerBuildConfigModel) *serverv1.DockerBuildConfig {
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

// Helper function to extract Docker build config from proto to model
func extractDockerConfigGCP(dockerConfig *serverv1.DockerBuildConfig, dockerConfigSlice *[]DockerBuildConfigModel) {
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
