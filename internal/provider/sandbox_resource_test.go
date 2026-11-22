package provider

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	containerv1 "github.com/chalk-ai/chalk-go/gen/chalk/container/v1"
	sandboxv2 "github.com/chalk-ai/chalk-go/gen/chalk/sandbox/v2"
	"github.com/chalk-ai/chalk-go/gen/chalk/sandbox/v2/sandboxv2connect"
	"github.com/chalk-ai/chalk-go/testserver"
	chalkclient "github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// sandboxMockState records what the mock server was asked to create so tests can
// assert on the request, and drives the state machine GetSandbox reports back.
type sandboxMockState struct {
	created              *sandboxv2.CreateSandboxRequest
	info                 *sandboxv2.SandboxInfo
	getCalls             int
	readyAfter           int
	createCalls          int
	suspendCalls         int
	restoreCalls         int
	terminateCalls       int
	terminated           bool
	terminateErr         error
	snapshots            []*sandboxv2.SandboxSnapshot
	snapshotPageSize     int
	snapshotListCalls    int
	snapshotListFilters  [][]string
	deletedSnapshotIDs   []string
	snapshotListErr      error
	snapshotListErrAfter int
	snapshotDeleteErrs   map[string]error
	events               []string
}

type sandboxV2MockHandler struct {
	sandboxv2connect.UnimplementedSandboxServiceHandler
	state *sandboxMockState
}

func (h *sandboxV2MockHandler) CreateSandbox(
	_ context.Context,
	req *connect.Request[sandboxv2.CreateSandboxRequest],
) (*connect.Response[sandboxv2.CreateSandboxResponse], error) {
	h.state.createCalls++
	h.state.created = req.Msg
	h.state.terminated = false
	status := sandboxv2.SandboxStatus_SANDBOX_STATUS_PENDING
	if h.state.readyAfter == 0 {
		status = sandboxv2.SandboxStatus_SANDBOX_STATUS_RUNNING
	}
	name := req.Msg.GetSpec().GetName()
	if name == "" {
		name = "server-generated-name"
	}
	h.state.info = &sandboxv2.SandboxInfo{
		Id:            "sandbox-test-id",
		Name:          name,
		Status:        status,
		CreatedAt:     timestamppb.New(time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)),
		Spec:          req.Msg.GetSpec(),
		StatusMessage: new("running"),
	}
	return connect.NewResponse(&sandboxv2.CreateSandboxResponse{Sandbox: h.state.info}), nil
}

func (h *sandboxV2MockHandler) GetSandbox(
	_ context.Context,
	_ *connect.Request[sandboxv2.GetSandboxRequest],
) (*connect.Response[sandboxv2.GetSandboxResponse], error) {
	if h.state.terminated {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("sandbox not found"))
	}
	h.state.getCalls++
	if h.state.info.GetStatus() == sandboxv2.SandboxStatus_SANDBOX_STATUS_PENDING && h.state.getCalls >= h.state.readyAfter {
		h.state.info.Status = sandboxv2.SandboxStatus_SANDBOX_STATUS_RUNNING
		h.state.info.StatusMessage = new("running")
	}
	return connect.NewResponse(&sandboxv2.GetSandboxResponse{Sandbox: h.state.info}), nil
}

func (h *sandboxV2MockHandler) TerminateSandbox(
	_ context.Context,
	_ *connect.Request[sandboxv2.TerminateSandboxRequest],
) (*connect.Response[sandboxv2.TerminateSandboxResponse], error) {
	h.state.terminateCalls++
	h.state.events = append(h.state.events, "terminate")
	if h.state.terminateErr != nil {
		return nil, h.state.terminateErr
	}
	h.state.terminated = true
	return connect.NewResponse(&sandboxv2.TerminateSandboxResponse{}), nil
}

func (h *sandboxV2MockHandler) SuspendSandbox(
	_ context.Context,
	req *connect.Request[sandboxv2.SuspendSandboxRequest],
) (*connect.Response[sandboxv2.SuspendSandboxResponse], error) {
	h.state.suspendCalls++
	h.state.events = append(h.state.events, "suspend")
	if h.state.info != nil && req.Msg.GetId() == h.state.info.GetId() {
		h.state.info.Status = sandboxv2.SandboxStatus_SANDBOX_STATUS_SUSPENDED
		h.state.info.StatusMessage = new("suspended")
	}
	return connect.NewResponse(&sandboxv2.SuspendSandboxResponse{}), nil
}

func (h *sandboxV2MockHandler) ResumeSandbox(
	_ context.Context,
	req *connect.Request[sandboxv2.ResumeSandboxRequest],
) (*connect.Response[sandboxv2.ResumeSandboxResponse], error) {
	h.state.restoreCalls++
	h.state.events = append(h.state.events, "restore")
	if h.state.info != nil && req.Msg.GetId() == h.state.info.GetId() {
		h.state.info.Status = sandboxv2.SandboxStatus_SANDBOX_STATUS_RUNNING
		h.state.info.StatusMessage = new("running")
	}
	return connect.NewResponse(&sandboxv2.ResumeSandboxResponse{}), nil
}

type sandboxSnapshotMockHandler struct {
	sandboxv2connect.UnimplementedSandboxSnapshotServiceHandler
	state *sandboxMockState
}

