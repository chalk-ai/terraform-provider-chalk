package provider

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func testClusterBinding(storageID string) *serverv1.ClusterCloudStorageBinding {
	ts := timestamppb.New(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	return &serverv1.ClusterCloudStorageBinding{
		Id:             "binding-cluster-1",
		ClusterId:      "cluster-1",
		CloudStorageId: storageID,
		StorageRole:    serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_PLAN_STAGES,
		CreatedAt:      ts,
		UpdatedAt:      ts,
	}
}

const clusterCloudStorageBindingConfig = `
resource "chalk_cluster_cloud_storage_binding" "test" {
  cluster_id       = "cluster-1"
  cloud_storage_id = "storage-1"
  storage_role     = "PLAN_STAGES"
}
`

// TestClusterCloudStorageBindingCreate verifies the create/read lifecycle and import.
func TestClusterCloudStorageBindingCreate(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingClusterCloudStorage().Return(&serverv1.CreateBindingClusterCloudStorageResponse{Binding: testClusterBinding("storage-1")})
	server.OnGetBindingClusterCloudStorage().Return(&serverv1.GetBindingClusterCloudStorageResponse{Binding: testClusterBinding("storage-1")})
	server.OnDeleteBindingClusterCloudStorage().Return(&serverv1.DeleteBindingClusterCloudStorageResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + clusterCloudStorageBindingConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_cluster_cloud_storage_binding.test", "cluster_id", "cluster-1"),
					resource.TestCheckResourceAttr("chalk_cluster_cloud_storage_binding.test", "cloud_storage_id", "storage-1"),
					resource.TestCheckResourceAttr("chalk_cluster_cloud_storage_binding.test", "storage_role", "PLAN_STAGES"),
					resource.TestCheckResourceAttr("chalk_cluster_cloud_storage_binding.test", "id", "binding-cluster-1"),
				),
			},
			{
				ResourceName:      "chalk_cluster_cloud_storage_binding.test",
				ImportState:       true,
				ImportStateId:     "cluster-1:PLAN_STAGES",
				ImportStateVerify: true,
			},
		},
	})
}

// TestClusterCloudStorageBindingAlreadyExists verifies the AlreadyExists mapping.
func TestClusterCloudStorageBindingAlreadyExists(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingClusterCloudStorage().ReturnError(
		connect.NewError(connect.CodeAlreadyExists, errors.New("binding exists")),
	)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      providerConfig(server.URL) + clusterCloudStorageBindingConfig,
				ExpectError: regexp.MustCompile(`already has a binding for this role`),
			},
		},
	})
}

// TestClusterCloudStorageBindingReassignedDrift verifies out-of-band reassignment
// of the (cluster, role) slot is reconciled and plans a recreate.
func TestClusterCloudStorageBindingReassignedDrift(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingClusterCloudStorage().Return(&serverv1.CreateBindingClusterCloudStorageResponse{Binding: testClusterBinding("storage-1")})
	server.OnDeleteBindingClusterCloudStorage().Return(&serverv1.DeleteBindingClusterCloudStorageResponse{})

	var getCallCount int
	server.OnGetBindingClusterCloudStorage().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getCallCount++
		if getCallCount > 1 {
			return &serverv1.GetBindingClusterCloudStorageResponse{Binding: testClusterBinding("storage-2")}, nil
		}
		return &serverv1.GetBindingClusterCloudStorageResponse{Binding: testClusterBinding("storage-1")}, nil
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: providerConfig(server.URL) + clusterCloudStorageBindingConfig},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
