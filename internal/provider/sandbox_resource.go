package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	containerv1 "github.com/chalk-ai/chalk-go/gen/chalk/container/v1"
	sandboxv2 "github.com/chalk-ai/chalk-go/gen/chalk/sandbox/v2"
	"github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_ resource.Resource                = &SandboxResource{}
	_ resource.ResourceWithImportState = &SandboxResource{}
)

// sandboxPollInterval is how often Create polls while waiting for a new sandbox
// to leave its transitional state, and sandboxReadyTimeout is how long it waits
// before giving up. Both are vars so tests can shorten them.
var (
	sandboxPollInterval = 3 * time.Second
	sandboxReadyTimeout = 15 * time.Minute
)

// sandboxReadyStates are the normalized v2 statuses in which the backing
// container can accept exec requests.
var sandboxReadyStates = map[string]bool{
	"running": true,
}

var sandboxSuspendedStates = map[string]bool{
	"suspended": true,
}

// sandboxFailedStates are the states from which a sandbox will never become
// ready, so waiting on one only burns the timeout.
var sandboxFailedStates = map[string]bool{
	"failed":     true,
	"succeeded":  true,
	"terminated": true,
	"error":      true,
}

func NewSandboxResource() resource.Resource {
	return &SandboxResource{}
}

type SandboxResource struct {
	client *client.Manager
}

type SandboxResourceModel struct {
	Id             types.String `tfsdk:"id"`
	EnvironmentId  types.String `tfsdk:"environment_id"`
	Name           types.String `tfsdk:"name"`
	Image          types.String `tfsdk:"image"`
	Entrypoint     types.List   `tfsdk:"entrypoint"`
	Env            types.Map    `tfsdk:"env"`
	Runtime        types.String `tfsdk:"runtime"`
	RestartPolicy  types.String `tfsdk:"restart_policy"`
	ResourceLimits types.Object `tfsdk:"resource_limits"`
	Volumes        types.List   `tfsdk:"volumes"`
	NetworkPolicy  types.Object `tfsdk:"network_policy"`
	WaitForReady   types.Bool   `tfsdk:"wait_for_ready"`
	DesiredState   types.String `tfsdk:"desired_state"`

	State         types.String `tfsdk:"state"`
	StatusMessage types.String `tfsdk:"status_message"`
	CreatedAt     types.String `tfsdk:"created_at"`
	BuildId       types.String `tfsdk:"build_id"`
}

type SandboxResourceLimitsModel struct {
	CPU    types.String `tfsdk:"cpu"`
	Memory types.String `tfsdk:"memory"`
}

type SandboxVolumeMountModel struct {
	Name      types.String `tfsdk:"name"`
	MountPath types.String `tfsdk:"mount_path"`
	Type      types.String `tfsdk:"type"`
	SizeLimit types.String `tfsdk:"size_limit"`
	VersionId types.Int64  `tfsdk:"version_id"`
}

type SandboxNetworkPolicyModel struct {
	AllowedRoutes types.List `tfsdk:"allowed_routes"`
	DeniedRoutes  types.List `tfsdk:"denied_routes"`
}

type SandboxAllowedRouteModel struct {
	Route      types.String `tfsdk:"route"`
	PortRanges types.List   `tfsdk:"port_ranges"`
}

type SandboxPortRangeModel struct {
	StartPort types.Int64 `tfsdk:"start_port"`
	EndPort   types.Int64 `tfsdk:"end_port"`
}

func (r *SandboxResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sandbox"
}

