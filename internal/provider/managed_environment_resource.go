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
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

var _ resource.Resource = &ManagedEnvironmentResource{}
var _ resource.ResourceWithImportState = &ManagedEnvironmentResource{}

func NewManagedEnvironmentResource() resource.Resource {
	return &ManagedEnvironmentResource{}
}

type ManagedEnvironmentResource struct {
	client *ClientManager
}

type ManagedEnvironmentResourceModel struct {
	BaseEnvironmentModel
}

func (r *ManagedEnvironmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_environment"
}

func (r *ManagedEnvironmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk managed environment resource",
		Attributes: commonEnvironmentSchemaAttributes(schema.StringAttribute{
			MarkdownDescription: "Kubernetes job namespace (immutable; auto-assigned by server if not provided)",
			Optional:            true,
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
				stringplanmodifier.RequiresReplaceIfConfigured(),
			},
		}),
	}
}

func (r *ManagedEnvironmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ManagedEnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ManagedEnvironmentResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env := baseEnvToProto(ctx, &data.BaseEnvironmentModel, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	managed := true
	env.Managed = &managed

	ec := r.client.NewEnvironmentServiceClient(ctx)
	createResp, err := ec.CreateEnvironmentV2(ctx, connect.NewRequest(&serverv1.CreateEnvironmentV2Request{
		Environment: env,
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Managed Environment",
			fmt.Sprintf("Could not create environment: %v", err),
		)
		return
	}

	baseUpdateStateFromEnvironment(&data.BaseEnvironmentModel, createResp.Msg.Environment, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "created a chalk_managed_environment resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ManagedEnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ManagedEnvironmentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readManagedEnv(ctx, r.client, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ManagedEnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state ManagedEnvironmentResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env := &serverv1.Environment{Id: state.Id.ValueString()}
	maskPaths := buildBaseEnvUpdateMask(&plan.BaseEnvironmentModel, &state.BaseEnvironmentModel)

	if len(maskPaths) > 0 {
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
				"Error Updating Chalk Managed Environment",
				fmt.Sprintf("Could not update environment: %v", err),
			)
			return
		}

		baseUpdateStateFromEnvironment(&plan.BaseEnvironmentModel, updateResp.Msg.Environment, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	tflog.Trace(ctx, "updated a chalk_managed_environment resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ManagedEnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ManagedEnvironmentResourceModel

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
			"Error Deleting Chalk Managed Environment",
			fmt.Sprintf("Could not delete environment %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "deleted chalk_managed_environment resource")
}

func (r *ManagedEnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// readManagedEnv fetches environment state via TeamService.GetEnv and populates data.
// Used by the Read handler only — Create and Update parse the RPC response directly.
func readManagedEnv(ctx context.Context, client *ClientManager, data *ManagedEnvironmentResourceModel, diagnostics *diag.Diagnostics) {
	tc := client.NewTeamClient(ctx, data.Id.ValueString())
	envResp, err := tc.GetEnv(ctx, connect.NewRequest(&serverv1.GetEnvRequest{}))
	if err != nil {
		diagnostics.AddError(
			"Error Reading Chalk Managed Environment",
			fmt.Sprintf("Could not read environment %s: %v", data.Id.ValueString(), err),
		)
		return
	}
	e := envResp.Msg.Environment
	if e != nil && e.Managed != nil && !*e.Managed {
		diagnostics.AddError(
			"Environment Type Mismatch",
			fmt.Sprintf("Environment %s is not a managed environment; use chalk_unmanaged_environment instead.", data.Id.ValueString()),
		)
		return
	}
	baseUpdateStateFromEnvironment(&data.BaseEnvironmentModel, e, diagnostics)
}
