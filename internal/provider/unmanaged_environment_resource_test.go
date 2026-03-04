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

// setupMockServerUnmanagedEnvironment creates a mock server that tracks environment
// state across Create/Update/Read operations, simulating a real API server.
func setupMockServerUnmanagedEnvironment(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	const envID = "test-env-id"
	var currentEnv *serverv1.Environment

	server.OnCreateEnvironmentV2().WithBehavior(func(req proto.Message) (proto.Message, error) {
		createReq := req.(*serverv1.CreateEnvironmentV2Request)
		env := proto.Clone(createReq.Environment).(*serverv1.Environment)
		env.Id = envID
		currentEnv = env
		return &serverv1.CreateEnvironmentV2Response{Environment: env}, nil
	})

	server.OnUpdateEnvironmentV2().WithBehavior(func(req proto.Message) (proto.Message, error) {
		updateReq := req.(*serverv1.UpdateEnvironmentV2Request)
		updated := proto.Clone(currentEnv).(*serverv1.Environment)
		for _, path := range updateReq.UpdateMask.Paths {
			applyUnmanagedEnvField(updated, updateReq.Environment, path)
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

// applyUnmanagedEnvField applies a single field-mask path update from src to dst.
func applyUnmanagedEnvField(dst, src *serverv1.Environment, path string) {
	switch path {
	case "service_url":
		dst.ServiceUrl = src.ServiceUrl
	case "kube_service_account_name":
		dst.KubeServiceAccountName = src.KubeServiceAccountName
	case "kube_cluster_mode":
		dst.KubeClusterMode = src.KubeClusterMode
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

// TestUnmanagedEnvironmentCreate verifies the basic create/read/delete lifecycle.
func TestUnmanagedEnvironmentCreate(t *testing.T) {
	server := setupMockServerUnmanagedEnvironment(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_unmanaged_environment" "test" {
  name               = "test-env"
  project_id         = "test-project"
  kube_cluster_id    = "test-cluster-id"
  kube_job_namespace = "test-namespace"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_unmanaged_environment.test", "id", "test-env-id"),
					resource.TestCheckResourceAttr("chalk_unmanaged_environment.test", "name", "test-env"),
					resource.TestCheckResourceAttr("chalk_unmanaged_environment.test", "project_id", "test-project"),
					resource.TestCheckResourceAttr("chalk_unmanaged_environment.test", "kube_cluster_id", "test-cluster-id"),
					resource.TestCheckResourceAttr("chalk_unmanaged_environment.test", "kube_job_namespace", "test-namespace"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("CreateEnvironmentV2")
						require.Len(t, captured, 1, "Expected exactly one CreateEnvironment call")

						req := captured[0].(*serverv1.CreateEnvironmentV2Request)
						require.NotNil(t, req.Environment)
						assert.Equal(t, "test-env", req.Environment.Name)
						assert.Equal(t, "test-project", req.Environment.ProjectId)
						require.NotNil(t, req.Environment.KubeClusterId)
						assert.Equal(t, "test-cluster-id", *req.Environment.KubeClusterId)
						require.NotNil(t, req.Environment.KubeJobNamespace)
						assert.Equal(t, "test-namespace", *req.Environment.KubeJobNamespace)

						return nil
					},
				),
			},
		},
	})
}

// TestUnmanagedEnvironmentUpdateFieldMask verifies that the update field mask contains
// only the paths of changed fields.
func TestUnmanagedEnvironmentUpdateFieldMask(t *testing.T) {
	server := setupMockServerUnmanagedEnvironment(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_unmanaged_environment" "test" {
  name                    = "test-env"
  project_id              = "test-project"
  kube_cluster_id         = "test-cluster-id"
  kube_job_namespace      = "test-namespace"
  private_pip_repositories = "https://pypi.example.com"
}
`,
			},
			{
				Config: `
resource "chalk_unmanaged_environment" "test" {
  name                    = "test-env"
  project_id              = "test-project"
  kube_cluster_id         = "test-cluster-id"
  kube_job_namespace      = "test-namespace"
  private_pip_repositories = "https://pypi2.example.com"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("UpdateEnvironmentV2")
						require.NotEmpty(t, captured, "Expected at least one UpdateEnvironment call")

						req := captured[len(captured)-1].(*serverv1.UpdateEnvironmentV2Request)
						require.NotNil(t, req.UpdateMask, "Expected UpdateMask to be set")
						assert.Equal(t, []string{"private_pip_repositories"}, req.UpdateMask.Paths,
							"Expected only 'private_pip_repositories' in field mask")
						require.NotNil(t, req.Environment.PrivatePipRepositories)
						assert.Equal(t, "https://pypi2.example.com", *req.Environment.PrivatePipRepositories)

						return nil
					},
				),
			},
		},
	})
}