func (r *SandboxResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	requiresReplaceString := []planmodifier.String{stringplanmodifier.RequiresReplace()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Chalk sandbox — an ephemeral container running in the " +
			"environment's cluster, addressable for command execution via `chalk sandbox exec`.\n\n" +
			"Only `desired_state` can be updated in place; every other argument forces replacement.\n\n" +
			"A sandbox has **no outbound network access by default**: without a `network_policy` " +
			"even DNS resolution fails. Grant egress explicitly via `network_policy.allowed_routes`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The sandbox id.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The environment ID that this sandbox runs in.",
				Required:            true,
				PlanModifiers:       requiresReplaceString,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Optional human-readable name for the sandbox.",
				Optional:            true,
				PlanModifiers:       requiresReplaceString,
			},
			"image": schema.StringAttribute{
				MarkdownDescription: "Pre-built container image to run, e.g. `debian:bookworm`.",
				Required:            true,
				PlanModifiers:       requiresReplaceString,
			},
			"entrypoint": schema.ListAttribute{
				MarkdownDescription: "Overrides the image's entrypoint. When unset, the image's own entrypoint is used. " +
					"`exec` into the sandbox keeps working either way — it does not go through the entrypoint process.",
				Optional:      true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
			"env": schema.MapAttribute{
				MarkdownDescription: "Environment variables to inject into the sandbox.",
				Optional:            true,
				Sensitive:           true,
				ElementType:         types.StringType,
				PlanModifiers:       []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			},
			"runtime": schema.StringAttribute{
				MarkdownDescription: "Runtime backend for the sandbox container: `kube` (default), `chalk_node`, or `local`.",
				Optional:            true,
				PlanModifiers:       requiresReplaceString,
			},
			"restart_policy": schema.StringAttribute{
				MarkdownDescription: "Whether to recreate the sandbox after its backing instance terminates: " +
					"`RESTART_POLICY_NEVER` (default) or `RESTART_POLICY_ALWAYS`.",
				Optional:      true,
				PlanModifiers: requiresReplaceString,
			},
			"resource_limits": schema.SingleNestedAttribute{
				MarkdownDescription: "CPU and memory limits for the sandbox container.",
				Optional:            true,
				PlanModifiers:       []planmodifier.Object{objectplanmodifier.RequiresReplace()},
				Attributes: map[string]schema.Attribute{
					"cpu": schema.StringAttribute{
						MarkdownDescription: "CPU limit, e.g. `2` or `500m`.",
						Optional:            true,
					},
					"memory": schema.StringAttribute{
						MarkdownDescription: "Memory limit, e.g. `4Gi` or `512Mi`.",
						Optional:            true,
					},
				},
			},
			"volumes": schema.ListNestedAttribute{
				MarkdownDescription: "Volumes to mount into the sandbox.",
				Optional:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Name of the volume.",
							Required:            true,
						},
						"mount_path": schema.StringAttribute{
							MarkdownDescription: "Path inside the sandbox to mount the volume at.",
							Required:            true,
						},
						"type": schema.StringAttribute{
							MarkdownDescription: "Volume type: `empty_dir`, `shared_memory`, `chalkfs`, or `versioned_chalkfs`.",
							Required:            true,
						},
						"size_limit": schema.StringAttribute{
							MarkdownDescription: "Size limit for the volume, e.g. `2Gi`. Optional for `empty_dir`.",
							Optional:            true,
						},
						"version_id": schema.Int64Attribute{
							MarkdownDescription: "Pins a `versioned_chalkfs` mount to an immutable committed snapshot.",
							Optional:            true,
						},
					},
				},
			},
			"network_policy": schema.SingleNestedAttribute{
				MarkdownDescription: "Outbound network policy. **Without this, the sandbox cannot reach anything — " +
					"not even DNS.** Grant full egress with a single `allowed_routes` entry for `0.0.0.0/0`.",
				Optional:      true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.RequiresReplace()},
				Attributes: map[string]schema.Attribute{
					"allowed_routes": schema.ListNestedAttribute{
						MarkdownDescription: "Allowlist of destination CIDRs and the ports permitted on each.",
						Optional:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"route": schema.StringAttribute{
									MarkdownDescription: "Destination CIDR, e.g. `10.0.0.0/8`.",
									Required:            true,
								},
								"port_ranges": schema.ListNestedAttribute{
									MarkdownDescription: "Destination ports allowed for this route. Omit to allow all ports.",
									Optional:            true,
									NestedObject: schema.NestedAttributeObject{
										Attributes: map[string]schema.Attribute{
											"start_port": schema.Int64Attribute{
												MarkdownDescription: "Inclusive start port.",
												Required:            true,
											},
											"end_port": schema.Int64Attribute{
												MarkdownDescription: "Inclusive end port, at least `start_port`.",
												Required:            true,
											},
										},
									},
								},
							},
						},
					},
					"denied_routes": schema.ListAttribute{
						MarkdownDescription: "Denylist of destination CIDRs. Takes precedence over `allowed_routes`.",
						Optional:            true,
						ElementType:         types.StringType,
					},
				},
			},
			"wait_for_ready": schema.BoolAttribute{
				MarkdownDescription: "Whether to block until the sandbox reports a ready state before completing the " +
					"apply. Defaults to `true`; set to `false` to return as soon as the create call is accepted.",
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"desired_state": schema.StringAttribute{
				MarkdownDescription: "Desired lifecycle state: `running` (default) or `suspended`. Changing this value suspends or restores the sandbox in place.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("running"),
				Validators:          []validator.String{stringvalidator.OneOf("running", "suspended")},
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "Current sandbox state, e.g. `running`, `pending`, or `suspended`.",
				Computed:            true,
			},
			"status_message": schema.StringAttribute{
				MarkdownDescription: "Additional detail about the current state.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "When the sandbox was created.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"build_id": schema.StringAttribute{
				MarkdownDescription: "Build ID backing this sandbox, when one was needed.",
				Computed:            true,
			},
		},
	}
}

