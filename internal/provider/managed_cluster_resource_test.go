package provider

import (
	"regexp"
	"sync/atomic"
	"testing"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// clusterResponse builds a CloudComponentClusterResponse with the given
// lifecycle status (and optional status_error).
func clusterResponse(status string, statusErr string) *serverv1.CloudComponentClusterResponse {
	resp := &serverv1.CloudComponentClusterResponse{
		Id:                "cluster-test-id",
		Kind:              "EKS_STANDARD",
		Managed:           true,
		CloudCredentialId: new("cc-test-id"),
		VpcId:             new("vpc-test-id"),
		Status:            status,
		Spec:              &serverv1.CloudComponentCluster{Name: "test-cluster"},
	}
	if statusErr != "" {
		resp.StatusError = new(statusErr)
	}
	return resp
}

func managedClusterConfig(serverURL string) string {
	return providerConfig(serverURL) + `
resource "chalk_managed_cluster" "cluster" {
  cloud_credential_id = "cc-test-id"
  vpc_id              = "vpc-test-id"
}
`
}

// clusterMock is a stateful mock for the managed cluster RPCs.
type clusterMock struct {
	server         *testserver.MockServer
	getCalls       atomic.Int32
	deleted        atomic.Bool
	activeAfterGet int32 // Get returns ACTIVE once getCalls >= this; <=0 means immediately
}

func setupClusterMock(t *testing.T, m *clusterMock) {
	m.server = testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { m.server.Close() })

	m.server.OnCreateCloudComponentCluster().WithBehavior(func(req proto.Message) (proto.Message, error) {
		return &serverv1.CreateCloudComponentClusterResponse{Cluster: clusterResponse("PENDING", "")}, nil
	})

	m.server.OnGetCloudComponentCluster().WithBehavior(func(req proto.Message) (proto.Message, error) {
		if m.deleted.Load() {
			return nil, notFound("cluster not found")
		}
		n := m.getCalls.Add(1)
		status := "PROVISIONING"
		if m.activeAfterGet <= 0 || n >= m.activeAfterGet {
			status = "ACTIVE"
		}
		return &serverv1.GetCloudComponentClusterResponse{Cluster: clusterResponse(status, "")}, nil
	})

	m.server.OnDeleteCloudComponentCluster().WithBehavior(func(req proto.Message) (proto.Message, error) {
		m.deleted.Store(true)
		return &serverv1.DeleteCloudComponentClusterResponse{}, nil
	})
}

// TestManagedClusterResourceCreatePollsUntilActive verifies that Create polls
// GetCloudComponentCluster until the status reaches the terminal ACTIVE state.
func TestManagedClusterResourceCreatePollsUntilActive(t *testing.T) {
	t.Parallel()

	m := &clusterMock{activeAfterGet: 3}
	setupClusterMock(t, m)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: managedClusterConfig(m.server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_managed_cluster.cluster", "id", "cluster-test-id"),
					resource.TestCheckResourceAttr("chalk_managed_cluster.cluster", "name", "test-cluster"),
					resource.TestCheckResourceAttr("chalk_managed_cluster.cluster", "vpc_id", "vpc-test-id"),
					// status must not be persisted to state.
					resource.TestCheckNoResourceAttr("chalk_managed_cluster.cluster", "status"),
					func(s *terraform.State) error {
						require.Len(t, m.server.GetCapturedRequests("CreateCloudComponentCluster"), 1)
						assert.GreaterOrEqual(t, int(m.getCalls.Load()), 3, "expected Create to poll until ACTIVE")
						return nil
					},
				),
			},
		},
	})
}

// TestManagedClusterResourceCreateActiveImmediately verifies the happy path
// where the status is already ACTIVE on the first poll (no waiting).
func TestManagedClusterResourceCreateActiveImmediately(t *testing.T) {
	t.Parallel()

	m := &clusterMock{}
	setupClusterMock(t, m)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: managedClusterConfig(m.server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_managed_cluster.cluster", "id", "cluster-test-id"),
					func(s *terraform.State) error {
						require.Len(t, m.server.GetCapturedRequests("GetCloudComponentCluster"), 1,
							"status was already ACTIVE, so a single poll should suffice")
						return nil
					},
				),
			},
		},
	})
}

// TestManagedClusterResourceCreateFailedErrors verifies that a FAILED status
// during create surfaces an error carrying the status_error detail.
func TestManagedClusterResourceCreateFailedErrors(t *testing.T) {
	t.Parallel()

	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateCloudComponentCluster().WithBehavior(func(req proto.Message) (proto.Message, error) {
		return &serverv1.CreateCloudComponentClusterResponse{Cluster: clusterResponse("PENDING", "")}, nil
	})
	// FAILED until the tainted resource is destroyed, then not-found.
	deleted := &atomic.Bool{}
	server.OnGetCloudComponentCluster().WithBehavior(func(req proto.Message) (proto.Message, error) {
		if deleted.Load() {
			return nil, notFound("cluster not found")
		}
		return &serverv1.GetCloudComponentClusterResponse{Cluster: clusterResponse("FAILED", "node group failed to join")}, nil
	})
	server.OnDeleteCloudComponentCluster().WithBehavior(func(req proto.Message) (proto.Message, error) {
		deleted.Store(true)
		return &serverv1.DeleteCloudComponentClusterResponse{}, nil
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      managedClusterConfig(server.URL),
				ExpectError: regexp.MustCompile(`did not become active`),
			},
		},
	})
}

// TestManagedClusterResourceDeleteWaitsForDeleting verifies that Delete keeps
// polling while the cluster reports a DELETING status and only completes once
// it is gone, rather than returning as soon as it enters DELETING.
func TestManagedClusterResourceDeleteWaitsForDeleting(t *testing.T) {
	t.Parallel()

	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateCloudComponentCluster().WithBehavior(func(req proto.Message) (proto.Message, error) {
		return &serverv1.CreateCloudComponentClusterResponse{Cluster: clusterResponse("PENDING", "")}, nil
	})

	// After delete, the cluster reports DELETING for two polls (which must NOT
	// be treated as gone) before finally reporting not-found.
	var deleteGets atomic.Int32
	deleted := &atomic.Bool{}
	server.OnGetCloudComponentCluster().WithBehavior(func(req proto.Message) (proto.Message, error) {
		if deleted.Load() {
			if deleteGets.Add(1) >= 3 {
				return nil, notFound("cluster not found")
			}
			return &serverv1.GetCloudComponentClusterResponse{Cluster: clusterResponse("DELETING", "")}, nil
		}
		return &serverv1.GetCloudComponentClusterResponse{Cluster: clusterResponse("ACTIVE", "")}, nil
	})

	server.OnDeleteCloudComponentCluster().WithBehavior(func(req proto.Message) (proto.Message, error) {
		deleted.Store(true)
		return &serverv1.DeleteCloudComponentClusterResponse{}, nil
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: managedClusterConfig(server.URL)},
			{
				Config: providerConfig(server.URL),
				Check: func(s *terraform.State) error {
					require.Len(t, server.GetCapturedRequests("DeleteCloudComponentCluster"), 1)
					assert.GreaterOrEqual(t, int(deleteGets.Load()), 3, "expected Delete to keep polling through DELETING")
					return nil
				},
			},
		},
	})
}