// TestUnmanagedEnvironmentNoOpUpdate verifies that no UpdateEnvironment call is made
// when no fields change between applies.
func TestUnmanagedEnvironmentNoOpUpdate(t *testing.T) {
	server := setupMockServerUnmanagedEnvironment(t)
	setupTestEnv(t, server.URL)

	config := `
resource "chalk_unmanaged_environment" "test" {
  name               = "test-env"
  project_id         = "test-project"
  kube_cluster_id    = "test-cluster-id"
  kube_job_namespace = "test-namespace"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
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

// TestUnmanagedEnvironmentImmutableFieldReplace verifies that changing an immutable field
// (name, project_id, kube_cluster_id, kube_job_namespace) triggers a destroy + recreate
// rather than an in-place update.
func TestUnmanagedEnvironmentImmutableFieldReplace(t *testing.T) {
	server := setupMockServerUnmanagedEnvironment(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_unmanaged_environment" "test" {
  name               = "test-env"
  project_id         = "test-project"
  kube_cluster_id    = "test-cluster-id"
  kube_job_namespace = "test-namespace"
}
`,
			},
			{
				Config: `
resource "chalk_unmanaged_environment" "test" {
  name               = "test-env-renamed"
  project_id         = "test-project"
  kube_cluster_id    = "test-cluster-id"
  kube_job_namespace = "test-namespace"
}
`,
				Check: func(s *terraform.State) error {
					createCalls := server.GetCapturedRequests("CreateEnvironmentV2")
					updateCalls := server.GetCapturedRequests("UpdateEnvironmentV2")
					deleteCalls := server.GetCapturedRequests("DeleteEnvironment")

					// Two creates (original + replacement) and one delete (the original).
					assert.Len(t, createCalls, 2, "Expected two CreateEnvironment calls (original + replacement)")
					assert.Empty(t, updateCalls, "Expected no UpdateEnvironment calls for immutable field change")
					assert.Len(t, deleteCalls, 1, "Expected one DeleteEnvironment call (original destroyed)")

					return nil
				},
			},
		},
	})
}

// TestUnmanagedEnvironmentUpdateError verifies error handling when UpdateEnvironment fails.
func TestUnmanagedEnvironmentUpdateError(t *testing.T) {
	server := setupMockServerUnmanagedEnvironment(t)
	setupTestEnv(t, server.URL)

	// Reset and reconfigure so the update error behavior takes effect.
	// (WithBehavior takes precedence over ReturnError in the registry, so we must reset.)
	const envID = "test-env-id"
	kubeClusterID := "test-cluster-id"
	kubeJobNS := "test-namespace"
	env := &serverv1.Environment{
		Id:               envID,
		Name:             "test-env",
		ProjectId:        "test-project",
		KubeClusterId:    &kubeClusterID,
		KubeJobNamespace: &kubeJobNS,
	}

	server.Reset()
	server.OnCreateEnvironmentV2().Return(&serverv1.CreateEnvironmentV2Response{Environment: env})
	server.OnGetEnv().Return(&serverv1.GetEnvResponse{Environment: env})
	server.OnUpdateEnvironmentV2().ReturnError(
		connect.NewError(connect.CodeInvalidArgument, errors.New("invalid service url")))
	server.OnDeleteEnvironment().Return(&serverv1.DeleteEnvironmentResponse{})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_unmanaged_environment" "test" {
  name               = "test-env"
  project_id         = "test-project"
  kube_cluster_id    = "test-cluster-id"
  kube_job_namespace = "test-namespace"
}
`,
			},
			{
				Config: `
