package provider

import (
	"errors"
	"regexp"
	"sync/atomic"
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

func notFound(msg string) error {
	return connect.NewError(connect.CodeNotFound, errors.New(msg))
}

// vpcResponse builds a CloudComponentVpcResponse with the given lifecycle
// status (and optional status_error).
func vpcResponse(status string, statusErr string) *serverv1.CloudComponentVpcResponse {
	resp := &serverv1.CloudComponentVpcResponse{
		Id:                "vpc-test-id",
		Name:              "test-vpc",
		Kind:              "aws",
		Managed:           true,
		CloudCredentialId: new("cc-test-id"),
		Status:            status,
		Spec: &serverv1.CloudComponentVpc{
			Name: "test-vpc",
			Config: &serverv1.CloudVpcConfig{
				Config: &serverv1.CloudVpcConfig_Aws{
					Aws: &serverv1.AWSVpcConfig{
						CidrBlock: "10.130.128.0/18",
						Subnets: []*serverv1.AwsSubnetConfig{{
							Name:             "primary",
							PrivateCidrBlock: "10.130.128.0/21",
							PublicCidrBlock:  "10.130.136.0/21",
							AvailabilityZone: "a",
						}},
					},
				},
			},
		},
	}
	if statusErr != "" {
		resp.StatusError = new(statusErr)
	}
	return resp
}

func managedAWSVPCConfig(serverURL string) string {
	return providerConfig(serverURL) + `
resource "chalk_managed_aws_vpc" "vpc" {
  cidr_block          = "10.130.128.0/18"
  cloud_credential_id = "cc-test-id"
  subnets = [
    {
      name               = "primary"
      private_cidr_block = "10.130.128.0/21"
      public_cidr_block  = "10.130.136.0/21"
      availability_zone  = "a"
    }
  ]
}
`
}

// vpcMock is a stateful mock for the managed VPC RPCs. Get reports
// PROVISIONING until activeAfterGet calls have been made, then ACTIVE. Once
// delete has happened Get returns a not-found error.
type vpcMock struct {
	server         *testserver.MockServer
	getCalls       atomic.Int32
	deleted        atomic.Bool
	activeAfterGet int32 // Get returns ACTIVE once getCalls >= this; <=0 means immediately
}

func setupVPCMock(t *testing.T, m *vpcMock) {
	m.server = testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { m.server.Close() })

	m.server.OnCreateCloudComponentVpc().WithBehavior(func(req proto.Message) (proto.Message, error) {
		return &serverv1.CreateCloudComponentVpcResponse{Vpc: vpcResponse("PENDING", "")}, nil
	})

	m.server.OnGetCloudComponentVpc().WithBehavior(func(req proto.Message) (proto.Message, error) {
		if m.deleted.Load() {
			return nil, notFound("vpc not found")
		}
		n := m.getCalls.Add(1)
		status := "PROVISIONING"
		if m.activeAfterGet <= 0 || n >= m.activeAfterGet {
			status = "ACTIVE"
		}
		return &serverv1.GetCloudComponentVpcResponse{Vpc: vpcResponse(status, "")}, nil
	})

	m.server.OnDeleteCloudComponentVpc().WithBehavior(func(req proto.Message) (proto.Message, error) {
		m.deleted.Store(true)
		return &serverv1.DeleteCloudComponentVpcResponse{}, nil
	})
}

// TestManagedAWSVPCResourceCreatePollsUntilActive verifies that Create polls
// GetCloudComponentVpc until the status reaches the terminal ACTIVE state.
func TestManagedAWSVPCResourceCreatePollsUntilActive(t *testing.T) {
	t.Parallel()

	m := &vpcMock{activeAfterGet: 3} // PROVISIONING for the first two polls, ACTIVE on the third
	setupVPCMock(t, m)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: managedAWSVPCConfig(m.server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_managed_aws_vpc.vpc", "id", "vpc-test-id"),
					resource.TestCheckResourceAttr("chalk_managed_aws_vpc.vpc", "cidr_block", "10.130.128.0/18"),
					resource.TestCheckResourceAttr("chalk_managed_aws_vpc.vpc", "subnets.0.name", "primary"),
					// status must not be persisted to state.
					resource.TestCheckNoResourceAttr("chalk_managed_aws_vpc.vpc", "status"),
					func(s *terraform.State) error {
						require.Len(t, m.server.GetCapturedRequests("CreateCloudComponentVpc"), 1)
						assert.GreaterOrEqual(t, int(m.getCalls.Load()), 3, "expected Create to poll until ACTIVE")
						return nil
					},
				),
			},
		},
	})
}

