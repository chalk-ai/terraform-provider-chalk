package provider

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"connectrpc.com/connect"
	containerv1 "github.com/chalk-ai/chalk-go/gen/chalk/container/v1"
	sandboxv1 "github.com/chalk-ai/chalk-go/gen/chalk/sandbox/v1"
	sandboxv2 "github.com/chalk-ai/chalk-go/gen/chalk/sandbox/v2"
	"github.com/chalk-ai/chalk-go/gen/chalk/sandbox/v2/sandboxv2connect"
	"github.com/chalk-ai/chalk-go/testserver"
	chalkclient "github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// sandboxMockState records what the mock server was asked to create so tests can
// assert on the request, and drives the state machine GetSandbox reports back.
type sandboxMockState struct {
	created             *sandboxv1.CreateSandboxRequest
	info                *sandboxv1.SandboxInfo
	getCalls            int
	readyAfter          int
	createCalls         int
	suspendCalls        int
	restoreCalls        int
	terminateCalls      int
	terminated          bool
	terminateErr        error
	snapshots           []*sandboxv2.SandboxSnapshot
	snapshotListFilters [][]string
	deletedSnapshotIDs  []string
	snapshotDeleteErr   error
	events              []string
}

type sandboxV2MockHandler struct {
	sandboxv2connect.UnimplementedSandboxServiceHandler
	state *sandboxMockState
}

