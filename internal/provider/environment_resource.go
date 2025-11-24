package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

var _ resource.Resource = &EnvironmentResource{}
var _ resource.ResourceWithImportState = &EnvironmentResource{}

func NewEnvironmentResource() resource.Resource {
	return &EnvironmentResource{}
}

type EnvironmentResource struct {
	client *client.Manager
}

type EnvironmentBucketsModel struct {
	DatasetBucket       types.String `tfsdk:"dataset_bucket"`
	PlanStagesBucket    types.String `tfsdk:"plan_stages_bucket"`
	SourceBundleBucket  types.String `tfsdk:"source_bundle_bucket"`
	ModelRegistryBucket types.String `tfsdk:"model_registry_bucket"`
}

type EnvironmentResourceModel struct {
	Id                       types.String `tfsdk:"id"`
	Name                     types.String `tfsdk:"name"`
	ProjectId                types.String `tfsdk:"project_id"`
	SourceBundleBucket       types.String `tfsdk:"source_bundle_bucket"`
	KubeClusterId            types.String `tfsdk:"kube_cluster_id"`
	EngineDockerRegistryPath types.String `tfsdk:"engine_docker_registry_path"`
	Managed                  types.Bool   `tfsdk:"managed"`
	OnlineStoreSecret        types.String `tfsdk:"online_store_secret"`
	OnlineStoreKind          types.String `tfsdk:"online_store_kind"`
	FeatureStoreSecret       types.String `tfsdk:"feature_store_secret"`
	PrivatePipRepositories   types.String `tfsdk:"private_pip_repositories"`
	AdditionalEnvVars        types.Map    `tfsdk:"additional_env_vars"`
	SpecsConfigJson          types.String `tfsdk:"specs_config_json"`
	ServiceUrl               types.String `tfsdk:"service_url"`
	WorkerUrl                types.String `tfsdk:"worker_url"`
	BranchUrl                types.String `tfsdk:"branch_url"`
	KubeJobNamespace         types.String `tfsdk:"kube_job_namespace"`
	KubeServiceAccountName   types.String `tfsdk:"kube_service_account_name"`
	EnvironmentBuckets       types.Object `tfsdk:"environment_buckets"`
}

func (r *EnvironmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (r *EnvironmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk environment resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Environment identifier",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Environment name",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Project ID",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"source_bundle_bucket": schema.StringAttribute{
				MarkdownDescription: "Source bundle bucket",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kube_cluster_id": schema.StringAttribute{
				MarkdownDescription: "Kubernetes cluster ID",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"online_store_secret": schema.StringAttribute{
				MarkdownDescription: "Online store secret",
				Optional:            true,
				Sensitive:           true,
			},
			"online_store_kind": schema.StringAttribute{
				MarkdownDescription: "Online store kind",
				Optional:            true,
			},
			"feature_store_secret": schema.StringAttribute{
				MarkdownDescription: "Feature store secret",
				Optional:            true,
				Sensitive:           true,
			},
			"private_pip_repositories": schema.StringAttribute{
				MarkdownDescription: "Private pip repositories",
				Optional:            true,
			},
			"additional_env_vars": schema.MapAttribute{
				MarkdownDescription: "Additional environment variables",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"specs_config_json": schema.StringAttribute{
				MarkdownDescription: "Specs config JSON",
				Optional:            true,
			},
			"service_url": schema.StringAttribute{
				MarkdownDescription: "Service URL",
				Optional:            true,
			},
			"worker_url": schema.StringAttribute{
				MarkdownDescription: "Worker URL",
				Optional:            true,
			},
			"branch_url": schema.StringAttribute{
				MarkdownDescription: "Branch URL",
				Optional:            true,
			},
			"kube_job_namespace": schema.StringAttribute{
				MarkdownDescription: "Kubernetes job namespace",
				Optional:            true,
			},
			"kube_service_account_name": schema.StringAttribute{
				MarkdownDescription: "Kubernetes service account name",
				Optional:            true,
			},
			"engine_docker_registry_path": schema.StringAttribute{
				MarkdownDescription: "Engine Docker registry path",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"managed": schema.BoolAttribute{
				MarkdownDescription: "Whether to bootstrap cloud infrastructure",
				Optional:            true,
			},
			"environment_buckets": schema.SingleNestedAttribute{
				MarkdownDescription: "Environment object storage configuration",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"dataset_bucket": schema.StringAttribute{
						MarkdownDescription: "Dataset bucket",
						Optional:            true,
					},
					"plan_stages_bucket": schema.StringAttribute{
						MarkdownDescription: "Plan stages bucket",
						Optional:            true,
					},
					"source_bundle_bucket": schema.StringAttribute{
						MarkdownDescription: "Source bundle bucket",
						Optional:            true,
					},
					"model_registry_bucket": schema.StringAttribute{
						MarkdownDescription: "Model registry bucket",
						Optional:            true,
					},
				},
			},
		},
	}
}

