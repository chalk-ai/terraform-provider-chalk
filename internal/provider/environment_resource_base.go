package provider

import (
	"context"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

// BaseEnvironmentModel holds the fields common to both managed and unmanaged environment resources.
type BaseEnvironmentModel struct {
	Id                       types.String         `tfsdk:"id"`
	Name                     types.String         `tfsdk:"name"`
	ProjectId                types.String         `tfsdk:"project_id"`
	KubeClusterId            types.String         `tfsdk:"kube_cluster_id"`
	KubeJobNamespace         types.String         `tfsdk:"kube_job_namespace"`
	EngineDockerRegistryPath types.String         `tfsdk:"engine_docker_registry_path"`
	ServiceUrl               types.String         `tfsdk:"service_url"`
	OnlineStoreKind          types.String         `tfsdk:"online_store_kind"`
	OnlineStoreSecret        types.String         `tfsdk:"online_store_secret"`
	AdditionalEnvVars        types.Map            `tfsdk:"additional_env_vars"`
	EnvironmentBuckets       types.Object         `tfsdk:"environment_buckets"`
	SpecsConfigJson          jsontypes.Normalized `tfsdk:"specs_config_json"`
	PrivatePipRepositories   types.String         `tfsdk:"private_pip_repositories"`
	PinnedBaseImage          types.String         `tfsdk:"pinned_base_image"`
	DefaultBuildProfile      types.String         `tfsdk:"default_build_profile"`
	CustomerMetadata         jsontypes.Normalized `tfsdk:"customer_metadata"`
}

// commonEnvironmentSchemaAttributes returns schema attributes shared by both environment resources.
// kubeJobNamespace is provided as a parameter because managed (Optional+Computed) and unmanaged
// (Required) differ on that attribute. Each call returns a fresh map.
func commonEnvironmentSchemaAttributes(kubeJobNamespace schema.Attribute) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			// Nominally computed-only (server generates the id). Can be set when running a dev api server.
			MarkdownDescription: "Environment identifier; server-generated (immutable)",
			Optional:            true,
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
				stringplanmodifier.RequiresReplaceIfConfigured(),
			},
		},
		"name": schema.StringAttribute{
			MarkdownDescription: "Environment name (immutable)",
			Required:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"project_id": schema.StringAttribute{
			MarkdownDescription: "Project ID (immutable)",
			Required:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"kube_cluster_id": schema.StringAttribute{
			MarkdownDescription: "Kubernetes cluster ID (immutable)",
			Required:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"kube_job_namespace": kubeJobNamespace,
		"service_url": schema.StringAttribute{
			MarkdownDescription: "Service URL (set by server if not provided)",
			Optional:            true,
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"engine_docker_registry_path": schema.StringAttribute{
			MarkdownDescription: "Engine Docker registry path (immutable)",
			Optional:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"online_store_kind": schema.StringAttribute{
			MarkdownDescription: "Online store kind",
			Optional:            true,
		},
		"online_store_secret": schema.StringAttribute{
			MarkdownDescription: "Online store secret",
			Optional:            true,
			Sensitive:           true,
		},
		"additional_env_vars": schema.MapAttribute{
			MarkdownDescription: "Additional environment variables",
			Optional:            true,
			ElementType:         types.StringType,
		},
		"environment_buckets": schema.SingleNestedAttribute{
			MarkdownDescription: "Environment object storage configuration; required for 'chalk apply' to work." +
				" Note that the buckets provided must be created externally first, and should have a CORS policy set" +
				" that allows GET access from the Chalk frontend.",
			Optional: true,
			Attributes: map[string]schema.Attribute{
				"dataset_bucket": schema.StringAttribute{
					MarkdownDescription: "Dataset bucket; required for 'chalk apply' to work.",
					Optional:            true,
				},
				"plan_stages_bucket": schema.StringAttribute{
					MarkdownDescription: "Plan stages bucket; required for 'chalk apply' to work.",
					Optional:            true,
				},
				"source_bundle_bucket": schema.StringAttribute{
					MarkdownDescription: "Source bundle bucket; required for 'chalk apply' to work.",
					Optional:            true,
				},
				"model_registry_bucket": schema.StringAttribute{
					MarkdownDescription: "Model registry bucket. This bucket is required if using the model registry.",
					Optional:            true,
				},
			},
		},
		"specs_config_json": schema.StringAttribute{
			MarkdownDescription: "Specs config JSON (serialized map of spec configuration values)",
			CustomType:          jsontypes.NormalizedType{},
			Optional:            true,
			Computed:            true,
		},
		"private_pip_repositories": schema.StringAttribute{
			MarkdownDescription: "Private pip repositories",
			Optional:            true,
		},
		"pinned_base_image": schema.StringAttribute{
			MarkdownDescription: "Pinned base image for deployments",
			Optional:            true,
		},
		"default_build_profile": schema.StringAttribute{
			MarkdownDescription: "Default deployment build profile",
			Optional:            true,
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
			Validators: []validator.String{
				stringvalidator.OneOf(validDeploymentBuildProfiles()...),
			},
		},
		"customer_metadata": schema.StringAttribute{
			MarkdownDescription: "Customer metadata as a JSON object",
			CustomType:          jsontypes.NormalizedType{},
			Optional:            true,
		},
	}
}

// baseEnvToProto builds an Environment proto from the common fields of a BaseEnvironmentModel.
// Returns nil (with a diagnostic error) if any conversion fails.
func baseEnvToProto(ctx context.Context, data *BaseEnvironmentModel, diagnostics *diag.Diagnostics) *serverv1.Environment {
	env := &serverv1.Environment{
		Name:      data.Name.ValueString(),
		ProjectId: data.ProjectId.ValueString(),
	}

	// Pass user-provided ID only when explicitly set (dev/testing override).
	// In production the field is omitted and the server generates the slug.
	if !data.Id.IsNull() && !data.Id.IsUnknown() {
		env.Id = data.Id.ValueString()
	}

	if !data.KubeClusterId.IsNull() {
		v := data.KubeClusterId.ValueString()
		env.KubeClusterId = &v
	}
	if !data.KubeJobNamespace.IsNull() && !data.KubeJobNamespace.IsUnknown() {
		v := data.KubeJobNamespace.ValueString()
		env.KubeJobNamespace = &v
	}
	if !data.EngineDockerRegistryPath.IsNull() {
		v := data.EngineDockerRegistryPath.ValueString()
		env.EngineDockerRegistryPath = &v
	}
	if !data.ServiceUrl.IsNull() && !data.ServiceUrl.IsUnknown() {
		v := data.ServiceUrl.ValueString()
		env.ServiceUrl = &v
	}
	if !data.OnlineStoreKind.IsNull() && !data.OnlineStoreKind.IsUnknown() {
		v := data.OnlineStoreKind.ValueString()
		env.OnlineStoreKind = &v
	}
	if !data.OnlineStoreSecret.IsNull() && !data.OnlineStoreSecret.IsUnknown() {
		v := data.OnlineStoreSecret.ValueString()
		env.OnlineStoreSecret = &v
	}
	if !data.PrivatePipRepositories.IsNull() && !data.PrivatePipRepositories.IsUnknown() {
		v := data.PrivatePipRepositories.ValueString()
		env.PrivatePipRepositories = &v
	}
	if !data.AdditionalEnvVars.IsNull() && !data.AdditionalEnvVars.IsUnknown() {
		envVars := make(map[string]string)
		diagnostics.Append(data.AdditionalEnvVars.ElementsAs(ctx, &envVars, false)...)
		if diagnostics.HasError() {
			return nil
		}
		env.AdditionalEnvVars = envVars
	}
	if !data.EnvironmentBuckets.IsNull() && !data.EnvironmentBuckets.IsUnknown() {
		var buckets EnvironmentBucketsModel
		diagnostics.Append(data.EnvironmentBuckets.As(ctx, &buckets, basetypes.ObjectAsOptions{})...)
		if diagnostics.HasError() {
			return nil
		}
		env.EnvironmentBuckets = &serverv1.EnvironmentObjectStorageConfig{
			DatasetBucket:       buckets.DatasetBucket.ValueString(),
			PlanStagesBucket:    buckets.PlanStagesBucket.ValueString(),
			SourceBundleBucket:  buckets.SourceBundleBucket.ValueString(),
			ModelRegistryBucket: buckets.ModelRegistryBucket.ValueString(),
		}
	}
	if !data.SpecsConfigJson.IsNull() && !data.SpecsConfigJson.IsUnknown() {
		var st structpb.Struct
		if err := protojson.Unmarshal([]byte(data.SpecsConfigJson.ValueString()), &st); err != nil {
			diagnostics.AddError("Invalid specs_config_json", err.Error())
			return nil
		}
		env.SpecConfigJson = st.Fields
	}
	if !data.PinnedBaseImage.IsNull() && !data.PinnedBaseImage.IsUnknown() {
		v := data.PinnedBaseImage.ValueString()
		env.PinnedBaseImage = &v
	}
	if !data.DefaultBuildProfile.IsNull() && !data.DefaultBuildProfile.IsUnknown() {
		v := serverv1.DeploymentBuildProfile(serverv1.DeploymentBuildProfile_value[data.DefaultBuildProfile.ValueString()])
		env.DefaultBuildProfile = &v
	}
	if !data.CustomerMetadata.IsNull() && !data.CustomerMetadata.IsUnknown() {
		var st structpb.Struct
		if err := protojson.Unmarshal([]byte(data.CustomerMetadata.ValueString()), &st); err != nil {
			diagnostics.AddError("Invalid customer_metadata", err.Error())
			return nil
		}
		env.CustomerMetadata = st.Fields
	}

	return env
}

// baseUpdateStateFromEnvironment maps the common fields of an Environment proto onto a BaseEnvironmentModel.
// Returns immediately (with a diagnostic error) if e is nil.
func baseUpdateStateFromEnvironment(data *BaseEnvironmentModel, e *serverv1.Environment, diagnostics *diag.Diagnostics) {
	if e == nil {
		diagnostics.AddError("Empty Environment Response", "Server returned a nil environment.")
		return
	}

	data.Id = types.StringValue(e.Id)
	data.Name = types.StringValue(e.Name)
	data.ProjectId = types.StringValue(e.ProjectId)

	data.KubeClusterId = types.StringPointerValue(e.KubeClusterId)
	data.KubeJobNamespace = types.StringPointerValue(e.KubeJobNamespace)
	data.EngineDockerRegistryPath = types.StringPointerValue(e.EngineDockerRegistryPath)
	data.ServiceUrl = types.StringPointerValue(e.ServiceUrl)
	data.OnlineStoreKind = types.StringPointerValue(e.OnlineStoreKind)
	data.OnlineStoreSecret = types.StringPointerValue(e.OnlineStoreSecret)
	data.PrivatePipRepositories = types.StringPointerValue(e.PrivatePipRepositories)
	data.PinnedBaseImage = types.StringPointerValue(e.PinnedBaseImage)
	if e.DefaultBuildProfile != nil && *e.DefaultBuildProfile != serverv1.DeploymentBuildProfile_DEPLOYMENT_BUILD_PROFILE_UNSPECIFIED {
		data.DefaultBuildProfile = types.StringValue(e.DefaultBuildProfile.String())
	} else {
		data.DefaultBuildProfile = types.StringNull()
	}

	if len(e.AdditionalEnvVars) > 0 {
		elements := make(map[string]attr.Value, len(e.AdditionalEnvVars))
		for k, v := range e.AdditionalEnvVars {
			elements[k] = types.StringValue(v)
		}
		data.AdditionalEnvVars = types.MapValueMust(types.StringType, elements)
	} else {
		data.AdditionalEnvVars = types.MapNull(types.StringType)
	}

	data.EnvironmentBuckets = environmentBucketsToTF(e.EnvironmentBuckets)

	if len(e.SpecConfigJson) > 0 {
		st := &structpb.Struct{Fields: e.SpecConfigJson}
		b, err := protojson.Marshal(st)
		if err != nil {
			diagnostics.AddError("Failed to marshal spec_config_json", err.Error())
			return
		}
		data.SpecsConfigJson = jsontypes.NewNormalizedValue(string(b))
	} else {
		data.SpecsConfigJson = jsontypes.NewNormalizedNull()
	}
	if len(e.CustomerMetadata) > 0 {
		st := &structpb.Struct{Fields: e.CustomerMetadata}
		b, err := protojson.Marshal(st)
		if err != nil {
			diagnostics.AddError("Failed to marshal customer_metadata", err.Error())
			return
		}
		data.CustomerMetadata = jsontypes.NewNormalizedValue(string(b))
	} else {
		data.CustomerMetadata = jsontypes.NewNormalizedNull()
	}
}

// populateBaseEnvUpdateProto sets the mutable common fields of an Environment proto from the
// BaseEnvironmentModel plan. Stops and sets a diagnostic error on the first conversion failure.
// The caller must check diagnostics.HasError() after calling this function.
func populateBaseEnvUpdateProto(ctx context.Context, plan *BaseEnvironmentModel, env *serverv1.Environment, diagnostics *diag.Diagnostics) {
	if !plan.ServiceUrl.IsNull() && !plan.ServiceUrl.IsUnknown() {
		v := plan.ServiceUrl.ValueString()
		env.ServiceUrl = &v
	}
	if !plan.OnlineStoreKind.IsNull() {
		v := plan.OnlineStoreKind.ValueString()
		env.OnlineStoreKind = &v
	}
	if !plan.OnlineStoreSecret.IsNull() {
		v := plan.OnlineStoreSecret.ValueString()
		env.OnlineStoreSecret = &v
	}
	if !plan.PrivatePipRepositories.IsNull() {
		v := plan.PrivatePipRepositories.ValueString()
		env.PrivatePipRepositories = &v
	}
	if !plan.AdditionalEnvVars.IsNull() && !plan.AdditionalEnvVars.IsUnknown() {
		envVars := make(map[string]string)
		diagnostics.Append(plan.AdditionalEnvVars.ElementsAs(ctx, &envVars, false)...)
		if diagnostics.HasError() {
			return
		}
		env.AdditionalEnvVars = envVars
	}
	if !plan.EnvironmentBuckets.IsNull() && !plan.EnvironmentBuckets.IsUnknown() {
		var buckets EnvironmentBucketsModel
		diagnostics.Append(plan.EnvironmentBuckets.As(ctx, &buckets, basetypes.ObjectAsOptions{})...)
		if diagnostics.HasError() {
			return
		}
		env.EnvironmentBuckets = &serverv1.EnvironmentObjectStorageConfig{
			DatasetBucket:       buckets.DatasetBucket.ValueString(),
			PlanStagesBucket:    buckets.PlanStagesBucket.ValueString(),
			SourceBundleBucket:  buckets.SourceBundleBucket.ValueString(),
			ModelRegistryBucket: buckets.ModelRegistryBucket.ValueString(),
		}
	}
	if !plan.SpecsConfigJson.IsNull() && !plan.SpecsConfigJson.IsUnknown() {
		var st structpb.Struct
		if err := protojson.Unmarshal([]byte(plan.SpecsConfigJson.ValueString()), &st); err != nil {
			diagnostics.AddError("Invalid specs_config_json", err.Error())
			return
		}
		env.SpecConfigJson = st.Fields
	}
	if !plan.PinnedBaseImage.IsNull() {
		v := plan.PinnedBaseImage.ValueString()
		env.PinnedBaseImage = &v
	}
	if !plan.DefaultBuildProfile.IsNull() && !plan.DefaultBuildProfile.IsUnknown() {
		s := plan.DefaultBuildProfile.ValueString()
		v := serverv1.DeploymentBuildProfile(serverv1.DeploymentBuildProfile_value[s])
		env.DefaultBuildProfile = &v
	}
	if !plan.CustomerMetadata.IsNull() && !plan.CustomerMetadata.IsUnknown() {
		var st structpb.Struct
		if err := protojson.Unmarshal([]byte(plan.CustomerMetadata.ValueString()), &st); err != nil {
			diagnostics.AddError("Invalid customer_metadata", err.Error())
			return
		}
		env.CustomerMetadata = st.Fields
	}
}

// buildBaseEnvUpdateMask compares plan and state BaseEnvironmentModel fields and returns
// field mask paths for mutable fields that changed.
func buildBaseEnvUpdateMask(plan, state *BaseEnvironmentModel) []string {
	var paths []string
	if !plan.ServiceUrl.IsUnknown() && !plan.ServiceUrl.Equal(state.ServiceUrl) {
		paths = append(paths, "service_url")
	}
	if !plan.OnlineStoreKind.IsUnknown() && !plan.OnlineStoreKind.Equal(state.OnlineStoreKind) {
		paths = append(paths, "online_store_kind")
	}
	if !plan.OnlineStoreSecret.IsUnknown() && !plan.OnlineStoreSecret.Equal(state.OnlineStoreSecret) {
		paths = append(paths, "online_store_secret")
	}
	if !plan.PrivatePipRepositories.IsUnknown() && !plan.PrivatePipRepositories.Equal(state.PrivatePipRepositories) {
		paths = append(paths, "private_pip_repositories")
	}
	if !plan.AdditionalEnvVars.IsUnknown() && !plan.AdditionalEnvVars.Equal(state.AdditionalEnvVars) {
		paths = append(paths, "additional_env_vars")
	}
	if !plan.EnvironmentBuckets.IsUnknown() && !plan.EnvironmentBuckets.Equal(state.EnvironmentBuckets) {
		paths = append(paths, "environment_buckets")
	}
	if !plan.SpecsConfigJson.IsUnknown() && !plan.SpecsConfigJson.Equal(state.SpecsConfigJson) {
		paths = append(paths, "spec_config_json")
	}
	if !plan.PinnedBaseImage.IsUnknown() && !plan.PinnedBaseImage.Equal(state.PinnedBaseImage) {
		paths = append(paths, "pinned_base_image")
	}
	if !plan.DefaultBuildProfile.IsUnknown() && !plan.DefaultBuildProfile.Equal(state.DefaultBuildProfile) {
		paths = append(paths, "default_build_profile")
	}
	if !plan.CustomerMetadata.IsUnknown() && !plan.CustomerMetadata.Equal(state.CustomerMetadata) {
		paths = append(paths, "customer_metadata")
	}
	return paths
}
