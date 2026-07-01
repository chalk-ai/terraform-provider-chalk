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

func testEnvBinding(storageID string) *serverv1.EnvironmentCloudStorageBinding {
	ts := timestamppb.New(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	return &serverv1.EnvironmentCloudStorageBinding{
		Id:             "binding-env-1",
		EnvironmentId:  "env-1",
		CloudStorageId: storageID,
		StorageRole:    serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_DATASET,
		CreatedAt:      ts,
		UpdatedAt:      ts,
	}
}

const envCloudStorageBindingConfig = `
resource "chalk_environment_cloud_storage_binding" "test" {
  environment_id   = "env-1"
  cloud_storage_id = "storage-1"
  storage_role     = "DATASET"
}
`

func setupMockServerEnvCloudStorageBinding(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingEnvironmentCloudStorage().Return(&serverv1.CreateBindingEnvironmentCloudStorageResponse{Binding: testEnvBinding("storage-1")})
	server.OnGetBindingEnvironmentCloudStorage().Return(&serverv1.GetBindingEnvironmentCloudStorageResponse{Binding: testEnvBinding("storage-1")})
	server.OnDeleteBindingEnvironmentCloudStorage().Return(&serverv1.DeleteBindingEnvironmentCloudStorageResponse{})

	return server
}

// TestEnvironmentCloudStorageBindingCreate verifies the create/read lifecycle.
func TestEnvironmentCloudStorageBindingCreate(t *testing.T) {
	t.Parallel()
	server := setupMockServerEnvCloudStorageBinding(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + envCloudStorageBindingConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_environment_cloud_storage_binding.test", "environment_id", "env-1"),
					resource.TestCheckResourceAttr("chalk_environment_cloud_storage_binding.test", "cloud_storage_id", "storage-1"),
					resource.TestCheckResourceAttr("chalk_environment_cloud_storage_binding.test", "storage_role", "DATASET"),
					resource.TestCheckResourceAttr("chalk_environment_cloud_storage_binding.test", "id", "binding-env-1"),
					resource.TestCheckResourceAttrSet("chalk_environment_cloud_storage_binding.test", "created_at"),
				),
			},
			// Import by the real key "<environment_id>:<storage_role>".
			{
				ResourceName:      "chalk_environment_cloud_storage_binding.test",
				ImportState:       true,
				ImportStateId:     "env-1:DATASET",
				ImportStateVerify: true,
			},
		},
	})
}

// TestEnvironmentCloudStorageBindingAlreadyExists verifies the AlreadyExists mapping.
func TestEnvironmentCloudStorageBindingAlreadyExists(t *testing.T) {
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
				Config:      providerConfig(server.URL) + envCloudStorageBindingConfig,
				ExpectError: regexp.MustCompile(`already has a binding for this role`),
			},
		},
	})
}

// TestEnvironmentCloudStorageBindingReassignedDrift verifies that when the
// (environment, role) slot is occupied by a different storage out-of-band, the
// refresh reconciles cloud_storage_id and the plan schedules a recreate.
func TestEnvironmentCloudStorageBindingReassignedDrift(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingEnvironmentCloudStorage().Return(&serverv1.CreateBindingEnvironmentCloudStorageResponse{Binding: testEnvBinding("storage-1")})
	server.OnDeleteBindingEnvironmentCloudStorage().Return(&serverv1.DeleteBindingEnvironmentCloudStorageResponse{})

	var getCallCount int
	server.OnGetBindingEnvironmentCloudStorage().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getCallCount++
		// First read matches; subsequent reads report a different storage in the slot.
		if getCallCount > 1 {
			return &serverv1.GetBindingEnvironmentCloudStorageResponse{Binding: testEnvBinding("storage-2")}, nil
		}
		return &serverv1.GetBindingEnvironmentCloudStorageResponse{Binding: testEnvBinding("storage-1")}, nil
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: providerConfig(server.URL) + envCloudStorageBindingConfig},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestEnvironmentCloudStorageBindingReadNotFound verifies removal from state on not_found.
func TestEnvironmentCloudStorageBindingReadNotFound(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateBindingEnvironmentCloudStorage().Return(&serverv1.CreateBindingEnvironmentCloudStorageResponse{Binding: testEnvBinding("storage-1")})
	server.OnDeleteBindingEnvironmentCloudStorage().Return(&serverv1.DeleteBindingEnvironmentCloudStorageResponse{})

	var getCallCount int
	server.OnGetBindingEnvironmentCloudStorage().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getCallCount++
		if getCallCount > 1 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("binding not found"))
		}
		return &serverv1.GetBindingEnvironmentCloudStorageResponse{Binding: testEnvBinding("storage-1")}, nil
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: providerConfig(server.URL) + envCloudStorageBindingConfig},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