// TestManagedAWSVPCResourceCreateActiveImmediately verifies the happy path
// where the status is already ACTIVE on the first poll (no waiting).
func TestManagedAWSVPCResourceCreateActiveImmediately(t *testing.T) {
	t.Parallel()

	m := &vpcMock{} // ACTIVE on the first Get
	setupVPCMock(t, m)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: managedAWSVPCConfig(m.server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_managed_aws_vpc.vpc", "id", "vpc-test-id"),
					func(s *terraform.State) error {
						require.Len(t, m.server.GetCapturedRequests("GetCloudComponentVpc"), 1,
							"status was already ACTIVE, so a single poll should suffice")
						return nil
					},
				),
			},
		},
	})
}

// TestManagedAWSVPCResourceCreateFailedErrors verifies that a FAILED status
// during create surfaces an error carrying the status_error detail.
func TestManagedAWSVPCResourceCreateFailedErrors(t *testing.T) {
	t.Parallel()

	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateCloudComponentVpc().WithBehavior(func(req proto.Message) (proto.Message, error) {
		return &serverv1.CreateCloudComponentVpcResponse{Vpc: vpcResponse("PENDING", "")}, nil
	})
	// FAILED until the tainted resource is destroyed, then not-found.
	deleted := &atomic.Bool{}
	server.OnGetCloudComponentVpc().WithBehavior(func(req proto.Message) (proto.Message, error) {
		if deleted.Load() {
			return nil, notFound("vpc not found")
		}
		return &serverv1.GetCloudComponentVpcResponse{Vpc: vpcResponse("FAILED", "subnet cidr overlaps")}, nil
	})
	server.OnDeleteCloudComponentVpc().WithBehavior(func(req proto.Message) (proto.Message, error) {
		deleted.Store(true)
		return &serverv1.DeleteCloudComponentVpcResponse{}, nil
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      managedAWSVPCConfig(server.URL),
				ExpectError: regexp.MustCompile(`did not become active`),
			},
		},
	})
}

// TestManagedAWSVPCResourceDeleteWaitsForDeleting verifies that Delete keeps
// polling while the VPC reports a DELETING status and only completes once it is
// gone, rather than returning as soon as deletion is requested.
func TestManagedAWSVPCResourceDeleteWaitsForDeleting(t *testing.T) {
	t.Parallel()

	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateCloudComponentVpc().WithBehavior(func(req proto.Message) (proto.Message, error) {
		return &serverv1.CreateCloudComponentVpcResponse{Vpc: vpcResponse("PENDING", "")}, nil
	})

	// Get returns ACTIVE until delete, then reports DELETING for two polls
	// (which must NOT be treated as gone) before finally reporting not-found.
	var deleteGets atomic.Int32
	deleted := &atomic.Bool{}
	server.OnGetCloudComponentVpc().WithBehavior(func(req proto.Message) (proto.Message, error) {
		if deleted.Load() {
			if deleteGets.Add(1) >= 3 {
				return nil, notFound("vpc not found")
			}
			return &serverv1.GetCloudComponentVpcResponse{Vpc: vpcResponse("DELETING", "")}, nil
		}
		return &serverv1.GetCloudComponentVpcResponse{Vpc: vpcResponse("ACTIVE", "")}, nil
	})

	server.OnDeleteCloudComponentVpc().WithBehavior(func(req proto.Message) (proto.Message, error) {
		deleted.Store(true)
		return &serverv1.DeleteCloudComponentVpcResponse{}, nil
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: managedAWSVPCConfig(server.URL)},
			{
				Config: providerConfig(server.URL),
				Check: func(s *terraform.State) error {
					require.Len(t, server.GetCapturedRequests("DeleteCloudComponentVpc"), 1)
					assert.GreaterOrEqual(t, int(deleteGets.Load()), 3, "expected Delete to keep polling through DELETING")
					return nil
				},
			},
		},
	})
}

// TestManagedAWSVPCResourceDeleteCompletesOnDeletedStatus verifies that a
// terminal DELETED status (without a not-found error) completes the delete.
func TestManagedAWSVPCResourceDeleteCompletesOnDeletedStatus(t *testing.T) {
	t.Parallel()

	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateCloudComponentVpc().WithBehavior(func(req proto.Message) (proto.Message, error) {
		return &serverv1.CreateCloudComponentVpcResponse{Vpc: vpcResponse("PENDING", "")}, nil
	})

	deleted := &atomic.Bool{}
	server.OnGetCloudComponentVpc().WithBehavior(func(req proto.Message) (proto.Message, error) {
		status := "ACTIVE"
		if deleted.Load() {
			status = "DELETED"
		}
		return &serverv1.GetCloudComponentVpcResponse{Vpc: vpcResponse(status, "")}, nil
	})

	server.OnDeleteCloudComponentVpc().WithBehavior(func(req proto.Message) (proto.Message, error) {
		deleted.Store(true)
		return &serverv1.DeleteCloudComponentVpcResponse{}, nil
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: managedAWSVPCConfig(server.URL)},
			{
				Config: providerConfig(server.URL),
				Check: func(s *terraform.State) error {
					require.Len(t, server.GetCapturedRequests("DeleteCloudComponentVpc"), 1)
					return nil
				},
			},
		},
	})
}
