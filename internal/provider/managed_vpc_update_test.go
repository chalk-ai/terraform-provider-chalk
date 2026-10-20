package provider

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/gen/chalk/server/v1/serverv1connect"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

type updatingVPCMock struct {
	server         *testserver.MockServer
	mu             sync.Mutex
	current        *serverv1.CloudComponentVpcResponse
	pending        *serverv1.CloudComponentVpc
	requests       []*serverv1.UpdateCloudComponentVpcRequest
	updatingReads  int
	failRPC        bool
	failDeployment bool
}

func newUpdatingVPCMock(t *testing.T) *updatingVPCMock {
	t.Helper()
	m := &updatingVPCMock{server: testserver.NewMockBuilderServer(t)}
	t.Cleanup(m.server.Close)
	m.server.OnCreateCloudComponentVpc().WithBehavior(func(msg proto.Message) (proto.Message, error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		req := msg.(*serverv1.CreateCloudComponentVpcRequest)
		spec := proto.Clone(req.Vpc.Spec).(*serverv1.CloudComponentVpc)
		spec.Name = "stable-vpc"
		spec.Designator = new("stable")
		m.current = &serverv1.CloudComponentVpcResponse{Id: "stable-vpc-id", Name: spec.Name, Designator: spec.Designator, Kind: req.Vpc.Kind, Managed: true, CloudCredentialId: req.Vpc.CloudCredentialId, Spec: spec, Status: "ACTIVE"}
		return &serverv1.CreateCloudComponentVpcResponse{Vpc: proto.Clone(m.current).(*serverv1.CloudComponentVpcResponse)}, nil
	})
	m.server.OnGetCloudComponentVpc().WithBehavior(func(msg proto.Message) (proto.Message, error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.current == nil {
			return nil, notFound("deleted")
		}
		if m.pending != nil {
			if m.failDeployment {
				m.failDeployment = false
				m.pending = nil
				m.current.Status = "FAILED"
				m.current.StatusError = new("subnet cidr overlaps")
			} else if m.updatingReads > 0 {
				m.updatingReads--
			} else {
				m.current.Spec = m.pending
				m.pending = nil
				m.current.Status = "ACTIVE"
				m.current.StatusError = nil
			}
		}
		return &serverv1.GetCloudComponentVpcResponse{Vpc: proto.Clone(m.current).(*serverv1.CloudComponentVpcResponse)}, nil
	})
	m.server.OnDeleteCloudComponentVpc().WithBehavior(func(msg proto.Message) (proto.Message, error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.current = nil
		return &serverv1.DeleteCloudComponentVpcResponse{}, nil
	})
	// The SDK mock server doesn't implement this newly added RPC yet.
	m.server.Config.Handler.(*http.ServeMux).Handle(serverv1connect.CloudComponentsServiceUpdateCloudComponentVpcProcedure,
		connect.NewUnaryHandler(serverv1connect.CloudComponentsServiceUpdateCloudComponentVpcProcedure, func(ctx context.Context, req *connect.Request[serverv1.UpdateCloudComponentVpcRequest]) (*connect.Response[serverv1.UpdateCloudComponentVpcResponse], error) {
			m.mu.Lock()
			defer m.mu.Unlock()
			m.requests = append(m.requests, proto.Clone(req.Msg).(*serverv1.UpdateCloudComponentVpcRequest))
			if m.failRPC {
				m.failRPC = false
				return nil, connect.NewError(connect.CodeUnimplemented, errors.New("update endpoint unavailable"))
			}
			next := proto.Clone(m.current.Spec).(*serverv1.CloudComponentVpc)
			for _, path := range req.Msg.GetUpdateMask().GetPaths() {
				switch path {
				case "spec.config.gcp.subnets":
					next.Config.GetGcp().Subnets = req.Msg.Vpc.Spec.Config.GetGcp().Subnets
				case "spec.config.gcp.backup_subnets":
					next.Config.GetGcp().BackupSubnets = req.Msg.Vpc.Spec.Config.GetGcp().BackupSubnets
				case "spec.config.aws.additional_cidr_blocks":
					next.Config.GetAws().AdditionalCidrBlocks = req.Msg.Vpc.Spec.Config.GetAws().AdditionalCidrBlocks
				case "spec.config.aws.subnets":
					next.Config.GetAws().Subnets = req.Msg.Vpc.Spec.Config.GetAws().Subnets
				case "spec.config.aws.additional_public_routes":
					next.Config.GetAws().AdditionalPublicRoutes = req.Msg.Vpc.Spec.Config.GetAws().AdditionalPublicRoutes
				case "spec.config.aws.additional_private_routes":
					next.Config.GetAws().AdditionalPrivateRoutes = req.Msg.Vpc.Spec.Config.GetAws().AdditionalPrivateRoutes
				default:
					return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unexpected update mask: "+path))
				}
			}
			m.pending = next
			m.current.Status = "UPDATING"
			return connect.NewResponse(&serverv1.UpdateCloudComponentVpcResponse{Vpc: proto.Clone(m.current).(*serverv1.CloudComponentVpcResponse)}), nil
		}))
	return m
}

func (m *updatingVPCMock) check(t *testing.T, paths []string, count int) resource.TestCheckFunc {
	t.Helper()
	return func(s *terraform.State) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		require.Len(t, m.requests, count)
		assert.Equal(t, paths, m.requests[count-1].UpdateMask.Paths)
		assert.Equal(t, "stable-vpc-id", m.requests[count-1].Id)
		assert.Empty(t, m.requests[count-1].Vpc.Spec.Name)
		assert.Nil(t, m.requests[count-1].Vpc.CloudCredentialId)
		assert.Equal(t, "ACTIVE", m.current.Status)
		assert.Nil(t, m.pending)
		require.Len(t, m.server.GetCapturedRequests("CreateCloudComponentVpc"), 1)
		assert.Empty(t, m.server.GetCapturedRequests("DeleteCloudComponentVpc"))
		return nil
	}
}