func (h *sandboxV2MockHandler) SuspendSandbox(
	_ context.Context,
	req *connect.Request[sandboxv2.SuspendSandboxRequest],
) (*connect.Response[sandboxv2.SuspendSandboxResponse], error) {
	h.state.suspendCalls++
	h.state.events = append(h.state.events, "suspend")
	if h.state.info != nil && req.Msg.GetId() == h.state.info.GetId() {
		h.state.info.State = "SUSPENDED"
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
		h.state.info.State = "RUNNING"
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
	// Return everything so the provider test also verifies its defensive
	// sandbox-id check instead of relying only on server-side filtering.
	return connect.NewResponse(&sandboxv2.ListSandboxSnapshotsResponse{Snapshots: h.state.snapshots}), nil
}

func (h *sandboxSnapshotMockHandler) DeleteSandboxSnapshot(
	_ context.Context,
	req *connect.Request[sandboxv2.DeleteSandboxSnapshotRequest],
) (*connect.Response[sandboxv2.DeleteSandboxSnapshotResponse], error) {
	h.state.events = append(h.state.events, "delete snapshot "+req.Msg.GetSnapshotId())
	if h.state.snapshotDeleteErr != nil {
		return nil, h.state.snapshotDeleteErr
	}
	h.state.deletedSnapshotIDs = append(h.state.deletedSnapshotIDs, req.Msg.GetSnapshotId())
	return connect.NewResponse(&sandboxv2.DeleteSandboxSnapshotResponse{}), nil
}

// setupMockSandboxServer wires a sandbox lifecycle onto the mock server. The
// created sandbox reports `connecting` until readyAfter GetSandbox calls have
// landed, which exercises the Create-time wait loop.
func setupMockSandboxServer(t *testing.T, readyAfter int) (*testserver.MockServer, *sandboxMockState) {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	st := &sandboxMockState{readyAfter: readyAfter}

	server.OnCreateSandbox().WithBehavior(func(req proto.Message) (proto.Message, error) {
		createReq := req.(*sandboxv1.CreateSandboxRequest)
		st.createCalls++
		st.created = createReq
		st.terminated = false
		state := "connecting"
		if readyAfter == 0 {
			state = "ready"
		}
		st.info = &sandboxv1.SandboxInfo{
			Id:            "sandbox-test-id",
			State:         state,
			CreatedAt:     "2026-08-19T00:00:00Z",
			Name:          createReq.Name,
			StatusMessage: new("running"),
		}
		return &sandboxv1.CreateSandboxResponse{Sandbox: st.info}, nil
	})

	server.OnGetSandbox().WithBehavior(func(req proto.Message) (proto.Message, error) {
		if st.terminated {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("sandbox not found"))
		}
		st.getCalls++
		if st.info.GetState() == "connecting" && st.getCalls >= st.readyAfter {
			st.info.State = "ready"
			st.info.StatusMessage = new("running")
		}
		return &sandboxv1.GetSandboxResponse{Sandbox: st.info}, nil
	})

	server.OnTerminateSandbox().WithBehavior(func(req proto.Message) (proto.Message, error) {
		st.terminateCalls++
		st.events = append(st.events, "terminate")
		if st.terminateErr != nil {
			return nil, st.terminateErr
		}
		st.terminated = true
		return &sandboxv1.TerminateSandboxResponse{}, nil
	})

	// chalk-go's test server serves sandbox/v1. Mount the v2 lifecycle and
	// snapshot handlers alongside it for this resource's mixed-version calls.
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

func sandboxMinimalConfig(serverURL, name string) string {
	return providerConfig(serverURL) + fmt.Sprintf(`
resource "chalk_sandbox" "test" {
  environment_id = "test-env-id"
  name           = %q
  image          = "debian:bookworm"
}
`, name)
}

func sandboxDesiredStateConfig(serverURL, name, desiredState string) string {
	return providerConfig(serverURL) + fmt.Sprintf(`
resource "chalk_sandbox" "test" {
  environment_id = "test-env-id"
  name           = %q
  image          = "debian:bookworm"
  desired_state  = %q
}
`, name, desiredState)
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
					resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "ready"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "created_at", "2026-08-19T00:00:00Z"),
					// wait_for_ready has a default rather than being left null.
					resource.TestCheckResourceAttr("chalk_sandbox.test", "wait_for_ready", "true"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "desired_state", "running"),
				),
			},
		},
	})

	require.NotNil(t, st.created)
	assert.Equal(t, "debian:bookworm", st.created.GetImage())
	assert.Equal(t, name, st.created.GetName())
	// Nothing unset should be sent: an empty network policy is not the same as
	// no network policy, and the server treats the former as deny-all.
	assert.Nil(t, st.created.NetworkPolicy)
	assert.Nil(t, st.created.ResourceLimits)
	assert.Nil(t, st.created.Runtime)
	assert.Empty(t, st.created.Entrypoint)
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
				resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "SUSPENDED"),
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
				Check:  resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "ready"),
			},
			{
				Config: sandboxDesiredStateConfig(server.URL, name, "suspended"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_sandbox.test", "id", "sandbox-test-id"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "SUSPENDED"),
				),
			},
			{
				Config: sandboxDesiredStateConfig(server.URL, name, "running"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_sandbox.test", "id", "sandbox-test-id"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "RUNNING"),
				),
			},
		},
	})

	assert.Equal(t, 1, st.createCalls, "desired_state updates must not replace the sandbox")
	assert.Equal(t, 1, st.suspendCalls)
	assert.Equal(t, 1, st.restoreCalls)
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
	st.info = &sandboxv1.SandboxInfo{Id: "sandbox-test-id", State: "RUNNING"}

	err := sandboxResourceForTest(server.URL).deleteSandbox(context.Background(), "test-env-id", "sandbox-test-id")
	require.NoError(t, err)
	assert.Zero(t, st.restoreCalls)
	assert.Equal(t, []string{"list snapshots", "terminate"}, st.events)
	assert.Equal(t, [][]string{{"sandbox-test-id"}}, st.snapshotListFilters)
}

