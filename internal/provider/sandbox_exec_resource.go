package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	sandboxv2 "github.com/chalk-ai/chalk-go/gen/chalk/sandbox/v2"
	"github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/durationpb"
)

const sandboxExecOutputLimit = 16 * 1024

var _ resource.Resource = &SandboxExecResource{}
var _ resource.ResourceWithConfigure = &SandboxExecResource{}

func NewSandboxExecResource() resource.Resource {
	return &SandboxExecResource{}
}

type SandboxExecResource struct {
	client *client.Manager
}

type SandboxExecResourceModel struct {
	Id            types.String `tfsdk:"id"`
	EnvironmentId types.String `tfsdk:"environment_id"`
	SandboxId     types.String `tfsdk:"sandbox_id"`
	Command       types.List   `tfsdk:"command"`
	Env           types.Map    `tfsdk:"env"`
	Stdin         types.String `tfsdk:"stdin"`
	Triggers      types.Map    `tfsdk:"triggers"`
	Timeout       types.String `tfsdk:"timeout"`
	ExitCode      types.Int64  `tfsdk:"exit_code"`
	Output        types.String `tfsdk:"output"`
}

func (r *SandboxExecResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sandbox_exec"
}

func (r *SandboxExecResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	requiresReplaceString := []planmodifier.String{stringplanmodifier.RequiresReplace()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Runs a command once in an existing running Chalk sandbox. All inputs force replacement, so changing `triggers` re-runs the command. Use `depends_on` to ensure the associated `chalk_sandbox` has `desired_state = \"running\"` before execution.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the sandbox where the command ran.",
				Computed:            true,
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The environment ID used to scope the sandbox request. When omitted, no environment header is sent.",
				Optional:            true,
				PlanModifiers:       requiresReplaceString,
			},
			"sandbox_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the sandbox in which to run the command.",
				Required:            true,
				PlanModifiers:       requiresReplaceString,
			},
			"command": schema.ListAttribute{
				MarkdownDescription: "Command to run. The first element is the executable and remaining elements are its arguments.",
				Required:            true,
				ElementType:         types.StringType,
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
				Validators:          []validator.List{listvalidator.SizeAtLeast(1)},
			},
			"env": schema.MapAttribute{
				MarkdownDescription: "Environment variables for the command.",
				Optional:            true,
				ElementType:         types.StringType,
				PlanModifiers:       []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			},
			"stdin": schema.StringAttribute{
				MarkdownDescription: "Standard input for the command. Use this instead of command arguments for secrets.",
				Optional:            true,
				Sensitive:           true,
				PlanModifiers:       requiresReplaceString,
			},
			"triggers": schema.MapAttribute{
				MarkdownDescription: "Arbitrary values that force the command to run again when changed.",
				Optional:            true,
				ElementType:         types.StringType,
				PlanModifiers:       []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			},
			"timeout": schema.StringAttribute{
				MarkdownDescription: "Maximum command runtime as a Go duration.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("5m"),
				PlanModifiers:       requiresReplaceString,
			},
			"exit_code": schema.Int64Attribute{
				MarkdownDescription: "The command's exit code.",
				Computed:            true,
			},
			"output": schema.StringAttribute{
				MarkdownDescription: "Combined stdout and stderr, truncated to 16 KiB.",
				Computed:            true,
			},
		},
	}
}

func (r *SandboxExecResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	manager, ok := req.ProviderData.(*client.Manager)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Manager, got: %T", req.ProviderData))
		return
	}
	r.client = manager
}

func (r *SandboxExecResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SandboxExecResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	execReq, diags := buildSandboxExecRequest(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	execClient := r.client.NewSandboxProcessClient(ctx, data.EnvironmentId.ValueString())
	execResp, err := execClient.ExecCommand(ctx, connect.NewRequest(execReq))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeFailedPrecondition {
			resp.Diagnostics.AddError("Sandbox is not running", "The sandbox must be running before a command can execute. Set chalk_sandbox.desired_state to \"running\" and add depends_on so the sandbox resumes first.")
			return
		}
		resp.Diagnostics.AddError("Error executing sandbox command", fmt.Sprintf("Could not execute command in sandbox %s: %s", data.SandboxId.ValueString(), err))
		return
	}

	output := truncateSandboxExecOutput(execResp.Msg.GetStdout(), execResp.Msg.GetStderr())
	if execResp.Msg.GetExitCode() != 0 {
		resp.Diagnostics.AddError(
			"Sandbox command failed",
			fmt.Sprintf("Command exited with code %d. Output:\n%s", execResp.Msg.GetExitCode(), output),
		)
		return
	}

	data.Id = data.SandboxId
	data.ExitCode = types.Int64Value(int64(execResp.Msg.GetExitCode()))
	data.Output = types.StringValue(output)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SandboxExecResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

func (r *SandboxExecResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}

func (r *SandboxExecResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}

func buildSandboxExecRequest(ctx context.Context, data *SandboxExecResourceModel) (*sandboxv2.ExecCommandRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	var command []string
	diags.Append(data.Command.ElementsAs(ctx, &command, false)...)
	if len(command) == 0 && !diags.HasError() {
		diags.AddError("Invalid sandbox command", "The command list must contain at least one element.")
	}

	env := map[string]string(nil)
	if !data.Env.IsNull() && !data.Env.IsUnknown() {
		diags.Append(data.Env.ElementsAs(ctx, &env, false)...)
	}

	timeout, err := time.ParseDuration(data.Timeout.ValueString())
	if err != nil {
		diags.AddError("Invalid sandbox command timeout", fmt.Sprintf("Could not parse timeout as a duration: %s", err))
	} else if timeout <= 0 {
		diags.AddError("Invalid sandbox command timeout", "Timeout must be greater than zero.")
	}
	if diags.HasError() {
		return nil, diags
	}

	stdin := []byte(nil)
	if !data.Stdin.IsNull() && !data.Stdin.IsUnknown() {
		stdin = []byte(data.Stdin.ValueString())
	}

	return &sandboxv2.ExecCommandRequest{
		SandboxId: data.SandboxId.ValueString(),
		Command:   command[0],
		Args:      command[1:],
		EnvVars:   env,
		Timeout:   durationpb.New(timeout),
		Stdin:     stdin,
	}, diags
}

func truncateSandboxExecOutput(stdout, stderr []byte) string {
	combined := append(append([]byte(nil), stdout...), stderr...)
	if len(combined) > sandboxExecOutputLimit {
		combined = combined[:sandboxExecOutputLimit]
	}
	return strings.ToValidUTF8(string(combined), "�")
}
