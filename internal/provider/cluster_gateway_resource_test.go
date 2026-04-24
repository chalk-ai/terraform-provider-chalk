package provider

import (
	"context"
	"testing"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gwStrPtr(s string) *string { return &s }
func gwInt32Ptr(i int32) *int32 { return &i }

// TestUpdateModelFromSpecs_RoutingPopulation covers the INF-1288 bug surface.
//
// The bug: Read populates state from the server response via updateModelFromSpecs.
// When the server returns a nil routing (which happens for ~60% of production
// gateways per /tmp/chalk_public_cluster_gateways.json — 54/89 cases),
// state.Routing ended up null. The schema declares routing as Optional+Computed
// with Default "PUBLIC", but Default only applies at plan time when the planned
// value is null — it does not populate state on Read. So state stayed null, HCL
// said "PUBLIC", and every plan showed `+ routing = "PUBLIC"`.
//
// The fix: when the server returns nil routing, populate state with "PUBLIC" —
// the same value the schema Default emits — so state and config agree.
func TestUpdateModelFromSpecs_RoutingPopulation(t *testing.T) {
	t.Parallel()
	r := &ClusterGatewayResource{}

	tests := []struct {
		name     string
		routing  *string
		expected types.String
	}{
		// 54/89 production gateways — the reproducer from the Linear issue.
		{"nil routing defaults to PUBLIC", nil, types.StringValue("PUBLIC")},
		// 29/89 production gateways.
		{"explicit PUBLIC", gwStrPtr("PUBLIC"), types.StringValue("PUBLIC")},
		// 6/89 production gateways.
		{"explicit PRIVATE", gwStrPtr("PRIVATE"), types.StringValue("PRIVATE")},
		// Valid value documented in the schema but not currently observed in prod.
		{"explicit PRIVATELINK", gwStrPtr("PRIVATELINK"), types.StringValue("PRIVATELINK")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var diags diag.Diagnostics
			data := &ClusterGatewayResourceModel{}
			specs := &serverv1.EnvoyGatewaySpecs{Routing: tc.routing}

			r.updateModelFromSpecs(context.Background(), data, specs, &diags)

			require.False(t, diags.HasError(), "diags: %s", diags)
			assert.Equal(t, tc.expected, data.Routing)
		})
	}
}

// TestUpdateModelFromSpecs_LoadBalancerClass documents the current behaviour for
// the other optional string field populated from the response: nil server value
// resolves to null state. Acceptable because there is no schema Default for
// load_balancer_class and the attribute is Optional-only.
func TestUpdateModelFromSpecs_LoadBalancerClass(t *testing.T) {
	t.Parallel()
	r := &ClusterGatewayResource{}

	tests := []struct {
		name     string
		lbClass  *string
		expected types.String
	}{
		{"nil stays null", nil, types.StringNull()},
		{"explicit service.k8s.aws/nlb", gwStrPtr("service.k8s.aws/nlb"), types.StringValue("service.k8s.aws/nlb")},
		{"explicit eks.amazonaws.com/nlb", gwStrPtr("eks.amazonaws.com/nlb"), types.StringValue("eks.amazonaws.com/nlb")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var diags diag.Diagnostics
			data := &ClusterGatewayResourceModel{}
			specs := &serverv1.EnvoyGatewaySpecs{LoadBalancerClass: tc.lbClass}

			r.updateModelFromSpecs(context.Background(), data, specs, &diags)

			require.False(t, diags.HasError(), "diags: %s", diags)
			assert.Equal(t, tc.expected, data.LoadBalancerClass)
		})
	}
}

// TestUpdateModelFromSpecs_NilSpecs guards against a nil *EnvoyGatewaySpecs
// (e.g. a server response that omits specs). The helper should no-op.
func TestUpdateModelFromSpecs_NilSpecs(t *testing.T) {
	t.Parallel()
	r := &ClusterGatewayResource{}
	var diags diag.Diagnostics
	data := &ClusterGatewayResourceModel{Routing: types.StringValue("PRIVATE")}

	r.updateModelFromSpecs(context.Background(), data, nil, &diags)

	require.False(t, diags.HasError())
	assert.Equal(t, types.StringValue("PRIVATE"), data.Routing)
}