func (h *sandboxSnapshotMockHandler) ListSandboxSnapshots(
	_ context.Context,
	req *connect.Request[sandboxv2.ListSandboxSnapshotsRequest],
) (*connect.Response[sandboxv2.ListSandboxSnapshotsResponse], error) {
	h.state.events = append(h.state.events, "list snapshots")
	h.state.snapshotListFilters = append(h.state.snapshotListFilters, append([]string(nil), req.Msg.GetFilters().GetSandboxIds()...))
	listCall := h.state.snapshotListCalls
	h.state.snapshotListCalls++
	if h.state.snapshotListErr != nil && listCall >= h.state.snapshotListErrAfter {
		return nil, h.state.snapshotListErr
	}
	start := 0
	if req.Msg.GetCursor() != "" {
		var err error
		start, err = strconv.Atoi(req.Msg.GetCursor())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}
	end := len(h.state.snapshots)
	if h.state.snapshotPageSize > 0 && start+h.state.snapshotPageSize < end {
		end = start + h.state.snapshotPageSize
	}
	var nextCursor *string
	if end < len(h.state.snapshots) {
		nextCursor = new(strconv.Itoa(end))
	}
	// Return snapshots for other sandboxes too so the provider tests its
	// defensive sandbox-id check instead of relying only on server filtering.
	return connect.NewResponse(&sandboxv2.ListSandboxSnapshotsResponse{
		Snapshots:  h.state.snapshots[start:end],
		NextCursor: nextCursor,
	}), nil
}

func (h *sandboxSnapshotMockHandler) DeleteSandboxSnapshot(
	_ context.Context,
	req *connect.Request[sandboxv2.DeleteSandboxSnapshotRequest],
) (*connect.Response[sandboxv2.DeleteSandboxSnapshotResponse], error) {
	h.state.events = append(h.state.events, "delete snapshot "+req.Msg.GetSnapshotId())
	if err := h.state.snapshotDeleteErrs[req.Msg.GetSnapshotId()]; err != nil {
		return nil, err
	}
	h.state.deletedSnapshotIDs = append(h.state.deletedSnapshotIDs, req.Msg.GetSnapshotId())
	return connect.NewResponse(&sandboxv2.DeleteSandboxSnapshotResponse{}), nil
}

// setupMockSandboxServer wires a sandbox lifecycle onto the mock server. The
// created sandbox reports pending until readyAfter GetSandbox calls have
// landed, which exercises the Create-time wait loop.
func setupMockSandboxServer(t *testing.T, readyAfter int) (*testserver.MockServer, *sandboxMockState) {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	st := &sandboxMockState{readyAfter: readyAfter}

	// Mount the v2 sandbox and snapshot handlers alongside the builder mock used
	// by provider configuration.
	existingHandler := server.Config.Handler
	mux := http.NewServeMux()
	v2Path, v2Handler := sandboxv2connect.NewSandboxServiceHandler(&sandboxV2MockHandler{state: st})
	mux.Handle(v2Path, v2Handler)
	snapshotPath, snapshotHandler := sandboxv2connect.NewSandboxSnapshotServiceHandler(&sandboxSnapshotMockHandler{state: st})
	mux.Handle(snapshotPath, snapshotHandler)
	mux.Handle("/", existingHandler)
	server.Config.Handler = mux

	return server, st
}

func sandboxMinimalConfig(serverURL, name string, prune ...bool) string {
	pruneConfig := ""
	if len(prune) > 0 {
		pruneConfig = fmt.Sprintf("\n  prune_snapshots_on_suspend = %t", prune[0])
	}
	return providerConfig(serverURL) + fmt.Sprintf(`
resource "chalk_sandbox" "test" {
  environment_id = "test-env-id"
  name           = %q
  image          = "debian:bookworm"%s
}
`, name, pruneConfig)
}

func sandboxDesiredStateConfig(serverURL, name, desiredState string, prune ...bool) string {
	pruneConfig := ""
	if len(prune) > 0 {
		pruneConfig = fmt.Sprintf("\n  prune_snapshots_on_suspend = %t", prune[0])
	}
	return providerConfig(serverURL) + fmt.Sprintf(`
resource "chalk_sandbox" "test" {
  environment_id = "test-env-id"
  name           = %q
  image          = "debian:bookworm"
  desired_state  = %q%s
}
`, name, desiredState, pruneConfig)
}

func sandboxKubeDesiredStateConfig(serverURL, desiredState string) string {
	return providerConfig(serverURL) + fmt.Sprintf(`
resource "chalk_sandbox" "test" {
  environment_id = "test-env-id"
  image          = "debian:bookworm"
  runtime        = "kube"
  desired_state  = %q
}
`, desiredState)
}

func sandboxConfigWithoutName(serverURL string) string {
	return providerConfig(serverURL) + `
resource "chalk_sandbox" "test" {
  environment_id = "test-env-id"
  image          = "debian:bookworm"
}
`
}

func sandboxResourceForTest(serverURL string) *SandboxResource {
	return &SandboxResource{client: chalkclient.NewManager(&chalkclient.Config{
		ApiServer: serverURL,
		JWT:       "test-jwt",
	})}
}

func sandboxFullConfig(serverURL, name string) string {
	return providerConfig(serverURL) + fmt.Sprintf(`
resource "chalk_sandbox" "test" {
  environment_id = "test-env-id"
  name           = %q
  image          = "debian:bookworm"
  entrypoint     = ["/bin/bash", "-c", "sleep infinity"]
  env            = { CODER_AGENT_TOKEN = "secret-token" }
  runtime        = "kube"
  restart_policy = "RESTART_POLICY_ALWAYS"

  resource_limits = {
    cpu    = "2"
    memory = "4Gi"
  }

  volumes = [{
    name       = "home"
    mount_path = "/home/coder"
    type       = "empty_dir"
    size_limit = "20Gi"
  }]

  network_policy = {
    allowed_routes = [{
      route       = "0.0.0.0/0"
      port_ranges = [{ start_port = 1, end_port = 65535 }]
    }]
    denied_routes = ["169.254.169.254/32"]
  }
}
`, name)
}

