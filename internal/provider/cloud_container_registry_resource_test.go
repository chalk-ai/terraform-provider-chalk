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

func testRegistryResponse(managed bool, kind, name string) *serverv1.CloudComponentContainerRegistryResponse {
	ts := timestamppb.New(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	return &serverv1.CloudComponentContainerRegistryResponse{
		Id:                "registry-id-1",
		Name:              name,
		TeamId:            "team-1",
		Kind:              kind,
		Managed:           managed,
		CloudCredentialId: new("cred-1"),
		Spec:              &serverv1.CloudComponentContainerRegistry{Name: name},
		CreatedAt:         ts,
		UpdatedAt:         ts,
	}
}

// TestUnmanagedContainerRegistryCreate verifies the create/read/import lifecycle.
func TestUnmanagedContainerRegistryCreate(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	resp := testRegistryResponse(false, "gar", "us-docker.pkg.dev/my-project/my-repo")
	server.OnCreateCloudComponentContainerRegistry().Return(&serverv1.CreateCloudComponentContainerRegistryResponse{ContainerRegistry: resp})
	server.OnGetCloudComponentContainerRegistry().Return(&serverv1.GetCloudComponentContainerRegistryResponse{ContainerRegistry: resp})
	server.OnDeleteCloudComponentContainerRegistry().Return(&serverv1.DeleteCloudComponentContainerRegistryResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_unmanaged_container_registry" "test" {
  name                = "us-docker.pkg.dev/my-project/my-repo"
  cloud_credential_id = "cred-1"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_unmanaged_container_registry.test", "name", "us-docker.pkg.dev/my-project/my-repo"),
					resource.TestCheckResourceAttr("chalk_unmanaged_container_registry.test", "cloud_credential_id", "cred-1"),
					resource.TestCheckResourceAttr("chalk_unmanaged_container_registry.test", "id", "registry-id-1"),
				),
			},
			{
				ResourceName:      "chalk_unmanaged_container_registry.test",
				ImportState:       true,
				ImportStateId:     "registry-id-1",
				ImportStateVerify: true,
			},
		},
	})
}

// TestManagedContainerRegistryCreate verifies the managed resource: no name input;
// name is computed and set by the server.
func TestManagedContainerRegistryCreate(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	resp := testRegistryResponse(true, "ecr", "123456789012.dkr.ecr.us-east-1.amazonaws.com/chalk-managed")
	server.OnCreateCloudComponentContainerRegistry().Return(&serverv1.CreateCloudComponentContainerRegistryResponse{ContainerRegistry: resp})
	server.OnGetCloudComponentContainerRegistry().Return(&serverv1.GetCloudComponentContainerRegistryResponse{ContainerRegistry: resp})
	server.OnDeleteCloudComponentContainerRegistry().Return(&serverv1.DeleteCloudComponentContainerRegistryResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_managed_container_registry" "test" {
  cloud_credential_id = "cred-1"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_managed_container_registry.test", "cloud_credential_id", "cred-1"),
					resource.TestCheckResourceAttr("chalk_managed_container_registry.test", "id", "registry-id-1"),
					// name is derived and set by the server.
					resource.TestCheckResourceAttr("chalk_managed_container_registry.test", "name", "123456789012.dkr.ecr.us-east-1.amazonaws.com/chalk-managed"),
				),
			},
		},
	})
}

// TestUnmanagedContainerRegistryAccessDenied verifies the create-time access-check
// failure surfaces as a clear diagnostic.
func TestUnmanagedContainerRegistryAccessDenied(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateCloudComponentContainerRegistry().ReturnError(
		connect.NewError(connect.CodeFailedPrecondition, errors.New("cannot access registry")),
	)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_unmanaged_container_registry" "test" {
  name                = "us-docker.pkg.dev/my-project/my-repo"
  cloud_credential_id = "cred-1"
}
`,
				ExpectError: regexp.MustCompile(`registry is not reachable`),
			},
		},
	})
}

// TestUnmanagedContainerRegistryReadNotFound verifies removal from state on not_found.
func TestUnmanagedContainerRegistryReadNotFound(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	resp := testRegistryResponse(false, "gar", "us-docker.pkg.dev/my-project/my-repo")
	server.OnCreateCloudComponentContainerRegistry().Return(&serverv1.CreateCloudComponentContainerRegistryResponse{ContainerRegistry: resp})
	server.OnDeleteCloudComponentContainerRegistry().Return(&serverv1.DeleteCloudComponentContainerRegistryResponse{})

	var getCallCount int
	server.OnGetCloudComponentContainerRegistry().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getCallCount++
		if getCallCount > 1 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("registry not found"))
		}
		return &serverv1.GetCloudComponentContainerRegistryResponse{ContainerRegistry: resp}, nil
	})

	config := providerConfig(server.URL) + `
resource "chalk_unmanaged_container_registry" "test" {
  name                = "us-docker.pkg.dev/my-project/my-repo"
  cloud_credential_id = "cred-1"
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
