package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &ClusterGatewayResource{}
var _ resource.ResourceWithImportState = &ClusterGatewayResource{}

func NewClusterGatewayResource() resource.Resource {
	return &ClusterGatewayResource{}
}

type ClusterGatewayResource struct {
	client *ClientManager
}

type EnvoyGatewayListenerModel struct {
	Port     types.Int64  `tfsdk:"port"`
	Protocol types.String `tfsdk:"protocol"`
	Name     types.String `tfsdk:"name"`
	From     types.String `tfsdk:"from"`
}

type TLSCertificateConfigModel struct {
	SecretName      types.String `tfsdk:"secret_name"`
	SecretNamespace types.String `tfsdk:"secret_namespace"`
}

type ClusterGatewayResourceModel struct {
	Id types.String `tfsdk:"id"`

	// Core gateway fields - optional and computed
	Namespace        types.String `tfsdk:"namespace"`
	GatewayName      types.String `tfsdk:"gateway_name"`
	GatewayClassName types.String `tfsdk:"gateway_class_name"`

	// Flattened Envoy config fields
	TimeoutDuration          types.String `tfsdk:"timeout_duration"`
	DNSHostname              types.String `tfsdk:"dns_hostname"`
	Replicas                 types.Int64  `tfsdk:"replicas"`
	MinAvailable             types.Int64  `tfsdk:"min_available"`
	LetsencryptClusterIssuer types.String `tfsdk:"letsencrypt_cluster_issuer"`
	AdditionalDNSNames       types.List   `tfsdk:"additional_dns_names"`

	// Optional fields
	IPAllowlist        types.List                 `tfsdk:"ip_allowlist"`
	TLSCertificate     *TLSCertificateConfigModel `tfsdk:"tls_certificate"`
	ServiceAnnotations types.Map                  `tfsdk:"service_annotations"`
	Listeners          types.List                 `tfsdk:"listeners"`
	Routing            types.String               `tfsdk:"routing"`

	// Required field
	KubeClusterId types.String `tfsdk:"kube_cluster_id"`
}

func (r *ClusterGatewayResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_gateway"
}