func TestSandboxResourceNameOwnership(t *testing.T) {
	name := "coder-jeff-sbxmeasure"
	data := SandboxResourceModel{
		Name:  types.StringValue(name),
		Image: types.StringValue("debian:bookworm"),
	}

	createReq, diags := buildCreateSandboxRequest(context.Background(), &data)
	require.False(t, diags.HasError())
	assert.Equal(t, name, createReq.GetSpec().GetName())

	data.Name = types.StringNull()
	updateSandboxState(&data, &sandboxv2.SandboxInfo{Name: name})
	assert.True(t, data.Name.IsNull())
}

func TestSandboxResourceCreate(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	name := "test-sandbox-" + uuid.New().String()[:8]

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: sandboxMinimalConfig(server.URL, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_sandbox.test", "id", "sandbox-test-id"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "environment_id", "test-env-id"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "name", name),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "image", "debian:bookworm"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "running"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "created_at", "2026-08-19T00:00:00Z"),
					// wait_for_ready has a default rather than being left null.
					resource.TestCheckResourceAttr("chalk_sandbox.test", "wait_for_ready", "true"),
					resource.TestCheckNoResourceAttr("chalk_sandbox.test", "desired_state"),
				),
			},
		},
	})

	require.NotNil(t, st.created)
	require.NotNil(t, st.created.GetSpec())
	assert.Equal(t, "debian:bookworm", st.created.GetSpec().GetImage())
	assert.Equal(t, name, st.created.GetSpec().GetName())
	assert.GreaterOrEqual(t, st.getCalls, 1, "Read must keep the name returned by sandbox v2")
	// Nothing unset should be sent: an empty network policy is not the same as
	// no network policy, and the server treats the former as deny-all.
	assert.Nil(t, st.created.GetSpec().NetworkPolicy)
	assert.Nil(t, st.created.GetSpec().Resources)
	assert.Nil(t, st.created.GetSpec().ComputeClass)
	assert.Empty(t, st.created.GetSpec().Entrypoint)
	assert.Nil(t, st.created.GetSpec().ChalkWorkloadIdentity)
}

func TestSandboxResourceRejectsDesiredStateWithKubeRuntime(t *testing.T) {
	t.Parallel()

	for _, desiredState := range []string{"running", "suspended"} {
		t.Run(desiredState, func(t *testing.T) {
			t.Parallel()
			server, st := setupMockSandboxServer(t, 0)

			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
				Steps: []resource.TestStep{{
					Config:      sandboxKubeDesiredStateConfig(server.URL, desiredState),
					PlanOnly:    true,
					ExpectError: regexp.MustCompile(`(?s)desired_state.*cannot be set.*runtime.*kube.*suspend and.*resume.*host-compute`),
				}},
			})

			assert.Zero(t, st.createCalls)
		})
	}
}

func TestSandboxResourceAcceptsServerGeneratedName(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: sandboxConfigWithoutName(server.URL),
				Check:  resource.TestCheckNoResourceAttr("chalk_sandbox.test", "name"),
			},
			{
				Config:             sandboxConfigWithoutName(server.URL),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})

	assert.Equal(t, 1, st.createCalls)
}

func TestSandboxResourceDoesNotManageStateWhenDesiredStateUnset(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: sandboxConfigWithoutName(server.URL)},
			{
				PreConfig: func() {
					st.info.Status = sandboxv2.SandboxStatus_SANDBOX_STATUS_SUSPENDED
					st.info.StatusMessage = new("suspended out of band")
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: false,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("chalk_sandbox.test", "desired_state"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "suspended"),
					func(*terraform.State) error {
						if st.suspendCalls != 0 || st.restoreCalls != 0 {
							return fmt.Errorf("unexpected lifecycle calls: suspend=%d resume=%d", st.suspendCalls, st.restoreCalls)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestSandboxResourceCreateSuspended(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 2)
	name := "test-sandbox-" + uuid.New().String()[:8]

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{{
			Config: sandboxDesiredStateConfig(server.URL, name, "suspended"),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("chalk_sandbox.test", "id", "sandbox-test-id"),
				resource.TestCheckResourceAttr("chalk_sandbox.test", "desired_state", "suspended"),
				resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "suspended"),
			),
		}},
	})

	assert.Equal(t, 1, st.suspendCalls)
	assert.GreaterOrEqual(t, st.getCalls, 2, "create must become ready before it is suspended")
}

func TestSandboxResourceUpdatesDesiredStateInPlace(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	name := "test-sandbox-" + uuid.New().String()[:8]

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: sandboxDesiredStateConfig(server.URL, name, "running"),
				Check:  resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "running"),
			},
			{
				Config: sandboxDesiredStateConfig(server.URL, name, "suspended"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_sandbox.test", "id", "sandbox-test-id"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "suspended"),
				),
			},
			{
				Config: sandboxDesiredStateConfig(server.URL, name, "running"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_sandbox.test", "id", "sandbox-test-id"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "running"),
				),
			},
		},
	})

	assert.Equal(t, 1, st.createCalls, "desired_state updates must not replace the sandbox")
	assert.Equal(t, 1, st.suspendCalls)
	assert.Equal(t, 1, st.restoreCalls)
}

