package provider

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"net/http"
)

var _ resource.Resource = &ClusterGatewayResource{}
var _ resource.ResourceWithImportState = &ClusterGatewayResource{}

func NewClusterGatewayResource() resource.Resource {
	return &ClusterGatewayResource{}
}

type ClusterGatewayResource struct {
	client *ChalkClient
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

type GatewayProviderConfigModel struct {
	Type                     types.String `tfsdk:"type"`
	TimeoutDuration          types.String `tfsdk:"timeout_duration"`
	DNSHostname              types.String `tfsdk:"dns_hostname"`
	Replicas                 types.Int64  `tfsdk:"replicas"`
	MinAvailable             types.Int64  `tfsdk:"min_available"`
	LetsencryptClusterIssuer types.String `tfsdk:"letsencrypt_cluster_issuer"`
	AdditionalDNSNames       types.List   `tfsdk:"additional_dns_names"`
}

type ClusterGatewayResourceModel struct {
	Id                       types.String                `tfsdk:"id"`
	EnvironmentIds           types.List                  `tfsdk:"environment_ids"`
	Namespace                types.String                `tfsdk:"namespace"`
	GatewayName              types.String                `tfsdk:"gateway_name"`
	GatewayClassName         types.String                `tfsdk:"gateway_class_name"`
	Listeners                types.List                  `tfsdk:"listeners"`
	Config                   *GatewayProviderConfigModel `tfsdk:"config"`
	IncludeChalkNodeSelector types.Bool                  `tfsdk:"include_chalk_node_selector"`
	IPAllowlist              types.List                  `tfsdk:"ip_allowlist"`
	TLSCertificate           *TLSCertificateConfigModel  `tfsdk:"tls_certificate"`
	ServiceAnnotations       types.Map                   `tfsdk:"service_annotations"`
	LoadBalancerClass        types.String                `tfsdk:"load_balancer_class"`
	ClusterGatewayId         types.String                `tfsdk:"cluster_gateway_id"`
	CreatedAt                types.String                `tfsdk:"created_at"`
	UpdatedAt                types.String                `tfsdk:"updated_at"`
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
			"environment_ids": schema.ListAttribute{
				MarkdownDescription: "List of environment IDs for the gateway",
				Required:            true,
				ElementType:         types.StringType,
			},
			"namespace": schema.StringAttribute{
				MarkdownDescription: "Kubernetes namespace for the gateway",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"gateway_name": schema.StringAttribute{
				MarkdownDescription: "Name of the gateway",
				Required:            true,
			},
			"gateway_class_name": schema.StringAttribute{
				MarkdownDescription: "Gateway class name",
				Required:            true,
			},
			"listeners": schema.ListNestedAttribute{
				MarkdownDescription: "Gateway listeners configuration",
				Required:            true,
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
			"config": schema.SingleNestedAttribute{
				MarkdownDescription: "Gateway provider configuration",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						MarkdownDescription: "Provider type (envoy or gcp)",
						Required:            true,
					},
					"timeout_duration": schema.StringAttribute{
						MarkdownDescription: "Timeout duration for Envoy gateway",
						Optional:            true,
					},
					"dns_hostname": schema.StringAttribute{
						MarkdownDescription: "DNS hostname",
						Optional:            true,
					},
					"replicas": schema.Int64Attribute{
						MarkdownDescription: "Number of replicas for Envoy gateway",
						Optional:            true,
						Computed:            true,
						Default:             int64default.StaticInt64(1),
					},
					"min_available": schema.Int64Attribute{
						MarkdownDescription: "Minimum available replicas for Envoy gateway",
						Optional:            true,
						Computed:            true,
						Default:             int64default.StaticInt64(1),
					},
					"letsencrypt_cluster_issuer": schema.StringAttribute{
						MarkdownDescription: "Let's Encrypt cluster issuer for Envoy gateway",
						Optional:            true,
					},
					"additional_dns_names": schema.ListAttribute{
						MarkdownDescription: "Additional DNS names for Envoy gateway",
						Optional:            true,
						ElementType:         types.StringType,
					},
				},
			},
			"include_chalk_node_selector": schema.BoolAttribute{
				MarkdownDescription: "Whether to include chalk node selector",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
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
			"load_balancer_class": schema.StringAttribute{
				MarkdownDescription: "Load balancer class",
				Optional:            true,
			},
			"cluster_gateway_id": schema.StringAttribute{
				MarkdownDescription: "Cluster gateway ID",
				Optional:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp",
				Computed:            true,
			},
		},
	}
}

