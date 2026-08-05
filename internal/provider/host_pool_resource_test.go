package provider

import (
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

// hostPool builds a HostPool proto with the given spec, as the server would
// return it.
func hostPool(id string, spec *serverv1.HostPoolSpec) *serverv1.HostPool {
	return &serverv1.HostPool{Id: id, Spec: spec}
}

func autoscalingSpec(name string) *serverv1.HostPoolSpec {
	return &serverv1.HostPoolSpec{
		Name:        name,
		MinHosts:    0,
		MaxHosts:    4,
		IdleTimeout: durationpb.New(5 * time.Minute),
		Cpu:         "4",
		Memory:      "8Gi",
	}
}

// TestEnvironmentHostPoolCreateRead verifies the create/read round-trip and that
// the spec is sent to the server as configured.
func TestEnvironmentHostPoolCreateRead(t *testing.T) {
	t.Parallel()

	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	spec := autoscalingSpec("workers")
	server.OnCreateEnvironmentHostPool().Return(&serverv1.CreateEnvironmentHostPoolResponse{
		HostPool: hostPool("hp-env-1", spec),
	})
	server.OnGetHostPool().Return(&serverv1.GetHostPoolResponse{
		HostPool: hostPool("hp-env-1", spec),
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_environment_host_pool" "test" {
  environment_id = "env-1"
  name           = "workers"
  min_hosts      = 0
  max_hosts      = 4
  idle_timeout   = "5m"
  cpu            = "4"
  memory         = "8Gi"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_environment_host_pool.test", "id", "hp-env-1"),
					resource.TestCheckResourceAttr("chalk_environment_host_pool.test", "environment_id", "env-1"),
					resource.TestCheckResourceAttr("chalk_environment_host_pool.test", "name", "workers"),
					resource.TestCheckResourceAttr("chalk_environment_host_pool.test", "idle_timeout", "5m"),
					resource.TestCheckNoResourceAttr("chalk_environment_host_pool.test", "machine_family"),
					func(s *terraform.State) error {
						reqs := server.GetCapturedRequests("CreateEnvironmentHostPool")
						require.Len(t, reqs, 1)
						sent := reqs[0].(*serverv1.CreateEnvironmentHostPoolRequest).GetSpec()
						assert.Equal(t, "workers", sent.GetName())
						assert.Equal(t, int32(0), sent.GetMinHosts())
						assert.Equal(t, int32(4), sent.GetMaxHosts())
						assert.Equal(t, 5*time.Minute, sent.GetIdleTimeout().AsDuration())
						assert.Equal(t, "4", sent.GetCpu())
						assert.Equal(t, "8Gi", sent.GetMemory())
						return nil
					},
				),
			},
		},
	})
}

// TestClusterHostPoolCreateRead verifies the cluster-scoped resource sends
// cluster_id on create.
func TestClusterHostPoolCreateRead(t *testing.T) {
	t.Parallel()

	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	spec := &serverv1.HostPoolSpec{
		Name:          "gpu",
		MinHosts:      2,
		MaxHosts:      2,
		Cpu:           "8",
		Memory:        "32Gi",
		MachineFamily: new("n2"),
	}
	server.OnCreateClusterHostPool().Return(&serverv1.CreateClusterHostPoolResponse{
		HostPool: hostPool("hp-cluster-1", spec),
	})
	server.OnGetHostPool().Return(&serverv1.GetHostPoolResponse{
		HostPool: hostPool("hp-cluster-1", spec),
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_cluster_host_pool" "test" {
  cluster_id     = "cluster-1"
  name           = "gpu"
  min_hosts      = 2
  max_hosts      = 2
  cpu            = "8"
  memory         = "32Gi"
  machine_family = "n2"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_cluster_host_pool.test", "id", "hp-cluster-1"),
					resource.TestCheckResourceAttr("chalk_cluster_host_pool.test", "cluster_id", "cluster-1"),
					resource.TestCheckResourceAttr("chalk_cluster_host_pool.test", "machine_family", "n2"),
					resource.TestCheckNoResourceAttr("chalk_cluster_host_pool.test", "idle_timeout"),
					func(s *terraform.State) error {
						reqs := server.GetCapturedRequests("CreateClusterHostPool")
						require.Len(t, reqs, 1)
						req := reqs[0].(*serverv1.CreateClusterHostPoolRequest)
						assert.Equal(t, "cluster-1", req.GetClusterId())
						assert.Nil(t, req.GetSpec().IdleTimeout)
						return nil
					},
				),
			},
		},
	})
}

