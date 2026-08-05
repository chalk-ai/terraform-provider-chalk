package provider

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"time"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// This file holds the host pool spec shared by chalk_environment_host_pool and
// chalk_cluster_host_pool, which differ only in how they are scoped.

// minHostPoolIdleTimeout mirrors the server's floor on idle_timeout.
const minHostPoolIdleTimeout = time.Minute

// dnsLabelPattern mirrors the server's DNS label validation of host pool names.
var dnsLabelPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// hostPoolSpecUpdateMaskPaths are the HostPoolSpec fields the server accepts in
// an update mask. Terraform always sends the full desired spec, so every
// mutable path is included on each update.
var hostPoolSpecUpdateMaskPaths = []string{
	"name",
	"min_hosts",
	"max_hosts",
	"cpu",
	"memory",
	"machine_family",
	"idle_timeout",
}

// hostPoolSpecModel is embedded by value into each host pool resource model, so
// its fields are promoted and map to top-level attributes.
type hostPoolSpecModel struct {
	Name          types.String         `tfsdk:"name"`
	MinHosts      types.Int64          `tfsdk:"min_hosts"`
	MaxHosts      types.Int64          `tfsdk:"max_hosts"`
	IdleTimeout   timetypes.GoDuration `tfsdk:"idle_timeout"`
	Cpu           types.String         `tfsdk:"cpu"`
	Memory        types.String         `tfsdk:"memory"`
	MachineFamily types.String         `tfsdk:"machine_family"`
}

// hostPoolSchemaAttributes returns the id and spec attributes common to both
// host pool resources. Callers add their own scope attribute.
func hostPoolSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			MarkdownDescription: "The host pool id.",
			Computed:            true,
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"name": schema.StringAttribute{
			MarkdownDescription: "Name of the host pool. Must be a valid DNS label.",
			Required:            true,
			Validators: []validator.String{
				stringvalidator.LengthBetween(1, 63),
				stringvalidator.RegexMatches(dnsLabelPattern, "must be a valid DNS label: lowercase alphanumeric characters or '-', starting and ending with an alphanumeric character"),
			},
		},
		"min_hosts": schema.Int64Attribute{
			MarkdownDescription: "Minimum number of hosts to keep running. Must be either `0` or equal to `max_hosts`.",
			Required:            true,
			Validators: []validator.Int64{
				int64validator.Between(0, math.MaxInt32),
			},
		},
		"max_hosts": schema.Int64Attribute{
			MarkdownDescription: "Maximum number of hosts in the pool.",
			Required:            true,
			Validators: []validator.Int64{
				int64validator.Between(1, math.MaxInt32),
			},
		},
		"idle_timeout": schema.StringAttribute{
			MarkdownDescription: "How long an idle host is kept before being scaled down, e.g. `5m` or `1h30m`. Required when `min_hosts` is less than `max_hosts`, and must not be set when they are equal. Must be at least `1m` and resolve to a whole number of seconds.",
			Optional:            true,
			CustomType:          timetypes.GoDurationType{},
		},
		"cpu": schema.StringAttribute{
			MarkdownDescription: "CPU resources for each host, e.g. `4`.",
			Required:            true,
		},
		"memory": schema.StringAttribute{
			MarkdownDescription: "Memory resources for each host, e.g. `8Gi`.",
			Required:            true,
		},
		"machine_family": schema.StringAttribute{
			MarkdownDescription: "Machine family for this pool's hosts to run on. Defaults to an internally chosen family when unset.",
			Optional:            true,
		},
	}
}

