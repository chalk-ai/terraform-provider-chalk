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
		// Echo the request specs back so computed fields stay consistent.
		storedSpecs = createReq.Specs
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
