package provider

import (
	"errors"
	"regexp"
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

// setupMockServerManagedEnvironment creates a mock server that tracks environment
// state across Create/Update/Read operations for managed environments.
func setupMockServerManagedEnvironment(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	const envID = "test-managed-env-id"
	kubeClusterID := "test-cluster-id"
	kubeJobNS := "test-managed-namespace" // server assigns this for managed envs

	var currentEnv *serverv1.Environment

	server.OnCreateEnvironmentV2().WithBehavior(func(req proto.Message) (proto.Message, error) {
		createReq := req.(*serverv1.CreateEnvironmentV2Request)
		env := proto.Clone(createReq.Environment).(*serverv1.Environment)
		env.Id = envID
		env.KubeClusterId = &kubeClusterID
		// Server auto-assigns kube_job_namespace for managed envs.
		env.KubeJobNamespace = &kubeJobNS
		currentEnv = env
		return &serverv1.CreateEnvironmentV2Response{Environment: env}, nil
	})

	server.OnUpdateEnvironmentV2().WithBehavior(func(req proto.Message) (proto.Message, error) {
		updateReq := req.(*serverv1.UpdateEnvironmentV2Request)
		updated := proto.Clone(currentEnv).(*serverv1.Environment)
		for _, path := range updateReq.UpdateMask.Paths {
			applyManagedEnvField(updated, updateReq.Environment, path)
		}
		currentEnv = updated
		return &serverv1.UpdateEnvironmentV2Response{Environment: updated}, nil
	})

	server.OnDeleteEnvironment().Return(&serverv1.DeleteEnvironmentResponse{})

	server.OnGetEnv().WithBehavior(func(req proto.Message) (proto.Message, error) {
		if currentEnv == nil {
			return &serverv1.GetEnvResponse{}, nil
		}
		return &serverv1.GetEnvResponse{Environment: proto.Clone(currentEnv).(*serverv1.Environment)}, nil
	})

	return server
}

// applyManagedEnvField applies a single field-mask path update from src to dst.
func applyManagedEnvField(dst, src *serverv1.Environment, path string) {
	switch path {
	case "service_url":
		dst.ServiceUrl = src.ServiceUrl
	case "online_store_kind":
		dst.OnlineStoreKind = src.OnlineStoreKind
	case "online_store_secret":
		dst.OnlineStoreSecret = src.OnlineStoreSecret
	case "private_pip_repositories":
		dst.PrivatePipRepositories = src.PrivatePipRepositories
	case "additional_env_vars":
		dst.AdditionalEnvVars = src.AdditionalEnvVars
	case "environment_buckets":
		dst.EnvironmentBuckets = src.EnvironmentBuckets
	case "spec_config_json":
		dst.SpecConfigJson = src.SpecConfigJson
	case "pinned_base_image":
		dst.PinnedBaseImage = src.PinnedBaseImage
	case "default_build_profile":
		dst.DefaultBuildProfile = src.DefaultBuildProfile
	case "customer_metadata":
		dst.CustomerMetadata = src.CustomerMetadata
	}
}

// TestManagedEnvironmentCreate verifies the basic create/read/delete lifecycle and that
// managed=true is sent in the CreateEnvironment request.
func TestManagedEnvironmentCreate(t *testing.T) {
	t.Parallel()
	server := setupMockServerManagedEnvironment(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_managed_environment" "test" {
  name            = "test-managed-env"
  project_id      = "test-project"
  kube_cluster_id = "test-cluster-id"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_managed_environment.test", "id", "test-managed-env-id"),
					resource.TestCheckResourceAttr("chalk_managed_environment.test", "name", "test-managed-env"),
					resource.TestCheckResourceAttr("chalk_managed_environment.test", "project_id", "test-project"),
					resource.TestCheckResourceAttr("chalk_managed_environment.test", "kube_cluster_id", "test-cluster-id"),
					// kube_job_namespace is computed; verify the server-assigned value is reflected in state.
					resource.TestCheckResourceAttr("chalk_managed_environment.test", "kube_job_namespace", "test-managed-namespace"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("CreateEnvironmentV2")
						require.Len(t, captured, 1, "Expected exactly one CreateEnvironment call")

						req := captured[0].(*serverv1.CreateEnvironmentV2Request)
						require.NotNil(t, req.Environment)
						assert.Equal(t, "test-managed-env", req.Environment.Name)
						assert.Equal(t, "test-project", req.Environment.ProjectId)
						require.NotNil(t, req.Environment.Managed, "managed field must be set")
						assert.True(t, *req.Environment.Managed, "managed must be true")

						return nil
					},
				),
			},
		},
	})
}