func (r *EnvironmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Manager)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Manager, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *EnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EnvironmentResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Get team client from ClientManager
	tc, err := r.client.NewTeamClient(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Team Client", err.Error())
		return
	}

	createReq := &serverv1.CreateEnvironmentRequest{
		ProjectId: data.ProjectId.ValueString(),
		Name:      data.Name.ValueString(),
	}

	if !data.SourceBundleBucket.IsNull() {
		createReq.SourceBundleBucket = data.SourceBundleBucket.ValueString()
	}

	if !data.KubeClusterId.IsNull() {
		clusterId := data.KubeClusterId.ValueString()
		createReq.KubeClusterId = &clusterId
	}

	if !data.EngineDockerRegistryPath.IsNull() {
		val := data.EngineDockerRegistryPath.ValueString()
		createReq.EngineDockerRegistryPath = &val
	}

	// Use the id field as environment_id_override
	if !data.Id.IsNull() {
		val := data.Id.ValueString()
		createReq.EnvironmentIdOverride = &val
	}

	if !data.Managed.IsNull() {
		createReq.Managed = data.Managed.ValueBool()
	}
	_, err = tc.CreateEnvironment(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Environment",
			fmt.Sprintf("Could not create environment: %v", err),
		)
		return
	}

	// If update fields were provided, update the environment
	if !data.OnlineStoreSecret.IsNull() || !data.FeatureStoreSecret.IsNull() ||
		!data.PrivatePipRepositories.IsNull() || !data.AdditionalEnvVars.IsNull() ||
		!data.SpecsConfigJson.IsNull() || !data.ServiceUrl.IsNull() ||
		!data.WorkerUrl.IsNull() || !data.BranchUrl.IsNull() ||
		!data.KubeJobNamespace.IsNull() || !data.KubeServiceAccountName.IsNull() ||
		!data.EnvironmentBuckets.IsNull() || !data.OnlineStoreKind.IsNull() {
		updateReq := &serverv1.UpdateEnvironmentRequest{
			Id:     data.Id.ValueString(),
			Update: &serverv1.UpdateEnvironmentOperation{},
		}

		var updateMaskPaths []string

		if !data.OnlineStoreSecret.IsNull() {
			val := data.OnlineStoreSecret.ValueString()
			updateReq.Update.OnlineStoreSecret = &val
			updateMaskPaths = append(updateMaskPaths, "online_store_secret")
		}

		if !data.OnlineStoreKind.IsNull() {
			val := data.OnlineStoreKind.ValueString()
			updateReq.Update.OnlineStoreKind = &val
			updateMaskPaths = append(updateMaskPaths, "online_store_kind")
		}

		if !data.FeatureStoreSecret.IsNull() {
			val := data.FeatureStoreSecret.ValueString()
			updateReq.Update.FeatureStoreSecret = &val
			updateMaskPaths = append(updateMaskPaths, "feature_store_secret")
		}

		if !data.PrivatePipRepositories.IsNull() {
			val := data.PrivatePipRepositories.ValueString()
			updateReq.Update.PrivatePipRepositories = &val
			updateMaskPaths = append(updateMaskPaths, "private_pip_repositories")
		}

		if !data.AdditionalEnvVars.IsNull() {
			envVars := make(map[string]string)
			diags := data.AdditionalEnvVars.ElementsAs(ctx, &envVars, false)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			updateReq.Update.AdditionalEnvVars = envVars
			updateMaskPaths = append(updateMaskPaths, "additional_env_vars")
		}

		if !data.SpecsConfigJson.IsNull() {
			val := data.SpecsConfigJson.ValueString()
			updateReq.Update.SpecsConfigJson = &val
			updateMaskPaths = append(updateMaskPaths, "specs_config_json")
		}

		if !data.ServiceUrl.IsNull() {
			val := data.ServiceUrl.ValueString()
			updateReq.Update.ServiceUrl = &val
			updateMaskPaths = append(updateMaskPaths, "service_url")
		}

		if !data.WorkerUrl.IsNull() {
			val := data.WorkerUrl.ValueString()
			updateReq.Update.WorkerUrl = &val
			updateMaskPaths = append(updateMaskPaths, "worker_url")
		}

		if !data.BranchUrl.IsNull() {
			val := data.BranchUrl.ValueString()
			updateReq.Update.BranchUrl = &val
			updateMaskPaths = append(updateMaskPaths, "branch_url")
		}

		if !data.KubeJobNamespace.IsNull() {
			val := data.KubeJobNamespace.ValueString()
			updateReq.Update.KubeJobNamespace = &val
			updateMaskPaths = append(updateMaskPaths, "kube_job_namespace")
		}

		if !data.KubeServiceAccountName.IsNull() {
			val := data.KubeServiceAccountName.ValueString()
			updateReq.Update.KubeServiceAccountName = &val
			updateMaskPaths = append(updateMaskPaths, "kube_service_account_name")
		}

		if !data.EnvironmentBuckets.IsNull() {
			var buckets EnvironmentBucketsModel
			diags := data.EnvironmentBuckets.As(ctx, &buckets, basetypes.ObjectAsOptions{})
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			updateReq.Update.EnvironmentBuckets = &serverv1.EnvironmentObjectStorageConfig{
				DatasetBucket:       buckets.DatasetBucket.ValueString(),
				PlanStagesBucket:    buckets.PlanStagesBucket.ValueString(),
				SourceBundleBucket:  buckets.SourceBundleBucket.ValueString(),
				ModelRegistryBucket: buckets.ModelRegistryBucket.ValueString(),
			}
			updateMaskPaths = append(updateMaskPaths, "environment_buckets")
		}

		updateReq.UpdateMask = &fieldmaskpb.FieldMask{
			Paths: updateMaskPaths,
		}

		// Use the environment ID for the header
		tcUpdate, err := r.client.NewTeamClient(ctx, data.Id.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Team Client", err.Error())
			return
		}

		_, err = tcUpdate.UpdateEnvironment(ctx, connect.NewRequest(updateReq))
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Chalk Environment",
				fmt.Sprintf("Environment was created but could not be updated: %v", err),
			)
			return
		}
	}

	if data.Managed.ValueBool() {
		bc, err := r.client.NewBuilderClient(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Team Client", err.Error())
			return
		}
		_, err = bc.CreateEnvironmentCloudResources(ctx, connect.NewRequest(&serverv1.CreateEnvironmentCloudResourcesRequest{
			EnvironmentId: data.Id.ValueString(),
		}))
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Bootstrapping Chalk Environment Cloud Resources",
				fmt.Sprintf("Environment was created but cloud resources could not be bootstrapped: %v", err),
			)
			return
		}
	}

	tflog.Trace(ctx, "created a chalk_environment resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EnvironmentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create team client
	tc, err := r.client.NewTeamClient(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Team Client", err.Error())
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

	// Update the model with the fetched data
	e := env.Msg.Environment
	data.Name = types.StringValue(e.Name)
	data.ProjectId = types.StringValue(e.ProjectId)

	if e.OnlineStoreSecret != nil {
		data.OnlineStoreSecret = types.StringValue(*e.OnlineStoreSecret)
	}

	if e.OnlineStoreKind != nil {
		data.OnlineStoreKind = types.StringValue(*e.OnlineStoreKind)
	}

	if e.FeatureStoreSecret != nil {
		data.FeatureStoreSecret = types.StringValue(*e.FeatureStoreSecret)
	}

	if e.PrivatePipRepositories != nil {
		data.PrivatePipRepositories = types.StringValue(*e.PrivatePipRepositories)
	}

	if e.SourceBundleBucket != nil {
		data.SourceBundleBucket = types.StringValue(*e.SourceBundleBucket)
	}

	if e.KubeClusterId != nil {
		data.KubeClusterId = types.StringValue(*e.KubeClusterId)
	}

	if e.AdditionalEnvVars != nil && len(e.AdditionalEnvVars) > 0 {
		elements := make(map[string]attr.Value)
		for k, v := range e.AdditionalEnvVars {
			elements[k] = types.StringValue(v)
		}
		data.AdditionalEnvVars = types.MapValueMust(types.StringType, elements)
	}

	if e.ServiceUrl != nil {
		data.ServiceUrl = types.StringValue(*e.ServiceUrl)
	}

	if e.WorkerUrl != nil {
		data.WorkerUrl = types.StringValue(*e.WorkerUrl)
	}

	if e.BranchUrl != nil {
		data.BranchUrl = types.StringValue(*e.BranchUrl)
	}

	if e.KubeJobNamespace != nil {
		data.KubeJobNamespace = types.StringValue(*e.KubeJobNamespace)
	}

	if e.KubeServiceAccountName != nil {
		data.KubeServiceAccountName = types.StringValue(*e.KubeServiceAccountName)
	}

	if e.EngineDockerRegistryPath != nil {
		data.EngineDockerRegistryPath = types.StringValue(*e.EngineDockerRegistryPath)
	}

	if e.EnvironmentBuckets != nil {
		bucketsAttrs := map[string]attr.Value{
			"dataset_bucket":        types.StringValue(e.EnvironmentBuckets.DatasetBucket),
			"plan_stages_bucket":    types.StringValue(e.EnvironmentBuckets.PlanStagesBucket),
			"source_bundle_bucket":  types.StringValue(e.EnvironmentBuckets.SourceBundleBucket),
			"model_registry_bucket": types.StringValue(e.EnvironmentBuckets.ModelRegistryBucket),
		}
		bucketsType := types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"dataset_bucket":        types.StringType,
				"plan_stages_bucket":    types.StringType,
				"source_bundle_bucket":  types.StringType,
				"model_registry_bucket": types.StringType,
			},
		}
		data.EnvironmentBuckets = types.ObjectValueMust(bucketsType.AttrTypes, bucketsAttrs)
	}

	// Note: specs_config_json is not directly available in the Environment message

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data EnvironmentResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create team client
	tc, err := r.client.NewTeamClient(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Team Client", err.Error())
		return
	}

	updateReq := &serverv1.UpdateEnvironmentRequest{
		Id:     data.Id.ValueString(),
		Update: &serverv1.UpdateEnvironmentOperation{},
	}

	var updateMaskPaths []string

	// Check what fields have changed
	var state EnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !data.OnlineStoreSecret.Equal(state.OnlineStoreSecret) {
		if !data.OnlineStoreSecret.IsNull() {
			val := data.OnlineStoreSecret.ValueString()
			updateReq.Update.OnlineStoreSecret = &val
		}
		updateMaskPaths = append(updateMaskPaths, "online_store_secret")
	}

	if !data.OnlineStoreKind.Equal(state.OnlineStoreKind) {
		if !data.OnlineStoreKind.IsNull() {
			val := data.OnlineStoreKind.ValueString()
			updateReq.Update.OnlineStoreKind = &val
		}
		updateMaskPaths = append(updateMaskPaths, "online_store_kind")
	}

	if !data.FeatureStoreSecret.Equal(state.FeatureStoreSecret) {
		if !data.FeatureStoreSecret.IsNull() {
			val := data.FeatureStoreSecret.ValueString()
			updateReq.Update.FeatureStoreSecret = &val
		}
		updateMaskPaths = append(updateMaskPaths, "feature_store_secret")
	}

	if !data.PrivatePipRepositories.Equal(state.PrivatePipRepositories) {
		if !data.PrivatePipRepositories.IsNull() {
			val := data.PrivatePipRepositories.ValueString()
			updateReq.Update.PrivatePipRepositories = &val
		}
		updateMaskPaths = append(updateMaskPaths, "private_pip_repositories")
	}

	if !data.AdditionalEnvVars.Equal(state.AdditionalEnvVars) {
		if !data.AdditionalEnvVars.IsNull() {
			envVars := make(map[string]string)
			diags := data.AdditionalEnvVars.ElementsAs(ctx, &envVars, false)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			updateReq.Update.AdditionalEnvVars = envVars
		}
		updateMaskPaths = append(updateMaskPaths, "additional_env_vars")
	}

	if !data.SpecsConfigJson.Equal(state.SpecsConfigJson) {
		if !data.SpecsConfigJson.IsNull() {
			val := data.SpecsConfigJson.ValueString()
			updateReq.Update.SpecsConfigJson = &val
		}
		updateMaskPaths = append(updateMaskPaths, "specs_config_json")
	}

	if !data.ServiceUrl.Equal(state.ServiceUrl) {
		if !data.ServiceUrl.IsNull() {
			val := data.ServiceUrl.ValueString()
			updateReq.Update.ServiceUrl = &val
		}
		updateMaskPaths = append(updateMaskPaths, "service_url")
	}

	if !data.WorkerUrl.Equal(state.WorkerUrl) {
		if !data.WorkerUrl.IsNull() {
			val := data.WorkerUrl.ValueString()
			updateReq.Update.WorkerUrl = &val
		}
		updateMaskPaths = append(updateMaskPaths, "worker_url")
	}

	if !data.BranchUrl.Equal(state.BranchUrl) {
		if !data.BranchUrl.IsNull() {
			val := data.BranchUrl.ValueString()
			updateReq.Update.BranchUrl = &val
		}
		updateMaskPaths = append(updateMaskPaths, "branch_url")
	}

	if !data.KubeJobNamespace.Equal(state.KubeJobNamespace) {
		if !data.KubeJobNamespace.IsNull() {
			val := data.KubeJobNamespace.ValueString()
			updateReq.Update.KubeJobNamespace = &val
		}
		updateMaskPaths = append(updateMaskPaths, "kube_job_namespace")
	}

	if !data.KubeServiceAccountName.Equal(state.KubeServiceAccountName) {
		if !data.KubeServiceAccountName.IsNull() {
			val := data.KubeServiceAccountName.ValueString()
			updateReq.Update.KubeServiceAccountName = &val
		}
		updateMaskPaths = append(updateMaskPaths, "kube_service_account_name")
	}

	if !data.EnvironmentBuckets.Equal(state.EnvironmentBuckets) {
		if !data.EnvironmentBuckets.IsNull() {
			var buckets EnvironmentBucketsModel
			diags := data.EnvironmentBuckets.As(ctx, &buckets, basetypes.ObjectAsOptions{})
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			updateReq.Update.EnvironmentBuckets = &serverv1.EnvironmentObjectStorageConfig{
				DatasetBucket:       buckets.DatasetBucket.ValueString(),
				PlanStagesBucket:    buckets.PlanStagesBucket.ValueString(),
				SourceBundleBucket:  buckets.SourceBundleBucket.ValueString(),
				ModelRegistryBucket: buckets.ModelRegistryBucket.ValueString(),
			}
		}
		updateMaskPaths = append(updateMaskPaths, "environment_buckets")
	}

	if len(updateMaskPaths) > 0 {
		updateReq.UpdateMask = &fieldmaskpb.FieldMask{
			Paths: updateMaskPaths,
		}

		_, err := tc.UpdateEnvironment(ctx, connect.NewRequest(updateReq))
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Chalk Environment",
				fmt.Sprintf("Could not update environment: %v", err),
			)
			return
		}
	}

	if data.Managed.ValueBool() {
		bc, err := r.client.NewBuilderClient(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Team Client", err.Error())
			return
		}
		_, err = bc.CreateEnvironmentCloudResources(ctx, connect.NewRequest(&serverv1.CreateEnvironmentCloudResourcesRequest{
			EnvironmentId: data.Id.ValueString(),
		}))
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Bootstrapping Chalk Environment Cloud Resources",
				fmt.Sprintf("Environment was created but cloud resources could not be bootstrapped: %v", err),
			)
			return
		}
	}

	tflog.Trace(ctx, "updated a chalk_environment resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EnvironmentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	if !data.Managed.IsNull() {
		bc, err := r.client.NewBuilderClient(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Team Client", err.Error())
			return
		}
		_, err = bc.DeleteEnvironmentCloudResources(ctx, connect.NewRequest(&serverv1.DeleteEnvironmentCloudResourcesRequest{
			EnvironmentId: data.Id.ValueString(),
		}))
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Delete Chalk Environment Cloud Resources",
				fmt.Sprintf("Environment was created but cloud resources could not be deleted: %v", err),
			)
			return
		}
	}

	// Create team client
	tc, err := r.client.NewTeamClient(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Team Client", err.Error())
		return
	}

	archiveReq := &serverv1.ArchiveEnvironmentRequest{
		Id: data.Id.ValueString(),
	}

	_, err = tc.ArchiveEnvironment(ctx, connect.NewRequest(archiveReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Archiving Chalk Environment",
			fmt.Sprintf("Could not archive environment %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "archived chalk_environment resource")
}

func (r *EnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