func TestSandboxResourcePrunesOnlyCompletedSnapshotsOnSuspendAcrossPages(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	name := "test-sandbox-" + uuid.New().String()[:8]
	completed := sandboxv2.SandboxSnapshotStatus_SANDBOX_SNAPSHOT_STATUS_COMPLETED
	st.snapshotPageSize = 2
	st.snapshots = []*sandboxv2.SandboxSnapshot{
		{Id: "old-first-page", SandboxId: "sandbox-test-id", Status: completed, CompletedAt: timestamppb.New(time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC))},
		{Id: "other-sandbox", SandboxId: "other-sandbox-id", Status: completed, CompletedAt: timestamppb.New(time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC))},
		{Id: "in-progress", SandboxId: "sandbox-test-id", Status: sandboxv2.SandboxSnapshotStatus_SANDBOX_SNAPSHOT_STATUS_IN_PROGRESS, CompletedAt: timestamppb.New(time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC))},
		{Id: "newest", SandboxId: "sandbox-test-id", Status: completed, CompletedAt: timestamppb.New(time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC))},
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: sandboxDesiredStateConfig(server.URL, name, "running", true)},
			{
				Config: sandboxDesiredStateConfig(server.URL, name, "suspended", true),
				Check: func(*terraform.State) error {
					assert.Len(t, st.snapshotListFilters, 2)
					assert.Equal(t, []string{"old-first-page"}, st.deletedSnapshotIDs)
					assert.NotContains(t, st.deletedSnapshotIDs, "newest")
					assert.NotContains(t, st.deletedSnapshotIDs, "in-progress")
					return nil
				},
			},
		},
	})
}

func TestSandboxResourcePruneDiagnosticsAndCompletionGuard(t *testing.T) {
	t.Parallel()
	completed := sandboxv2.SandboxSnapshotStatus_SANDBOX_SNAPSHOT_STATUS_COMPLETED
	tests := []struct {
		name        string
		snapshots   []*sandboxv2.SandboxSnapshot
		listErr     error
		deleteErrs  map[string]error
		wantDeleted []string
		wantWarning string
	}{
		{
			name: "no completed snapshot",
			snapshots: []*sandboxv2.SandboxSnapshot{
				{Id: "creating", SandboxId: "sandbox-test-id", Status: sandboxv2.SandboxSnapshotStatus_SANDBOX_SNAPSHOT_STATUS_PENDING},
				{Id: "failed", SandboxId: "sandbox-test-id", Status: sandboxv2.SandboxSnapshotStatus_SANDBOX_SNAPSHOT_STATUS_FAILED},
			},
		},
		{
			name:        "list failure",
			listErr:     connect.NewError(connect.CodePermissionDenied, fmt.Errorf("permission denied")),
			wantWarning: "Could not prune sandbox snapshots",
		},
		{
			name: "delete failures",
			snapshots: []*sandboxv2.SandboxSnapshot{
				{Id: "held-old", SandboxId: "sandbox-test-id", Status: completed, CompletedAt: timestamppb.New(time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC))},
				{Id: "denied-old", SandboxId: "sandbox-test-id", Status: completed, CompletedAt: timestamppb.New(time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC))},
				{Id: "deleted-old", SandboxId: "sandbox-test-id", Status: completed, CompletedAt: timestamppb.New(time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC))},
				{Id: "newest", SandboxId: "sandbox-test-id", Status: completed, CompletedAt: timestamppb.New(time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC))},
			},
			deleteErrs: map[string]error{
				"held-old":   connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("snapshot is in use")),
				"denied-old": connect.NewError(connect.CodePermissionDenied, fmt.Errorf("permission denied")),
			},
			wantDeleted: []string{"deleted-old"},
			wantWarning: "Could not prune sandbox snapshot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, st := setupMockSandboxServer(t, 0)
			st.snapshots = tt.snapshots
			st.snapshotListErr = tt.listErr
			st.snapshotDeleteErrs = tt.deleteErrs

			diags := sandboxResourceForTest(server.URL).pruneSandboxSnapshots(context.Background(), "test-env-id", "sandbox-test-id")

			assert.False(t, diags.HasError())
			assert.Equal(t, tt.wantDeleted, st.deletedSnapshotIDs)
			if tt.wantWarning == "" {
				assert.Empty(t, diags)
			} else {
				require.Len(t, diags, 1)
				assert.Equal(t, diag.SeverityWarning, diags[0].Severity())
				assert.Equal(t, tt.wantWarning, diags[0].Summary())
			}
		})
	}
}

func TestSandboxResourceDoesNotPruneSnapshotsWhenDisabled(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	name := "test-sandbox-" + uuid.New().String()[:8]
	st.snapshots = []*sandboxv2.SandboxSnapshot{{
		Id: "old", SandboxId: "sandbox-test-id",
		Status: sandboxv2.SandboxSnapshotStatus_SANDBOX_SNAPSHOT_STATUS_COMPLETED,
	}}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: sandboxDesiredStateConfig(server.URL, name, "running", false)},
			{
				Config: sandboxDesiredStateConfig(server.URL, name, "suspended", false),
				Check: func(*terraform.State) error {
					if len(st.snapshotListFilters) != 0 || len(st.deletedSnapshotIDs) != 0 {
						return fmt.Errorf("disabled pruning touched snapshots: lists=%v deletes=%v", st.snapshotListFilters, st.deletedSnapshotIDs)
					}
					return nil
				},
			},
		},
	})
}