// TestManagedEnvironmentUpdateFieldMask verifies that the update field mask contains
// only the paths of changed fields.
func TestManagedEnvironmentUpdateFieldMask(t *testing.T) {
	t.Parallel()
	server := setupMockServerManagedEnvironment(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_managed_environment" "test" {
  name             = "test-managed-env"
  project_id       = "test-project"
  kube_cluster_id  = "test-cluster-id"
  pinned_base_image = "my-registry/base:1.0"
}
`,
			},
			{
				Config: providerConfig(server.URL) + `
resource "chalk_managed_environment" "test" {
  name             = "test-managed-env"
  project_id       = "test-project"
  kube_cluster_id  = "test-cluster-id"
  pinned_base_image = "my-registry/base:2.0"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("UpdateEnvironmentV2")
						require.NotEmpty(t, captured, "Expected at least one UpdateEnvironment call")

						req := captured[len(captured)-1].(*serverv1.UpdateEnvironmentV2Request)
						require.NotNil(t, req.UpdateMask, "Expected UpdateMask to be set")
						assert.Equal(t, []string{"pinned_base_image"}, req.UpdateMask.Paths,
							"Expected only 'pinned_base_image' in field mask")
						require.NotNil(t, req.Environment.PinnedBaseImage)
						assert.Equal(t, "my-registry/base:2.0", *req.Environment.PinnedBaseImage)

						return nil
					},
				),
			},
		},
	})
}

// TestManagedEnvironmentNoOpUpdate verifies that no UpdateEnvironment call is made
// when no fields change between applies.
func TestManagedEnvironmentNoOpUpdate(t *testing.T) {
	t.Parallel()
	server := setupMockServerManagedEnvironment(t)

	config := providerConfig(server.URL) + `
resource "chalk_managed_environment" "test" {
  name            = "test-managed-env"
  project_id      = "test-project"
  kube_cluster_id = "test-cluster-id"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: config},
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						createCalls := server.GetCapturedRequests("CreateEnvironmentV2")
						updateCalls := server.GetCapturedRequests("UpdateEnvironmentV2")

						assert.Len(t, createCalls, 1, "Expected exactly one CreateEnvironment call")
						assert.Empty(t, updateCalls, "Expected no UpdateEnvironment calls for no-op update")

						return nil
					},
				),
			},
		},
	})
}

// TestManagedEnvironmentImmutableFieldReplace verifies that changing an immutable field
// (name, project_id, kube_cluster_id) triggers a destroy + recreate rather than an update.
func TestManagedEnvironmentImmutableFieldReplace(t *testing.T) {
	t.Parallel()
	server := setupMockServerManagedEnvironment(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_managed_environment" "test" {
  name            = "test-managed-env"
  project_id      = "test-project"
  kube_cluster_id = "test-cluster-id"
}
`,
			},
			{
				Config: providerConfig(server.URL) + `
resource "chalk_managed_environment" "test" {
  name            = "test-managed-env"
  project_id      = "test-project-renamed"
  kube_cluster_id = "test-cluster-id"
}
`,
				Check: func(s *terraform.State) error {
					createCalls := server.GetCapturedRequests("CreateEnvironmentV2")
					updateCalls := server.GetCapturedRequests("UpdateEnvironmentV2")
					deleteCalls := server.GetCapturedRequests("DeleteEnvironment")

					assert.Len(t, createCalls, 2, "Expected two CreateEnvironment calls (original + replacement)")
					assert.Empty(t, updateCalls, "Expected no UpdateEnvironment calls for immutable field change")
					assert.Len(t, deleteCalls, 1, "Expected one DeleteEnvironment call (original destroyed)")

					return nil
				},
			},
		},
	})
}

// TestManagedEnvironmentUpdateError verifies error handling when UpdateEnvironment fails.
func TestManagedEnvironmentUpdateError(t *testing.T) {
	t.Parallel()
	server := setupMockServerManagedEnvironment(t)

	// Reset and reconfigure so the update error behavior takes effect.
	// (WithBehavior takes precedence over ReturnError in the registry, so we must reset.)
	const envID = "test-managed-env-id"
	kubeClusterID := "test-cluster-id"
	kubeJobNS := "test-managed-namespace"
	env := &serverv1.Environment{
		Id:               envID,
		Name:             "test-managed-env",
		ProjectId:        "test-project",
		KubeClusterId:    &kubeClusterID,
		KubeJobNamespace: &kubeJobNS,
	}

	server.Reset()
	server.OnCreateEnvironmentV2().Return(&serverv1.CreateEnvironmentV2Response{Environment: env})
	server.OnGetEnv().Return(&serverv1.GetEnvResponse{Environment: env})
	server.OnUpdateEnvironmentV2().ReturnError(
		connect.NewError(connect.CodeInvalidArgument, errors.New("invalid pinned base image")))
	server.OnDeleteEnvironment().Return(&serverv1.DeleteEnvironmentResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_managed_environment" "test" {
  name            = "test-managed-env"
  project_id      = "test-project"
  kube_cluster_id = "test-cluster-id"
}
`,
			},
			{
				Config: providerConfig(server.URL) + `
resource "chalk_managed_environment" "test" {
  name             = "test-managed-env"
  project_id       = "test-project"
  kube_cluster_id  = "test-cluster-id"
  pinned_base_image = "invalid-image"
}
`,
				ExpectError: regexp.MustCompile("invalid pinned base image"),
			},
		},
	})
}

// TestManagedEnvironmentCreateError verifies error handling when CreateEnvironment fails.
func TestManagedEnvironmentCreateError(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnCreateEnvironmentV2().ReturnError(
		connect.NewError(connect.CodePermissionDenied, errors.New("insufficient permissions for environment.create")))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_managed_environment" "test" {
  name            = "test-managed-env"
  project_id      = "test-project"
  kube_cluster_id = "test-cluster-id"
}
`,
				ExpectError: regexp.MustCompile("insufficient permissions"),
			},
		},
	})
}

