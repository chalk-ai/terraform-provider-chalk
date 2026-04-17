package provider

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	containerv1 "github.com/chalk-ai/chalk-go/gen/chalk/container/v1"
	scalinggroupv1 "github.com/chalk-ai/chalk-go/gen/chalk/scalinggroup/v1"
	"github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_ resource.Resource                = &ScalingGroupResource{}
	_ resource.ResourceWithImportState = &ScalingGroupResource{}
)

func NewScalingGroupResource() resource.Resource {
	return &ScalingGroupResource{}
}

type ScalingGroupResource struct {
	client *client.Manager
}

type ScalingGroupResourceModel struct {
	Id            types.String        `tfsdk:"id"`
	Name          types.String        `tfsdk:"name"`
	EnvironmentId types.String        `tfsdk:"environment_id"`
	Status        types.String        `tfsdk:"status"`
	StatusMessage types.String        `tfsdk:"status_message"`
	WebURL        types.String        `tfsdk:"web_url"`
	ContainerSpec *ContainerSpecModel `tfsdk:"container_spec"`
	ScalingSpec   *ScalingSpecModel   `tfsdk:"scaling_spec"`
}

type ContainerSpecModel struct {
	Image          types.String `tfsdk:"image"`
	Entrypoint     types.List   `tfsdk:"entrypoint"`
	Port           types.Int64  `tfsdk:"port"`
	EnvVars        types.Map    `tfsdk:"env_vars"`
	Tags           types.Map    `tfsdk:"tags"`
	Protocol       types.String `tfsdk:"protocol"`
	Routing        types.String `tfsdk:"routing"`
	Authentication types.String `tfsdk:"authentication"`
	Resources      types.Object `tfsdk:"resources"`
	Volumes        types.List   `tfsdk:"volumes"`
}

type ScalingSpecModel struct {
	MinReplicas                    types.Int64 `tfsdk:"min_replicas"`
	MaxReplicas                    types.Int64 `tfsdk:"max_replicas"`
	TargetCPUUtilizationPercentage types.Int64 `tfsdk:"target_cpu_utilization_percentage"`
	ShutdownDelaySeconds           types.Int64 `tfsdk:"shutdown_delay_seconds"`
}

type ResourceLimitsModel struct {
	CPU    types.String `tfsdk:"cpu"`
	Memory types.String `tfsdk:"memory"`
	GPU    types.String `tfsdk:"gpu"`
}

type VolumeMountModel struct {
	Name      types.String `tfsdk:"name"`
	MountPath types.String `tfsdk:"mount_path"`
	Type      types.String `tfsdk:"type"`
	SizeLimit types.String `tfsdk:"size_limit"`
}

var resourceLimitsAttrTypes = map[string]attr.Type{
	"cpu":    types.StringType,
	"memory": types.StringType,
	"gpu":    types.StringType,
}

var volumeMountAttrTypes = map[string]attr.Type{
	"name":       types.StringType,
	"mount_path": types.StringType,
	"type":       types.StringType,
	"size_limit": types.StringType,
}

func (r *ScalingGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scaling_group"
}

