package provider

import (
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// This file holds the cluster-level configuration that both chalk_managed_cluster
// and chalk_kubernetes_cluster expose on the CloudComponentCluster spec.

// Maintenance window modes, exposed as short (prefix-stripped) strings.
const (
	maintenanceModeUnspecified  = "UNSPECIFIED"
	maintenanceModeUnrestricted = "UNRESTRICTED"
	maintenanceModeCustom       = "CUSTOM"
)

// Data plane redis kinds. These are free-form strings the server matches
// case-sensitively, so they must match exactly.
const (
	dataPlaneRedisKindManaged    = "MANAGED"
	dataPlaneRedisKindSelfHosted = "SELF_HOSTED"
)

// Dataplane controller tiers, exposed as short (prefix-stripped) strings.
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
	Managed    *dataPlaneRedisManagedModel    `tfsdk:"managed"`
	SelfHosted *dataPlaneRedisSelfHostedModel `tfsdk:"self_hosted"`
}

type dataPlaneRedisManagedModel struct {
	Memory types.String `tfsdk:"memory"`
	Cpu    types.String `tfsdk:"cpu"`
}

type dataPlaneRedisSelfHostedModel struct {
	CloudSecretName types.String `tfsdk:"cloud_secret_name"`
}

type dataPlaneControllerModel struct {
	Tier               types.String    `tfsdk:"tier"`
	NodePool           types.String    `tfsdk:"node_pool"`
	RestrictedNodePool types.String    `tfsdk:"restricted_node_pool"`
	HostPools          []hostPoolModel `tfsdk:"host_pools"`
}

type hostPoolModel struct {
	Name          types.String `tfsdk:"name"`
	Count         types.Int64  `tfsdk:"count"`
	Cpu           types.String `tfsdk:"cpu"`
	Memory        types.String `tfsdk:"memory"`
	MachineFamily types.String `tfsdk:"machine_family"`
}