func (r *SandboxResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Manager)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Manager, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = c
}

func (r *SandboxResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SandboxResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq, diags := buildCreateSandboxRequest(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	sbClient := r.client.NewSandboxV2Client(ctx, data.EnvironmentId.ValueString())
	createResp, err := sbClient.CreateSandbox(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError("Error creating sandbox", fmt.Sprintf("Could not create sandbox: %s", err))
		return
	}

	info := createResp.Msg.GetSandbox()
	if info == nil {
		resp.Diagnostics.AddError("Error creating sandbox", "The server accepted the sandbox but returned no sandbox info.")
		return
	}
	updateSandboxState(&data, info)

	// Persist the id before waiting: if the wait times out, the sandbox still
	// exists server-side and Terraform must know about it to clean it up.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if (data.WaitForReady.ValueBool() || data.DesiredState.ValueString() == "suspended") && !sandboxReadyStates[sandboxStatusString(info.GetStatus())] {
		ready, err := waitForSandboxReady(ctx, sbClient, info.GetId())
		if err != nil {
			resp.Diagnostics.AddError("Error waiting for sandbox", fmt.Sprintf("Sandbox %s did not become ready: %s", info.GetId(), err))
			return
		}
		updateSandboxState(&data, ready)
	}

	if data.DesiredState.ValueString() == "suspended" {
		if err := r.suspendSandbox(ctx, data.EnvironmentId.ValueString(), info.GetId()); err != nil {
			resp.Diagnostics.AddError("Error suspending sandbox", fmt.Sprintf("Could not suspend sandbox %s: %s", info.GetId(), err))
			return
		}
		suspended, err := waitForSandboxState(ctx, sbClient, info.GetId(), sandboxSuspendedStates, "suspended")
		if err != nil {
			resp.Diagnostics.AddError("Error waiting for sandbox", fmt.Sprintf("Sandbox %s did not become suspended: %s", info.GetId(), err))
			return
		}
		updateSandboxState(&data, suspended)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SandboxResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SandboxResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sbClient := r.client.NewSandboxV2Client(ctx, data.EnvironmentId.ValueString())
	getResp, err := sbClient.GetSandbox(ctx, connect.NewRequest(&sandboxv2.GetSandboxRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading sandbox", fmt.Sprintf("Could not read sandbox %s: %s", data.Id.ValueString(), err))
		return
	}

	info := getResp.Msg.GetSandbox()
	// A terminated sandbox is gone for practical purposes: it can no longer be
	// exec'd into, so treat it the same as a deleted resource and let Terraform
	// plan a replacement rather than reporting it as healthy.
	if info == nil || sandboxFailedStates[sandboxStatusString(info.GetStatus())] {
		resp.State.RemoveResource(ctx)
		return
	}

	updateSandboxState(&data, info)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SandboxResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, state SandboxResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.Id.ValueString()
	desired := data.DesiredState.ValueString()
	if desired == state.DesiredState.ValueString() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	var err error
	if desired == "suspended" {
		err = r.suspendSandbox(ctx, state.EnvironmentId.ValueString(), id)
	} else {
		err = r.restoreSandbox(ctx, state.EnvironmentId.ValueString(), id)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error updating sandbox state", fmt.Sprintf("Could not set sandbox %s to %s: %s", id, desired, err))
		return
	}

	targetStates := sandboxReadyStates
	if desired == "suspended" {
		targetStates = sandboxSuspendedStates
	}
	sbClient := r.client.NewSandboxV2Client(ctx, state.EnvironmentId.ValueString())
	info, err := waitForSandboxState(ctx, sbClient, id, targetStates, desired)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for sandbox", fmt.Sprintf("Sandbox %s did not become %s: %s", id, desired, err))
		return
	}
	updateSandboxState(&data, info)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SandboxResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SandboxResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.deleteSandbox(ctx, data.EnvironmentId.ValueString(), data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting sandbox", err.Error())
	}
}

func (r *SandboxResource) deleteSandbox(ctx context.Context, environmentID, id string) error {
	sbClient := r.client.NewSandboxV2Client(ctx, environmentID)
	getResp, err := sbClient.GetSandbox(ctx, connect.NewRequest(&sandboxv2.GetSandboxRequest{Id: id}))
	alreadyGone := false
	lastState := ""
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			alreadyGone = true
		} else {
			return fmt.Errorf("could not read sandbox %s before deletion: %w", id, err)
		}
	} else {
		info := getResp.Msg.GetSandbox()
		if info == nil {
			return fmt.Errorf("could not read sandbox %s before deletion: the server returned no sandbox info", id)
		}
		lastState = sandboxStatusString(info.GetStatus())
		if sandboxSuspendedStates[lastState] {
			if err := r.restoreSandbox(ctx, environmentID, id); err != nil {
				return fmt.Errorf("could not restore suspended sandbox %s before deletion: %w", id, err)
			}
			if _, err := waitForSandboxState(ctx, sbClient, id, sandboxReadyStates, "running"); err != nil {
				return fmt.Errorf("sandbox %s did not become running before deletion: %w", id, err)
			}
		}
	}

	if err := r.deleteSandboxSnapshots(ctx, environmentID, id); err != nil {
		return fmt.Errorf("could not delete snapshots for sandbox %s: %w", id, err)
	}
	if alreadyGone {
		return nil
	}

	_, err = sbClient.TerminateSandbox(ctx, connect.NewRequest(&sandboxv2.TerminateSandboxRequest{
		Id: id,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound && lastState == "failed" {
			return nil
		}
		return fmt.Errorf("could not terminate sandbox %s: %w", id, err)
	}
	return nil
}

func (r *SandboxResource) suspendSandbox(ctx context.Context, environmentID, id string) error {
	_, err := r.client.NewSandboxV2Client(ctx, environmentID).SuspendSandbox(ctx, connect.NewRequest(&sandboxv2.SuspendSandboxRequest{Id: id}))
	return err
}

func (r *SandboxResource) restoreSandbox(ctx context.Context, environmentID, id string) error {
	_, err := r.client.NewSandboxV2Client(ctx, environmentID).ResumeSandbox(ctx, connect.NewRequest(&sandboxv2.ResumeSandboxRequest{Id: id}))
	return err
}

func (r *SandboxResource) deleteSandboxSnapshots(ctx context.Context, environmentID, sandboxID string) error {
	snapshotClient := r.client.NewSandboxSnapshotClient(ctx, environmentID)
	var cursor *string
	for {
		listResp, err := snapshotClient.ListSandboxSnapshots(ctx, connect.NewRequest(&sandboxv2.ListSandboxSnapshotsRequest{
			Cursor: cursor,
			Filters: &sandboxv2.ListSandboxSnapshotsFilters{
				SandboxIds: []string{sandboxID},
			},
		}))
		if err != nil {
			return fmt.Errorf("list snapshots: %w", err)
		}
		for _, snapshot := range listResp.Msg.GetSnapshots() {
			if snapshot.GetSandboxId() != sandboxID {
				continue
			}
			if _, err := snapshotClient.DeleteSandboxSnapshot(ctx, connect.NewRequest(&sandboxv2.DeleteSandboxSnapshotRequest{
				SnapshotId: snapshot.GetId(),
			})); err != nil {
				return fmt.Errorf("delete snapshot %s: %w", snapshot.GetId(), err)
			}
		}
		nextCursor := listResp.Msg.GetNextCursor()
		if nextCursor == "" {
			return nil
		}
		cursor = &nextCursor
	}
}

func (r *SandboxResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import ID in the format 'environment_id/sandbox_id', got: %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// waitForSandboxReady polls GetSandbox until the sandbox reports a ready state,
// reports a state it can never leave, or the timeout expires.
func waitForSandboxReady(
	ctx context.Context,
	sbClient interface {
		GetSandbox(context.Context, *connect.Request[sandboxv2.GetSandboxRequest]) (*connect.Response[sandboxv2.GetSandboxResponse], error)
	},
	id string,
) (*sandboxv2.SandboxInfo, error) {
	return waitForSandboxState(ctx, sbClient, id, sandboxReadyStates, "ready")
}

func waitForSandboxState(
	ctx context.Context,
	sbClient interface {
		GetSandbox(context.Context, *connect.Request[sandboxv2.GetSandboxRequest]) (*connect.Response[sandboxv2.GetSandboxResponse], error)
	},
	id string,
	targetStates map[string]bool,
	targetDescription string,
) (*sandboxv2.SandboxInfo, error) {
	deadline := time.Now().Add(sandboxReadyTimeout)
	ticker := time.NewTicker(sandboxPollInterval)
	defer ticker.Stop()

	var last *sandboxv2.SandboxInfo
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}

		getResp, err := sbClient.GetSandbox(ctx, connect.NewRequest(&sandboxv2.GetSandboxRequest{Id: id}))
		if err != nil {
			return nil, err
		}
		last = getResp.Msg.GetSandbox()
		state := sandboxStatusString(last.GetStatus())
		if targetStates[state] {
			return last, nil
		}
		if sandboxFailedStates[state] {
			return nil, fmt.Errorf("sandbox entered state %q: %s", state, last.GetStatusMessage())
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out after %s waiting for sandbox to become %s (last state %q: %s)",
				sandboxReadyTimeout, targetDescription, state, last.GetStatusMessage())
		}
	}
}

