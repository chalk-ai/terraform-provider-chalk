package provider

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	sandboxv2 "github.com/chalk-ai/chalk-go/gen/chalk/sandbox/v2"
	"github.com/chalk-ai/chalk-go/gen/chalk/sandbox/v2/sandboxv2connect"
	"github.com/chalk-ai/chalk-go/testserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	frameworkschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

type sandboxExecMockState struct {
	request  *sandboxv2.ExecCommandRequest
	response *sandboxv2.ExecCommandResponse
	err      error
	calls    int
}

type sandboxExecMockHandler struct {
	sandboxv2connect.UnimplementedSandboxProcessServiceHandler
	state *sandboxExecMockState
}

func (h *sandboxExecMockHandler) ExecCommand(
	_ context.Context,
	req *connect.Request[sandboxv2.ExecCommandRequest],
) (*connect.Response[sandboxv2.ExecCommandResponse], error) {
	h.state.calls++
	h.state.request = req.Msg
	if h.state.err != nil {
		return nil, h.state.err
	}
	return connect.NewResponse(h.state.response), nil
}

func setupMockSandboxExecServer(t *testing.T, response *sandboxv2.ExecCommandResponse) (*testserver.MockServer, *sandboxExecMockState) {
	t.Helper()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	state := &sandboxExecMockState{response: response}
	existingHandler := server.Config.Handler
	mux := http.NewServeMux()
	processPath, processHandler := sandboxv2connect.NewSandboxProcessServiceHandler(&sandboxExecMockHandler{state: state})
	mux.Handle(processPath, processHandler)
	mux.Handle("/", existingHandler)
	server.Config.Handler = mux
	return server, state
}

func sandboxExecConfig(serverURL, trigger string) string {
	return sandboxExecConfigWithStdin(serverURL, trigger, "secret-input")
}

func sandboxExecConfigWithStdin(serverURL, trigger, stdin string) string {
	return providerConfig(serverURL) + fmt.Sprintf(`
resource "chalk_sandbox_exec" "test" {
  environment_id = "test-env-id"
  sandbox_id     = "sandbox-test-id"
  command        = ["sh", "-c", "read token; printf output"]
  env = {
    TEST_ENV = "test-value"
  }
  stdin   = %q
  timeout = "2m"
  triggers = {
    token = %q
  }
}
`, stdin, trigger)
}

func TestSandboxExecResourceCreate(t *testing.T) {
	t.Parallel()
	server, state := setupMockSandboxExecServer(t, &sandboxv2.ExecCommandResponse{
		Stdout:   []byte("hello\n"),
		Stderr:   []byte("warning\n"),
		ExitCode: 0,
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{{
			Config: sandboxExecConfig(server.URL, "first"),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("chalk_sandbox_exec.test", "id", "sandbox-test-id"),
				resource.TestCheckResourceAttr("chalk_sandbox_exec.test", "output", "hello\nwarning\n"),
				resource.TestCheckNoResourceAttr("chalk_sandbox_exec.test", "stdin"),
				resource.TestCheckNoResourceAttr("chalk_sandbox_exec.test", "exit_code"),
			),
		}},
	})

	require.NotNil(t, state.request)
	assert.Equal(t, "sandbox-test-id", state.request.GetSandboxId())
	assert.Equal(t, "sh", state.request.GetCommand())
	assert.Equal(t, []string{"-c", "read token; printf output"}, state.request.GetArgs())
	assert.Equal(t, map[string]string{"TEST_ENV": "test-value"}, state.request.GetEnvVars())
	assert.Equal(t, []byte("secret-input"), state.request.GetStdin())
	assert.True(t, proto.Equal(durationpb.New(2*time.Minute), state.request.GetTimeout()))
}

func TestSandboxExecResourceNonZeroExitDoesNotSaveState(t *testing.T) {
	t.Parallel()
	server, state := setupMockSandboxExecServer(t, &sandboxv2.ExecCommandResponse{
		Stdout:   []byte(strings.Repeat("x", sandboxExecOutputLimit)),
		Stderr:   []byte("command failed at the end"),
		ExitCode: 23,
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      sandboxExecConfig(server.URL, "first"),
				ExpectError: regexp.MustCompile(`Command exited with code 23(?s:.*)command(?s:.*)failed at the end`),
			},
			{
				PreConfig: func() {
					state.response = &sandboxv2.ExecCommandResponse{}
				},
				Config: sandboxExecConfig(server.URL, "first"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("chalk_sandbox_exec.test", plancheck.ResourceActionCreate),
				}},
			},
		},
	})

	assert.Equal(t, 2, state.calls)
}