func (r *ScalingGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	requiresReplaceString := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	requiresReplaceObject := []planmodifier.Object{objectplanmodifier.RequiresReplace()}
	useStateForUnknownString := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Chalk scaling group — a horizontally (auto)scalable container deployment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The scaling group id.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Unique name for the scaling group.",
				Required:            true,
				PlanModifiers:       requiresReplaceString,
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The environment ID that this scaling group is scoped to.",
				Required:            true,
				PlanModifiers:       requiresReplaceString,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current status of the scaling group (e.g. Pending, Available, etc).",
				Computed:            true,
				PlanModifiers:       useStateForUnknownString,
			},
			"status_message": schema.StringAttribute{
				MarkdownDescription: "Additional status details.",
				Computed:            true,
				PlanModifiers:       useStateForUnknownString,
			},
			"web_url": schema.StringAttribute{
				MarkdownDescription: "Web URL to access the scaling group.",
				Computed:            true,
				PlanModifiers:       useStateForUnknownString,
			},
			"container_spec": schema.SingleNestedAttribute{
				MarkdownDescription: "Container specification describing what to run in each replica.",
				Required:            true,
				PlanModifiers:       requiresReplaceObject,
				Attributes: map[string]schema.Attribute{
					"image": schema.StringAttribute{
						MarkdownDescription: "Image to run.",
						Required:            true,
					},
					"entrypoint": schema.ListAttribute{
						MarkdownDescription: "Entrypoint command and arguments.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"port": schema.Int64Attribute{
						MarkdownDescription: "Port to expose from the container.",
						Optional:            true,
					},
					"env_vars": schema.MapAttribute{
						MarkdownDescription: "Environment variables to inject into the container.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"tags": schema.MapAttribute{
						MarkdownDescription: "User-defined tags for the container.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"protocol": schema.StringAttribute{
						MarkdownDescription: "Protocol for the container's HTTP route: `http` (default) or `grpc`.",
						Optional:            true,
					},
					"routing": schema.StringAttribute{
						MarkdownDescription: "Routing mode: `PUBLIC` or `PRIVATE`.",
						Optional:            true,
					},
					"authentication": schema.StringAttribute{
						MarkdownDescription: "Authentication mode: `UNAUTHENTICATED` (default) or `AUTHENTICATED`.",
						Optional:            true,
					},
					"resources": schema.SingleNestedAttribute{
						MarkdownDescription: "Resource limits for the container.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"cpu": schema.StringAttribute{
								MarkdownDescription: "CPU limit, e.g. `2` or `500m`.",
								Optional:            true,
							},
							"memory": schema.StringAttribute{
								MarkdownDescription: "Memory limit, e.g. `4Gi` or `512Mi`.",
								Optional:            true,
							},
							"gpu": schema.StringAttribute{
								MarkdownDescription: "GPU request, e.g. `nvidia-tesla-t4:1`.",
								Optional:            true,
							},
						},
					},
					"volumes": schema.ListNestedAttribute{
						MarkdownDescription: "Volumes to mount into the container.",
						Optional:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"name": schema.StringAttribute{
									MarkdownDescription: "Name of the volume.",
									Required:            true,
								},
								"mount_path": schema.StringAttribute{
									MarkdownDescription: "Path inside the container to mount the volume.",
									Required:            true,
								},
								"type": schema.StringAttribute{
									MarkdownDescription: "Volume type: `empty_dir`, `shared_memory`, or `chalkfs`.",
									Required:            true,
								},
								"size_limit": schema.StringAttribute{
									MarkdownDescription: "Size limit for the volume, e.g. `2Gi`.",
									Optional:            true,
								},
							},
						},
					},
				},
			},
			"scaling_spec": schema.SingleNestedAttribute{
				MarkdownDescription: "Autoscaling configuration.",
				Required:            true,
				PlanModifiers:       requiresReplaceObject,
				Attributes: map[string]schema.Attribute{
					"min_replicas": schema.Int64Attribute{
						MarkdownDescription: "Minimum number of replicas (0 for scale-to-zero).",
						Required:            true,
					},
					"max_replicas": schema.Int64Attribute{
						MarkdownDescription: "Maximum number of replicas.",
						Required:            true,
					},
					"target_cpu_utilization_percentage": schema.Int64Attribute{
						MarkdownDescription: "Target CPU utilization percentage for autoscaling.",
						Optional:            true,
					},
					"shutdown_delay_seconds": schema.Int64Attribute{
						MarkdownDescription: "Graceful termination period in seconds (default: 30).",
						Optional:            true,
					},
				},
			},
		},
	}
}