// TestManagedEnvironmentImport verifies the import lifecycle.
func TestManagedEnvironmentImport(t *testing.T) {
	t.Parallel()
	server := setupMockServerManagedEnvironment(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + `
resource "chalk_managed_environment" "test" {
  name            = "test-managed-env"
  project_id      = "test-project"
  kube_cluster_id = "test-cluster-id"
}
`,
			},
			{
				ResourceName:      "chalk_managed_environment.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "test-managed-env-id",
			},
		},
	})
}

// TestManagedEnvironmentImportTypeMismatch verifies that importing an unmanaged environment
// into chalk_managed_environment fails with a clear error.
func TestManagedEnvironmentImportTypeMismatch(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	managed := false
	server.OnGetEnv().Return(&serverv1.GetEnvResponse{
		Environment: &serverv1.Environment{
			Id:        "unmanaged-env-id",
			Name:      "unmanaged-env",
			ProjectId: "test-project",
			Managed:   &managed,
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				ResourceName:  "chalk_managed_environment.test",
				ImportState:   true,
				ImportStateId: "unmanaged-env-id",
				Config: providerConfig(server.URL) + `
resource "chalk_managed_environment" "test" {
  name            = "placeholder"
  project_id      = "placeholder"
  kube_cluster_id = "placeholder"
}
`,
				ExpectError: regexp.MustCompile("not a managed environment"),
			},
		},
	})
}

// TestManagedEnvironmentServerDefaults verifies that computed fields set by the server
// (like kube_job_namespace) do not cause drift on re-plan.
func TestManagedEnvironmentServerDefaults(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	const envID = "test-managed-env-id"
	kubeClusterID := "test-cluster-id"
	kubeJobNS := "server-assigned-namespace"

	server.OnCreateEnvironmentV2().Return(&serverv1.CreateEnvironmentV2Response{
		Environment: &serverv1.Environment{
			Id:               envID,
			Name:             "test-managed-env",
			ProjectId:        "test-project",
			KubeClusterId:    &kubeClusterID,
			KubeJobNamespace: &kubeJobNS,
		},
	})
	server.OnGetEnv().Return(&serverv1.GetEnvResponse{
		Environment: &serverv1.Environment{
			Id:               envID,
			Name:             "test-managed-env",
			ProjectId:        "test-project",
			KubeClusterId:    &kubeClusterID,
			KubeJobNamespace: &kubeJobNS,
		},
	})
	server.OnDeleteEnvironment().Return(&serverv1.DeleteEnvironmentResponse{})

	config := providerConfig(server.URL) + `
resource "chalk_managed_environment" "test" {
  name            = "test-managed-env"
  project_id      = "test-project"
  kube_cluster_id = "test-cluster-id"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Step 1: apply should succeed; kube_job_namespace is populated from server response.
			{
				Config: config,
				Check:  resource.TestCheckResourceAttr("chalk_managed_environment.test", "kube_job_namespace", "server-assigned-namespace"),
			},
			// Step 2: re-plan should show no changes.
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestManagedEnvironmentJsonNormalization verifies that pretty-printed JSON in
// specs_config_json and customer_metadata doesn't cause "inconsistent result after apply"
// errors, and that a subsequent plan shows no diff. updateStateFromManagedEnvironment
// re-serializes proto maps via protojson.Marshal (always compact), and jsontypes.Normalized
// treats the plan value (pretty) and state value (compact) as semantically equivalent.
func TestManagedEnvironmentJsonNormalization(t *testing.T) {
	t.Parallel()
	server := setupMockServerManagedEnvironment(t)

	config := providerConfig(server.URL) + `
resource "chalk_managed_environment" "test" {
  name              = "test-managed-env"
  project_id        = "test-project"
  kube_cluster_id   = "test-cluster-id"
  specs_config_json = "{\"services\": {\"engine\": {\"max_instances\": 1, \"min_instances\": 1}}}"
  customer_metadata = "{\"tier\": \"enterprise\", \"region\": \"us-east-1\"}"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Step 1: apply must succeed without "inconsistent result after apply" error.
			{Config: config},
			// Step 2: re-plan must show no diff despite the server returning compact JSON.
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}