// TestHostPoolUpdateSendsFullMask verifies that updates send every mutable path,
// which the server requires (it rejects an empty update mask).
func TestHostPoolUpdateSendsFullMask(t *testing.T) {
	t.Parallel()

	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	initial := autoscalingSpec("workers")
	updated := autoscalingSpec("workers")
	updated.MaxHosts = 8

	server.OnCreateEnvironmentHostPool().Return(&serverv1.CreateEnvironmentHostPoolResponse{
		HostPool: hostPool("hp-env-1", initial),
	})
	server.OnUpdateEnvironmentHostPool().Return(&serverv1.UpdateEnvironmentHostPoolResponse{
		HostPool: hostPool("hp-env-1", updated),
	})

	// Get reflects whichever spec was last written, so the second step plans a diff.
	current := initial
	server.OnGetHostPool().WithBehavior(func(proto.Message) (proto.Message, error) {
		return &serverv1.GetHostPoolResponse{HostPool: hostPool("hp-env-1", current)}, nil
	})
	server.OnUpdateEnvironmentHostPool().WithBehavior(func(proto.Message) (proto.Message, error) {
		current = updated
		return &serverv1.UpdateEnvironmentHostPoolResponse{HostPool: hostPool("hp-env-1", updated)}, nil
	})

	config := func(maxHosts int) string {
		return providerConfig(server.URL) + fmt.Sprintf(`
resource "chalk_environment_host_pool" "test" {
  environment_id = "env-1"
  name           = "workers"
  min_hosts      = 0
  max_hosts      = %d
  idle_timeout   = "5m"
  cpu            = "4"
  memory         = "8Gi"
}
`, maxHosts)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: config(4)},
			{
				Config: config(8),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_environment_host_pool.test", "max_hosts", "8"),
					func(s *terraform.State) error {
						reqs := server.GetCapturedRequests("UpdateEnvironmentHostPool")
						require.Len(t, reqs, 1)
						req := reqs[0].(*serverv1.UpdateEnvironmentHostPoolRequest)
						assert.Equal(t, "hp-env-1", req.GetId())
						assert.ElementsMatch(t, hostPoolSpecUpdateMaskPaths, req.GetUpdateMask().GetPaths())
						assert.Equal(t, int32(8), req.GetSpec().GetMaxHosts())
						return nil
					},
				),
			},
		},
	})
}