func TestSandboxExecResourceTriggerChangeRequiresReplace(t *testing.T) {
	t.Parallel()
	server, state := setupMockSandboxExecServer(t, &sandboxv2.ExecCommandResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: sandboxExecConfig(server.URL, "first")},
			{
				Config: sandboxExecConfig(server.URL, "second"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("chalk_sandbox_exec.test", plancheck.ResourceActionDestroyBeforeCreate),
				}},
			},
		},
	})

	assert.Equal(t, 2, state.calls)
}

func TestSandboxExecResourceStdinChangeDoesNotRequireReplace(t *testing.T) {
	t.Parallel()
	server, state := setupMockSandboxExecServer(t, &sandboxv2.ExecCommandResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: sandboxExecConfigWithStdin(server.URL, "first", "first-secret")},
			{Config: sandboxExecConfigWithStdin(server.URL, "first", "second-secret")},
		},
	})

	assert.Equal(t, 1, state.calls)
}

func TestSandboxExecResourceSchema(t *testing.T) {
	t.Parallel()
	var response frameworkresource.SchemaResponse
	NewSandboxExecResource().Schema(context.Background(), frameworkresource.SchemaRequest{}, &response)

	stdin, ok := response.Schema.Attributes["stdin"].(frameworkschema.StringAttribute)
	require.True(t, ok)
	assert.True(t, stdin.Sensitive)
	assert.True(t, stdin.WriteOnly)
	assert.False(t, stdin.IsComputed())

	environmentID, ok := response.Schema.Attributes["environment_id"].(frameworkschema.StringAttribute)
	require.True(t, ok)
	assert.True(t, environmentID.Required)

	_, hasExitCode := response.Schema.Attributes["exit_code"]
	assert.False(t, hasExitCode)
}

func TestSandboxExecResourceFailedPreconditionIncludesServerError(t *testing.T) {
	t.Parallel()
	server, state := setupMockSandboxExecServer(t, &sandboxv2.ExecCommandResponse{})
	state.err = connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("sandbox suspended"))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{{
			Config:      sandboxExecConfig(server.URL, "first"),
			ExpectError: regexp.MustCompile(`sandbox suspended`),
		}},
	})
}

func TestSandboxExecResourceInvalidTimeoutFailsAtPlan(t *testing.T) {
	t.Parallel()
	server, state := setupMockSandboxExecServer(t, &sandboxv2.ExecCommandResponse{})
	config := strings.Replace(sandboxExecConfig(server.URL, "first"), `timeout = "2m"`, `timeout = "not-a-duration"`, 1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{{
			Config:      config,
			ExpectError: regexp.MustCompile(`Invalid Time Duration String Value`),
		}},
	})

	assert.Zero(t, state.calls)
}

func TestSandboxExecResourceNonPositiveTimeoutFailsAtPlan(t *testing.T) {
	t.Parallel()
	server, state := setupMockSandboxExecServer(t, &sandboxv2.ExecCommandResponse{})
	config := strings.Replace(sandboxExecConfig(server.URL, "first"), `timeout = "2m"`, `timeout = "0s"`, 1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{{
			Config:      config,
			ExpectError: regexp.MustCompile(`timeout must be greater than zero`),
		}},
	})

	assert.Zero(t, state.calls)
}

func TestSandboxExecResourceEmptyCommandElementFailsAtPlan(t *testing.T) {
	t.Parallel()
	server, state := setupMockSandboxExecServer(t, &sandboxv2.ExecCommandResponse{})
	config := strings.Replace(sandboxExecConfig(server.URL, "first"), `command        = ["sh", "-c", "read token; printf output"]`, `command        = [""]`, 1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{{
			Config:      config,
			ExpectError: regexp.MustCompile(`must be at least 1`),
		}},
	})

	assert.Zero(t, state.calls)
}

func TestTruncateSandboxExecOutputKeepsTail(t *testing.T) {
	t.Parallel()
	stdout := []byte(strings.Repeat("x", sandboxExecOutputLimit))
	stderr := []byte("stderr tail")

	output := truncateSandboxExecOutput(stdout, stderr)

	assert.Len(t, output, sandboxExecOutputLimit)
	assert.NotContains(t, output, strings.Repeat("x", sandboxExecOutputLimit))
	assert.True(t, strings.HasSuffix(output, "stderr tail"))
}
