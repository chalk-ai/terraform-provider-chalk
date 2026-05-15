package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/client"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &ProjectDataSource{}

func NewProjectDataSource() datasource.DataSource {
	return &ProjectDataSource{}
}

type ProjectDataSource struct {
	client *client.Manager
}

type ProjectDataSourceModel struct {
	Id      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	TeamId  types.String `tfsdk:"team_id"`
	GitRepo types.String `tfsdk:"git_repo"`
}

func (d *ProjectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *ProjectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Chalk project by ID. Looks up the project in the authenticated team via TeamService.GetTeam.",
		Attributes: map[string]schema.Attribute{
			"id":       schema.StringAttribute{MarkdownDescription: "Project identifier.", Required: true},
			"name":     schema.StringAttribute{Computed: true},
			"team_id":  schema.StringAttribute{Computed: true},
			"git_repo": schema.StringAttribute{Computed: true},
		},
	}
}

func (d *ProjectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Manager)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Manager, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *ProjectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProjectDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "read chalk_project data source", map[string]any{"id": data.Id.ValueString()})

	tc := d.client.NewTeamClient(ctx)
	team, err := tc.GetTeam(ctx, connect.NewRequest(&serverv1.GetTeamRequest{}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Chalk Project",
			fmt.Sprintf("Could not read team to find project %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	var found *serverv1.Project
	for _, p := range team.Msg.Team.Projects {
		if p.Id == data.Id.ValueString() {
			found = p
			break
		}
	}
	if found == nil {
		available := make([]string, 0, len(team.Msg.Team.Projects))
		for _, p := range team.Msg.Team.Projects {
			available = append(available, p.Id)
		}
		resp.Diagnostics.AddError(
			"Error Reading Chalk Project",
			fmt.Sprintf("Project '%s' not found in team '%s'; available projects: %s", data.Id.ValueString(), team.Msg.Team.Id, available),
		)
		return
	}

	data.Name = types.StringValue(found.Name)
	data.TeamId = types.StringValue(found.TeamId)
	if found.GitRepo != nil && *found.GitRepo != "" {
		data.GitRepo = types.StringValue(*found.GitRepo)
	} else {
		data.GitRepo = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