func buildCreateSandboxRequest(ctx context.Context, data *SandboxResourceModel) (*sandboxv2.CreateSandboxRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	spec := &containerv1.ChalkContainerSpec{Image: data.Image.ValueString()}
	req := &sandboxv2.CreateSandboxRequest{
		Source: &sandboxv2.CreateSandboxRequest_Spec{Spec: spec},
	}

	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		spec.Name = data.Name.ValueString()
	}

	if !data.Runtime.IsNull() && !data.Runtime.IsUnknown() {
		var computeClass containerv1.ComputeClass
		switch data.Runtime.ValueString() {
		case "kube":
			computeClass = containerv1.ComputeClass_COMPUTE_CLASS_K8S
		case "chalk_node", "local":
			computeClass = containerv1.ComputeClass_COMPUTE_CLASS_HOST
		default:
			diags.AddError(
				"Invalid runtime",
				fmt.Sprintf("%q is not a known runtime. Valid values: kube, chalk_node, local.", data.Runtime.ValueString()),
			)
			return nil, diags
		}
		spec.ComputeClass = &computeClass
	}

	if !data.RestartPolicy.IsNull() && !data.RestartPolicy.IsUnknown() {
		name := data.RestartPolicy.ValueString()
		v, ok := containerv1.RestartPolicy_value[name]
		if !ok {
			diags.AddError(
				"Invalid restart_policy",
				fmt.Sprintf("%q is not a known restart policy. Valid values: RESTART_POLICY_NEVER, RESTART_POLICY_ALWAYS.", name),
			)
			return nil, diags
		}
		policy := containerv1.RestartPolicy(v)
		spec.RestartPolicy = &policy
	}

	if !data.Entrypoint.IsNull() && !data.Entrypoint.IsUnknown() {
		var entrypoint []string
		diags.Append(data.Entrypoint.ElementsAs(ctx, &entrypoint, false)...)
		spec.Entrypoint = entrypoint
	}

	if !data.Env.IsNull() && !data.Env.IsUnknown() {
		var env map[string]string
		diags.Append(data.Env.ElementsAs(ctx, &env, false)...)
		spec.EnvVars = env
	}

	if !data.ResourceLimits.IsNull() && !data.ResourceLimits.IsUnknown() {
		var rl SandboxResourceLimitsModel
		diags.Append(data.ResourceLimits.As(ctx, &rl, basetypes.ObjectAsOptions{})...)
		spec.Resources = &containerv1.ResourceLimits{
			Cpu:    rl.CPU.ValueStringPointer(),
			Memory: rl.Memory.ValueStringPointer(),
		}
	}

	if !data.Volumes.IsNull() && !data.Volumes.IsUnknown() {
		var vms []SandboxVolumeMountModel
		diags.Append(data.Volumes.ElementsAs(ctx, &vms, false)...)
		for _, vm := range vms {
			v := &containerv1.VolumeMount{
				Name:      vm.Name.ValueString(),
				MountPath: vm.MountPath.ValueString(),
				Type:      vm.Type.ValueString(),
			}
			if !vm.SizeLimit.IsNull() && !vm.SizeLimit.IsUnknown() {
				v.SizeLimit = vm.SizeLimit.ValueStringPointer()
			}
			if !vm.VersionId.IsNull() && !vm.VersionId.IsUnknown() {
				version := uint64(vm.VersionId.ValueInt64())
				v.VersionId = &version
			}
			spec.Volumes = append(spec.Volumes, v)
		}
	}

	if !data.NetworkPolicy.IsNull() && !data.NetworkPolicy.IsUnknown() {
		policy, d := buildSandboxNetworkPolicy(ctx, data.NetworkPolicy)
		diags.Append(d...)
		spec.NetworkPolicy = policy
	}

	return req, diags
}