func (r *ScalingGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ScalingGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ScalingGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	spec, diags := buildScalingGroupSpec(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	sgClient := r.client.NewScalingGroupManagerClient(ctx, data.EnvironmentId.ValueString())
	createResp, err := sgClient.CreateScalingGroup(ctx, connect.NewRequest(&scalinggroupv1.CreateScalingGroupRequest{
		Spec: spec,
	}))
	if err != nil {
		resp.Diagnostics.AddError("Error creating scaling group", fmt.Sprintf("Could not create scaling group: %s", err))
		return
	}

	resp.Diagnostics.Append(updateScalingGroupState(ctx, &data, createResp.Msg.ScalingGroup)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ScalingGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ScalingGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := data.Id.ValueString()
	sgClient := r.client.NewScalingGroupManagerClient(ctx, data.EnvironmentId.ValueString())
	getResp, err := sgClient.GetScalingGroup(ctx, connect.NewRequest(&scalinggroupv1.GetScalingGroupRequest{
		Id: &id,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading scaling group", fmt.Sprintf("Could not read scaling group %s: %s", data.Id.ValueString(), err))
		return
	}

	resp.Diagnostics.Append(updateScalingGroupState(ctx, &data, getResp.Msg.ScalingGroup)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ScalingGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Scaling groups cannot be updated in place. All fields require replacement.",
	)
}

func (r *ScalingGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ScalingGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := data.Id.ValueString()
	sgClient := r.client.NewScalingGroupManagerClient(ctx, data.EnvironmentId.ValueString())
	_, err := sgClient.DeleteScalingGroup(ctx, connect.NewRequest(&scalinggroupv1.DeleteScalingGroupRequest{
		Id: &id,
	}))
	if err != nil {
		resp.Diagnostics.AddError("Error deleting scaling group", fmt.Sprintf("Could not delete scaling group %s: %s", id, err))
	}
}

func (r *ScalingGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import ID in the format 'environment_id/scaling_group_id', got: %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func buildScalingGroupSpec(ctx context.Context, data *ScalingGroupResourceModel) (*scalinggroupv1.ScalingGroupSpec, diag.Diagnostics) {
	var diags diag.Diagnostics

	containerSpec, d := buildContainerSpec(ctx, data)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	scalingSpec := buildScalingSpec(data.ScalingSpec)

	return &scalinggroupv1.ScalingGroupSpec{
		ContainerSpec: containerSpec,
		ScalingSpec:   scalingSpec,
	}, diags
}

func buildContainerSpec(ctx context.Context, data *ScalingGroupResourceModel) (*containerv1.ChalkContainerSpec, diag.Diagnostics) {
	var diags diag.Diagnostics
	cs := data.ContainerSpec

	spec := &containerv1.ChalkContainerSpec{
		Name:  data.Name.ValueString(),
		Image: cs.Image.ValueString(),
	}

	if !cs.Entrypoint.IsNull() && !cs.Entrypoint.IsUnknown() {
		var entrypoint []string
		diags.Append(cs.Entrypoint.ElementsAs(ctx, &entrypoint, false)...)
		spec.Entrypoint = entrypoint
	}

	if !cs.Port.IsNull() && !cs.Port.IsUnknown() {
		port := int32(cs.Port.ValueInt64())
		spec.Port = &port
	}

	if !cs.EnvVars.IsNull() && !cs.EnvVars.IsUnknown() {
		var envVars map[string]string
		diags.Append(cs.EnvVars.ElementsAs(ctx, &envVars, false)...)
		spec.EnvVars = envVars
	}

	if !cs.Tags.IsNull() && !cs.Tags.IsUnknown() {
		var tags map[string]string
		diags.Append(cs.Tags.ElementsAs(ctx, &tags, false)...)
		spec.Tags = tags
	}

	if !cs.Protocol.IsNull() && !cs.Protocol.IsUnknown() {
		spec.Protocol = cs.Protocol.ValueStringPointer()
	}

	if !cs.Routing.IsNull() && !cs.Routing.IsUnknown() {
		spec.Routing = cs.Routing.ValueStringPointer()
	}

	if !cs.Authentication.IsNull() && !cs.Authentication.IsUnknown() {
		spec.Authentication = cs.Authentication.ValueStringPointer()
	}

	if !cs.Resources.IsNull() && !cs.Resources.IsUnknown() {
		var rm ResourceLimitsModel
		diags.Append(cs.Resources.As(ctx, &rm, basetypes.ObjectAsOptions{})...)
		spec.Resources = &containerv1.ResourceLimits{
			Cpu:    rm.CPU.ValueStringPointer(),
			Memory: rm.Memory.ValueStringPointer(),
			Gpu:    rm.GPU.ValueStringPointer(),
		}
	}

	if !cs.Volumes.IsNull() && !cs.Volumes.IsUnknown() {
		var vms []VolumeMountModel
		diags.Append(cs.Volumes.ElementsAs(ctx, &vms, false)...)
		for _, vm := range vms {
			v := &containerv1.VolumeMount{
				Name:      vm.Name.ValueString(),
				MountPath: vm.MountPath.ValueString(),
				Type:      vm.Type.ValueString(),
			}
			if !vm.SizeLimit.IsNull() && !vm.SizeLimit.IsUnknown() {
				v.SizeLimit = vm.SizeLimit.ValueStringPointer()
			}
			spec.Volumes = append(spec.Volumes, v)
		}
	}

	return spec, diags
}

func buildScalingSpec(m *ScalingSpecModel) *scalinggroupv1.ScalingSpec {
	spec := &scalinggroupv1.ScalingSpec{
		MinReplicas: int32(m.MinReplicas.ValueInt64()),
		MaxReplicas: int32(m.MaxReplicas.ValueInt64()),
	}

	if !m.TargetCPUUtilizationPercentage.IsNull() && !m.TargetCPUUtilizationPercentage.IsUnknown() {
		v := int32(m.TargetCPUUtilizationPercentage.ValueInt64())
		spec.TargetCpuUtilizationPercentage = &v
	}

	if !m.ShutdownDelaySeconds.IsNull() && !m.ShutdownDelaySeconds.IsUnknown() {
		v := int32(m.ShutdownDelaySeconds.ValueInt64())
		spec.ShutdownDelaySeconds = &v
	}

	return spec
}

func updateScalingGroupState(ctx context.Context, data *ScalingGroupResourceModel, sg *scalinggroupv1.ScalingGroupResponse) diag.Diagnostics {
	var diags diag.Diagnostics
	data.Id = types.StringValue(sg.Id)
	data.Name = types.StringValue(sg.Name)
	data.Status = types.StringValue(sg.Status)
	data.StatusMessage = optionalStringValue(sg.GetStatusMessage())
	data.WebURL = optionalStringValue(sg.GetWebUrl())
	if sg.Spec != nil {
		cs, d := containerSpecModelFromProto(sg.Spec.ContainerSpec)
		diags.Append(d...)
		if cs != nil {
			data.ContainerSpec = cs
		}
		if ss := scalingSpecModelFromProto(sg.Spec.ScalingSpec); ss != nil {
			data.ScalingSpec = ss
		}
	}
	return diags
}

func containerSpecModelFromProto(cs *containerv1.ChalkContainerSpec) (*ContainerSpecModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	if cs == nil {
		return nil, diags
	}

	m := &ContainerSpecModel{
		Image:          types.StringValue(cs.Image),
		Entrypoint:     stringSliceToListValue(cs.Entrypoint),
		Protocol:       optionalStringValue(cs.GetProtocol()),
		Routing:        optionalStringValue(cs.GetRouting()),
		Authentication: optionalStringValue(cs.GetAuthentication()),
		Resources:      resourceLimitsToObject(cs.Resources),
		Volumes:        volumesToList(cs.Volumes),
	}

	if cs.Port != nil {
		m.Port = types.Int64Value(int64(*cs.Port))
	} else {
		m.Port = types.Int64Null()
	}

	if len(cs.EnvVars) > 0 {
		elems := make(map[string]attr.Value, len(cs.EnvVars))
		for k, v := range cs.EnvVars {
			elems[k] = types.StringValue(v)
		}
		mv, d := types.MapValue(types.StringType, elems)
		diags.Append(d...)
		m.EnvVars = mv
	} else {
		m.EnvVars = types.MapNull(types.StringType)
	}

	if len(cs.Tags) > 0 {
		elems := make(map[string]attr.Value, len(cs.Tags))
		for k, v := range cs.Tags {
			elems[k] = types.StringValue(v)
		}
		mv, d := types.MapValue(types.StringType, elems)
		diags.Append(d...)
		m.Tags = mv
	} else {
		m.Tags = types.MapNull(types.StringType)
	}

	return m, diags
}

func scalingSpecModelFromProto(ss *scalinggroupv1.ScalingSpec) *ScalingSpecModel {
	if ss == nil {
		return nil
	}
	m := &ScalingSpecModel{
		MinReplicas: types.Int64Value(int64(ss.MinReplicas)),
		MaxReplicas: types.Int64Value(int64(ss.MaxReplicas)),
	}
	if ss.TargetCpuUtilizationPercentage != nil {
		m.TargetCPUUtilizationPercentage = types.Int64Value(int64(*ss.TargetCpuUtilizationPercentage))
	} else {
		m.TargetCPUUtilizationPercentage = types.Int64Null()
	}
	if ss.ShutdownDelaySeconds != nil {
		m.ShutdownDelaySeconds = types.Int64Value(int64(*ss.ShutdownDelaySeconds))
	} else {
		m.ShutdownDelaySeconds = types.Int64Null()
	}
	return m
}

func resourceLimitsToObject(rl *containerv1.ResourceLimits) types.Object {
	if rl == nil {
		return types.ObjectNull(resourceLimitsAttrTypes)
	}
	return types.ObjectValueMust(resourceLimitsAttrTypes, map[string]attr.Value{
		"cpu":    optionalStringValue(rl.GetCpu()),
		"memory": optionalStringValue(rl.GetMemory()),
		"gpu":    optionalStringValue(rl.GetGpu()),
	})
}

func volumesToList(volumes []*containerv1.VolumeMount) types.List {
	if len(volumes) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: volumeMountAttrTypes})
	}
	elems := make([]attr.Value, len(volumes))
	for i, v := range volumes {
		elems[i] = types.ObjectValueMust(volumeMountAttrTypes, map[string]attr.Value{
			"name":       types.StringValue(v.Name),
			"mount_path": types.StringValue(v.MountPath),
			"type":       types.StringValue(v.Type),
			"size_limit": optionalStringValue(v.GetSizeLimit()),
		})
	}
	return types.ListValueMust(types.ObjectType{AttrTypes: volumeMountAttrTypes}, elems)
}
