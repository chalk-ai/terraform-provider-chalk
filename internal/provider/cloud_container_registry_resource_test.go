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

func testRegistryResponse(kind, name string, cfg *serverv1.CloudContainerRegistryConfig) *serverv1.CloudComponentContainerRegistryResponse {
	ts := timestamppb.New(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	return &serverv1.CloudComponentContainerRegistryResponse{
		Id:                "registry-id-1",
		Name:              name,
		TeamId:            "team-1",
		Kind:              kind,
		Managed:           false,
		CloudCredentialId: new("cred-1"),
		Spec:              &serverv1.CloudComponentContainerRegistry{Name: name, Config: cfg},
		CreatedAt:         ts,
		UpdatedAt:         ts,
	}
}

// TestCloudContainerRegistryCreateGAR verifies the create/read/import lifecycle for
// a GAR registry, and that kind is derived from the config block.
func TestCloudContainerRegistryCreateGAR(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	cfg := &serverv1.CloudContainerRegistryConfig{
		Config: &serverv1.CloudContainerRegistryConfig_Gar{
			Gar: &serverv1.GarContainerRegistryConfig{RepositoryName: "my-images"},
		},
	}
	resp := testRegistryResponse("gar", "us-central1-docker.pkg.dev/my-project/my-repo", cfg)
	server.OnCreateCloudComponentContainerRegistry().Return(&serverv1.CreateCloudComponentContainerRegistryResponse{ContainerRegistry: resp})
	server.OnGetCloudComponentContainerRegistry().Return(&serverv1.GetCloudComponentContainerRegistryResponse{ContainerRegistry: resp})
	server.OnDeleteCloudComponentContainerRegistry().Return(&serverv1.DeleteCloudComponentContainerRegistryResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_cloud_container_registry" "test" {
  name                = "us-central1-docker.pkg.dev/my-project/my-repo"
  cloud_credential_id = "cred-1"
  config = {
    gar = {
      repository_name = "my-images"
    }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_cloud_container_registry.test", "name", "us-central1-docker.pkg.dev/my-project/my-repo"),
					resource.TestCheckResourceAttr("chalk_cloud_container_registry.test", "cloud_credential_id", "cred-1"),
					resource.TestCheckResourceAttr("chalk_cloud_container_registry.test", "kind", "gar"),
					resource.TestCheckResourceAttr("chalk_cloud_container_registry.test", "managed", "false"),
					resource.TestCheckResourceAttr("chalk_cloud_container_registry.test", "id", "registry-id-1"),
					resource.TestCheckResourceAttr("chalk_cloud_container_registry.test", "team_id", "team-1"),
					resource.TestCheckResourceAttr("chalk_cloud_container_registry.test", "config.gar.repository_name", "my-images"),
				),
			},
			{
				ResourceName:      "chalk_cloud_container_registry.test",
				ImportState:       true,
				ImportStateId:     "registry-id-1",
				ImportStateVerify: true,
			},
		},
	})
}

// TestCloudContainerRegistryCreateECR verifies the ECR variant, including the
// optional registry_id round-trip.
func TestCloudContainerRegistryCreateECR(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	cfg := &serverv1.CloudContainerRegistryConfig{
		Config: &serverv1.CloudContainerRegistryConfig_Ecr{
			Ecr: &serverv1.EcrContainerRegistryConfig{RegistryId: "123456789012", RepositoryName: "my-images"},
		},
	}
	resp := testRegistryResponse("ecr", "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo", cfg)
	server.OnCreateCloudComponentContainerRegistry().Return(&serverv1.CreateCloudComponentContainerRegistryResponse{ContainerRegistry: resp})
	server.OnGetCloudComponentContainerRegistry().Return(&serverv1.GetCloudComponentContainerRegistryResponse{ContainerRegistry: resp})
	server.OnDeleteCloudComponentContainerRegistry().Return(&serverv1.DeleteCloudComponentContainerRegistryResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_cloud_container_registry" "test" {
  name                = "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo"
  cloud_credential_id = "cred-1"
  config = {
    ecr = {
      registry_id     = "123456789012"
      repository_name = "my-images"
    }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_cloud_container_registry.test", "kind", "ecr"),
					resource.TestCheckResourceAttr("chalk_cloud_container_registry.test", "config.ecr.registry_id", "123456789012"),
					resource.TestCheckResourceAttr("chalk_cloud_container_registry.test", "config.ecr.repository_name", "my-images"),
				),
			},
		},
	})
}

// TestCloudContainerRegistryInvalidNameForKind verifies plan-time validation that
// the registry name path matches the kind implied by the config block.
func TestCloudContainerRegistryInvalidNameForKind(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_cloud_container_registry" "test" {
  name                = "not-a-valid-gar-path"
  cloud_credential_id = "cred-1"
  config = {
    gar = {
      repository_name = "my-images"
    }
  }
}
`,
				ExpectError: regexp.MustCompile(`Invalid registry name for kind`),
			},
		},
	})
}

// TestCloudContainerRegistryAmbiguousConfig verifies that setting more than one
// config block fails at plan time.
func TestCloudContainerRegistryAmbiguousConfig(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_cloud_container_registry" "test" {
  name                = "us-central1-docker.pkg.dev/my-project/my-repo"
  cloud_credential_id = "cred-1"
  config = {
    gar = {
      repository_name = "my-images"
    }
    acr = {
      repository_name = "my-images"
    }
  }
}
`,
				ExpectError: regexp.MustCompile(`Ambiguous container registry config`),
			},
		},
	})
}

// TestCloudContainerRegistryMissingConfig verifies that an empty config block
// (none of gar/ecr/acr set) fails at plan time.
func TestCloudContainerRegistryMissingConfig(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_cloud_container_registry" "test" {
  name                = "us-central1-docker.pkg.dev/my-project/my-repo"
  cloud_credential_id = "cred-1"
  config              = {}
}
`,
				ExpectError: regexp.MustCompile(`Missing container registry config`),
			},
		},
	})
}

// TestCloudContainerRegistryAccessDenied verifies the create-time access-check
// failure surfaces as a clear diagnostic.
func TestCloudContainerRegistryAccessDenied(t *testing.T) {
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
resource "chalk_cloud_container_registry" "test" {
  name                = "us-central1-docker.pkg.dev/my-project/my-repo"
  cloud_credential_id = "cred-1"
  config = {
    gar = {
      repository_name = "my-images"
    }
  }
}
`,
				ExpectError: regexp.MustCompile(`registry is not reachable`),
			},
		},
	})
}

// TestCloudContainerRegistryReadNotFound verifies removal from state on not_found.
func TestCloudContainerRegistryReadNotFound(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	cfg := &serverv1.CloudContainerRegistryConfig{
		Config: &serverv1.CloudContainerRegistryConfig_Gar{
			Gar: &serverv1.GarContainerRegistryConfig{RepositoryName: "my-images"},
		},
	}
	resp := testRegistryResponse("gar", "us-central1-docker.pkg.dev/my-project/my-repo", cfg)
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
resource "chalk_cloud_container_registry" "test" {
  name                = "us-central1-docker.pkg.dev/my-project/my-repo"
  cloud_credential_id = "cred-1"
  config = {
    gar = {
      repository_name = "my-images"
    }
  }
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
