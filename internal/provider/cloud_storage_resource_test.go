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

func testCloudStorageResponse() *serverv1.CloudComponentStorageResponse {
	ts := timestamppb.New(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	return &serverv1.CloudComponentStorageResponse{
		Id:                "storage-id-1",
		Name:              "s3://my-bucket/prefix",
		TeamId:            "team-1",
		Kind:              "s3",
		Managed:           false,
		CloudCredentialId: new("cred-1"),
		Spec:              &serverv1.CloudComponentStorage{Uri: "s3://my-bucket/prefix"},
		CreatedAt:         ts,
		UpdatedAt:         ts,
	}
}

func setupMockServerCloudStorage(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateCloudComponentStorage().Return(&serverv1.CreateCloudComponentStorageResponse{Storage: testCloudStorageResponse()})
	server.OnGetCloudComponentStorage().Return(&serverv1.GetCloudComponentStorageResponse{Storage: testCloudStorageResponse()})
	server.OnDeleteCloudComponentStorage().Return(&serverv1.DeleteCloudComponentStorageResponse{})

	return server
}

// TestCloudStorageCreate verifies the create/read lifecycle and computed fields.
func TestCloudStorageCreate(t *testing.T) {
	t.Parallel()
	server := setupMockServerCloudStorage(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_cloud_storage" "test" {
  kind                = "s3"
  uri                 = "s3://my-bucket/prefix"
  cloud_credential_id = "cred-1"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_cloud_storage.test", "kind", "s3"),
					resource.TestCheckResourceAttr("chalk_cloud_storage.test", "uri", "s3://my-bucket/prefix"),
					resource.TestCheckResourceAttr("chalk_cloud_storage.test", "cloud_credential_id", "cred-1"),
					resource.TestCheckResourceAttr("chalk_cloud_storage.test", "managed", "false"),
					resource.TestCheckResourceAttr("chalk_cloud_storage.test", "id", "storage-id-1"),
					// name is server-set to the uri.
					resource.TestCheckResourceAttr("chalk_cloud_storage.test", "name", "s3://my-bucket/prefix"),
					resource.TestCheckResourceAttr("chalk_cloud_storage.test", "team_id", "team-1"),
					resource.TestCheckResourceAttrSet("chalk_cloud_storage.test", "created_at"),
				),
			},
		},
	})
}

// TestCloudStorageInvalidURIForKind verifies plan-time URI/kind validation fails
// before any server call.
func TestCloudStorageInvalidURIForKind(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_cloud_storage" "test" {
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

// TestCloudStorageReadNotFound verifies that a not_found on refresh removes the
// resource from state so a subsequent plan recreates it.
func TestCloudStorageReadNotFound(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateCloudComponentStorage().Return(&serverv1.CreateCloudComponentStorageResponse{Storage: testCloudStorageResponse()})
	server.OnDeleteCloudComponentStorage().Return(&serverv1.DeleteCloudComponentStorageResponse{})

	var getCallCount int
	server.OnGetCloudComponentStorage().WithBehavior(func(req proto.Message) (proto.Message, error) {
		getCallCount++
		if getCallCount > 1 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("storage not found"))
		}
		return &serverv1.GetCloudComponentStorageResponse{Storage: testCloudStorageResponse()}, nil
	})

	config := providerConfig(server.URL) + `
resource "chalk_cloud_storage" "test" {
  kind                = "s3"
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

// TestCloudStoragePermissionDenied verifies the create-time bucket-access failure
// surfaces as a clear diagnostic.
func TestCloudStoragePermissionDenied(t *testing.T) {
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
resource "chalk_cloud_storage" "test" {
  kind                = "s3"
  uri                 = "s3://my-bucket/prefix"
  cloud_credential_id = "cred-1"
}
`,
				ExpectError: regexp.MustCompile(`bucket is not reachable`),
			},
		},
	})
}