// clusterConfigSchemaAttributes returns the shared cluster-level config
// attributes to merge into a cluster resource's schema. All three blocks are
// optional.
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
			MarkdownDescription: "Optional Redis instance for cluster. Specify exactly one of `managed` or `self_hosted`.",
			Optional:            true,
			Attributes: map[string]schema.Attribute{
				"managed": schema.SingleNestedAttribute{
					MarkdownDescription: "Provision a Chalk-managed Redis instance.",
					Optional:            true,
					Validators: []validator.Object{
						objectvalidator.ExactlyOneOf(
							path.MatchRelative().AtParent().AtName("self_hosted"),
						),
					},
					Attributes: map[string]schema.Attribute{
						"memory": schema.StringAttribute{
							MarkdownDescription: "Memory size of the Redis instance, e.g. `1Gi`, `2Gi`.",
							Optional:            true,
						},
						"cpu": schema.StringAttribute{
							MarkdownDescription: "CPU size of the Redis instance, e.g. `500m`, `1`.",
							Optional:            true,
						},
					},
				},
				"self_hosted": schema.SingleNestedAttribute{
					MarkdownDescription: "Use a self-hosted Redis instance.",
					Optional:            true,
					Validators: []validator.Object{
						objectvalidator.ExactlyOneOf(
							path.MatchRelative().AtParent().AtName("managed"),
						),
					},
					Attributes: map[string]schema.Attribute{
						"cloud_secret_name": schema.StringAttribute{
							MarkdownDescription: "Name of the cloud secret holding credentials for the self-hosted Redis instance.",
							Required:            true,
						},
					},
				},
			},
		},
		"data_plane_controller": schema.SingleNestedAttribute{
			MarkdownDescription: "Dataplane controller configuration, required to use Chalk Compute.",
			Optional:            true,
			Attributes: map[string]schema.Attribute{
				"tier": schema.StringAttribute{
					MarkdownDescription: "Resource tier for the dataplane controller. One of `DISABLED`, `SMALL`, `MEDIUM`, `LARGE`.",
					Optional:            true,
					Validators: []validator.String{
						stringvalidator.OneOf(dataPlaneControllerTierDisabled, dataPlaneControllerTierSmall, dataPlaneControllerTierMedium, dataPlaneControllerTierLarge),
						// Reject an empty data_plane_controller block: at least one field
						// must be set. An empty block is indistinguishable from omitting
						// it and would otherwise drift to null after apply.
						stringvalidator.AtLeastOneOf(
							path.MatchRelative().AtParent().AtName("node_pool"),
							path.MatchRelative().AtParent().AtName("restricted_node_pool"),
							path.MatchRelative().AtParent().AtName("host_pools"),
						),
					},
				},
				"node_pool": schema.StringAttribute{
					MarkdownDescription: "Node pool to pin non-restricted (open) container/scaling-group workloads to.",
					Optional:            true,
				},
				"restricted_node_pool": schema.StringAttribute{
					MarkdownDescription: "Node pool to pin restricted container/scaling-group workloads to.",
					Optional:            true,
				},
				"host_pools": schema.ListNestedAttribute{
					MarkdownDescription: "Host pools to deploy for this cluster.",
					Optional:            true,
					Validators: []validator.List{
						// An empty list means the same as omitting the attribute and would
						// drift to null after apply, so require at least one entry.
						listvalidator.SizeAtLeast(1),
					},
					NestedObject: schema.NestedAttributeObject{
						Attributes: map[string]schema.Attribute{
							"name": schema.StringAttribute{
								MarkdownDescription: "Name of the pool.",
								Required:            true,
							},
							"count": schema.Int64Attribute{
								MarkdownDescription: "Number of hosts in the pool.",
								Required:            true,
								Validators: []validator.Int64{
									int64validator.AtLeast(1),
								},
							},
							"cpu": schema.StringAttribute{
								MarkdownDescription: "CPU resources for each host, e.g. `4`.",
								Optional:            true,
							},
							"memory": schema.StringAttribute{
								MarkdownDescription: "Memory resources for each host, e.g. `8Gi`.",
								Optional:            true,
							},
							"machine_family": schema.StringAttribute{
								MarkdownDescription: "Machine family for this pool's hosts to run on.",
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
	if m.SelfHosted != nil {
		kind := dataPlaneRedisKindSelfHosted
		return &serverv1.DataPlaneRedis{
			Kind:            &kind,
			CloudSecretName: m.SelfHosted.CloudSecretName.ValueStringPointer(),
		}
	}
	// Exactly one of managed/self_hosted is set (enforced by schema validators),
	// so reaching here means managed.
	kind := dataPlaneRedisKindManaged
	return &serverv1.DataPlaneRedis{
		Kind:   &kind,
		Memory: m.Managed.Memory.ValueStringPointer(),
		Cpu:    m.Managed.Cpu.ValueStringPointer(),
	}
}

func (m *dataPlaneControllerModel) toProto() *serverv1.DataplaneController {
	if m == nil {
		return nil
	}
	c := &serverv1.DataplaneController{
		Tier:               dataPlaneControllerTierToProto[m.Tier.ValueString()],
		NodePool:           m.NodePool.ValueStringPointer(),
		RestrictedNodePool: m.RestrictedNodePool.ValueStringPointer(),
	}
	for _, pool := range m.HostPools {
		c.HostPools = append(c.HostPools, &serverv1.ChalkHostPool{
			Name:          pool.Name.ValueString(),
			Count:         int32(pool.Count.ValueInt64()),
			Cpu:           pool.Cpu.ValueStringPointer(),
			Memory:        pool.Memory.ValueStringPointer(),
			MachineFamily: pool.MachineFamily.ValueStringPointer(),
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
	if p.GetKind() == dataPlaneRedisKindSelfHosted {
		return &dataPlaneRedisModel{
			SelfHosted: &dataPlaneRedisSelfHostedModel{
				CloudSecretName: stringPointerValue(p.CloudSecretName),
			},
		}
	}
	// The server treats an empty kind the same as MANAGED.
	return &dataPlaneRedisModel{
		Managed: &dataPlaneRedisManagedModel{
			Memory: stringPointerValue(p.Memory),
			Cpu:    stringPointerValue(p.Cpu),
		},
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
		NodePool:           stringPointerValue(p.NodePool),
		RestrictedNodePool: stringPointerValue(p.RestrictedNodePool),
	}
	for _, pool := range p.GetHostPools() {
		m.HostPools = append(m.HostPools, hostPoolModel{
			Name:          types.StringValue(pool.GetName()),
			Count:         types.Int64Value(int64(pool.GetCount())),
			Cpu:           stringPointerValue(pool.Cpu),
			Memory:        stringPointerValue(pool.Memory),
			MachineFamily: stringPointerValue(pool.MachineFamily),
		})
	}
	return m
}
