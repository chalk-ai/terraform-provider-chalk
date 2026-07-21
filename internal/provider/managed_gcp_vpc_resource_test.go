package provider

import (
	"sync/atomic"
	"testing"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

const managedGCPVPCHCL = `
resource "chalk_managed_gcp_vpc" "vpc" {
  cloud_credential_id = "cc-gcp-test"
  vpc_peer_addr       = "10.0.0.0/16"
  subnets = [
    {
      name       = "primary"
      cidr_range = "10.1.0.0/20"
      purpose    = "PRIVATE"
      secondary_ip_ranges = [
        { range_name = "pods", ip_cidr_range = "10.2.0.0/16" },
        { range_name = "services", ip_cidr_range = "10.3.0.0/20" },
      ]
    },
    {
      name       = "proxy-active"
      cidr_range = "10.4.0.0/23"
      purpose    = "REGIONAL_MANAGED_PROXY"
      role       = "ACTIVE"
    },
  ]
  backup_subnets = [
    {
      name       = "proxy-backup"
      cidr_range = "10.5.0.0/23"
      purpose    = "REGIONAL_MANAGED_PROXY"
    },
  ]
}
`

// setupMockGCPVPCServer wires a stateful mock that echoes the create request's
// GCP spec back (status ACTIVE) so the model round-trips, and reports not-found
// once the resource is deleted.
func setupMockGCPVPCServer(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	var current *serverv1.CloudComponentVpcResponse
	deleted := &atomic.Bool{}

	server.OnCreateCloudComponentVpc().WithBehavior(func(req proto.Message) (proto.Message, error) {
		createReq := req.(*serverv1.CreateCloudComponentVpcRequest)
		spec := createReq.Vpc.Spec
		spec.Name = "test-gcp-vpc" // the server assigns the managed name
		current = &serverv1.CloudComponentVpcResponse{
			Id:                "gcp-vpc-test-id",
			Name:              "test-gcp-vpc",
			Kind:              createReq.Vpc.Kind,
			Managed:           true,
			CloudCredentialId: createReq.Vpc.CloudCredentialId,
			Status:            "ACTIVE",
			Spec:              spec,
		}
		return &serverv1.CreateCloudComponentVpcResponse{Vpc: current}, nil
	})

	server.OnGetCloudComponentVpc().WithBehavior(func(req proto.Message) (proto.Message, error) {
		if deleted.Load() {
			return nil, notFound("vpc not found")
		}
		return &serverv1.GetCloudComponentVpcResponse{Vpc: current}, nil
	})

	server.OnDeleteCloudComponentVpc().WithBehavior(func(req proto.Message) (proto.Message, error) {
		deleted.Store(true)
		return &serverv1.DeleteCloudComponentVpcResponse{}, nil
	})

	return server
}

func TestManagedGCPVPCResourceCreate(t *testing.T) {
	t.Parallel()
	server := setupMockGCPVPCServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + managedGCPVPCHCL,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_managed_gcp_vpc.vpc", "id", "gcp-vpc-test-id"),
					resource.TestCheckResourceAttr("chalk_managed_gcp_vpc.vpc", "vpc_peer_addr", "10.0.0.0/16"),
					resource.TestCheckResourceAttr("chalk_managed_gcp_vpc.vpc", "subnets.0.name", "primary"),
					resource.TestCheckResourceAttr("chalk_managed_gcp_vpc.vpc", "subnets.0.purpose", "PRIVATE"),
					resource.TestCheckResourceAttr("chalk_managed_gcp_vpc.vpc", "subnets.0.secondary_ip_ranges.0.range_name", "pods"),
					resource.TestCheckResourceAttr("chalk_managed_gcp_vpc.vpc", "subnets.0.secondary_ip_ranges.1.ip_cidr_range", "10.3.0.0/20"),
					resource.TestCheckResourceAttr("chalk_managed_gcp_vpc.vpc", "subnets.1.role", "ACTIVE"),
					// role omitted on a backup subnet defaults to BACKUP and is
					// reflected back into state (Optional + Computed).
					resource.TestCheckResourceAttr("chalk_managed_gcp_vpc.vpc", "backup_subnets.0.role", "BACKUP"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("CreateCloudComponentVpc")
						require.Len(t, captured, 1)
						req := captured[0].(*serverv1.CreateCloudComponentVpcRequest)
						assert.Equal(t, "gcp", req.Vpc.GetKind())
						gcp := req.Vpc.Spec.Config.GetGcp()
						require.NotNil(t, gcp)
						assert.Equal(t, "10.0.0.0/16", gcp.GetVpcPeerAddr())
						require.Len(t, gcp.Subnets, 2)
						// Primary PRIVATE subnet sends no role.
						assert.Nil(t, gcp.Subnets[0].Role)
						assert.Equal(t, "ACTIVE", gcp.Subnets[1].GetRole())
						require.Len(t, gcp.BackupSubnets, 1)
						// Backup subnet without a role is defaulted to BACKUP.
						assert.Equal(t, "BACKUP", gcp.BackupSubnets[0].GetRole())
						return nil
					},
				),
			},
			{
				// Re-applying the identical config must not plan any change — in
				// particular the computed BACKUP role default must round-trip
				// without a perpetual diff.
				Config: providerConfig(server.URL) + managedGCPVPCHCL,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestManagedGCPVPCResourceDelete(t *testing.T) {
	t.Parallel()
	server := setupMockGCPVPCServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: providerConfig(server.URL) + managedGCPVPCHCL},
			{
				Config: providerConfig(server.URL),
				Check: func(s *terraform.State) error {
					captured := server.GetCapturedRequests("DeleteCloudComponentVpc")
					require.Len(t, captured, 1)
					assert.Equal(t, "gcp-vpc-test-id", captured[0].(*serverv1.DeleteCloudComponentVpcRequest).GetId())
					return nil
				},
			},
		},
	})
}