// validateHostPoolSpec reports the scaling constraints the server enforces, so
// invalid combinations fail at plan time rather than on apply. Unknown values
// are skipped; the server remains the source of truth.
func validateHostPoolSpec(m hostPoolSpecModel, diags *diag.Diagnostics) {
	if m.MinHosts.IsUnknown() || m.MaxHosts.IsUnknown() {
		return
	}
	minHosts, maxHosts := m.MinHosts.ValueInt64(), m.MaxHosts.ValueInt64()

	if minHosts > maxHosts {
		diags.AddAttributeError(
			path.Root("min_hosts"),
			"Invalid Host Pool Scaling",
			"min_hosts must be less than or equal to max_hosts.",
		)
		return
	}
	if minHosts != 0 && minHosts != maxHosts {
		diags.AddAttributeError(
			path.Root("min_hosts"),
			"Invalid Host Pool Scaling",
			"min_hosts must be either 0 or equal to max_hosts; other values are not currently supported.",
		)
		return
	}

	if m.IdleTimeout.IsUnknown() {
		return
	}
	autoscaling := minHosts < maxHosts
	switch {
	case autoscaling && m.IdleTimeout.IsNull():
		diags.AddAttributeError(
			path.Root("idle_timeout"),
			"Missing Idle Timeout",
			"idle_timeout is required when min_hosts is less than max_hosts.",
		)
	case !autoscaling && !m.IdleTimeout.IsNull():
		diags.AddAttributeError(
			path.Root("idle_timeout"),
			"Unexpected Idle Timeout",
			"idle_timeout must not be set when min_hosts equals max_hosts, since the pool does not scale down.",
		)
	case autoscaling:
		idleTimeout, valid := validateWholeSecondDuration(
			m.IdleTimeout,
			path.Root("idle_timeout"),
			"idle_timeout",
			diags,
		)
		if !valid {
			return
		}
		if idleTimeout < minHostPoolIdleTimeout {
			diags.AddAttributeError(
				path.Root("idle_timeout"),
				"Invalid Idle Timeout",
				"idle_timeout must be at least 1m.",
			)
			return
		}
		if idleTimeout/time.Second > math.MaxInt32 {
			diags.AddAttributeError(
				path.Root("idle_timeout"),
				"Invalid Idle Timeout",
				fmt.Sprintf("idle_timeout must not exceed %d seconds.", math.MaxInt32),
			)
		}
	}
}

func (m hostPoolSpecModel) toProto() (*serverv1.HostPoolSpec, diag.Diagnostics) {
	var diags diag.Diagnostics
	validateHostPoolSpec(m, &diags)
	if diags.HasError() {
		return nil, diags
	}

	spec := &serverv1.HostPoolSpec{
		Name:          m.Name.ValueString(),
		MinHosts:      int32(m.MinHosts.ValueInt64()),
		MaxHosts:      int32(m.MaxHosts.ValueInt64()),
		Cpu:           m.Cpu.ValueString(),
		Memory:        m.Memory.ValueString(),
		MachineFamily: m.MachineFamily.ValueStringPointer(),
	}

	if !m.IdleTimeout.IsNull() && !m.IdleTimeout.IsUnknown() {
		idleTimeout, d := m.IdleTimeout.ValueGoDuration()
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		spec.IdleTimeout = durationpb.New(idleTimeout)
	}

	return spec, diags
}

func hostPoolSpecFromProto(p *serverv1.HostPoolSpec) hostPoolSpecModel {
	m := hostPoolSpecModel{
		Name:          types.StringValue(p.GetName()),
		MinHosts:      types.Int64Value(int64(p.GetMinHosts())),
		MaxHosts:      types.Int64Value(int64(p.GetMaxHosts())),
		Cpu:           types.StringValue(p.GetCpu()),
		Memory:        types.StringValue(p.GetMemory()),
		MachineFamily: stringPointerValue(p.MachineFamily),
		IdleTimeout:   timetypes.NewGoDurationNull(),
	}
	if p.GetIdleTimeout() != nil {
		m.IdleTimeout = timetypes.NewGoDurationValue(p.GetIdleTimeout().AsDuration())
	}
	return m
}

func hostPoolUpdateMask() *fieldmaskpb.FieldMask {
	return &fieldmaskpb.FieldMask{Paths: hostPoolSpecUpdateMaskPaths}
}

// readHostPool fetches a host pool by id. It reports whether the pool still
// exists so callers can remove it from state when it does not.
func readHostPool(
	ctx context.Context,
	c *client.Manager,
	envId string,
	id string,
) (*serverv1.HostPool, bool, error) {
	resp, err := c.NewHostPoolClient(ctx, envId).GetHostPool(ctx, connect.NewRequest(&serverv1.GetHostPoolRequest{
		Id: id,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	return resp.Msg.GetHostPool(), true, nil
}
