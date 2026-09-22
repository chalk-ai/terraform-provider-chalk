package provider

import (
	"regexp"
	"testing"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func setupMockBuilderServerGateway(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	var (
		storedSpecs *serverv1.EnvoyGatewaySpecs
		storedKube  string
	)

	server.OnCreateClusterGateway().WithBehavior(func(req proto.Message) (proto.Message, error) {
		createReq := req.(*serverv1.CreateClusterGatewayRequest)
		// The API hydrates affinity only when creating a new gateway. Clone the
		// specs so captured requests still reflect what the provider sent.
		storedSpecs = proto.Clone(createReq.Specs).(*serverv1.EnvoyGatewaySpecs)
		if envoy := storedSpecs.GetConfig().GetEnvoy(); createReq.GetId() == "" && envoy != nil {
			if envoy.TrafficZonalAffinity == serverv1.TrafficZonalAffinity_TRAFFIC_ZONAL_AFFINITY_UNSPECIFIED {
				envoy.TrafficZonalAffinity = serverv1.TrafficZonalAffinity_TRAFFIC_ZONAL_AFFINITY_CROSS_ZONE
			}
		}
		storedKube = createReq.GetKubeClusterId()
		return &serverv1.CreateClusterGatewayResponse{Id: "test-gateway-id", Specs: storedSpecs}, nil
	})

	server.OnGetClusterGateway().WithBehavior(func(req proto.Message) (proto.Message, error) {
		if storedSpecs == nil {
			return nil, connect.NewError(connect.CodeNotFound, nil)
		}
		return &serverv1.GetClusterGatewayResponse{
			Id:            "test-gateway-id",
			Specs:         storedSpecs,
			KubeClusterId: &storedKube,
		}, nil
	})

	return server
}

func TestClusterGatewayCreate(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerGateway(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_cluster_gateway" "test" {
  kube_cluster_id = "test-kube-cluster"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_cluster_gateway.test", "id", "test-gateway-id"),
					resource.TestCheckResourceAttr("chalk_cluster_gateway.test", "kube_cluster_id", "test-kube-cluster"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("CreateClusterGateway")
						require.Len(t, captured, 1, "Expected exactly one CreateClusterGateway call")
						req := captured[0].(*serverv1.CreateClusterGatewayRequest)
						assert.Equal(t, "test-kube-cluster", req.GetKubeClusterId())
						return nil
					},
				),
			},
		},
	})
}

func TestClusterGatewayCertificateIssuerRef(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerGateway(t)

	config := func(issuerName string) string {
		return providerConfig(server.URL) + `
resource "chalk_cluster_gateway" "test" {
  kube_cluster_id = "test-kube-cluster"
  certificate_issuer_ref = {
    name  = "` + issuerName + `"
    kind  = "AWSPCAClusterIssuer"
    group = "awspca.cert-manager.io"
  }
}
`
	}

	checkIssuer := func(requestIndex int, expectedName string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			captured := server.GetCapturedRequests("CreateClusterGateway")
			require.Len(t, captured, requestIndex+1)
			req := captured[requestIndex].(*serverv1.CreateClusterGatewayRequest)
			issuer, err := certificateIssuerRefFromProto(req.Specs.GetConfig().GetEnvoy())
			require.NoError(t, err)
			require.NotNil(t, issuer)
			assert.Equal(t, expectedName, issuer.Name.ValueString())
			assert.Equal(t, "AWSPCAClusterIssuer", issuer.Kind.ValueString())
			assert.Equal(t, "awspca.cert-manager.io", issuer.Group.ValueString())
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config("corporate-pca"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_cluster_gateway.test", "certificate_issuer_ref.name", "corporate-pca"),
					resource.TestCheckResourceAttr("chalk_cluster_gateway.test", "certificate_issuer_ref.kind", "AWSPCAClusterIssuer"),
					resource.TestCheckResourceAttr("chalk_cluster_gateway.test", "certificate_issuer_ref.group", "awspca.cert-manager.io"),
					checkIssuer(0, "corporate-pca"),
				),
			},
			{
				Config: config("replacement-pca"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_cluster_gateway.test", "certificate_issuer_ref.name", "replacement-pca"),
					checkIssuer(1, "replacement-pca"),
				),
			},
		},
	})
}