func TestSandboxResourcePruneFlagUpdatesInPlace(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	name := "test-sandbox-" + uuid.New().String()[:8]

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: sandboxMinimalConfig(server.URL, name)},
			{
				Config: sandboxMinimalConfig(server.URL, name, true),
				Check:  resource.TestCheckResourceAttr("chalk_sandbox.test", "prune_snapshots_on_suspend", "true"),
			},
			{
				Config: sandboxMinimalConfig(server.URL, name, false),
				Check:  resource.TestCheckResourceAttr("chalk_sandbox.test", "prune_snapshots_on_suspend", "false"),
			},
		},
	})

	assert.Equal(t, 1, st.createCalls, "adding or flipping the prune flag must not replace the sandbox")
}

func TestSandboxResourceRepairsDesiredStateDrift(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	name := "test-sandbox-" + uuid.New().String()[:8]
	config := sandboxDesiredStateConfig(server.URL, name, "running")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					st.info.Status = sandboxv2.SandboxStatus_SANDBOX_STATUS_SUSPENDED
					st.info.StatusMessage = new("suspended out of band")
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_sandbox.test", "desired_state", "suspended"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "suspended"),
				),
			},
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_sandbox.test", "desired_state", "running"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "running"),
				),
			},
		},
	})

	assert.Equal(t, 1, st.restoreCalls)
}

func TestSandboxResourceRemovingDesiredStateDoesNotTransition(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	name := "test-sandbox-" + uuid.New().String()[:8]

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: sandboxDesiredStateConfig(server.URL, name, "running")},
			{
				Config: sandboxMinimalConfig(server.URL, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("chalk_sandbox.test", "desired_state"),
					func(*terraform.State) error {
						if st.suspendCalls != 0 || st.restoreCalls != 0 {
							return fmt.Errorf("unexpected lifecycle calls: suspend=%d resume=%d", st.suspendCalls, st.restoreCalls)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestSandboxResourceSkipsTransitionAlreadyAppliedServerSide(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	name := "test-sandbox-" + uuid.New().String()[:8]

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: sandboxDesiredStateConfig(server.URL, name, "running")},
			{
				PreConfig: func() {
					st.info.Status = sandboxv2.SandboxStatus_SANDBOX_STATUS_SUSPENDED
					st.info.StatusMessage = new("suspended")
				},
				Config: sandboxDesiredStateConfig(server.URL, name, "suspended"),
				Check:  resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "suspended"),
			},
			{
				PreConfig: func() {
					st.info.Status = sandboxv2.SandboxStatus_SANDBOX_STATUS_RUNNING
					st.info.StatusMessage = new("running")
				},
				Config: sandboxDesiredStateConfig(server.URL, name, "running"),
				Check:  resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "running"),
			},
		},
	})

	assert.Zero(t, st.suspendCalls)
	assert.Zero(t, st.restoreCalls)
}

func TestSandboxResourceOtherUpdatesStillReplace(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: sandboxMinimalConfig(server.URL, "original-name")},
			{Config: sandboxMinimalConfig(server.URL, "replacement-name")},
		},
	})

	assert.Equal(t, 2, st.createCalls)
	assert.Equal(t, 2, st.terminateCalls)
}

func TestSandboxResourceDeleteRunning(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	st.info = &sandboxv2.SandboxInfo{Id: "sandbox-test-id", Status: sandboxv2.SandboxStatus_SANDBOX_STATUS_RUNNING}

	err := sandboxResourceForTest(server.URL).deleteSandbox(context.Background(), "test-env-id", "sandbox-test-id")
	require.NoError(t, err)
	assert.Zero(t, st.restoreCalls)
	assert.Equal(t, []string{"list snapshots", "terminate"}, st.events)
	assert.Equal(t, [][]string{{"sandbox-test-id"}}, st.snapshotListFilters)
}

func TestSandboxResourceDeleteSuspendedRestoresAndCleansSnapshots(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	st.info = &sandboxv2.SandboxInfo{Id: "sandbox-test-id", Status: sandboxv2.SandboxStatus_SANDBOX_STATUS_SUSPENDED}
	st.snapshots = []*sandboxv2.SandboxSnapshot{
		{Id: "snapshot-owned", SandboxId: "sandbox-test-id"},
		{Id: "snapshot-other", SandboxId: "other-sandbox-id"},
	}

	err := sandboxResourceForTest(server.URL).deleteSandbox(context.Background(), "test-env-id", "sandbox-test-id")
	require.NoError(t, err)
	assert.Equal(t, 1, st.restoreCalls)
	assert.Equal(t, []string{"snapshot-owned"}, st.deletedSnapshotIDs)
	assert.Equal(t, []string{
		"restore",
		"list snapshots",
		"delete snapshot snapshot-owned",
		"terminate",
	}, st.events)
}

func TestSandboxResourceDeleteFinishedTreatsTerminateNotFoundAsGone(t *testing.T) {
	t.Parallel()
	for _, status := range []sandboxv2.SandboxStatus{
		sandboxv2.SandboxStatus_SANDBOX_STATUS_FAILED,
		sandboxv2.SandboxStatus_SANDBOX_STATUS_SUCCEEDED,
		sandboxv2.SandboxStatus_SANDBOX_STATUS_TERMINATED,
	} {
		t.Run(status.String(), func(t *testing.T) {
			server, st := setupMockSandboxServer(t, 0)
			st.info = &sandboxv2.SandboxInfo{Id: "sandbox-test-id", Status: status}
			st.terminateErr = connect.NewError(connect.CodeNotFound, fmt.Errorf("container not found"))

			err := sandboxResourceForTest(server.URL).deleteSandbox(context.Background(), "test-env-id", "sandbox-test-id")
			require.NoError(t, err)
			assert.Equal(t, []string{"list snapshots", "terminate"}, st.events)
		})
	}
}