func TestSandboxResourceDeleteSuspendedRestoresAndCleansSnapshots(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	st.info = &sandboxv1.SandboxInfo{Id: "sandbox-test-id", State: "SUSPENDED"}
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

func TestSandboxResourceDeleteFailedTreatsTerminateNotFoundAsGone(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	st.info = &sandboxv1.SandboxInfo{Id: "sandbox-test-id", State: "FAILED"}
	st.terminateErr = connect.NewError(connect.CodeNotFound, fmt.Errorf("container not found"))

	err := sandboxResourceForTest(server.URL).deleteSandbox(context.Background(), "test-env-id", "sandbox-test-id")
	require.NoError(t, err)
	assert.Equal(t, []string{"list snapshots", "terminate"}, st.events)
}

func TestSandboxResourceDeleteRunningDoesNotIgnoreTerminateNotFound(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	st.info = &sandboxv1.SandboxInfo{Id: "sandbox-test-id", State: "RUNNING"}
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

func TestSandboxResourceSnapshotDeleteFailureStopsDeletion(t *testing.T) {
	t.Parallel()
	server, st := setupMockSandboxServer(t, 0)
	st.info = &sandboxv1.SandboxInfo{Id: "sandbox-test-id", State: "RUNNING"}
	st.snapshots = []*sandboxv2.SandboxSnapshot{{Id: "snapshot-owned", SandboxId: "sandbox-test-id"}}
	st.snapshotDeleteErr = connect.NewError(connect.CodeInternal, fmt.Errorf("delete failed"))

	err := sandboxResourceForTest(server.URL).deleteSandbox(context.Background(), "test-env-id", "sandbox-test-id")
	require.ErrorContains(t, err, "delete snapshot snapshot-owned")
	assert.Zero(t, st.terminateCalls, "the sandbox must not terminate while its snapshots are leaked")
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
	assert.Equal(t, []string{"/bin/bash", "-c", "sleep infinity"}, st.created.GetEntrypoint())
	assert.Equal(t, map[string]string{"CODER_AGENT_TOKEN": "secret-token"}, st.created.GetEnv())
	assert.Equal(t, "kube", st.created.GetRuntime())
	assert.Equal(t, containerv1.RestartPolicy_RESTART_POLICY_ALWAYS, st.created.GetRestartPolicy())

	require.NotNil(t, st.created.ResourceLimits)
	assert.Equal(t, "2", st.created.ResourceLimits.GetCpu())
	assert.Equal(t, "4Gi", st.created.ResourceLimits.GetMemory())

	require.Len(t, st.created.Volumes, 1)
	assert.Equal(t, "home", st.created.Volumes[0].GetName())
	assert.Equal(t, "/home/coder", st.created.Volumes[0].GetMountPath())
	assert.Equal(t, "empty_dir", st.created.Volumes[0].GetType())
	assert.Equal(t, "20Gi", st.created.Volumes[0].GetSizeLimit())

	require.NotNil(t, st.created.NetworkPolicy)
	require.Len(t, st.created.NetworkPolicy.AllowedRoutes, 1)
	assert.Equal(t, "0.0.0.0/0", st.created.NetworkPolicy.AllowedRoutes[0].GetRoute())
	require.Len(t, st.created.NetworkPolicy.AllowedRoutes[0].PortRanges, 1)
	assert.Equal(t, int32(1), st.created.NetworkPolicy.AllowedRoutes[0].PortRanges[0].GetStartPort())
	assert.Equal(t, int32(65535), st.created.NetworkPolicy.AllowedRoutes[0].PortRanges[0].GetEndPort())
	assert.Equal(t, []string{"169.254.169.254/32"}, st.created.NetworkPolicy.GetDeniedRoutes())
}

// A freshly created sandbox reports "connecting"; the apply must not return
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
					resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "ready"),
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
					resource.TestCheckResourceAttr("chalk_sandbox.test", "state", "connecting"),
					resource.TestCheckResourceAttr("chalk_sandbox.test", "wait_for_ready", "false"),
				),
			},
		},
	})
}

func TestSandboxResourceImport(t *testing.T) {
	t.Parallel()
	server, _ := setupMockSandboxServer(t, 0)
	name := "test-sandbox-" + uuid.New().String()[:8]

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: sandboxMinimalConfig(server.URL, name),
			},
			{
				ResourceName:      "chalk_sandbox.test",
				ImportState:       true,
				ImportStateId:     "test-env-id/sandbox-test-id",
				ImportStateVerify: true,
				// Neither CreateSandbox nor GetSandbox echoes the spec back, so
				// the configured fields cannot be recovered on import.
				ImportStateVerifyIgnore: []string{"desired_state", "image", "wait_for_ready"},
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