func (r *ClusterGatewayResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ChalkClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ChalkClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *ClusterGatewayResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ClusterGatewayResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         r.client.ApiServer,
			interceptors: []connect.Interceptor{MakeApiServerHeaderInterceptor("x-chalk-server", "go-api")},
		},
	)

	// Create builder client with token injection interceptor
	bc := NewBuilderClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	})

	// Convert terraform model to proto request
	createReq := &serverv1.CreateClusterGatewayRequest{
		Specs: &serverv1.EnvoyGatewaySpecs{
			Namespace:                data.Namespace.ValueString(),
			GatewayName:              data.GatewayName.ValueString(),
			GatewayClassName:         data.GatewayClassName.ValueString(),
			IncludeChalkNodeSelector: data.IncludeChalkNodeSelector.ValueBool(),
		},
	}

	// Convert environment IDs
	var envIds []string
	diags := data.EnvironmentIds.ElementsAs(ctx, &envIds, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	createReq.EnvironmentIds = envIds

	// Convert listeners
	var listeners []EnvoyGatewayListenerModel
	diags = data.Listeners.ElementsAs(ctx, &listeners, false)
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

	// Convert provider config
	if data.Config != nil {
		switch data.Config.Type.ValueString() {
		case "envoy":
			envoyConfig := &serverv1.EnvoyGatewayProviderConfig{}
			if !data.Config.TimeoutDuration.IsNull() {
				val := data.Config.TimeoutDuration.ValueString()
				envoyConfig.TimeoutDuration = &val
			}
			if !data.Config.DNSHostname.IsNull() {
				val := data.Config.DNSHostname.ValueString()
				envoyConfig.DnsHostname = &val
			}
			if !data.Config.Replicas.IsNull() {
				val := int32(data.Config.Replicas.ValueInt64())
				envoyConfig.Replicas = &val
			}
			if !data.Config.MinAvailable.IsNull() {
				val := int32(data.Config.MinAvailable.ValueInt64())
				envoyConfig.MinAvailable = &val
			}
			if !data.Config.LetsencryptClusterIssuer.IsNull() {
				val := data.Config.LetsencryptClusterIssuer.ValueString()
				envoyConfig.LetsencryptClusterIssuer = &val
			}
			if !data.Config.AdditionalDNSNames.IsNull() {
				var dnsNames []string
				diags = data.Config.AdditionalDNSNames.ElementsAs(ctx, &dnsNames, false)
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
		case "gcp":
			gcpConfig := &serverv1.GCPGatewayProviderConfig{
				DnsHostname: data.Config.DNSHostname.ValueString(),
			}
			createReq.Specs.Config = &serverv1.GatewayProviderConfig{
				Config: &serverv1.GatewayProviderConfig_Gcp{
					Gcp: gcpConfig,
				},
			}
		}
	}

	// Convert IP allowlist
	if !data.IPAllowlist.IsNull() {
		var ipList []string
		diags = data.IPAllowlist.ElementsAs(ctx, &ipList, false)
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
		diags = data.ServiceAnnotations.ElementsAs(ctx, &annotations, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Specs.ServiceAnnotations = annotations
	}

	// Set optional fields
	if !data.LoadBalancerClass.IsNull() {
		val := data.LoadBalancerClass.ValueString()
		createReq.Specs.LoadBalancerClass = &val
	}
	if !data.ClusterGatewayId.IsNull() {
		val := data.ClusterGatewayId.ValueString()
		createReq.Specs.ClusterGatewayId = &val
	}

	_, err := bc.CreateClusterGateway(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Cluster Gateway",
			fmt.Sprintf("Could not create cluster gateway: %v", err),
		)
		return
	}

	// Since CreateClusterGateway doesn't return the created gateway, we need to get it
	// We'll use the first environment ID to query for the gateway
	if len(envIds) > 0 {
		getReq := &serverv1.GetClusterGatewayRequest{
			EnvironmentId: envIds[0],
		}

		gateway, err := bc.GetClusterGateway(ctx, connect.NewRequest(getReq))
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Created Chalk Cluster Gateway",
				fmt.Sprintf("Gateway was created but could not be read: %v", err),
			)
			return
		}

		// Update with created values
		data.Id = types.StringValue(gateway.Msg.Id)
		if gateway.Msg.CreatedAt != nil {
			data.CreatedAt = types.StringValue(gateway.Msg.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"))
		}
		if gateway.Msg.UpdatedAt != nil {
			data.UpdatedAt = types.StringValue(gateway.Msg.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z"))
		}
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
	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         r.client.ApiServer,
			interceptors: []connect.Interceptor{MakeApiServerHeaderInterceptor("x-chalk-server", "go-api")},
		},
	)

	// Create builder client with token injection interceptor
	bc := NewBuilderClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	})

	// Get the first environment ID to query the gateway
	var envIds []string
	diags := data.EnvironmentIds.ElementsAs(ctx, &envIds, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(envIds) == 0 {
		resp.Diagnostics.AddError(
			"No Environment IDs",
			"No environment IDs found in state for reading cluster gateway",
		)
		return
	}

	getReq := &serverv1.GetClusterGatewayRequest{
		EnvironmentId: envIds[0],
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
	specs := gateway.Msg.Specs
	if specs != nil {
		data.Namespace = types.StringValue(specs.Namespace)
		data.GatewayName = types.StringValue(specs.GatewayName)
		data.GatewayClassName = types.StringValue(specs.GatewayClassName)
		data.IncludeChalkNodeSelector = types.BoolValue(specs.IncludeChalkNodeSelector)

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

		// Update optional string fields
		if specs.LoadBalancerClass != nil {
			data.LoadBalancerClass = types.StringValue(*specs.LoadBalancerClass)
		}
		if specs.ClusterGatewayId != nil {
			data.ClusterGatewayId = types.StringValue(*specs.ClusterGatewayId)
		}
	}

	if gateway.Msg.CreatedAt != nil {
		data.CreatedAt = types.StringValue(gateway.Msg.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"))
	}
	if gateway.Msg.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(gateway.Msg.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z"))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterGatewayResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Note: According to the proto definition, there's no UpdateClusterGateway method
	// So we'll need to delete and recreate, but this would be disruptive
	// For now, we'll return an error indicating updates are not supported
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Cluster gateway updates are not supported by the Chalk API. Please recreate the resource if changes are needed.",
	)
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