func TestSandboxResourceDeleteRunningDoesNotIgnoreTerminateNotFound(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	st.info = &sandboxv2.SandboxInfo{Id: "sandbox-test-id", Status: sandboxv2.SandboxStatus_SANDBOX_STATUS_RUNNING}
	st.terminateErr = connect.NewError(connect.CodeNotFound, fmt.Errorf("container not found"))

	err := sandboxResourceForTest(server.URL).deleteSandbox(context.Background(), "test-env-id", "sandbox-test-id")
	require.ErrorContains(t, err, "could not terminate sandbox sandbox-test-id")
}

func TestSandboxResourceDeleteGoneStillCleansSnapshots(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	st.terminated = true
	st.snapshots = []*sandboxv2.SandboxSnapshot{{Id: "snapshot-owned", SandboxId: "sandbox-test-id"}}

	err := sandboxResourceForTest(server.URL).deleteSandbox(context.Background(), "test-env-id", "sandbox-test-id")
	require.NoError(t, err)
	assert.Equal(t, []string{"snapshot-owned"}, st.deletedSnapshotIDs)
	assert.Equal(t, []string{"list snapshots", "delete snapshot snapshot-owned"}, st.events)
	assert.Zero(t, st.terminateCalls)
}

func TestSandboxResourceSnapshotDeleteFailuresDoNotStopDeletion(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	st.info = &sandboxv2.SandboxInfo{Id: "sandbox-test-id", Status: sandboxv2.SandboxStatus_SANDBOX_STATUS_RUNNING}
	st.snapshots = []*sandboxv2.SandboxSnapshot{
		{Id: "snapshot-in-use", SandboxId: "sandbox-test-id"},
		{Id: "snapshot-gone", SandboxId: "sandbox-test-id"},
		{Id: "snapshot-error", SandboxId: "sandbox-test-id"},
		{Id: "snapshot-deleted", SandboxId: "sandbox-test-id"},
	}
	st.snapshotDeleteErrs = map[string]error{
		"snapshot-in-use": connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("snapshot is in use")),
		"snapshot-gone":   connect.NewError(connect.CodeNotFound, fmt.Errorf("snapshot not found")),
		"snapshot-error":  connect.NewError(connect.CodeInternal, fmt.Errorf("delete failed")),
	}

	err := sandboxResourceForTest(server.URL).deleteSandbox(context.Background(), "test-env-id", "sandbox-test-id")
	require.NoError(t, err)
	assert.Equal(t, []string{"snapshot-deleted"}, st.deletedSnapshotIDs)
	assert.Equal(t, 1, st.terminateCalls)
}

func TestSandboxResourceSnapshotListFailureDoesNotStopDeletion(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name         string
		snapshots    []*sandboxv2.SandboxSnapshot
		pageSize     int
		listErrAfter int
		wantDeleted  []string
		wantEvents   []string
	}{
		{
			name:       "first page",
			wantEvents: []string{"list snapshots", "terminate"},
		},
		{
			name: "later page",
			snapshots: []*sandboxv2.SandboxSnapshot{
				{Id: "partial", SandboxId: "sandbox-test-id"},
				{Id: "unlisted", SandboxId: "sandbox-test-id"},
			},
			pageSize:     1,
			listErrAfter: 1,
			wantDeleted:  []string{"partial"},
			wantEvents: []string{
				"list snapshots",
				"delete snapshot partial",
				"list snapshots",
				"terminate",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server, st := setupMockSandboxServer(t, 0)
			st.info = &sandboxv2.SandboxInfo{Id: "sandbox-test-id", Status: sandboxv2.SandboxStatus_SANDBOX_STATUS_RUNNING}
			st.snapshots = tt.snapshots
			st.snapshotPageSize = tt.pageSize
			st.snapshotListErr = connect.NewError(connect.CodeInternal, fmt.Errorf("list failed"))
			st.snapshotListErrAfter = tt.listErrAfter

			err := sandboxResourceForTest(server.URL).deleteSandbox(context.Background(), "test-env-id", "sandbox-test-id")
			require.NoError(t, err)
			assert.Equal(t, tt.wantDeleted, st.deletedSnapshotIDs)
			assert.Equal(t, tt.wantEvents, st.events)
		})
	}
}

func TestSandboxResourceCreateFullSpec(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	name := "test-sandbox-" + uuid.New().String()[:8]

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: sandboxFullConfig(server.URL, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_sandbox.test", "id", "sandbox-test-id"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "entrypoint.#", "3"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "resource_limits.cpu", "2"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "volumes.0.mount_path", "/home/coder"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "network_policy.allowed_routes.0.route", "0.0.0.0/0"),
				),
			},
		},
	})

	require.NotNil(t, st.created)
	spec := st.created.GetSpec()
	require.NotNil(t, spec)
	assert.Equal(t, []string{"/bin/bash", "-c", "sleep infinity"}, spec.GetEntrypoint())
	assert.Equal(t, map[string]string{"CODER_AGENT_TOKEN": "secret-token"}, spec.GetEnvVars())
	assert.Equal(t, containerv1.ComputeClass_COMPUTE_CLASS_K8S, spec.GetComputeClass())
	assert.Equal(t, containerv1.RestartPolicy_RESTART_POLICY_ALWAYS, spec.GetRestartPolicy())

	require.NotNil(t, spec.Resources)
	assert.Equal(t, "2", spec.Resources.GetCpu())
	assert.Equal(t, "4Gi", spec.Resources.GetMemory())

	require.Len(t, spec.Volumes, 1)
	assert.Equal(t, "home", spec.Volumes[0].GetName())
	assert.Equal(t, "/home/coder", spec.Volumes[0].GetMountPath())
	assert.Equal(t, "empty_dir", spec.Volumes[0].GetType())
	assert.Equal(t, "20Gi", spec.Volumes[0].GetSizeLimit())

	require.NotNil(t, spec.NetworkPolicy)
	require.Len(t, spec.NetworkPolicy.AllowedRoutes, 1)
	assert.Equal(t, "0.0.0.0/0", spec.NetworkPolicy.AllowedRoutes[0].GetRoute())
	require.Len(t, spec.NetworkPolicy.AllowedRoutes[0].PortRanges, 1)
	assert.Equal(t, int32(1), spec.NetworkPolicy.AllowedRoutes[0].PortRanges[0].GetStartPort())
	assert.Equal(t, int32(65535), spec.NetworkPolicy.AllowedRoutes[0].PortRanges[0].GetEndPort())
	assert.Equal(t, []string{"169.254.169.254/32"}, spec.NetworkPolicy.GetDeniedRoutes())
}

