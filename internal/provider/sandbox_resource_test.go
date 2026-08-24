package provider

import (
	"fmt"
	"regexp"
	"testing"

	"connectrpc.com/connect"
	containerv1 "github.com/chalk-ai/chalk-go/gen/chalk/container/v1"
	sandboxv1 "github.com/chalk-ai/chalk-go/gen/chalk/sandbox/v1"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// sandboxMockState records what the mock server was asked to create so tests can
// assert on the request, and drives the state machine GetSandbox reports back.
type sandboxMockState struct {
	created    *sandboxv1.CreateSandboxRequest
	info       *sandboxv1.SandboxInfo
	getCalls   int
	readyAfter int
	terminated bool
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
		st.created = createReq
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
		if st.getCalls >= st.readyAfter {
			st.info.State = "ready"
			st.info.StatusMessage = new("running")
		}
		return &sandboxv1.GetSandboxResponse{Sandbox: st.info}, nil
	})

	server.OnTerminateSandbox().WithBehavior(func(req proto.Message) (proto.Message, error) {
		st.terminated = true
		return &sandboxv1.TerminateSandboxResponse{}, nil
	})

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
