package provider

import (
	"fmt"
	"testing"

	scalinggroupv1 "github.com/chalk-ai/chalk-go/gen/chalk/scalinggroup/v1"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func setupMockScalingGroupServer(t *testing.T) *testserver.MockServer {
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	current := &scalinggroupv1.ScalingGroupResponse{}

	server.OnCreateScalingGroup().WithBehavior(func(req proto.Message) (proto.Message, error) {
		createReq := req.(*scalinggroupv1.CreateScalingGroupRequest)
		current = &scalinggroupv1.ScalingGroupResponse{
			Id:     "sg-test-id",
			Name:   createReq.Spec.ContainerSpec.Name,
			Status: "Running",
			Spec:   createReq.Spec,
		}
		return &scalinggroupv1.CreateScalingGroupResponse{ScalingGroup: current}, nil
	})

	server.OnGetScalingGroup().WithBehavior(func(req proto.Message) (proto.Message, error) {
		return &scalinggroupv1.GetScalingGroupResponse{ScalingGroup: current}, nil
	})

	server.OnDeleteScalingGroup().WithBehavior(func(req proto.Message) (proto.Message, error) {
		return &scalinggroupv1.DeleteScalingGroupResponse{ScalingGroup: current}, nil
	})

	return server
}

func scalingGroupMinimalConfig(serverURL, name string) string {
	return providerConfig(serverURL) + fmt.Sprintf(`
resource "chalk_scaling_group" "test" {
  name           = %q
  environment_id = "test-env-id"
  container_spec = {
    image = "my-image:latest"
  }
  scaling_spec = {
    min_replicas = 1
    max_replicas = 3
  }
}
`, name)
}

func TestScalingGroupResourceCreate(t *testing.T) {
	t.Parallel()
	server := setupMockScalingGroupServer(t)
	name := "test-sg-" + uuid.New().String()[:8]

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: scalingGroupMinimalConfig(server.URL, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_scaling_group.test", "id", "sg-test-id"),
					resource.TestCheckResourceAttr("chalk_scaling_group.test", "name", name),
					resource.TestCheckResourceAttr("chalk_scaling_group.test", "environment_id", "test-env-id"),
					resource.TestCheckResourceAttr("chalk_scaling_group.test", "status", "Running"),
					resource.TestCheckResourceAttr("chalk_scaling_group.test", "container_spec.image", "my-image:latest"),
					resource.TestCheckResourceAttr("chalk_scaling_group.test", "scaling_spec.min_replicas", "1"),
					resource.TestCheckResourceAttr("chalk_scaling_group.test", "scaling_spec.max_replicas", "3"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("CreateScalingGroup")
						require.Len(t, captured, 1)
						req := captured[0].(*scalinggroupv1.CreateScalingGroupRequest)
						assert.Equal(t, name, req.Spec.ContainerSpec.Name)
						assert.Equal(t, "my-image:latest", req.Spec.ContainerSpec.Image)
						assert.Equal(t, int32(1), req.Spec.ScalingSpec.MinReplicas)
						assert.Equal(t, int32(3), req.Spec.ScalingSpec.MaxReplicas)
						return nil
					},
				),
			},
		},
	})
}

func TestScalingGroupResourceCreateWithOptionals(t *testing.T) {
	t.Parallel()
	server := setupMockScalingGroupServer(t)
	name := "test-sg-" + uuid.New().String()[:8]

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + fmt.Sprintf(`
resource "chalk_scaling_group" "test" {
  name           = %q
  environment_id = "test-env-id"
  container_spec = {
    image    = "my-image:latest"
    protocol = "grpc"
    env_vars = { FOO = "bar", BAZ = "qux" }
    tags     = { team = "ml" }
    resources = {
      cpu    = "2"
      memory = "4Gi"
    }
    volumes = [
      {
        name       = "shm"
        mount_path = "/dev/shm"
        type       = "shared_memory"
        size_limit = "2Gi"
      }
    ]
  }
  scaling_spec = {
    min_replicas                      = 0
    max_replicas                      = 10
    target_cpu_utilization_percentage = 80
    shutdown_delay_seconds            = 60
  }
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("chalk_scaling_group.test", "id", "sg-test-id"),
					resource.TestCheckResourceAttr("chalk_scaling_group.test", "container_spec.protocol", "grpc"),
					resource.TestCheckResourceAttr("chalk_scaling_group.test", "container_spec.resources.cpu", "2"),
					resource.TestCheckResourceAttr("chalk_scaling_group.test", "container_spec.resources.memory", "4Gi"),
					resource.TestCheckResourceAttr("chalk_scaling_group.test", "container_spec.volumes.0.name", "shm"),
					resource.TestCheckResourceAttr("chalk_scaling_group.test", "container_spec.volumes.0.size_limit", "2Gi"),
					resource.TestCheckResourceAttr("chalk_scaling_group.test", "scaling_spec.target_cpu_utilization_percentage", "80"),
					resource.TestCheckResourceAttr("chalk_scaling_group.test", "scaling_spec.shutdown_delay_seconds", "60"),
					func(s *terraform.State) error {
						captured := server.GetCapturedRequests("CreateScalingGroup")
						require.Len(t, captured, 1)
						req := captured[0].(*scalinggroupv1.CreateScalingGroupRequest)
						cs := req.Spec.ContainerSpec

						assert.Equal(t, "grpc", cs.GetProtocol())
						assert.Equal(t, map[string]string{"FOO": "bar", "BAZ": "qux"}, cs.EnvVars)
						assert.Equal(t, map[string]string{"team": "ml"}, cs.Tags)
						assert.Equal(t, "2", cs.Resources.GetCpu())
						assert.Equal(t, "4Gi", cs.Resources.GetMemory())
						require.Len(t, cs.Volumes, 1)
						assert.Equal(t, "shm", cs.Volumes[0].Name)
						assert.Equal(t, "/dev/shm", cs.Volumes[0].MountPath)
						assert.Equal(t, "shared_memory", cs.Volumes[0].Type)
						assert.Equal(t, "2Gi", cs.Volumes[0].GetSizeLimit())

						ss := req.Spec.ScalingSpec
						assert.Equal(t, int32(0), ss.MinReplicas)
						assert.Equal(t, int32(10), ss.MaxReplicas)
						assert.Equal(t, int32(80), ss.GetTargetCpuUtilizationPercentage())
						assert.Equal(t, int32(60), ss.GetShutdownDelaySeconds())
						return nil
					},
				),
			},
		},
	})
}

func TestScalingGroupResourceDelete(t *testing.T) {
	t.Parallel()
	server := setupMockScalingGroupServer(t)
	name := "test-sg-" + uuid.New().String()[:8]

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: scalingGroupMinimalConfig(server.URL, name)},
			{
				Config: providerConfig(server.URL),
				Check: func(s *terraform.State) error {
					captured := server.GetCapturedRequests("DeleteScalingGroup")
					require.Len(t, captured, 1)
					req := captured[0].(*scalinggroupv1.DeleteScalingGroupRequest)
					assert.Equal(t, "sg-test-id", req.GetId())
					return nil
				},
			},
		},
	})
}

func TestScalingGroupResourceImport(t *testing.T) {
	t.Parallel()
	server := setupMockScalingGroupServer(t)
	name := "test-sg-" + uuid.New().String()[:8]

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{Config: scalingGroupMinimalConfig(server.URL, name)},
			{
				ResourceName:      "chalk_scaling_group.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "test-env-id/sg-test-id",
			},
		},
	})
}
