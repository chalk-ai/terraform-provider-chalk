package provider

import (
	"errors"
	"regexp"
	"testing"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/testserver"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const testManagedAwsVpcDataSourceConfig = `
data "chalk_managed_aws_vpc" "test" {
  id = "test-vpc-id"
}
`

func buildAwsVpcResponse(designator *string, additionalCidr []string, subnets []*serverv1.AwsSubnetConfig) *serverv1.GetCloudComponentVpcResponse {
	return &serverv1.GetCloudComponentVpcResponse{
		Vpc: &serverv1.CloudComponentVpcResponse{
			Id:                "test-vpc-id",
			Kind:              "aws",
			Designator:        designator,
			CloudCredentialId: new("test-cred-id"),
			Spec: &serverv1.CloudComponentVpc{
				Name: "test-vpc-name",
				Config: &serverv1.CloudVpcConfig{
					Config: &serverv1.CloudVpcConfig_Aws{
						Aws: &serverv1.AWSVpcConfig{
							CidrBlock:            "10.0.0.0/16",
							AdditionalCidrBlocks: additionalCidr,
							Subnets:              subnets,
						},
					},
				},
			},
		},
	}
}

func TestManagedAwsVpcDataSource_AllAttributes(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetCloudComponentVpc().Return(buildAwsVpcResponse(
		new("vpcd"),
		[]string{"10.1.0.0/16"},
		[]*serverv1.AwsSubnetConfig{
			{
				Name:             "primary",
				PrivateCidrBlock: "10.0.0.0/20",
				PublicCidrBlock:  "10.0.16.0/20",
				AvailabilityZone: "a",
			},
		},
	))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testManagedAwsVpcDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.chalk_managed_aws_vpc.test", "id", "test-vpc-id"),
					resource.TestCheckResourceAttr("data.chalk_managed_aws_vpc.test", "name", "test-vpc-name"),
					resource.TestCheckResourceAttr("data.chalk_managed_aws_vpc.test", "designator", "vpcd"),
					resource.TestCheckResourceAttr("data.chalk_managed_aws_vpc.test", "cloud_credential_id", "test-cred-id"),
					resource.TestCheckResourceAttr("data.chalk_managed_aws_vpc.test", "cidr_block", "10.0.0.0/16"),
					resource.TestCheckResourceAttr("data.chalk_managed_aws_vpc.test", "additional_cidr_blocks.0", "10.1.0.0/16"),
					resource.TestCheckResourceAttr("data.chalk_managed_aws_vpc.test", "subnets.0.name", "primary"),
					resource.TestCheckResourceAttr("data.chalk_managed_aws_vpc.test", "subnets.0.private_cidr_block", "10.0.0.0/20"),
					resource.TestCheckResourceAttr("data.chalk_managed_aws_vpc.test", "subnets.0.public_cidr_block", "10.0.16.0/20"),
					resource.TestCheckResourceAttr("data.chalk_managed_aws_vpc.test", "subnets.0.availability_zone", "a"),
				),
			},
		},
	})
}

func TestManagedAwsVpcDataSource_NoAdditionalCidrs(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetCloudComponentVpc().Return(buildAwsVpcResponse(
		new("vpcd"),
		nil,
		[]*serverv1.AwsSubnetConfig{
			{Name: "primary", PrivateCidrBlock: "10.0.0.0/20", PublicCidrBlock: "10.0.16.0/20", AvailabilityZone: "a"},
		},
	))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testManagedAwsVpcDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("data.chalk_managed_aws_vpc.test", "additional_cidr_blocks.0"),
					resource.TestCheckResourceAttr("data.chalk_managed_aws_vpc.test", "subnets.0.name", "primary"),
				),
			},
		},
	})
}

func TestManagedAwsVpcDataSource_RpcError(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetCloudComponentVpc().ReturnError(
		connect.NewError(connect.CodeInternal, errors.New("backend exploded")),
	)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      providerConfig(server.URL) + testManagedAwsVpcDataSourceConfig,
				ExpectError: regexp.MustCompile(`Could not read managed VPC`),
			},
		},
	})
}
