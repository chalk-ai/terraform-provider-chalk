package provider

import (
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// This file holds the cluster-level configuration that both chalk_managed_cluster
// and chalk_kubernetes_cluster expose on the CloudComponentCluster spec:
// maintenance_window, data_plane_redis, and data_plane_controller. Both resources
// share the schema, expand (model -> proto) and flatten (proto -> model) logic so
// the two stay in lockstep.
//
// The server (go-api-server/cloudcomponents) treats all three blocks as the only
// mutable spec fields, applying them identically for managed and unmanaged
// clusters, which is why the config is common.

// Maintenance window modes, exposed as short (prefix-stripped) tokens.
const (
	maintenanceModeUnspecified  = "UNSPECIFIED"
	maintenanceModeUnrestricted = "UNRESTRICTED"
	maintenanceModeCustom       = "CUSTOM"
)

// Data plane redis kinds. These are free-form strings the server matches
// case-sensitively, so the tokens must match exactly (see
// go-api-server/cloudcomponents/utils.go validateDataPlaneRedisSpec).
const (
	dataPlaneRedisKindManaged    = "MANAGED"
	dataPlaneRedisKindSelfHosted = "SELF_HOSTED"
)

// Dataplane controller tiers, exposed as short (prefix-stripped) tokens.
const (
	dataPlaneControllerTierDisabled = "DISABLED"
	dataPlaneControllerTierSmall    = "SMALL"
	dataPlaneControllerTierMedium   = "MEDIUM"
	dataPlaneControllerTierLarge    = "LARGE"
)

var maintenanceModeToProto = map[string]serverv1.MaintenanceWindow_Mode{
	maintenanceModeUnspecified:  serverv1.MaintenanceWindow_MODE_UNSPECIFIED,
	maintenanceModeUnrestricted: serverv1.MaintenanceWindow_MODE_UNRESTRICTED,
	maintenanceModeCustom:       serverv1.MaintenanceWindow_MODE_CUSTOM,
}

var maintenanceModeFromProto = map[serverv1.MaintenanceWindow_Mode]string{
	serverv1.MaintenanceWindow_MODE_UNSPECIFIED:  maintenanceModeUnspecified,
	serverv1.MaintenanceWindow_MODE_UNRESTRICTED: maintenanceModeUnrestricted,
	serverv1.MaintenanceWindow_MODE_CUSTOM:       maintenanceModeCustom,
}

var dataPlaneControllerTierToProto = map[string]serverv1.DataplaneController_Tier{
	dataPlaneControllerTierDisabled: serverv1.DataplaneController_TIER_DISABLED,
	dataPlaneControllerTierSmall:    serverv1.DataplaneController_TIER_SMALL,
	dataPlaneControllerTierMedium:   serverv1.DataplaneController_TIER_MEDIUM,
	dataPlaneControllerTierLarge:    serverv1.DataplaneController_TIER_LARGE,
}

var dataPlaneControllerTierFromProto = map[serverv1.DataplaneController_Tier]string{
	serverv1.DataplaneController_TIER_DISABLED: dataPlaneControllerTierDisabled,
	serverv1.DataplaneController_TIER_SMALL:    dataPlaneControllerTierSmall,
	serverv1.DataplaneController_TIER_MEDIUM:   dataPlaneControllerTierMedium,
	serverv1.DataplaneController_TIER_LARGE:    dataPlaneControllerTierLarge,
}

type maintenanceWindowModel struct {
	Mode     types.String `tfsdk:"mode"`
	Schedule types.String `tfsdk:"schedule"`
	Duration types.String `tfsdk:"duration"`
}

type dataPlaneRedisModel struct {
	Kind            types.String `tfsdk:"kind"`
	Memory          types.String `tfsdk:"memory"`
	Cpu             types.String `tfsdk:"cpu"`
	CloudSecretName types.String `tfsdk:"cloud_secret_name"`
}

type dataPlaneControllerModel struct {
	Tier               types.String    `tfsdk:"tier"`
	NodePool           types.String    `tfsdk:"node_pool"`
	RestrictedNodePool types.String    `tfsdk:"restricted_node_pool"`
	HostPools          []hostPoolModel `tfsdk:"host_pools"`
}

type hostPoolModel struct {
	Name   types.String `tfsdk:"name"`
	Count  types.Int64  `tfsdk:"count"`
	Cpu    types.String `tfsdk:"cpu"`
	Memory types.String `tfsdk:"memory"`
}

// clusterConfigSchemaAttributes returns the shared cluster-level config
// attributes to merge into a cluster resource's schema. All three blocks are
// optional; omitting one leaves the corresponding spec field unset.
func clusterConfigSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"maintenance_window": schema.SingleNestedAttribute{
			MarkdownDescription: "Controls when disruptive maintenance operations may run on this cluster.",
			Optional:            true,
			Attributes: map[string]schema.Attribute{
				"mode": schema.StringAttribute{
					MarkdownDescription: "Maintenance scheduling mode. `UNSPECIFIED` uses the system default window, `UNRESTRICTED` allows operations at any time, and `CUSTOM` uses `schedule`/`duration`.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.OneOf(maintenanceModeUnspecified, maintenanceModeUnrestricted, maintenanceModeCustom),
					},
				},
				"schedule": schema.StringAttribute{
					MarkdownDescription: "5-field UTC cron expression defining when the window opens, e.g. `0 2 * * *`. Required when `mode = CUSTOM`.",
					Optional:            true,
				},
				"duration": schema.StringAttribute{
					MarkdownDescription: "How long the window stays open, e.g. `30m` or `1h`. Required when `mode = CUSTOM`.",
					Optional:            true,
				},
			},
		},
		"data_plane_redis": schema.SingleNestedAttribute{
			MarkdownDescription: "Optional Redis instance for cluster operators to use as a shared cache.",
			Optional:            true,
			Attributes: map[string]schema.Attribute{
				"kind": schema.StringAttribute{
					MarkdownDescription: "Redis provisioning kind. `MANAGED` provisions a Chalk-managed instance. `SELF_HOSTED` is defined but not yet supported server-side.",
					Optional:            true,
					Validators: []validator.String{
						stringvalidator.OneOf(dataPlaneRedisKindManaged, dataPlaneRedisKindSelfHosted),
					},
				},
				"memory": schema.StringAttribute{
					MarkdownDescription: "Memory size of the Redis instance, e.g. `1Gi`, `2Gi`.",
					Optional:            true,
				},
				"cpu": schema.StringAttribute{
					MarkdownDescription: "CPU size of the Redis instance, e.g. `500m`, `1`.",
					Optional:            true,
				},
				"cloud_secret_name": schema.StringAttribute{
					MarkdownDescription: "Name of the cloud secret holding credentials for a self-hosted Redis instance (only used when `kind = SELF_HOSTED`).",
					Optional:            true,
				},
			},
		},
		"data_plane_controller": schema.SingleNestedAttribute{
			MarkdownDescription: "Dataplane controller configuration, required to use Chalk Compute.",
			Optional:            true,
			Attributes: map[string]schema.Attribute{
				"tier": schema.StringAttribute{
					MarkdownDescription: "Resource tier for the dataplane controller. One of `DISABLED`, `SMALL`, `MEDIUM`, `LARGE`. Unset resolves to `SMALL` server-side.",
					Optional:            true,
					Validators: []validator.String{
						stringvalidator.OneOf(dataPlaneControllerTierDisabled, dataPlaneControllerTierSmall, dataPlaneControllerTierMedium, dataPlaneControllerTierLarge),
					},
				},
				"node_pool": schema.StringAttribute{
					MarkdownDescription: "Node pool to pin non-gVisor (open) container/scaling-group workloads to.",
					Optional:            true,
				},
				"restricted_node_pool": schema.StringAttribute{
					MarkdownDescription: "Node pool to pin gVisor (restricted) container/scaling-group workloads to.",
					Optional:            true,
				},
				"host_pools": schema.ListNestedAttribute{
					MarkdownDescription: "Host pools to deploy for this cluster. Each entry provisions a ChalkHostPool.",
					Optional:            true,
					NestedObject: schema.NestedAttributeObject{
						Attributes: map[string]schema.Attribute{
							"name": schema.StringAttribute{
								MarkdownDescription: "Name of the pool. Must be a valid DNS label.",
								Required:            true,
							},
							"count": schema.Int64Attribute{
								MarkdownDescription: "Number of hypervisor pods in the pool.",
								Required:            true,
								Validators: []validator.Int64{
									int64validator.AtLeast(1),
								},
							},
							"cpu": schema.StringAttribute{
								MarkdownDescription: "CPU resources for each hypervisor pod, e.g. `4`.",
								Optional:            true,
							},
							"memory": schema.StringAttribute{
								MarkdownDescription: "Memory resources for each hypervisor pod, e.g. `8Gi`.",
								Optional:            true,
							},
						},
					},
				},
			},
		},
	}
}