// TestHostPoolIdleTimeoutSemanticEquality verifies that an idle_timeout written
// in a different but equivalent spelling than the server round-trips does not
// produce a perpetual diff.
func TestHostPoolIdleTimeoutSemanticEquality(t *testing.T) {
	t.Parallel()

	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	// The server stores seconds, so it echoes back 300s, which formats as "5m0s"
	// while the config says "300s".
	spec := autoscalingSpec("workers")
	server.OnCreateEnvironmentHostPool().Return(&serverv1.CreateEnvironmentHostPoolResponse{
		HostPool: hostPool("hp-env-1", spec),
	})
	server.OnGetHostPool().Return(&serverv1.GetHostPoolResponse{
		HostPool: hostPool("hp-env-1", spec),
	})

	config := providerConfig(server.URL) + `
resource "chalk_environment_host_pool" "test" {
  environment_id = "env-1"
  name           = "workers"
  min_hosts      = 0
  max_hosts      = 4
  idle_timeout   = "300s"
  cpu            = "4"
  memory         = "8Gi"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					// State keeps the practitioner's spelling rather than "5m0s".
					resource.TestCheckResourceAttr("chalk_environment_host_pool.test", "idle_timeout", "300s"),
				),
			},
			{
				// An empty plan proves semantic equality suppressed the diff.
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

// TestHostPoolReadNotFound verifies the pool is removed from state when the
// server reports it is gone, so Terraform recreates it.
func TestHostPoolReadNotFound(t *testing.T) {
	t.Parallel()

	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	spec := autoscalingSpec("workers")
	server.OnCreateEnvironmentHostPool().Return(&serverv1.CreateEnvironmentHostPoolResponse{
		HostPool: hostPool("hp-env-1", spec),
	})

	// The pool disappears server-side after the initial post-create read.
	var getCallCount int
	server.OnGetHostPool().WithBehavior(func(proto.Message) (proto.Message, error) {
		getCallCount++
		if getCallCount > 1 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("host pool not found"))
		}
		return &serverv1.GetHostPoolResponse{HostPool: hostPool("hp-env-1", spec)}, nil
	})

	config := providerConfig(server.URL) + `
resource "chalk_environment_host_pool" "test" {
  environment_id = "env-1"
  name           = "workers"
  min_hosts      = 0
  max_hosts      = 4
  idle_timeout   = "5m"
  cpu            = "4"
  memory         = "8Gi"
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

// TestHostPoolConfigValidation covers the scaling constraints the provider
// enforces at plan time, mirroring the server's rules.
func TestHostPoolConfigValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		minHosts    int
		maxHosts    int
		idleTimeout string
		expectError *regexp.Regexp
	}{
		{
			name:        "minGreaterThanMax",
			minHosts:    5,
			maxHosts:    2,
			expectError: regexp.MustCompile(`(?s)min_hosts must be less than or equal to max_hosts`),
		},
		{
			name:        "minNeitherZeroNorMax",
			minHosts:    1,
			maxHosts:    4,
			idleTimeout: "5m",
			expectError: regexp.MustCompile(`(?s)must be either 0 or equal to max_hosts`),
		},
		{
			name:        "idleTimeoutMissingWhenScaling",
			minHosts:    0,
			maxHosts:    4,
			expectError: regexp.MustCompile(`(?s)idle_timeout is required`),
		},
		{
			name:        "idleTimeoutSetWhenFixed",
			minHosts:    2,
			maxHosts:    2,
			idleTimeout: "5m",
			expectError: regexp.MustCompile(`(?s)must not be set when min_hosts equals max_hosts`),
		},
		{
			name:        "idleTimeoutBelowFloor",
			minHosts:    0,
			maxHosts:    4,
			idleTimeout: "30s",
			expectError: regexp.MustCompile(`(?s)idle_timeout must be at least 1m`),
		},
		{
			name:        "idleTimeoutUnparseable",
			minHosts:    0,
			maxHosts:    4,
			idleTimeout: "not-a-duration",
			expectError: regexp.MustCompile(`(?s)Invalid Duration String Value|time: invalid duration`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := testserver.NewMockBuilderServer(t)
			t.Cleanup(func() { server.Close() })

			idleTimeout := ""
			if tc.idleTimeout != "" {
				idleTimeout = fmt.Sprintf("  idle_timeout = %q\n", tc.idleTimeout)
			}

			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
				Steps: []resource.TestStep{
					{
						Config: providerConfig(server.URL) + fmt.Sprintf(`
resource "chalk_environment_host_pool" "test" {
  environment_id = "env-1"
  name           = "workers"
  min_hosts      = %d
  max_hosts      = %d
%s  cpu            = "4"
  memory         = "8Gi"
}
`, tc.minHosts, tc.maxHosts, idleTimeout),
						ExpectError: tc.expectError,
					},
				},
			})
		})
	}
}

// TestHostPoolInvalidName verifies host pool names are validated as DNS labels
// before reaching the server.
func TestHostPoolInvalidName(t *testing.T) {
	t.Parallel()

	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_cluster_host_pool" "test" {
  cluster_id = "cluster-1"
  name       = "Invalid_Name"
  min_hosts  = 1
  max_hosts  = 1
  cpu        = "4"
  memory     = "8Gi"
}
`,
				ExpectError: regexp.MustCompile(`(?s)valid DNS label`),
			},
		},
	})
}
