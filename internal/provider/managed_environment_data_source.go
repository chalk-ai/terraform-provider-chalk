package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/client"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

var _ datasource.DataSource = &ManagedEnvironmentDataSource{}

func NewManagedEnvironmentDataSource() datasource.DataSource {
	return &ManagedEnvironmentDataSource{}
}

type ManagedEnvironmentDataSource struct {
	client *client.Manager
}

// ManagedEnvironmentDataSourceModel is the secret-free read view of a
// managed environment. Compared to ManagedEnvironmentResourceModel it omits
// online_store_secret deliberately.
type ManagedEnvironmentDataSourceModel struct {
	Id                       types.String         `tfsdk:"id"`
	Name                     types.String         `tfsdk:"name"`
	ProjectId                types.String         `tfsdk:"project_id"`
	KubeClusterId            types.String         `tfsdk:"kube_cluster_id"`
	KubeJobNamespace         types.String         `tfsdk:"kube_job_namespace"`
	EngineDockerRegistryPath types.String         `tfsdk:"engine_docker_registry_path"`
	ServiceUrl               types.String         `tfsdk:"service_url"`
	OnlineStoreKind          types.String         `tfsdk:"online_store_kind"`
	AdditionalEnvVars        types.Map            `tfsdk:"additional_env_vars"`
	EnvironmentBuckets       types.Object         `tfsdk:"environment_buckets"`
	SpecsConfigJson          jsontypes.Normalized `tfsdk:"specs_config_json"`
	PrivatePipRepositories   types.String         `tfsdk:"private_pip_repositories"`
	PinnedBaseImage          types.String         `tfsdk:"pinned_base_image"`
	DefaultBuildProfile      types.String         `tfsdk:"default_build_profile"`
	CustomerMetadata         jsontypes.Normalized `tfsdk:"customer_metadata"`
}

func (d *ManagedEnvironmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_environment"
}

func (d *ManagedEnvironmentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Chalk managed environment by ID. Surfaces project_id, kube_cluster_id, service URL, build/runtime settings, and storage buckets. **Does not** surface `online_store_secret` (a deliberate design choice for new data sources).",
		Attributes: map[string]schema.Attribute{
			"id":                          schema.StringAttribute{MarkdownDescription: "Environment identifier.", Required: true},
			"name":                        schema.StringAttribute{Computed: true},
			"project_id":                  schema.StringAttribute{Computed: true},
			"kube_cluster_id":             schema.StringAttribute{Computed: true},
			"kube_job_namespace":          schema.StringAttribute{Computed: true},
			"engine_docker_registry_path": schema.StringAttribute{Computed: true},
			"service_url":                 schema.StringAttribute{Computed: true},
			"online_store_kind":           schema.StringAttribute{Computed: true},
			"additional_env_vars":         schema.MapAttribute{Computed: true, ElementType: types.StringType},
			"environment_buckets": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"dataset_bucket":        schema.StringAttribute{Computed: true},
					"plan_stages_bucket":    schema.StringAttribute{Computed: true},
					"source_bundle_bucket":  schema.StringAttribute{Computed: true},
					"model_registry_bucket": schema.StringAttribute{Computed: true},
				},
			},
			"specs_config_json":        schema.StringAttribute{Computed: true, CustomType: jsontypes.NormalizedType{}},
			"private_pip_repositories": schema.StringAttribute{Computed: true},
			"pinned_base_image":        schema.StringAttribute{Computed: true},
			"default_build_profile":    schema.StringAttribute{Computed: true},
			"customer_metadata":        schema.StringAttribute{Computed: true, CustomType: jsontypes.NormalizedType{}},
		},
	}
}

func (d *ManagedEnvironmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Manager)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Manager, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *ManagedEnvironmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ManagedEnvironmentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "read chalk_managed_environment data source", map[string]any{"id": data.Id.ValueString()})

	tc := d.client.NewTeamClient(ctx, data.Id.ValueString())
	envResp, err := tc.GetEnv(ctx, connect.NewRequest(&serverv1.GetEnvRequest{}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Managed Environment",
			fmt.Sprintf("Could not read managed environment %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	e := envResp.Msg.Environment
	if e == nil {
		resp.Diagnostics.AddError("Empty Environment Response", "Server returned a nil environment.")
		return
	}
	if e.Managed != nil && !*e.Managed {
		resp.Diagnostics.AddError(
			"Environment Type Mismatch",
			fmt.Sprintf("Environment %s is not a managed environment; use chalk_unmanaged_environment instead.", data.Id.ValueString()),
		)
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
	data.PrivatePipRepositories = types.StringPointerValue(e.PrivatePipRepositories)
	data.PinnedBaseImage = types.StringPointerValue(e.PinnedBaseImage)
	if e.DefaultBuildProfile != nil && *e.DefaultBuildProfile != serverv1.DeploymentBuildProfile_DEPLOYMENT_BUILD_PROFILE_UNSPECIFIED {
		data.DefaultBuildProfile = types.StringValue(e.DefaultBuildProfile.String())
	} else {
		data.DefaultBuildProfile = types.StringNull()
	}

	if len(e.AdditionalEnvVars) > 0 {
		elems := make(map[string]attr.Value, len(e.AdditionalEnvVars))
		for k, v := range e.AdditionalEnvVars {
			elems[k] = types.StringValue(v)
		}
		data.AdditionalEnvVars = types.MapValueMust(types.StringType, elems)
	} else {
		data.AdditionalEnvVars = types.MapNull(types.StringType)
	}

	data.EnvironmentBuckets = environmentBucketsToTF(e.EnvironmentBuckets)

	if len(e.SpecConfigJson) > 0 {
		st := &structpb.Struct{Fields: e.SpecConfigJson}
		b, err := protojson.Marshal(st)
		if err != nil {
			resp.Diagnostics.AddError("Failed to marshal spec_config_json", err.Error())
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
			resp.Diagnostics.AddError("Failed to marshal customer_metadata", err.Error())
			return
		}
		data.CustomerMetadata = jsontypes.NewNormalizedValue(string(b))
	} else {
		data.CustomerMetadata = jsontypes.NewNormalizedNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