resource "chalk_unmanaged_environment" "test" {
  name               = "test-env"
  project_id         = "test-project"
  kube_cluster_id    = "test-cluster-id"
  kube_job_namespace = "test-namespace"
  service_url        = "not-a-valid-url"
}
`,
				ExpectError: regexp.MustCompile("invalid service url"),
			},
		},
	})
}

// TestUnmanagedEnvironmentCreateError verifies error handling when CreateEnvironment fails.
func TestUnmanagedEnvironmentCreateError(t *testing.T) {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })
	setupTestEnv(t, server.URL)

	server.OnCreateEnvironmentV2().ReturnError(
		connect.NewError(connect.CodeInvalidArgument, errors.New("invalid environment name")))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_unmanaged_environment" "test" {
  name               = "invalid!"
  project_id         = "test-project"
  kube_cluster_id    = "test-cluster-id"
  kube_job_namespace = "test-namespace"
}
`,
				ExpectError: regexp.MustCompile("invalid environment name"),
			},
		},
	})
}

// TestUnmanagedEnvironmentImport verifies the import lifecycle.
func TestUnmanagedEnvironmentImport(t *testing.T) {
	server := setupMockServerUnmanagedEnvironment(t)
	setupTestEnv(t, server.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
resource "chalk_unmanaged_environment" "test" {
  name               = "test-env"
  project_id         = "test-project"
  kube_cluster_id    = "test-cluster-id"
  kube_job_namespace = "test-namespace"
}
`,
			},
			{
				ResourceName:      "chalk_unmanaged_environment.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "test-env-id",
			},
		},
	})
}

// TestUnmanagedEnvironmentImportTypeMismatch verifies that importing a managed environment
// into chalk_unmanaged_environment fails with a clear error.
func TestUnmanagedEnvironmentImportTypeMismatch(t *testing.T) {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })
	setupTestEnv(t, server.URL)

	managed := true
	server.OnGetEnv().Return(&serverv1.GetEnvResponse{
		Environment: &serverv1.Environment{
			Id:        "managed-env-id",
			Name:      "managed-env",
			ProjectId: "test-project",
			Managed:   &managed,
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
		Steps: []resource.TestStep{
			{
				ResourceName:  "chalk_unmanaged_environment.test",
				ImportState:   true,
				ImportStateId: "managed-env-id",
				Config: `
resource "chalk_unmanaged_environment" "test" {
  name               = "placeholder"
  project_id         = "placeholder"
  kube_cluster_id    = "placeholder"
  kube_job_namespace = "placeholder"
}
`,
				ExpectError: regexp.MustCompile("managed environment"),
			},
		},
	})
}

// TestUnmanagedEnvironmentJsonNormalization verifies that pretty-printed JSON in
// specs_config_json and customer_metadata doesn't cause "inconsistent result after apply"
// errors, and that a subsequent plan shows no diff. updateStateFromEnvironment
// re-serializes proto maps via protojson.Marshal (always compact), and jsontypes.Normalized
// treats the plan value (pretty) and state value (compact) as semantically equivalent.
func TestUnmanagedEnvironmentJsonNormalization(t *testing.T) {
	server := setupMockServerUnmanagedEnvironment(t)
	setupTestEnv(t, server.URL)

	config := `
resource "chalk_unmanaged_environment" "test" {
  name               = "test-env"
  project_id         = "test-project"
  kube_cluster_id    = "test-cluster-id"
  kube_job_namespace = "test-namespace"
  specs_config_json  = "{\"services\": {\"engine\": {\"max_instances\": 1, \"min_instances\": 1}}}"
  customer_metadata  = "{\"tier\": \"enterprise\", \"region\": \"us-east-1\"}"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(server.URL),
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