// TestUpdateModelFromSpecs_CommonProductionShapes exercises representative specs
// seen in /tmp/chalk_public_cluster_gateways.json (89 gateways total). It is a
// regression guard: the fields the provider models today must round-trip through
// updateModelFromSpecs without loss, and the routing default must hold across
// config shapes.
func TestUpdateModelFromSpecs_CommonProductionShapes(t *testing.T) {
	t.Parallel()
	r := &ClusterGatewayResource{}

	// Shape 1: most common — nwst-style, envoy config, no routing/TLS/IPs.
	t.Run("envoy minimal no routing", func(t *testing.T) {
		t.Parallel()
		var diags diag.Diagnostics
		data := &ClusterGatewayResourceModel{}
		specs := &serverv1.EnvoyGatewaySpecs{
			Namespace:        "chalk-envoy",
			GatewayName:      "chalk-gw",
			GatewayClassName: "chalk-gw-class",
			Listeners: []*serverv1.EnvoyGatewayListener{
				{
					Port:     80,
					Protocol: "HTTP",
					Name:     "http",
					AllowedRoutes: &serverv1.EnvoyGatewayAllowedRoutes{
						Namespaces: &serverv1.EnvoyGatewayAllowedNamespaces{From: "All"},
					},
				},
			},
			Config: &serverv1.GatewayProviderConfig{
				Config: &serverv1.GatewayProviderConfig_Envoy{
					Envoy: &serverv1.EnvoyGatewayProviderConfig{
						TimeoutDuration: gwStrPtr("300s"),
					},
				},
			},
		}

		r.updateModelFromSpecs(context.Background(), data, specs, &diags)
		require.False(t, diags.HasError(), "diags: %s", diags)

		assert.Equal(t, types.StringValue("chalk-envoy"), data.Namespace)
		assert.Equal(t, types.StringValue("chalk-gw"), data.GatewayName)
		assert.Equal(t, types.StringValue("chalk-gw-class"), data.GatewayClassName)
		assert.Equal(t, types.StringValue("300s"), data.TimeoutDuration)
		// The bug fix: missing server routing resolves to PUBLIC.
		assert.Equal(t, types.StringValue("PUBLIC"), data.Routing)
	})

	// Shape 2: full AWS public gateway with TLS-less listener, IP allowlist,
	// service annotations and load balancer class. Matches the gahid/fetch-style
	// sample in the prod dump.
	t.Run("envoy public with annotations and allowlist", func(t *testing.T) {
		t.Parallel()
		var diags diag.Diagnostics
		data := &ClusterGatewayResourceModel{}
		specs := &serverv1.EnvoyGatewaySpecs{
			Namespace:        "chalk-envoy",
			GatewayName:      "chalk-gateway",
			GatewayClassName: "chalk-gateway-class",
			Config: &serverv1.GatewayProviderConfig{
				Config: &serverv1.GatewayProviderConfig_Envoy{
					Envoy: &serverv1.EnvoyGatewayProviderConfig{
						TimeoutDuration:          gwStrPtr("300s"),
						DnsHostname:              gwStrPtr("gahid.fetch.example.chalk.ai"),
						Replicas:                 gwInt32Ptr(2),
						MinAvailable:             gwInt32Ptr(1),
						LetsencryptClusterIssuer: gwStrPtr("chalk-letsencrypt-issuer"),
					},
				},
			},
			IpAllowlist: []string{"203.0.113.1/32", "203.0.113.2/32"},
			ServiceAnnotations: map[string]string{
				"external-dns.alpha.kubernetes.io/hostname":         "*.gahid.fetch.example.chalk.ai",
				"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
			},
			LoadBalancerClass: gwStrPtr("service.k8s.aws/nlb"),
		}

		r.updateModelFromSpecs(context.Background(), data, specs, &diags)
		require.False(t, diags.HasError(), "diags: %s", diags)

		assert.Equal(t, types.StringValue("gahid.fetch.example.chalk.ai"), data.DNSHostname)
		assert.Equal(t, types.Int64Value(2), data.Replicas)
		assert.Equal(t, types.Int64Value(1), data.MinAvailable)
		assert.Equal(t, types.StringValue("chalk-letsencrypt-issuer"), data.LetsencryptClusterIssuer)
		assert.Equal(t, types.StringValue("service.k8s.aws/nlb"), data.LoadBalancerClass)
		// nil routing -> default PUBLIC
		assert.Equal(t, types.StringValue("PUBLIC"), data.Routing)

		ips := []string{}
		data.IPAllowlist.ElementsAs(context.Background(), &ips, false)
		assert.Equal(t, []string{"203.0.113.1/32", "203.0.113.2/32"}, ips)

		annots := map[string]string{}
		data.ServiceAnnotations.ElementsAs(context.Background(), &annots, false)
		assert.Equal(t, 2, len(annots))
	})

	// Shape 3: private routing explicitly set. 6/89 prod gateways.
	t.Run("envoy private routing preserved", func(t *testing.T) {
		t.Parallel()
		var diags diag.Diagnostics
		data := &ClusterGatewayResourceModel{}
		specs := &serverv1.EnvoyGatewaySpecs{
			Namespace:         "chalk-envoy",
			GatewayName:       "chalk-gw-internal",
			GatewayClassName:  "chalk-gw-internal-class",
			Routing:           gwStrPtr("PRIVATE"),
			LoadBalancerClass: gwStrPtr("eks.amazonaws.com/nlb"),
		}

		r.updateModelFromSpecs(context.Background(), data, specs, &diags)
		require.False(t, diags.HasError(), "diags: %s", diags)

		assert.Equal(t, types.StringValue("PRIVATE"), data.Routing)
		assert.Equal(t, types.StringValue("eks.amazonaws.com/nlb"), data.LoadBalancerClass)
	})

	// Shape 4: TLS manual certificate with HTTPS listener. Matches the eg/egc
	// sample in the prod dump.
	t.Run("tls manual certificate", func(t *testing.T) {
		t.Parallel()
		var diags diag.Diagnostics
		data := &ClusterGatewayResourceModel{}
		specs := &serverv1.EnvoyGatewaySpecs{
			Namespace:        "chalk-envoy",
			GatewayName:      "eg",
			GatewayClassName: "egc",
			Listeners: []*serverv1.EnvoyGatewayListener{
				{
					Port:     443,
					Protocol: "HTTPS",
					Name:     "gateway-listener",
					AllowedRoutes: &serverv1.EnvoyGatewayAllowedRoutes{
						Namespaces: &serverv1.EnvoyGatewayAllowedNamespaces{From: "All"},
					},
				},
			},
			TlsCertificate: &serverv1.TLSCertificateConfig{
				CertificateSource: &serverv1.TLSCertificateConfig_ManualCertificate{
					ManualCertificate: &serverv1.TLSManualCertificateRef{
						SecretName:      "envoy-gateway-frontend-tls",
						SecretNamespace: "default",
					},
				},
			},
		}

		r.updateModelFromSpecs(context.Background(), data, specs, &diags)
		require.False(t, diags.HasError(), "diags: %s", diags)

		require.NotNil(t, data.TLSCertificate)
		assert.Equal(t, types.StringValue("envoy-gateway-frontend-tls"), data.TLSCertificate.SecretName)
		assert.Equal(t, types.StringValue("default"), data.TLSCertificate.SecretNamespace)
		assert.Equal(t, types.StringValue("PUBLIC"), data.Routing)
	})

	// Shape 5: TCP listener. 7/89 prod gateways use TCP listeners.
	t.Run("envoy tcp listener", func(t *testing.T) {
		t.Parallel()
		var diags diag.Diagnostics
		data := &ClusterGatewayResourceModel{}
		specs := &serverv1.EnvoyGatewaySpecs{
			Namespace:        "chalk-envoy",
			GatewayName:      "chalk-gw",
			GatewayClassName: "chalk-gw-class",
			Listeners: []*serverv1.EnvoyGatewayListener{
				{
					Port:     5432,
					Protocol: "TCP",
					Name:     "pg",
					AllowedRoutes: &serverv1.EnvoyGatewayAllowedRoutes{
						Namespaces: &serverv1.EnvoyGatewayAllowedNamespaces{From: "All"},
					},
				},
			},
		}

		r.updateModelFromSpecs(context.Background(), data, specs, &diags)
		require.False(t, diags.HasError(), "diags: %s", diags)

		listeners := []EnvoyGatewayListenerModel{}
		data.Listeners.ElementsAs(context.Background(), &listeners, false)
		require.Len(t, listeners, 1)
		assert.Equal(t, types.StringValue("TCP"), listeners[0].Protocol)
		assert.Equal(t, types.Int64Value(5432), listeners[0].Port)
	})

	// Shape 6: envoy config with additional_dns_names, nodepool, colocation.
	// Covers the envoy fields the provider does model end-to-end.
	t.Run("envoy advanced config", func(t *testing.T) {
		t.Parallel()
		var diags diag.Diagnostics
		data := &ClusterGatewayResourceModel{}
		specs := &serverv1.EnvoyGatewaySpecs{
			Config: &serverv1.GatewayProviderConfig{
				Config: &serverv1.GatewayProviderConfig_Envoy{
					Envoy: &serverv1.EnvoyGatewayProviderConfig{
						AdditionalDnsNames:                []string{"alt1.example.com", "alt2.example.com"},
						Nodepool:                          "envoy-pool",
						AllowColocationWithChalkWorkloads: true,
					},
				},
			},
		}

		r.updateModelFromSpecs(context.Background(), data, specs, &diags)
		require.False(t, diags.HasError(), "diags: %s", diags)

		dns := []string{}
		data.AdditionalDNSNames.ElementsAs(context.Background(), &dns, false)
		assert.Equal(t, []string{"alt1.example.com", "alt2.example.com"}, dns)
		assert.Equal(t, types.StringValue("envoy-pool"), data.Nodepool)
		assert.Equal(t, types.BoolValue(true), data.AllowCollocationWithChalkWorkloads)
	})

	// Shape 7: empty listeners list. 50/89 prod gateways have no listeners.
	t.Run("no listeners", func(t *testing.T) {
		t.Parallel()
		var diags diag.Diagnostics
		data := &ClusterGatewayResourceModel{}
		specs := &serverv1.EnvoyGatewaySpecs{
			Namespace:        "chalk-envoy",
			GatewayName:      "chalk-gw",
			GatewayClassName: "chalk-gw-class",
		}

		r.updateModelFromSpecs(context.Background(), data, specs, &diags)
		require.False(t, diags.HasError(), "diags: %s", diags)

		assert.True(t, data.Listeners.IsNull(), "expected listeners to remain null when spec has none")
	})
}
