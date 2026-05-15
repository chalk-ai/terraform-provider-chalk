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

const testProjectDataSourceConfig = `
data "chalk_project" "test" {
  id = "test-project-id"
}
`

func TestProjectDataSource_FoundWithGitRepo(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	gitRepo := "git@github.com:example/repo.git"
	server.OnGetTeam().Return(&serverv1.GetTeamResponse{
		Team: &serverv1.Team{
			Id: "test-team-id",
			Projects: []*serverv1.Project{
				{Id: "other-project", TeamId: "test-team-id", Name: "Other"},
				{Id: "test-project-id", TeamId: "test-team-id", Name: "Features", GitRepo: &gitRepo},
			},
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testProjectDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.chalk_project.test", "id", "test-project-id"),
					resource.TestCheckResourceAttr("data.chalk_project.test", "name", "Features"),
					resource.TestCheckResourceAttr("data.chalk_project.test", "team_id", "test-team-id"),
					resource.TestCheckResourceAttr("data.chalk_project.test", "git_repo", "git@github.com:example/repo.git"),
				),
			},
		},
	})
}

func TestProjectDataSource_FoundWithoutGitRepo(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetTeam().Return(&serverv1.GetTeamResponse{
		Team: &serverv1.Team{
			Id: "test-team-id",
			Projects: []*serverv1.Project{
				{Id: "test-project-id", TeamId: "test-team-id", Name: "Features"},
			},
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(server.URL) + testProjectDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("data.chalk_project.test", "git_repo"),
				),
			},
		},
	})
}

func TestProjectDataSource_NotFound(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetTeam().Return(&serverv1.GetTeamResponse{
		Team: &serverv1.Team{
			Id:       "test-team-id",
			Projects: []*serverv1.Project{{Id: "other-id", TeamId: "test-team-id", Name: "Other"}},
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      providerConfig(server.URL) + testProjectDataSourceConfig,
				ExpectError: regexp.MustCompile(`not found in team`),
			},
		},
	})
}

func TestProjectDataSource_RpcError(t *testing.T) {
	t.Parallel()
	server := testserver.NewMockBuilderServer(t)
	t.Cleanup(func() { server.Close() })

	server.OnGetTeam().ReturnError(connect.NewError(connect.CodeInternal, errors.New("backend exploded")))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      providerConfig(server.URL) + testProjectDataSourceConfig,
				ExpectError: regexp.MustCompile(`Could not read team`),
			},
		},
	})
}