func (r *ClusterGatewayResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk cluster gateway resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Gateway identifier",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"kube_cluster_id": schema.StringAttribute{
				MarkdownDescription: "Kubernetes cluster ID",
				Required:            true,
			},
			"namespace": schema.StringAttribute{
				MarkdownDescription: "Kubernetes namespace for the gateway",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"gateway_name": schema.StringAttribute{
				MarkdownDescription: "Name of the gateway",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"gateway_class_name": schema.StringAttribute{
				MarkdownDescription: "Gateway class name",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"listeners": schema.ListNestedAttribute{
				MarkdownDescription: "Gateway listeners configuration",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"port": schema.Int64Attribute{
							MarkdownDescription: "Port number for the listener",
							Required:            true,
						},
						"protocol": schema.StringAttribute{
							MarkdownDescription: "Protocol for the listener",
							Required:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "Name of the listener",
							Required:            true,
						},
						"from": schema.StringAttribute{
							MarkdownDescription: "Allowed namespaces from field",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("All"),
						},
					},
				},
			},
			"timeout_duration": schema.StringAttribute{
				MarkdownDescription: "Timeout duration for Envoy gateway",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"dns_hostname": schema.StringAttribute{
				MarkdownDescription: "DNS hostname",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"replicas": schema.Int64Attribute{
				MarkdownDescription: "Number of replicas for Envoy gateway",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"min_available": schema.Int64Attribute{
				MarkdownDescription: "Minimum available replicas for Envoy gateway",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"letsencrypt_cluster_issuer": schema.StringAttribute{
				MarkdownDescription: "Let's Encrypt cluster issuer for Envoy gateway",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"additional_dns_names": schema.ListAttribute{
				MarkdownDescription: "Additional DNS names for Envoy gateway",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"ip_allowlist": schema.ListAttribute{
				MarkdownDescription: "IP allowlist for the gateway",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"tls_certificate": schema.SingleNestedAttribute{
				MarkdownDescription: "TLS certificate configuration",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"secret_name": schema.StringAttribute{
						MarkdownDescription: "Name of the Kubernetes secret containing the TLS certificate",
						Required:            true,
					},
					"secret_namespace": schema.StringAttribute{
						MarkdownDescription: "Namespace of the Kubernetes secret",
						Required:            true,
					},
				},
			},
			"service_annotations": schema.MapAttribute{
				MarkdownDescription: "Service annotations",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"routing": schema.StringAttribute{
				MarkdownDescription: "Routing type for the gateway (e.g., 'PUBLIC', 'PRIVATE', 'PRIVATELINK')",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("PUBLIC"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *ClusterGatewayResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// updateModelFromSpecs updates the terraform model with values from the API response specs
func (r *ClusterGatewayResource) updateModelFromSpecs(ctx context.Context, data *ClusterGatewayResourceModel, specs *serverv1.EnvoyGatewaySpecs, diags *diag.Diagnostics) {
	if specs == nil {
		return
	}

	data.Namespace = types.StringValue(specs.Namespace)
	data.GatewayName = types.StringValue(specs.GatewayName)
	data.GatewayClassName = types.StringValue(specs.GatewayClassName)

	// Update listeners
	if len(specs.Listeners) > 0 {
		var listenerModels []attr.Value
		for _, listener := range specs.Listeners {
			listenerModel := map[string]attr.Value{
				"port":     types.Int64Value(int64(listener.Port)),
				"protocol": types.StringValue(listener.Protocol),
				"name":     types.StringValue(listener.Name),
				"from":     types.StringValue(listener.AllowedRoutes.Namespaces.From),
			}
			listenerModels = append(listenerModels, types.ObjectValueMust(
				map[string]attr.Type{
					"port":     types.Int64Type,
					"protocol": types.StringType,
					"name":     types.StringType,
					"from":     types.StringType,
				},
				listenerModel,
			))
		}
		data.Listeners = types.ListValueMust(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"port":     types.Int64Type,
				"protocol": types.StringType,
				"name":     types.StringType,
				"from":     types.StringType,
			},
		}, listenerModels)
	}

	// Update IP allowlist
	if len(specs.IpAllowlist) > 0 {
		var ipValues []attr.Value
		for _, ip := range specs.IpAllowlist {
			ipValues = append(ipValues, types.StringValue(ip))
		}
		data.IPAllowlist = types.ListValueMust(types.StringType, ipValues)
	}

	// Update service annotations
	if len(specs.ServiceAnnotations) > 0 {
		annotations := make(map[string]attr.Value)
		for k, v := range specs.ServiceAnnotations {
			annotations[k] = types.StringValue(v)
		}
		data.ServiceAnnotations = types.MapValueMust(types.StringType, annotations)
	}

	// Update flattened Envoy config fields
	if specs.Config != nil && specs.Config.GetEnvoy() != nil {
		envoyConfig := specs.Config.GetEnvoy()

		if envoyConfig.TimeoutDuration != nil {
			data.TimeoutDuration = types.StringValue(*envoyConfig.TimeoutDuration)
		} else {
			data.TimeoutDuration = types.StringNull()
		}

		if envoyConfig.DnsHostname != nil {
			data.DNSHostname = types.StringValue(*envoyConfig.DnsHostname)
		} else {
			data.DNSHostname = types.StringNull()
		}

		if envoyConfig.Replicas != nil {
			data.Replicas = types.Int64Value(int64(*envoyConfig.Replicas))
		} else {
			data.Replicas = types.Int64Null()
		}

		if envoyConfig.MinAvailable != nil {
			data.MinAvailable = types.Int64Value(int64(*envoyConfig.MinAvailable))
		} else {
			data.MinAvailable = types.Int64Null()
		}

		if envoyConfig.LetsencryptClusterIssuer != nil {
			data.LetsencryptClusterIssuer = types.StringValue(*envoyConfig.LetsencryptClusterIssuer)
		} else {
			data.LetsencryptClusterIssuer = types.StringNull()
		}

		if len(envoyConfig.AdditionalDnsNames) > 0 {
			var dnsValues []attr.Value
			for _, dns := range envoyConfig.AdditionalDnsNames {
				dnsValues = append(dnsValues, types.StringValue(dns))
			}
			data.AdditionalDNSNames = types.ListValueMust(types.StringType, dnsValues)
		} else {
			data.AdditionalDNSNames = types.ListNull(types.StringType)
		}
	}

	// Update TLS certificate
	if specs.TlsCertificate != nil && specs.TlsCertificate.GetManualCertificate() != nil {
		manualCert := specs.TlsCertificate.GetManualCertificate()
		data.TLSCertificate = &TLSCertificateConfigModel{
			SecretName:      types.StringValue(manualCert.SecretName),
			SecretNamespace: types.StringValue(manualCert.SecretNamespace),
		}
	}

	// Update routing
	if specs.Routing != nil {
		data.Routing = types.StringValue(*specs.Routing)
	} else {
		data.Routing = types.StringNull()
	}
}

func (r *ClusterGatewayResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ClusterGatewayResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	// Create builder client
	bc := r.client.NewBuilderClient(ctx)

	// Convert terraform model to proto request
	createReq := &serverv1.CreateClusterGatewayRequest{
		Specs: &serverv1.EnvoyGatewaySpecs{
			Namespace:        data.Namespace.ValueString(),
			GatewayName:      data.GatewayName.ValueString(),
			GatewayClassName: data.GatewayClassName.ValueString(),
		},
	}

	// Convert listeners
	if !data.Listeners.IsNull() {
		var listeners []EnvoyGatewayListenerModel
		diags := data.Listeners.ElementsAs(ctx, &listeners, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		for _, listener := range listeners {
			protoListener := &serverv1.EnvoyGatewayListener{
				Port:     int32(listener.Port.ValueInt64()),
				Protocol: listener.Protocol.ValueString(),
				Name:     listener.Name.ValueString(),
				AllowedRoutes: &serverv1.EnvoyGatewayAllowedRoutes{
					Namespaces: &serverv1.EnvoyGatewayAllowedNamespaces{
						From: listener.From.ValueString(),
					},
				},
			}
			createReq.Specs.Listeners = append(createReq.Specs.Listeners, protoListener)
		}
	}

	// Convert flattened Envoy config
	envoyConfig := &serverv1.EnvoyGatewayProviderConfig{}
	envoyConfig.TimeoutDuration = data.TimeoutDuration.ValueStringPointer()
	envoyConfig.DnsHostname = data.DNSHostname.ValueStringPointer()
	if !data.Replicas.IsNull() {
		val := int32(data.Replicas.ValueInt64())
		envoyConfig.Replicas = &val
	}
	if !data.MinAvailable.IsNull() {
		val := int32(data.MinAvailable.ValueInt64())
		envoyConfig.MinAvailable = &val
	}
	envoyConfig.LetsencryptClusterIssuer = data.LetsencryptClusterIssuer.ValueStringPointer()
	if !data.AdditionalDNSNames.IsNull() {
		var dnsNames []string
		diags := data.AdditionalDNSNames.ElementsAs(ctx, &dnsNames, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		envoyConfig.AdditionalDnsNames = dnsNames
	}
	createReq.Specs.Config = &serverv1.GatewayProviderConfig{
		Config: &serverv1.GatewayProviderConfig_Envoy{
			Envoy: envoyConfig,
		},
	}

	// Convert IP allowlist
	if !data.IPAllowlist.IsNull() {
		var ipList []string
		diags := data.IPAllowlist.ElementsAs(ctx, &ipList, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Specs.IpAllowlist = ipList
	}

	// Convert TLS certificate
	if data.TLSCertificate != nil {
		createReq.Specs.TlsCertificate = &serverv1.TLSCertificateConfig{
			CertificateSource: &serverv1.TLSCertificateConfig_ManualCertificate{
				ManualCertificate: &serverv1.TLSManualCertificateRef{
					SecretName:      data.TLSCertificate.SecretName.ValueString(),
					SecretNamespace: data.TLSCertificate.SecretNamespace.ValueString(),
				},
			},
		}
	}

	// Convert service annotations
	if !data.ServiceAnnotations.IsNull() {
		annotations := make(map[string]string)
		diags := data.ServiceAnnotations.ElementsAs(ctx, &annotations, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Specs.ServiceAnnotations = annotations
	}

	// Set routing
	createReq.Specs.Routing = data.Routing.ValueStringPointer()

	if !data.Id.IsNull() {
		createReq.Id = data.Id.ValueStringPointer()
	}

	createReq.KubeClusterId = data.KubeClusterId.ValueStringPointer()

	response, err := bc.CreateClusterGateway(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Cluster Gateway",
			fmt.Sprintf("Could not create cluster gateway: %v", err),
		)
		return
	}

	data.Id = types.StringValue(response.Msg.Id)

	// Update the model with the returned specs (including computed/default values)
	r.updateModelFromSpecs(ctx, &data, response.Msg.Specs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "created a chalk_cluster_gateway resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterGatewayResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ClusterGatewayResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	// Create builder client
	bc := r.client.NewBuilderClient(ctx)

	getReq := &serverv1.GetClusterGatewayRequest{
		Id: data.Id.ValueStringPointer(),
	}

	gateway, err := bc.GetClusterGateway(ctx, connect.NewRequest(getReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Chalk Cluster Gateway",
			fmt.Sprintf("Could not read cluster gateway %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Update the model with the fetched data
	r.updateModelFromSpecs(ctx, &data, gateway.Msg.Specs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update kube cluster ID
	if gateway.Msg.GetKubeClusterId() != "" {
		data.KubeClusterId = types.StringValue(gateway.Msg.GetKubeClusterId())
	} else {
		data.KubeClusterId = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterGatewayResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ClusterGatewayResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	// Create builder client
	bc := r.client.NewBuilderClient(ctx)

	// Convert terraform model to proto request - reuse create logic since it's an upsert
	createReq := &serverv1.CreateClusterGatewayRequest{
		Specs: &serverv1.EnvoyGatewaySpecs{
			Namespace:        data.Namespace.ValueString(),
			GatewayName:      data.GatewayName.ValueString(),
			GatewayClassName: data.GatewayClassName.ValueString(),
		},
	}

	// Convert listeners
	if !data.Listeners.IsNull() {
		var listeners []EnvoyGatewayListenerModel
		diags := data.Listeners.ElementsAs(ctx, &listeners, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		for _, listener := range listeners {
			protoListener := &serverv1.EnvoyGatewayListener{
				Port:     int32(listener.Port.ValueInt64()),
				Protocol: listener.Protocol.ValueString(),
				Name:     listener.Name.ValueString(),
				AllowedRoutes: &serverv1.EnvoyGatewayAllowedRoutes{
					Namespaces: &serverv1.EnvoyGatewayAllowedNamespaces{
						From: listener.From.ValueString(),
					},
				},
			}
			createReq.Specs.Listeners = append(createReq.Specs.Listeners, protoListener)
		}
	}

	// Convert flattened Envoy config
	envoyConfig := &serverv1.EnvoyGatewayProviderConfig{}
	envoyConfig.TimeoutDuration = data.TimeoutDuration.ValueStringPointer()
	envoyConfig.DnsHostname = data.DNSHostname.ValueStringPointer()
	if !data.Replicas.IsNull() {
		val := int32(data.Replicas.ValueInt64())
		envoyConfig.Replicas = &val
	}
	if !data.MinAvailable.IsNull() {
		val := int32(data.MinAvailable.ValueInt64())
		envoyConfig.MinAvailable = &val
	}
	envoyConfig.LetsencryptClusterIssuer = data.LetsencryptClusterIssuer.ValueStringPointer()
	if !data.AdditionalDNSNames.IsNull() {
		var dnsNames []string
		diags := data.AdditionalDNSNames.ElementsAs(ctx, &dnsNames, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		envoyConfig.AdditionalDnsNames = dnsNames
	}
	createReq.Specs.Config = &serverv1.GatewayProviderConfig{
		Config: &serverv1.GatewayProviderConfig_Envoy{
			Envoy: envoyConfig,
		},
	}

	// Convert IP allowlist
	if !data.IPAllowlist.IsNull() {
		var ipList []string
		diags := data.IPAllowlist.ElementsAs(ctx, &ipList, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Specs.IpAllowlist = ipList
	}

	// Convert TLS certificate
	if data.TLSCertificate != nil {
		createReq.Specs.TlsCertificate = &serverv1.TLSCertificateConfig{
			CertificateSource: &serverv1.TLSCertificateConfig_ManualCertificate{
				ManualCertificate: &serverv1.TLSManualCertificateRef{
					SecretName:      data.TLSCertificate.SecretName.ValueString(),
					SecretNamespace: data.TLSCertificate.SecretNamespace.ValueString(),
				},
			},
		}
	}

	// Convert service annotations
	if !data.ServiceAnnotations.IsNull() {
		annotations := make(map[string]string)
		diags := data.ServiceAnnotations.ElementsAs(ctx, &annotations, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Specs.ServiceAnnotations = annotations
	}

	// Set routing
	createReq.Specs.Routing = data.Routing.ValueStringPointer()

	// Use the known ID from the current state for the upsert
	createReq.Id = data.Id.ValueStringPointer()

	createReq.KubeClusterId = data.KubeClusterId.ValueStringPointer()

	response, err := bc.CreateClusterGateway(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Chalk Cluster Gateway",
			fmt.Sprintf("Could not update cluster gateway: %v", err),
		)
		return
	}

	data.Id = types.StringValue(response.Msg.Id)

	// Update the model with the returned specs (including computed/default values)
	r.updateModelFromSpecs(ctx, &data, response.Msg.Specs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "updated a chalk_cluster_gateway resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterGatewayResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Note: According to the proto definition, there's no DeleteClusterGateway method
	// This means the gateway lifecycle might be managed differently
	// For now, we'll just remove it from Terraform state
	tflog.Trace(ctx, "cluster gateway deletion - removing from terraform state only (no API delete available)")
}

func (r *ClusterGatewayResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