func updateVPCPlanCheck(address string) resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate)}}
}

func TestManagedGCPVPCUpdateRangesInPlace(t *testing.T) {
	t.Parallel()
	m := newUpdatingVPCMock(t)
	base := providerConfig(m.server.URL) + managedGCPVPCHCL
	// Insert ahead of services to reproduce the positional list diff from production.
	expanded := strings.Replace(base, `{ range_name = "services"`, `{ range_name = "pods-2", ip_cidr_range = "10.6.0.0/17" },
        { range_name = "services"`, 1)
	before, _, found := strings.Cut(expanded, "  backup_subnets = [")
	require.True(t, found)
	withoutBackup := before + "}\n"
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testProtoV6ProviderFactories(), Steps: []resource.TestStep{
		{Config: base},
		{PreConfig: func() { m.mu.Lock(); defer m.mu.Unlock(); m.updatingReads = 1 }, Config: expanded, ConfigPlanChecks: updateVPCPlanCheck("chalk_managed_gcp_vpc.vpc"), Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("chalk_managed_gcp_vpc.vpc", "id", "stable-vpc-id"),
			resource.TestCheckResourceAttr("chalk_managed_gcp_vpc.vpc", "subnets.0.secondary_ip_ranges.#", "3"),
			resource.TestCheckResourceAttr("chalk_managed_gcp_vpc.vpc", "vpc_peer_addr", "10.0.0.0/16"),
			m.check(t, []string{"spec.config.gcp.subnets"}, 1),
		)},
		{Config: expanded, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{Config: withoutBackup, ConfigPlanChecks: updateVPCPlanCheck("chalk_managed_gcp_vpc.vpc"), Check: m.check(t, []string{"spec.config.gcp.backup_subnets"}, 2)},
		{Config: base, ConfigPlanChecks: updateVPCPlanCheck("chalk_managed_gcp_vpc.vpc"), Check: m.check(t, []string{"spec.config.gcp.subnets", "spec.config.gcp.backup_subnets"}, 3)},
	}})
}

func TestManagedVPCUpdateFailurePreservesState(t *testing.T) {
	for _, mode := range []string{"rpc", "deployment"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			m := newUpdatingVPCMock(t)
			base := providerConfig(m.server.URL) + managedGCPVPCHCL
			expanded := strings.Replace(base, "10.2.0.0/16", "10.2.0.0/15", 1)
			resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testProtoV6ProviderFactories(), Steps: []resource.TestStep{
				{Config: base},
				{PreConfig: func() {
					m.mu.Lock()
					defer m.mu.Unlock()
					m.failRPC = mode == "rpc"
					m.failDeployment = mode == "deployment"
				}, Config: expanded, ExpectError: regexp.MustCompile(`(?s)update\s+endpoint unavailable|subnet cidr overlaps`)},
				{Config: base, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
				{Config: expanded, ConfigPlanChecks: updateVPCPlanCheck("chalk_managed_gcp_vpc.vpc"), Check: m.check(t, []string{"spec.config.gcp.subnets"}, 2)},
			}})
		})
	}
}

func TestManagedAWSVPCUpdateInPlace(t *testing.T) {
	t.Parallel()
	m := newUpdatingVPCMock(t)
	base := managedAWSVPCConfig(m.server.URL)
	expanded := strings.Replace(base, `cidr_block          = "10.130.128.0/18"`, `cidr_block = "10.130.128.0/18"
 additional_cidr_blocks = ["10.131.0.0/16"]
 additional_private_routes = [{ name = "peer", destination_cidr_block = "10.140.0.0/16", peer_id = "pcx-test" }]`, 1)
	cleared := strings.Replace(expanded, `additional_private_routes = [{ name = "peer", destination_cidr_block = "10.140.0.0/16", peer_id = "pcx-test" }]`, "", 1)
	require.NotEqual(t, base, expanded)
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testProtoV6ProviderFactories(), Steps: []resource.TestStep{
		{Config: base},
		{Config: expanded, ConfigPlanChecks: updateVPCPlanCheck("chalk_managed_aws_vpc.vpc"), Check: m.check(t, []string{"spec.config.aws.additional_cidr_blocks", "spec.config.aws.additional_private_routes"}, 1)},
		{Config: cleared, ConfigPlanChecks: updateVPCPlanCheck("chalk_managed_aws_vpc.vpc"), Check: m.check(t, []string{"spec.config.aws.additional_private_routes"}, 2)},
		{Config: cleared, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}

func TestVPCUpdateMasks(t *testing.T) {
	t.Parallel()
	stringsList := func(v ...string) types.List {
		l, d := types.ListValueFrom(t.Context(), types.StringType, v)
		require.False(t, d.HasError())
		return l
	}
	state := ManagedAWSVPCResourceModel{AdditionalCidrBlocks: stringsList("10.1.0.0/16"), Subnets: types.ListNull(types.StringType), AdditionalPublicRoutes: types.ListNull(types.StringType), AdditionalPrivateRoutes: types.ListNull(types.StringType)}
	plan := state
	plan.AdditionalCidrBlocks = types.ListNull(types.StringType)
	assert.Equal(t, []string{"spec.config.aws.additional_cidr_blocks"}, buildAWSVPCUpdateMask(plan, state))
	plan = state
	plan.AdditionalCidrBlocks = types.ListUnknown(types.StringType)
	assert.Empty(t, buildAWSVPCUpdateMask(plan, state))
	gcp := &serverv1.GCPVpcConfig{}
	assert.Empty(t, buildGCPVPCUpdateMask(gcp, gcp))
}