// A freshly created sandbox reports pending; the apply must not return
// until it settles, or downstream resources race a sandbox that cannot yet
// accept exec.
func TestSandboxResourceWaitsForReady(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 3)
	name := "test-sandbox-" + uuid.New().String()[:8]

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: sandboxMinimalConfig(server.URL, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "running"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "status_message", "running"),
				),
			},
		},
	})

	assert.GreaterOrEqual(t, st.getCalls, 3, "expected Create to poll GetSandbox until ready")
}

func TestSandboxResourceWaitForReadyDisabled(t *testing.T) {
	t.Parallel()
	server, _ := setupMockSandboxServer(t, 1000)
	name := "test-sandbox-" + uuid.New().String()[:8]

	config := providerConfig(server.URL) + fmt.Sprintf(`
resource "chalk_sandbox" "test" {
  environment_id = "test-env-id"
  name           = %q
  image          = "debian:bookworm"
  wait_for_ready = false
}
`, name)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "pending"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "wait_for_ready", "false"),
				),
			},
		},
	})
}

func TestSandboxResourceImport(t *testing.T) {
	t.Parallel()
	server, _ := setupMockSandboxServer(t, 0)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: sandboxConfigWithoutName(server.URL),
			},
			{
				ResourceName:      "chalk_sandbox.test",
				ImportState:       true,
				ImportStateId:     "test-env-id/sandbox-test-id",
				ImportStateVerify: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("chalk_sandbox.test", "desired_state"),
					resource.TestCheckNoResourceAttr("chalk_sandbox.test", "name"),
				),
				// Create-only arguments and defaults cannot be recovered on import.
				ImportStateVerifyIgnore: []string{"image", "wait_for_ready"},
			},
		},
	})
}

