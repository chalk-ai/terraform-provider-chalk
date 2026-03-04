package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

var _ resource.Resource = &UnmanagedEnvironmentResource{}
var _ resource.ResourceWithImportState = &UnmanagedEnvironmentResource{}

func NewUnmanagedEnvironmentResource() resource.Resource {
	return &UnmanagedEnvironmentResource{}
}

type UnmanagedEnvironmentResource struct {
	client *ClientManager
}

type UnmanagedEnvironmentResourceModel struct {
	BaseEnvironmentModel
	ServiceUrl             types.String `tfsdk:"service_url"`
	KubeServiceAccountName types.String `tfsdk:"kube_service_account_name"`
	KubeClusterMode        types.String `tfsdk:"kube_cluster_mode"`
}

func (r *UnmanagedEnvironmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_unmanaged_environment"
}

func (r *UnmanagedEnvironmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := commonEnvironmentSchemaAttributes(schema.StringAttribute{
		MarkdownDescription: "Kubernetes job namespace (immutable, required for unmanaged environments)",
		Required:            true,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	})
	attrs["service_url"] = schema.StringAttribute{
		MarkdownDescription: "Service URL",
		Optional:            true,
	}
	attrs["kube_service_account_name"] = schema.StringAttribute{
		MarkdownDescription: "Kubernetes service account name",
		Optional:            true,
	}
	attrs["kube_cluster_mode"] = schema.StringAttribute{
		MarkdownDescription: "Kubernetes cluster mode (unmanaged environments only)",
		Optional:            true,
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk unmanaged environment resource",
		Attributes:          attrs,
	}
}

