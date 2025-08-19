package provider

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
)

var _ datasource.DataSource = &ClusterGatewayDataSource{}

func NewClusterGatewayDataSource() datasource.DataSource {
	return &ClusterGatewayDataSource{}
}

type ClusterGatewayDataSource struct {
	client *ChalkClient
}

type ClusterGatewayDataSourceModel struct {
	Id                       types.String                `tfsdk:"id"`
	EnvironmentId            types.String                `tfsdk:"environment_id"`
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

func (d *ClusterGatewayDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_gateway"
}

func (d *ClusterGatewayDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source for Chalk cluster gateway",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Gateway ID",
				Computed:            true,
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "Environment ID to fetch gateway for",
				Required:            true,
			},
			"namespace": schema.StringAttribute{
				MarkdownDescription: "Kubernetes namespace",
				Computed:            true,
			},
			"gateway_name": schema.StringAttribute{
				MarkdownDescription: "Gateway name",
				Computed:            true,
			},
			"gateway_class_name": schema.StringAttribute{
				MarkdownDescription: "Gateway class name",
				Computed:            true,
			},
			"listeners": schema.ListNestedAttribute{
				MarkdownDescription: "Gateway listeners",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"port": schema.Int64Attribute{
							MarkdownDescription: "Listener port",
							Computed:            true,
						},
						"protocol": schema.StringAttribute{
							MarkdownDescription: "Listener protocol",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "Listener name",
							Computed:            true,
						},
						"allowed_routes": schema.SingleNestedAttribute{
							MarkdownDescription: "Allowed routes configuration",
							Computed:            true,
							Attributes: map[string]schema.Attribute{
								"namespaces": schema.SingleNestedAttribute{
									MarkdownDescription: "Namespaces configuration",
									Computed:            true,
									Attributes: map[string]schema.Attribute{
										"from": schema.StringAttribute{
											MarkdownDescription: "From selector",
											Computed:            true,
										},
									},
								},
							},
						},
					},
				},
			},
			"config": schema.SingleNestedAttribute{
				MarkdownDescription: "Gateway provider configuration",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						MarkdownDescription: "Provider type (envoy or gcp)",
						Computed:            true,
					},
					"timeout_duration": schema.StringAttribute{
						MarkdownDescription: "Timeout duration (Envoy)",
						Computed:            true,
					},
					"dns_hostname": schema.StringAttribute{
						MarkdownDescription: "DNS hostname",
						Computed:            true,
					},
					"replicas": schema.Int64Attribute{
						MarkdownDescription: "Number of replicas (Envoy)",
						Computed:            true,
					},
					"min_available": schema.Int64Attribute{
						MarkdownDescription: "Minimum available replicas (Envoy)",
						Computed:            true,
					},
					"letsencrypt_cluster_issuer": schema.StringAttribute{
						MarkdownDescription: "LetsEncrypt cluster issuer (Envoy)",
						Computed:            true,
					},
					"additional_dns_names": schema.ListAttribute{
						MarkdownDescription: "Additional DNS names (Envoy)",
						Computed:            true,
						ElementType:         types.StringType,
					},
				},
			},
			"include_chalk_node_selector": schema.BoolAttribute{
				MarkdownDescription: "Include chalk node selector",
				Computed:            true,
			},
			"ip_allowlist": schema.ListAttribute{
				MarkdownDescription: "IP allowlist",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"tls_certificate": schema.SingleNestedAttribute{
				MarkdownDescription: "TLS certificate configuration",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"secret_name": schema.StringAttribute{
						MarkdownDescription: "Secret name",
						Computed:            true,
					},
					"secret_namespace": schema.StringAttribute{
						MarkdownDescription: "Secret namespace",
						Computed:            true,
					},
				},
			},
			"service_annotations": schema.MapAttribute{
				MarkdownDescription: "Service annotations",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"load_balancer_class": schema.StringAttribute{
				MarkdownDescription: "Load balancer class",
				Computed:            true,
			},
			"cluster_gateway_id": schema.StringAttribute{
				MarkdownDescription: "Cluster gateway ID",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Update timestamp",
				Computed:            true,
			},
		},
	}
}

func (d *ClusterGatewayDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ChalkClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ChalkClient, got: %T", req.ProviderData),
		)
		return
	}

	d.client = client
}

type GatewayListenerModel struct {
	Port          types.Int64                      `tfsdk:"port"`
	Protocol      types.String                     `tfsdk:"protocol"`
	Name          types.String                     `tfsdk:"name"`
	AllowedRoutes *GatewayAllowedRoutesModel       `tfsdk:"allowed_routes"`
}

type GatewayAllowedRoutesModel struct {
	Namespaces *GatewayAllowedNamespacesModel `tfsdk:"namespaces"`
}

type GatewayAllowedNamespacesModel struct {
	From types.String `tfsdk:"from"`
}

func mapListenersFromProto(ctx context.Context, listeners []*serverv1.EnvoyGatewayListener) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	if len(listeners) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: getListenerAttrTypes()}), diags
	}

	var listenersList []GatewayListenerModel
	for _, listener := range listeners {
		listenerModel := GatewayListenerModel{
			Port:     types.Int64Value(int64(listener.Port)),
			Protocol: types.StringValue(listener.Protocol),
			Name:     types.StringValue(listener.Name),
		}
		if listener.AllowedRoutes != nil && listener.AllowedRoutes.Namespaces != nil {
			listenerModel.AllowedRoutes = &GatewayAllowedRoutesModel{
				Namespaces: &GatewayAllowedNamespacesModel{
					From: types.StringValue(listener.AllowedRoutes.Namespaces.From),
				},
			}
		}
		listenersList = append(listenersList, listenerModel)
	}

	listValue, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: getListenerAttrTypes()}, listenersList)
	return listValue, diags
}

func getListenerAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"port":           types.Int64Type,
		"protocol":       types.StringType,
		"name":           types.StringType,
		"allowed_routes": types.ObjectType{AttrTypes: getAllowedRoutesAttrTypes()},
	}
}

func getAllowedRoutesAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"namespaces": types.ObjectType{AttrTypes: getAllowedNamespacesAttrTypes()},
	}
}

func getAllowedNamespacesAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"from": types.StringType,
	}
}

func mapGatewayConfigFromProto(config *serverv1.GatewayProviderConfig) *GatewayProviderConfigModel {
	configModel := &GatewayProviderConfigModel{}
	
	if envoyConfig := config.GetEnvoy(); envoyConfig != nil {
		configModel.Type = types.StringValue("envoy")
		if envoyConfig.TimeoutDuration != nil {
			configModel.TimeoutDuration = types.StringValue(*envoyConfig.TimeoutDuration)
		}
		if envoyConfig.DnsHostname != nil {
			configModel.DNSHostname = types.StringValue(*envoyConfig.DnsHostname)
		}
		if envoyConfig.Replicas != nil {
			configModel.Replicas = types.Int64Value(int64(*envoyConfig.Replicas))
		}
		if envoyConfig.MinAvailable != nil {
			configModel.MinAvailable = types.Int64Value(int64(*envoyConfig.MinAvailable))
		}
		if envoyConfig.LetsencryptClusterIssuer != nil {
			configModel.LetsencryptClusterIssuer = types.StringValue(*envoyConfig.LetsencryptClusterIssuer)
		}
		if len(envoyConfig.AdditionalDnsNames) > 0 {
			names, _ := types.ListValueFrom(context.Background(), types.StringType, envoyConfig.AdditionalDnsNames)
			configModel.AdditionalDNSNames = names
		}
	} else if gcpConfig := config.GetGcp(); gcpConfig != nil {
		configModel.Type = types.StringValue("gcp")
		configModel.DNSHostname = types.StringValue(gcpConfig.DnsHostname)
	}
	
	return configModel
}

func (d *ClusterGatewayDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ClusterGatewayDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// First get auth token
	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         d.client.ApiServer,
			interceptors: []connect.Interceptor{},
		},
	)

	// Create builder client with token injection interceptor
	builderClient := NewBuilderClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       d.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeTokenInjectionInterceptor(authClient, d.client.ClientID, d.client.ClientSecret),
		},
	})
	
	// Get gateway
	getReq := connect.NewRequest(&serverv1.GetClusterGatewayRequest{
		EnvironmentId: data.EnvironmentId.ValueString(),
	})

	getResp, err := builderClient.GetClusterGateway(ctx, getReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read cluster gateway, got error: %s", err))
		return
	}

	// Map response to model
	data.Id = types.StringValue(getResp.Msg.Id)
	if getResp.Msg.CreatedAt != nil {
		data.CreatedAt = types.StringValue(getResp.Msg.CreatedAt.AsTime().String())
	}
	if getResp.Msg.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(getResp.Msg.UpdatedAt.AsTime().String())
	}

	// Map specs if available
	if getResp.Msg.Specs != nil {
		specs := getResp.Msg.Specs
		data.Namespace = types.StringValue(specs.Namespace)
		data.GatewayName = types.StringValue(specs.GatewayName)
		data.GatewayClassName = types.StringValue(specs.GatewayClassName)
		data.IncludeChalkNodeSelector = types.BoolValue(specs.IncludeChalkNodeSelector)

		// Map listeners
		listeners, diags := mapListenersFromProto(ctx, specs.Listeners)
		resp.Diagnostics.Append(diags...)
		data.Listeners = listeners

		// Map IP allowlist
		if len(specs.IpAllowlist) > 0 {
			ipList, diags := types.ListValueFrom(ctx, types.StringType, specs.IpAllowlist)
			resp.Diagnostics.Append(diags...)
			data.IPAllowlist = ipList
		}

		// Map config
		if specs.Config != nil {
			data.Config = mapGatewayConfigFromProto(specs.Config)
		}

		// Map TLS certificate
		if specs.TlsCertificate != nil && specs.TlsCertificate.GetManualCertificate() != nil {
			cert := specs.TlsCertificate.GetManualCertificate()
			data.TLSCertificate = &TLSCertificateConfigModel{
				SecretName:      types.StringValue(cert.SecretName),
				SecretNamespace: types.StringValue(cert.SecretNamespace),
			}
		}

		// Map service annotations
		if len(specs.ServiceAnnotations) > 0 {
			annotations, diags := types.MapValueFrom(ctx, types.StringType, specs.ServiceAnnotations)
			resp.Diagnostics.Append(diags...)
			data.ServiceAnnotations = annotations
		}

		if specs.LoadBalancerClass != nil {
			data.LoadBalancerClass = types.StringValue(*specs.LoadBalancerClass)
		}

		if specs.ClusterGatewayId != nil {
			data.ClusterGatewayId = types.StringValue(*specs.ClusterGatewayId)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	tflog.Trace(ctx, "read chalk_cluster_gateway data source")
}