func TestSandboxResourceInvalidRestartPolicy(t *testing.T) {
	t.Parallel()
	server, _ := setupMockSandboxServer(t, 0)

	config := providerConfig(server.URL) + `
resource "chalk_sandbox" "test" {
  environment_id = "test-env-id"
  image          = "debian:bookworm"
  restart_policy = "SOMETIMES"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      config,
				ExpectError: regexp.MustCompile(`not a known restart policy`),
			},
		},
	})
}

const testGCPAudience = "//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/providers/chalk"

func sandboxWorkloadIdentityConfig(serverURL, identity string) string {
	return providerConfig(serverURL) + fmt.Sprintf(`
resource "chalk_sandbox" "test" {
  environment_id          = "test-env-id"
  image                   = "debian:bookworm"
  chalk_workload_identity = %s
}
`, identity)
}

func gcpWorkloadIdentity(mode, envVar string) string {
	return fmt.Sprintf(`{
    mode = %q
    oidc_token_files = [{ audience = %q, env_var = %q }]
  }`, mode, testGCPAudience, envVar)
}

func TestSandboxResourceCreateWorkloadIdentity(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: sandboxWorkloadIdentityConfig(server.URL, gcpWorkloadIdentity("workload_principal", "GCP_OIDC_TOKEN_FILE")),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_sandbox.test", "chalk_workload_identity.mode", "workload_principal"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "chalk_workload_identity.oidc_token_files.0.audience", testGCPAudience),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "chalk_workload_identity.oidc_token_files.0.env_var", "GCP_OIDC_TOKEN_FILE"),
				),
			},
			{
				ResourceName:      "chalk_sandbox.test",
				ImportState:       true,
				ImportStateId:     "test-env-id/sandbox-test-id",
				ImportStateVerify: true,
				// Create-only arguments and defaults cannot be recovered on import.
				ImportStateVerifyIgnore: []string{"image", "wait_for_ready"},
			},
		},
	})

	identity := st.created.GetSpec().GetChalkWorkloadIdentity()
	require.NotNil(t, identity)
	assert.Equal(t, containerv1.IdentityMode_IDENTITY_MODE_WORKLOAD_PRINCIPAL, identity.GetMode())
	assert.Nil(t, identity.GetWorkloadPrincipal())
	require.Len(t, identity.GetOidcTokenFiles(), 1)
	assert.Equal(t, testGCPAudience, identity.GetOidcTokenFiles()[0].GetAudience())
	assert.Equal(t, "GCP_OIDC_TOKEN_FILE", identity.GetOidcTokenFiles()[0].GetEnvVar())
}

func TestSandboxResourceWorkloadIdentityKeepsStateWhenServerOmitsIt(t *testing.T) {
	ctx := context.Background()
	files, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: sandboxOIDCTokenFileAttrTypes}, []SandboxOIDCTokenFileModel{{
		Audience: types.StringValue(testGCPAudience),
		EnvVar:   types.StringValue("GCP_OIDC_TOKEN_FILE"),
	}})
	require.False(t, diags.HasError())
	prior, diags := types.ObjectValueFrom(ctx, sandboxWorkloadIdentityAttrTypes, SandboxWorkloadIdentityModel{
		Mode:           types.StringValue("workload_principal"),
		OIDCTokenFiles: files,
	})
	require.False(t, diags.HasError())

	data := SandboxResourceModel{WorkloadIdentity: prior}
	diags = updateSandboxWorkloadIdentity(ctx, &data, &sandboxv2.SandboxInfo{})
	require.False(t, diags.HasError())
	assert.True(t, data.WorkloadIdentity.Equal(prior))

	identityWithoutFiles := &sandboxv2.SandboxInfo{Spec: &containerv1.ChalkContainerSpec{
		ChalkWorkloadIdentity: &containerv1.ChalkWorkloadIdentity{Mode: containerv1.IdentityMode_IDENTITY_MODE_WORKLOAD_PRINCIPAL},
	}}
	fileType := types.ObjectType{AttrTypes: sandboxOIDCTokenFileAttrTypes}

	// Token files removed out of band are drift: state takes the server's (empty) value.
	data = SandboxResourceModel{WorkloadIdentity: prior}
	diags = updateSandboxWorkloadIdentity(ctx, &data, identityWithoutFiles)
	require.False(t, diags.HasError())
	var got SandboxWorkloadIdentityModel
	require.False(t, data.WorkloadIdentity.As(ctx, &got, basetypes.ObjectAsOptions{}).HasError())
	assert.Equal(t, "workload_principal", got.Mode.ValueString())
	assert.True(t, got.OIDCTokenFiles.IsNull())

	// A configured empty set stays empty rather than flipping to null.
	emptyPrior, diags := types.ObjectValueFrom(ctx, sandboxWorkloadIdentityAttrTypes, SandboxWorkloadIdentityModel{
		Mode:           types.StringValue("workload_principal"),
		OIDCTokenFiles: types.SetValueMust(fileType, nil),
	})
	require.False(t, diags.HasError())
	data = SandboxResourceModel{WorkloadIdentity: emptyPrior}
	diags = updateSandboxWorkloadIdentity(ctx, &data, identityWithoutFiles)
	require.False(t, diags.HasError())
	assert.True(t, data.WorkloadIdentity.Equal(emptyPrior))

	data = SandboxResourceModel{WorkloadIdentity: types.ObjectNull(sandboxWorkloadIdentityAttrTypes)}
	diags = updateSandboxWorkloadIdentity(ctx, &data, &sandboxv2.SandboxInfo{})
	require.False(t, diags.HasError())
	assert.True(t, data.WorkloadIdentity.IsNull())
}

func TestSandboxResourceRejectsTokenFilesWithoutWorkloadPrincipal(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{{
			Config:      sandboxWorkloadIdentityConfig(server.URL, gcpWorkloadIdentity("creator_identity", "GCP_OIDC_TOKEN_FILE")),
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(?s)oidc_token_files.*workload_principal`),
		}},
	})

	assert.Zero(t, st.createCalls)
}

func TestSandboxResourceWorkloadIdentityChangeReplaces(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: sandboxWorkloadIdentityConfig(server.URL, gcpWorkloadIdentity("workload_principal", "GCP_OIDC_TOKEN_FILE"))},
			{
				Config: sandboxWorkloadIdentityConfig(server.URL, gcpWorkloadIdentity("workload_principal", "OTHER_TOKEN_FILE")),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("chalk_sandbox.test", plancheck.ResourceActionReplace),
					},
				},
			},
		},
	})

	assert.Equal(t, 2, st.createCalls)
	assert.Equal(t, "OTHER_TOKEN_FILE", st.created.GetSpec().GetChalkWorkloadIdentity().GetOidcTokenFiles()[0].GetEnvVar())
}

func TestSandboxResourceWorkloadIdentityReorderIsNoOp(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)

	twoFiles := func(first, second string) string {
		return fmt.Sprintf(`{
    mode = "workload_principal"
    oidc_token_files = [
      { audience = %q, env_var = %q },
      { audience = %q, env_var = %q },
    ]
  }`, testGCPAudience, first, "other-audience", second)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: sandboxWorkloadIdentityConfig(server.URL, twoFiles("GCP_OIDC_TOKEN_FILE", "OTHER_TOKEN_FILE"))},
			{
				Config: sandboxWorkloadIdentityConfig(server.URL, fmt.Sprintf(`{
    mode = "workload_principal"
    oidc_token_files = [
      { audience = %q, env_var = "OTHER_TOKEN_FILE" },
      { audience = %q, env_var = "GCP_OIDC_TOKEN_FILE" },
    ]
  }`, "other-audience", testGCPAudience)),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})

	assert.Equal(t, 1, st.createCalls)
}

func TestSandboxResourceRejectsTokenFilesWithKubeRuntime(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{{
			Config: providerConfig(server.URL) + fmt.Sprintf(`
resource "chalk_sandbox" "test" {
  environment_id          = "test-env-id"
  image                   = "debian:bookworm"
  runtime                 = "kube"
  chalk_workload_identity = %s
}
`, gcpWorkloadIdentity("workload_principal", "GCP_OIDC_TOKEN_FILE")),
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(?s)oidc_token_files.*host compute`),
		}},
	})

	assert.Zero(t, st.createCalls)
}
