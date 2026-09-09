package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/gen/chalk/server/v1/serverv1connect"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"google.golang.org/protobuf/proto"
)

type servicesGatewayBindingTestServer struct {
	*httptest.Server

	mu                  sync.Mutex
	exists              bool
	driftAfterFirstRead bool
	getCallCount        int
	createRequest       *serverv1.CreateBindingServicesGatewayRequest
	deleteRequest       *serverv1.DeleteBindingServicesGatewayRequest
}

func newServicesGatewayBindingTestServer(t *testing.T, driftAfterFirstRead bool) *servicesGatewayBindingTestServer {
	t.Helper()

	server := &servicesGatewayBindingTestServer{driftAfterFirstRead: driftAfterFirstRead}
	mux := http.NewServeMux()

	mux.Handle(serverv1connect.CloudComponentsServiceCreateBindingServicesGatewayProcedure, connect.NewUnaryHandler(
		serverv1connect.CloudComponentsServiceCreateBindingServicesGatewayProcedure,
		func(ctx context.Context, req *connect.Request[serverv1.CreateBindingServicesGatewayRequest]) (*connect.Response[serverv1.CreateBindingServicesGatewayResponse], error) {
			server.mu.Lock()
			defer server.mu.Unlock()
			server.exists = true
			server.createRequest = proto.Clone(req.Msg).(*serverv1.CreateBindingServicesGatewayRequest)
			return connect.NewResponse(&serverv1.CreateBindingServicesGatewayResponse{}), nil
		},
	))
	mux.Handle(serverv1connect.CloudComponentsServiceGetBindingServicesGatewayProcedure, connect.NewUnaryHandler(
		serverv1connect.CloudComponentsServiceGetBindingServicesGatewayProcedure,
		func(ctx context.Context, req *connect.Request[serverv1.GetBindingServicesGatewayRequest]) (*connect.Response[serverv1.GetBindingServicesGatewayResponse], error) {
			server.mu.Lock()
			defer server.mu.Unlock()
			server.getCallCount++
			if !server.exists || (server.driftAfterFirstRead && server.getCallCount > 1) {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("binding not found"))
			}
			return connect.NewResponse(&serverv1.GetBindingServicesGatewayResponse{
				ClusterId:         req.Msg.ClusterId,
				ServicesGatewayId: "test-services-gateway-id",
			}), nil
		},
	))
	mux.Handle(serverv1connect.CloudComponentsServiceDeleteBindingServicesGatewayProcedure, connect.NewUnaryHandler(
		serverv1connect.CloudComponentsServiceDeleteBindingServicesGatewayProcedure,
		func(ctx context.Context, req *connect.Request[serverv1.DeleteBindingServicesGatewayRequest]) (*connect.Response[serverv1.DeleteBindingServicesGatewayResponse], error) {
			server.mu.Lock()
			defer server.mu.Unlock()
			server.exists = false
			server.deleteRequest = proto.Clone(req.Msg).(*serverv1.DeleteBindingServicesGatewayRequest)
			return connect.NewResponse(&serverv1.DeleteBindingServicesGatewayResponse{}), nil
		},
	))

	server.Server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func servicesGatewayBindingProviderConfig(serverURL string) string {
	return fmt.Sprintf(`
provider "chalk" {
  api_server = %q
  jwt        = "test-jwt"
}
`, serverURL)
}

func TestServicesGatewayBindingCreateAndImport(t *testing.T) {
	t.Parallel()
	server := newServicesGatewayBindingTestServer(t, false)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: servicesGatewayBindingProviderConfig(server.URL) + `
resource "chalk_services_gateway_binding" "test" {
  cluster_id          = "test-cluster-id"
  services_gateway_id = "test-services-gateway-id"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_services_gateway_binding.test", "cluster_id", "test-cluster-id"),
					resource.TestCheckResourceAttr("chalk_services_gateway_binding.test", "services_gateway_id", "test-services-gateway-id"),
				),
			},
			{
				ResourceName:                         "chalk_services_gateway_binding.test",
				ImportState:                          true,
				ImportStateId:                        "test-cluster-id",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "cluster_id",
			},
		},
	})

	server.mu.Lock()
	defer server.mu.Unlock()
	if server.createRequest.GetClusterId() != "test-cluster-id" || server.createRequest.GetServicesGatewayId() != "test-services-gateway-id" {
		t.Fatalf("unexpected create request: %v", server.createRequest)
	}
	if server.deleteRequest.GetClusterId() != "test-cluster-id" {
		t.Fatalf("unexpected delete request: %v", server.deleteRequest)
	}
}

func TestServicesGatewayBindingReadNotFound(t *testing.T) {
	t.Parallel()
	server := newServicesGatewayBindingTestServer(t, true)

	config := servicesGatewayBindingProviderConfig(server.URL) + `
resource "chalk_services_gateway_binding" "test" {
  cluster_id          = "test-cluster-id"
  services_gateway_id = "test-services-gateway-id"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: config},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