// applyClusterConfigToSpec writes the three optional config blocks onto the
// given spec. Nil models leave the corresponding spec field unset, which the
// server interprets as "clear this config".
func applyClusterConfigToSpec(
	spec *serverv1.CloudComponentCluster,
	maintenance *maintenanceWindowModel,
	redis *dataPlaneRedisModel,
	controller *dataPlaneControllerModel,
) {
	spec.MaintenanceWindow = maintenance.toProto()
	spec.DataPlaneRedis = redis.toProto()
	spec.DataplaneController = controller.toProto()
}

func (m *maintenanceWindowModel) toProto() *serverv1.MaintenanceWindow {
	if m == nil {
		return nil
	}
	return &serverv1.MaintenanceWindow{
		Mode:     maintenanceModeToProto[m.Mode.ValueString()],
		Schedule: m.Schedule.ValueString(),
		Duration: m.Duration.ValueString(),
	}
}

func (m *dataPlaneRedisModel) toProto() *serverv1.DataPlaneRedis {
	if m == nil {
		return nil
	}
	return &serverv1.DataPlaneRedis{
		Kind:            optionalStringPtr(m.Kind),
		Memory:          optionalStringPtr(m.Memory),
		Cpu:             optionalStringPtr(m.Cpu),
		CloudSecretName: optionalStringPtr(m.CloudSecretName),
	}
}