func buildSandboxNetworkPolicy(ctx context.Context, obj types.Object) (*containerv1.NetworkPolicy, diag.Diagnostics) {
	var diags diag.Diagnostics
	var np SandboxNetworkPolicyModel
	diags.Append(obj.As(ctx, &np, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	policy := &containerv1.NetworkPolicy{}

	if !np.DeniedRoutes.IsNull() && !np.DeniedRoutes.IsUnknown() {
		var denied []string
		diags.Append(np.DeniedRoutes.ElementsAs(ctx, &denied, false)...)
		policy.DeniedRoutes = denied
	}

	if !np.AllowedRoutes.IsNull() && !np.AllowedRoutes.IsUnknown() {
		var routes []SandboxAllowedRouteModel
		diags.Append(np.AllowedRoutes.ElementsAs(ctx, &routes, false)...)
		for _, route := range routes {
			ar := &containerv1.AllowedRoute{Route: route.Route.ValueString()}
			if !route.PortRanges.IsNull() && !route.PortRanges.IsUnknown() {
				var ranges []SandboxPortRangeModel
				diags.Append(route.PortRanges.ElementsAs(ctx, &ranges, false)...)
				for _, pr := range ranges {
					ar.PortRanges = append(ar.PortRanges, &containerv1.PortRange{
						StartPort: int32(pr.StartPort.ValueInt64()),
						EndPort:   int32(pr.EndPort.ValueInt64()),
					})
				}
			}
			policy.AllowedRoutes = append(policy.AllowedRoutes, ar)
		}
	}

	return policy, diags
}

// updateSandboxState copies server-owned fields onto the model. The configured
// fields are deliberately left alone so a server-normalized spec does not
// overwrite the practitioner's configuration. Name is the exception: v2 makes
// it authoritative on SandboxInfo, which also lets imports recover it.
func updateSandboxState(data *SandboxResourceModel, info *sandboxv2.SandboxInfo) {
	data.Id = types.StringValue(info.GetId())
	data.Name = optionalStringValue(info.GetName())
	data.State = optionalStringValue(sandboxStatusString(info.GetStatus()))
	if info.GetCreatedAt() == nil {
		data.CreatedAt = types.StringNull()
	} else {
		data.CreatedAt = types.StringValue(info.GetCreatedAt().AsTime().Format(time.RFC3339Nano))
	}
	data.StatusMessage = optionalStringValue(info.GetStatusMessage())
	data.BuildId = optionalStringValue(info.GetSpec().GetTags()["chalk.ai/tag-sandbox-build-id"])
}

func sandboxStatusString(status sandboxv2.SandboxStatus) string {
	return strings.ToLower(strings.TrimPrefix(status.String(), "SANDBOX_STATUS_"))
}