func TestClusterGatewayCertificateIssuerRefConflictsWithLegacyIssuer(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerGateway(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_cluster_gateway" "test" {
  kube_cluster_id            = "test-kube-cluster"
  letsencrypt_cluster_issuer = "legacy-issuer"
  certificate_issuer_ref = {
    name  = "custom-issuer"
    kind  = "AWSPCAClusterIssuer"
    group = "awspca.cert-manager.io"
  }
}
`,
				ExpectError: regexp.MustCompile(".*cannot be configured together.*"),
			},
		},
	})
}

func clusterGatewayAffinityConfig(serverURL, affinity string) string {
	attribute := ""
	if affinity != "" {
		attribute = `traffic_zonal_affinity = "` + affinity + `"`
	}
	return providerConfig(serverURL) + `
resource "chalk_cluster_gateway" "test" {
  kube_cluster_id = "test-kube-cluster"
  ` + attribute + `
}
`
}

func TestClusterGatewayTrafficZonalAffinity(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerGateway(t)

	check := func(affinity string, expected serverv1.TrafficZonalAffinity, requests int) resource.TestCheckFunc {
		stateCheck := resource.TestCheckNoResourceAttr("chalk_cluster_gateway.test", "traffic_zonal_affinity")
		if affinity != "" {
			stateCheck = resource.TestCheckResourceAttr("chalk_cluster_gateway.test", "traffic_zonal_affinity", affinity)
		}
		return resource.ComposeAggregateTestCheckFunc(stateCheck, func(s *terraform.State) error {
			captured := server.GetCapturedRequests("CreateClusterGateway")
			require.Len(t, captured, requests)
			req := captured[requests-1].(*serverv1.CreateClusterGatewayRequest)
			assert.Equal(t, expected, req.Specs.GetConfig().GetEnvoy().GetTrafficZonalAffinity())
			if requests > 1 {
				assert.Equal(t, "test-gateway-id", req.GetId())
			}
			return nil
		})
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				// The API returns CROSS_ZONE, but omitted configuration must stay null.
				Config: clusterGatewayAffinityConfig(server.URL, ""),
				Check:  check("", serverv1.TrafficZonalAffinity_TRAFFIC_ZONAL_AFFINITY_UNSPECIFIED, 1),
			},
			{
				Config:   clusterGatewayAffinityConfig(server.URL, ""),
				PlanOnly: true,
			},
			{
				ResourceName:      "chalk_cluster_gateway.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: clusterGatewayAffinityConfig(server.URL, "LOCAL"),
				Check:  check("LOCAL", serverv1.TrafficZonalAffinity_TRAFFIC_ZONAL_AFFINITY_LOCAL, 2),
			},
			{
				Config: clusterGatewayAffinityConfig(server.URL, "CROSS_ZONE"),
				Check:  check("CROSS_ZONE", serverv1.TrafficZonalAffinity_TRAFFIC_ZONAL_AFFINITY_CROSS_ZONE, 3),
			},
			{
				// Removing the setting must send UNSPECIFIED and clear Terraform state.
				Config: clusterGatewayAffinityConfig(server.URL, ""),
				Check:  check("", serverv1.TrafficZonalAffinity_TRAFFIC_ZONAL_AFFINITY_UNSPECIFIED, 4),
			},
			{
				Config:   clusterGatewayAffinityConfig(server.URL, ""),
				PlanOnly: true,
			},
		},
	})
}

func TestClusterGatewayTrafficZonalAffinityDrift(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerGateway(t)
	config := clusterGatewayAffinityConfig(server.URL, "LOCAL")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  resource.TestCheckResourceAttr("chalk_cluster_gateway.test", "traffic_zonal_affinity", "LOCAL"),
			},
			{
				PreConfig: func() {
					captured := server.GetCapturedRequests("CreateClusterGateway")
					require.Len(t, captured, 1)
					specs := proto.Clone(captured[0].(*serverv1.CreateClusterGatewayRequest).Specs).(*serverv1.EnvoyGatewaySpecs)
					specs.GetConfig().GetEnvoy().TrafficZonalAffinity = serverv1.TrafficZonalAffinity_TRAFFIC_ZONAL_AFFINITY_CROSS_ZONE
					server.OnGetClusterGateway().WithBehavior(func(req proto.Message) (proto.Message, error) {
						return &serverv1.GetClusterGatewayResponse{
							Id:            "test-gateway-id",
							Specs:         specs,
							KubeClusterId: new("test-kube-cluster"),
						}, nil
					})
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestClusterGatewayTrafficZonalAffinityValidation(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerGateway(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_cluster_gateway" "test" {
  kube_cluster_id        = "test-kube-cluster"
  traffic_zonal_affinity = "INVALID"
}
`,
				ExpectError: regexp.MustCompile(`Attribute traffic_zonal_affinity value must be one of`),
			},
		},
	})
}

func TestClusterGatewayDelete(t *testing.T) {
	t.Parallel()
	server := setupMockBuilderServerGateway(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_cluster_gateway" "test" {
  kube_cluster_id = "test-kube-cluster"
}
`,
				Check: resource.TestCheckResourceAttr("chalk_cluster_gateway.test", "id", "test-gateway-id"),
			},
			{
				// Removing the resource triggers Delete, which must call
				// DeleteClusterGateway with the stored id.
				Config: providerConfig(server.URL),
				Check: func(s *terraform.State) error {
					captured := server.GetCapturedRequests("DeleteClusterGateway")
					require.Len(t, captured, 1, "Expected exactly one DeleteClusterGateway call")
					req := captured[0].(*serverv1.DeleteClusterGatewayRequest)
					assert.Equal(t, "test-gateway-id", req.GetId())
					return nil
				},
			},
		},
	})
}