func (m *dataPlaneControllerModel) toProto() *serverv1.DataplaneController {
	if m == nil {
		return nil
	}
	// available_tiers is output-only and is cleared server-side before persist,
	// so it is intentionally never sent.
	c := &serverv1.DataplaneController{
		Tier:               dataPlaneControllerTierToProto[m.Tier.ValueString()],
		NodePool:           optionalStringPtr(m.NodePool),
		RestrictedNodePool: optionalStringPtr(m.RestrictedNodePool),
	}
	for _, pool := range m.HostPools {
		c.HostPools = append(c.HostPools, &serverv1.ChalkHostPool{
			Name:   pool.Name.ValueString(),
			Count:  int32(pool.Count.ValueInt64()),
			Cpu:    optionalStringPtr(pool.Cpu),
			Memory: optionalStringPtr(pool.Memory),
		})
	}
	return c
}

func maintenanceWindowFromProto(p *serverv1.MaintenanceWindow) *maintenanceWindowModel {
	if p == nil {
		return nil
	}
	mode, ok := maintenanceModeFromProto[p.GetMode()]
	if !ok {
		mode = maintenanceModeUnspecified
	}
	return &maintenanceWindowModel{
		Mode:     types.StringValue(mode),
		Schedule: optionalStringValue(p.GetSchedule()),
		Duration: optionalStringValue(p.GetDuration()),
	}
}

func dataPlaneRedisFromProto(p *serverv1.DataPlaneRedis) *dataPlaneRedisModel {
	if p == nil {
		return nil
	}
	return &dataPlaneRedisModel{
		Kind:            optionalStringValue(p.GetKind()),
		Memory:          optionalStringValue(p.GetMemory()),
		Cpu:             optionalStringValue(p.GetCpu()),
		CloudSecretName: optionalStringValue(p.GetCloudSecretName()),
	}
}

func dataPlaneControllerFromProto(p *serverv1.DataplaneController) *dataPlaneControllerModel {
	// The server always hydrates a non-nil DataplaneController on read to carry
	// the output-only available_tiers (see hydrateOutputOnlyFields in
	// go-api-server). Treat an otherwise-empty controller as unconfigured so a
	// cluster with no controller block doesn't drift into a phantom object.
	if p == nil || (p.GetTier() == serverv1.DataplaneController_TIER_UNSPECIFIED &&
		p.GetNodePool() == "" && p.GetRestrictedNodePool() == "" && len(p.GetHostPools()) == 0) {
		return nil
	}
	m := &dataPlaneControllerModel{
		Tier:               optionalStringValue(dataPlaneControllerTierFromProto[p.GetTier()]),
		NodePool:           optionalStringValue(p.GetNodePool()),
		RestrictedNodePool: optionalStringValue(p.GetRestrictedNodePool()),
	}
	for _, pool := range p.GetHostPools() {
		m.HostPools = append(m.HostPools, hostPoolModel{
			Name:   types.StringValue(pool.GetName()),
			Count:  types.Int64Value(int64(pool.GetCount())),
			Cpu:    optionalStringValue(pool.GetCpu()),
			Memory: optionalStringValue(pool.GetMemory()),
		})
	}
	return m
}

// optionalStringPtr converts a types.String to a *string, returning nil for
// null, unknown, or empty values so unset optional fields are omitted from the
// proto rather than sent as empty strings.
func optionalStringPtr(s types.String) *string {
	if s.IsNull() || s.IsUnknown() || s.ValueString() == "" {
		return nil
	}
	v := s.ValueString()
	return &v
}