func (r *UnmanagedEnvironmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *UnmanagedEnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UnmanagedEnvironmentResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env := unmanagedEnvToProto(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	ec := r.client.NewEnvironmentServiceClient(ctx)
	createResp, err := ec.CreateEnvironmentV2(ctx, connect.NewRequest(&serverv1.CreateEnvironmentV2Request{
		Environment: env,
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Unmanaged Environment",
			fmt.Sprintf("Could not create environment: %v", err),
		)
		return
	}

	updateStateFromEnvironment(&data, createResp.Msg.Environment, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "created a chalk_unmanaged_environment resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UnmanagedEnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UnmanagedEnvironmentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readUnmanagedEnv(ctx, r.client, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UnmanagedEnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state UnmanagedEnvironmentResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env := &serverv1.Environment{Id: state.Id.ValueString()}
	maskPaths := buildUnmanagedEnvUpdateMask(&plan, &state)

	if len(maskPaths) > 0 {
		if !plan.ServiceUrl.IsNull() {
			v := plan.ServiceUrl.ValueString()
			env.ServiceUrl = &v
		}
		if !plan.KubeServiceAccountName.IsNull() {
			v := plan.KubeServiceAccountName.ValueString()
			env.KubeServiceAccountName = &v
		}
		if !plan.KubeClusterMode.IsNull() {
			v := plan.KubeClusterMode.ValueString()
			env.KubeClusterMode = &v
		}
		populateBaseEnvUpdateProto(ctx, &plan.BaseEnvironmentModel, env, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}

		ec := r.client.NewEnvironmentServiceClient(ctx, state.Id.ValueString())
		updateResp, err := ec.UpdateEnvironmentV2(ctx, connect.NewRequest(&serverv1.UpdateEnvironmentV2Request{
			Environment: env,
			UpdateMask:  &fieldmaskpb.FieldMask{Paths: maskPaths},
		}))
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Chalk Unmanaged Environment",
				fmt.Sprintf("Could not update environment: %v", err),
			)
			return
		}

		updateStateFromEnvironment(&plan, updateResp.Msg.Environment, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	tflog.Trace(ctx, "updated a chalk_unmanaged_environment resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UnmanagedEnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UnmanagedEnvironmentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ec := r.client.NewEnvironmentServiceClient(ctx)
	_, err := ec.DeleteEnvironment(ctx, connect.NewRequest(&serverv1.DeleteEnvironmentRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Chalk Unmanaged Environment",
			fmt.Sprintf("Could not delete environment %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "deleted chalk_unmanaged_environment resource")
}

func (r *UnmanagedEnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// buildUnmanagedEnvUpdateMask compares plan and state to determine which fields changed.
func buildUnmanagedEnvUpdateMask(plan, state *UnmanagedEnvironmentResourceModel) []string {
	paths := buildBaseEnvUpdateMask(&plan.BaseEnvironmentModel, &state.BaseEnvironmentModel)
	if !plan.ServiceUrl.IsUnknown() && !plan.ServiceUrl.Equal(state.ServiceUrl) {
		paths = append(paths, "service_url")
	}
	if !plan.KubeServiceAccountName.IsUnknown() && !plan.KubeServiceAccountName.Equal(state.KubeServiceAccountName) {
		paths = append(paths, "kube_service_account_name")
	}
	if !plan.KubeClusterMode.IsUnknown() && !plan.KubeClusterMode.Equal(state.KubeClusterMode) {
		paths = append(paths, "kube_cluster_mode")
	}
	return paths
}

// unmanagedEnvToProto builds an Environment proto from the Terraform model for Create.
func unmanagedEnvToProto(ctx context.Context, data *UnmanagedEnvironmentResourceModel, diagnostics *diag.Diagnostics) *serverv1.Environment {
	env := baseEnvToProto(ctx, &data.BaseEnvironmentModel, diagnostics)
	if env == nil {
		return nil
	}
	if !data.ServiceUrl.IsNull() && !data.ServiceUrl.IsUnknown() {
		v := data.ServiceUrl.ValueString()
		env.ServiceUrl = &v
	}
	if !data.KubeServiceAccountName.IsNull() && !data.KubeServiceAccountName.IsUnknown() {
		v := data.KubeServiceAccountName.ValueString()
		env.KubeServiceAccountName = &v
	}
	if !data.KubeClusterMode.IsNull() && !data.KubeClusterMode.IsUnknown() {
		v := data.KubeClusterMode.ValueString()
		env.KubeClusterMode = &v
	}
	return env
}

// readUnmanagedEnv fetches environment state via TeamService.GetEnv and populates data.
// Used by the Read handler only — Create and Update parse the RPC response directly.
func readUnmanagedEnv(ctx context.Context, client *ClientManager, data *UnmanagedEnvironmentResourceModel, diagnostics *diag.Diagnostics) {
	tc := client.NewTeamClient(ctx, data.Id.ValueString())
	envResp, err := tc.GetEnv(ctx, connect.NewRequest(&serverv1.GetEnvRequest{}))
	if err != nil {
		diagnostics.AddError(
			"Error Reading Chalk Unmanaged Environment",
			fmt.Sprintf("Could not read environment %s: %v", data.Id.ValueString(), err),
		)
		return
	}
	e := envResp.Msg.Environment
	if e != nil && e.Managed != nil && *e.Managed {
		diagnostics.AddError(
			"Environment Type Mismatch",
			fmt.Sprintf("Environment %s is a managed environment; use chalk_managed_environment instead.", data.Id.ValueString()),
		)
		return
	}
	updateStateFromEnvironment(data, e, diagnostics)
}

// updateStateFromEnvironment maps an Environment proto onto an UnmanagedEnvironmentResourceModel.
func updateStateFromEnvironment(data *UnmanagedEnvironmentResourceModel, e *serverv1.Environment, diagnostics *diag.Diagnostics) {
	baseUpdateStateFromEnvironment(&data.BaseEnvironmentModel, e, diagnostics)
	if diagnostics.HasError() {
		return
	}
	data.ServiceUrl = types.StringPointerValue(e.ServiceUrl)
	data.KubeServiceAccountName = types.StringPointerValue(e.KubeServiceAccountName)
	data.KubeClusterMode = types.StringPointerValue(e.KubeClusterMode)
}
