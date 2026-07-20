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

func testStorageResponse(managed bool, uri string) *serverv1.CloudComponentStorageResponse {
	ts := timestamppb.New(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	return &serverv1.CloudComponentStorageResponse{
		Id:                "storage-id-1",
		Name:              uri,
		TeamId:            "team-1",
		Kind:              "s3",
		Managed:           managed,
		CloudCredentialId: new("cred-1"),
		Spec:              &serverv1.CloudComponentStorage{Uri: uri},
		CreatedAt:         ts,
		UpdatedAt:         ts,
	}
}

// TestUnmanagedCloudStorageCreate verifies the create/read lifecycle.
func TestUnmanagedCloudStorageCreate(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	resp := testStorageResponse(false, "s3://my-bucket/prefix")
	server.OnCreateCloudComponentStorage().Return(&serverv1.CreateCloudComponentStorageResponse{Storage: resp})
	server.OnGetCloudComponentStorage().Return(&serverv1.GetCloudComponentStorageResponse{Storage: resp})
	server.OnDeleteCloudComponentStorage().Return(&serverv1.DeleteCloudComponentStorageResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_unmanaged_cloud_storage" "test" {
  uri                 = "s3://my-bucket/prefix"
  cloud_credential_id = "cred-1"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_unmanaged_cloud_storage.test", "uri", "s3://my-bucket/prefix"),
					resource.TestCheckResourceAttr("chalk_unmanaged_cloud_storage.test", "cloud_credential_id", "cred-1"),
					// kind is inferred/echoed by the server.
					resource.TestCheckResourceAttr("chalk_unmanaged_cloud_storage.test", "kind", "s3"),
					resource.TestCheckResourceAttr("chalk_unmanaged_cloud_storage.test", "id", "storage-id-1"),
				),
			},
			{
				ResourceName:      "chalk_unmanaged_cloud_storage.test",
				ImportState:       true,
				ImportStateId:     "storage-id-1",
				ImportStateVerify: true,
			},
		},
	})
}

// TestUnmanagedCloudStorageInvalidURIForKind verifies plan-time URI/kind validation
// fires only when kind is set.
func TestUnmanagedCloudStorageInvalidURIForKind(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_unmanaged_cloud_storage" "test" {
  kind                = "gcs"
  uri                 = "s3://wrong-scheme"
  cloud_credential_id = "cred-1"
}
`,
				ExpectError: regexp.MustCompile(`Invalid storage URI for kind`),
			},
		},
	})
}

// TestManagedCloudStorageCreate verifies the managed resource, which has no uri
// attribute at all.
func TestManagedCloudStorageCreate(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	resp := testStorageResponse(true, "s3://chalk-managed/abc")
	server.OnCreateCloudComponentStorage().Return(&serverv1.CreateCloudComponentStorageResponse{Storage: resp})
	server.OnGetCloudComponentStorage().Return(&serverv1.GetCloudComponentStorageResponse{Storage: resp})
	server.OnDeleteCloudComponentStorage().Return(&serverv1.DeleteCloudComponentStorageResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_managed_cloud_storage" "test" {
  cloud_credential_id = "cred-1"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_managed_cloud_storage.test", "cloud_credential_id", "cred-1"),
					resource.TestCheckResourceAttr("chalk_managed_cloud_storage.test", "id", "storage-id-1"),
					resource.TestCheckNoResourceAttr("chalk_managed_cloud_storage.test", "uri"),
				),
			},
		},
	})
}

// TestUnmanagedCloudStoragePermissionDenied verifies the create-time bucket-access
// failure surfaces as a clear diagnostic.
func TestUnmanagedCloudStoragePermissionDenied(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateCloudComponentStorage().ReturnError(
		connect.NewError(connect.CodePermissionDenied, errors.New("cannot access storage")),
	)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_unmanaged_cloud_storage" "test" {
  uri                 = "s3://my-bucket/prefix"
  cloud_credential_id = "cred-1"
}
`,
				ExpectError: regexp.MustCompile(`bucket is not reachable`),
			},
		},
	})
}

// TestUnmanagedCloudStorageReadNotFound verifies removal from state on not_found.
func TestUnmanagedCloudStorageReadNotFound(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	resp := testStorageResponse(false, "s3://my-bucket/prefix")
	server.OnCreateCloudComponentStorage().Return(&serverv1.CreateCloudComponentStorageResponse{Storage: resp})
	server.OnDeleteCloudComponentStorage().Return(&serverv1.DeleteCloudComponentStorageResponse{})

	var getCallCount int
	server.OnGetCloudComponentStorage().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getCallCount++
		if getCallCount > 1 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("storage not found"))
		}
		return &serverv1.GetCloudComponentStorageResponse{Storage: resp}, nil
	})

	config := providerConfig(server.URL) + `
resource "chalk_unmanaged_cloud_storage" "test" {
  uri                 = "s3://my-bucket/prefix"
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
