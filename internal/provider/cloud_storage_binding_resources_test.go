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

func testEnvBindingResp(storageID string, role serverv1.CloudStorageRole) *serverv1.EnvironmentCloudStorageBinding {
	ts := timestamppb.New(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	return &serverv1.EnvironmentCloudStorageBinding{
		Id:             "binding-env-1",
		EnvironmentId:  "env-1",
		CloudStorageId: storageID,
		StorageRole:    role,
		CreatedAt:      ts,
		UpdatedAt:      ts,
	}
}

func testClusterBindingResp(storageID string, role serverv1.CloudStorageRole) *serverv1.ClusterCloudStorageBinding {
	ts := timestamppb.New(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	return &serverv1.ClusterCloudStorageBinding{
		Id:             "binding-cluster-1",
		ClusterId:      "cluster-1",
		CloudStorageId: storageID,
		StorageRole:    role,
		CreatedAt:      ts,
		UpdatedAt:      ts,
	}
}

// TestEnvironmentDatasetBindingCreate verifies create/read and import-by-target-id.
func TestEnvironmentDatasetBindingCreate(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	b := testEnvBindingResp("storage-1", serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_DATASET)
	server.OnCreateBindingEnvironmentCloudStorage().Return(&serverv1.CreateBindingEnvironmentCloudStorageResponse{Binding: b})
	server.OnGetBindingEnvironmentCloudStorage().Return(&serverv1.GetBindingEnvironmentCloudStorageResponse{Binding: b})
	server.OnDeleteBindingEnvironmentCloudStorage().Return(&serverv1.DeleteBindingEnvironmentCloudStorageResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_environment_dataset_cloud_storage_binding" "test" {
  environment_id   = "env-1"
  cloud_storage_id = "storage-1"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_environment_dataset_cloud_storage_binding.test", "environment_id", "env-1"),
					resource.TestCheckResourceAttr("chalk_environment_dataset_cloud_storage_binding.test", "cloud_storage_id", "storage-1"),
					resource.TestCheckResourceAttr("chalk_environment_dataset_cloud_storage_binding.test", "id", "binding-env-1"),
				),
			},
			{
				// Role is fixed by the type, so import is by target id alone.
				ResourceName:      "chalk_environment_dataset_cloud_storage_binding.test",
				ImportState:       true,
				ImportStateId:     "env-1",
				ImportStateVerify: true,
			},
		},
	})
}

// TestEnvironmentDatasetBindingAlreadyExists verifies the AlreadyExists mapping.
func TestEnvironmentDatasetBindingAlreadyExists(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingEnvironmentCloudStorage().ReturnError(
		connect.NewError(connect.CodeAlreadyExists, errors.New("binding exists")),
	)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_environment_dataset_cloud_storage_binding" "test" {
  environment_id   = "env-1"
  cloud_storage_id = "storage-1"
}
`,
				ExpectError: regexp.MustCompile(`already has a binding for this role`),
			},
		},
	})
}

// TestClusterVolumeBindingCreate verifies the cluster-only VOLUME role resource.
func TestClusterVolumeBindingCreate(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	b := testClusterBindingResp("storage-1", serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_VOLUME)
	server.OnCreateBindingClusterCloudStorage().Return(&serverv1.CreateBindingClusterCloudStorageResponse{Binding: b})
	server.OnGetBindingClusterCloudStorage().Return(&serverv1.GetBindingClusterCloudStorageResponse{Binding: b})
	server.OnDeleteBindingClusterCloudStorage().Return(&serverv1.DeleteBindingClusterCloudStorageResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_cluster_volume_cloud_storage_binding" "test" {
  cluster_id       = "cluster-1"
  cloud_storage_id = "storage-1"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_cluster_volume_cloud_storage_binding.test", "cluster_id", "cluster-1"),
					resource.TestCheckResourceAttr("chalk_cluster_volume_cloud_storage_binding.test", "cloud_storage_id", "storage-1"),
					resource.TestCheckResourceAttr("chalk_cluster_volume_cloud_storage_binding.test", "id", "binding-cluster-1"),
				),
			},
			{
				ResourceName:      "chalk_cluster_volume_cloud_storage_binding.test",
				ImportState:       true,
				ImportStateId:     "cluster-1",
				ImportStateVerify: true,
			},
		},
	})
}

// TestClusterPlanStagesBindingReassignedDrift verifies out-of-band reassignment of
// the (cluster, role) slot is reconciled and plans a recreate.
func TestClusterPlanStagesBindingReassignedDrift(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingClusterCloudStorage().Return(&serverv1.CreateBindingClusterCloudStorageResponse{
		Binding: testClusterBindingResp("storage-1", serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_PLAN_STAGES),
	})
	server.OnDeleteBindingClusterCloudStorage().Return(&serverv1.DeleteBindingClusterCloudStorageResponse{})

	var getCallCount int
	server.OnGetBindingClusterCloudStorage().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getCallCount++
		storageID := "storage-1"
		if getCallCount > 1 {
			storageID = "storage-2"
		}
		return &serverv1.GetBindingClusterCloudStorageResponse{
			Binding: testClusterBindingResp(storageID, serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_PLAN_STAGES),
		}, nil
	})

	config := providerConfig(server.URL) + `
resource "chalk_cluster_plan_stages_cloud_storage_binding" "test" {
  cluster_id       = "cluster-1"
  cloud_storage_id = "storage-1"
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
