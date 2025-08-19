package provider

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"net/http"
)

var _ resource.Resource = &ProjectResource{}
var _ resource.ResourceWithImportState = &ProjectResource{}

func NewProjectResource() resource.Resource {
	return &ProjectResource{}
}

type ProjectResource struct {
	client *ChalkClient
}

type ProjectResourceModel struct {
	Id      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	TeamId  types.String `tfsdk:"team_id"`
	GitRepo types.String `tfsdk:"git_repo"`
}

func (r *ProjectResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *ProjectResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk project resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Project identifier",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Project name",
				Required:            true,
			},
			"team_id": schema.StringAttribute{
				MarkdownDescription: "Team ID",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"git_repo": schema.StringAttribute{
				MarkdownDescription: "Git repository URL",
				Optional:            true,
			},
		},
	}
}

func (r *ProjectResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ChalkClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ChalkClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *ProjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProjectResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         r.client.ApiServer,
			interceptors: []connect.Interceptor{MakeApiServerHeaderInterceptor("x-chalk-server", "go-api")},
		},
	)

	// Create team client with token injection interceptor
	tc := NewTeamClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	})

	createReq := &serverv1.CreateProjectRequest{
		Name: data.Name.ValueString(),
	}

	project, err := tc.CreateProject(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Project",
			fmt.Sprintf("Could not create project: %v", err),
		)
		return
	}

	// Update with created values
	data.Id = types.StringValue(project.Msg.Project.Id)
	data.TeamId = types.StringValue(project.Msg.Project.TeamId)

	// If git_repo was provided, we need to update the project
	if !data.GitRepo.IsNull() {
		updateReq := &serverv1.UpdateProjectRequest{
			Id:     data.Id.ValueString(),
			Update: &serverv1.UpdateProjectOperation{},
		}

		gitRepo := data.GitRepo.ValueString()
		updateReq.Update.GitRepo = &gitRepo

		updateReq.UpdateMask = &fieldmaskpb.FieldMask{
			Paths: []string{"git_repo"},
		}

		_, err = tc.UpdateProject(ctx, connect.NewRequest(updateReq))
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Chalk Project",
				fmt.Sprintf("Project was created but could not be updated with git_repo: %v", err),
			)
			return
		}
	}

	tflog.Trace(ctx, "created a chalk_project resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProjectResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         r.client.ApiServer,
			interceptors: []connect.Interceptor{MakeApiServerHeaderInterceptor("x-chalk-server", "go-api")},
		},
	)

	// Create team client with token injection interceptor
	tc := NewTeamClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	})

	// Get the project by fetching the team and finding the project
	team, err := tc.GetTeam(ctx, connect.NewRequest(&serverv1.GetTeamRequest{}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Chalk Project",
			fmt.Sprintf("Could not read team to find project %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Find the project in the team's projects
	var foundProject *serverv1.Project
	for _, p := range team.Msg.Team.Projects {
		if p.Id == data.Id.ValueString() {
			foundProject = p
			break
		}
	}

	if foundProject == nil {
		availableProjects := make([]string, 0)
		for _, p := range team.Msg.Team.Projects {
			availableProjects = append(availableProjects, p.Id)
		}

		resp.Diagnostics.AddError(
			"Error Reading Chalk Project",
			fmt.Sprintf("Project '%s' not found in team '%s'; available projects: %s", data.Id.ValueString(), team.Msg.Team.Id, availableProjects),
		)
		return
	}

	// Update the model with the fetched data
	data.Name = types.StringValue(foundProject.Name)
	data.TeamId = types.StringValue(foundProject.TeamId)

	if foundProject.GitRepo != nil && *foundProject.GitRepo != "" {
		data.GitRepo = types.StringValue(*foundProject.GitRepo)
	} else {
		data.GitRepo = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProjectResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         r.client.ApiServer,
			interceptors: []connect.Interceptor{MakeApiServerHeaderInterceptor("x-chalk-server", "go-api")},
		},
	)

	// Create team client with token injection interceptor
	tc := NewTeamClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	})

	updateReq := &serverv1.UpdateProjectRequest{
		Id:     data.Id.ValueString(),
		Update: &serverv1.UpdateProjectOperation{},
	}

	var updateMaskPaths []string

	// Check what fields have changed
	var state ProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !data.Name.Equal(state.Name) {
		name := data.Name.ValueString()
		updateReq.Update.Name = &name
		updateMaskPaths = append(updateMaskPaths, "name")
	}

	if !data.GitRepo.Equal(state.GitRepo) {
		if !data.GitRepo.IsNull() {
			gitRepo := data.GitRepo.ValueString()
			updateReq.Update.GitRepo = &gitRepo
		} else {
			emptyRepo := ""
			updateReq.Update.GitRepo = &emptyRepo
		}
		updateMaskPaths = append(updateMaskPaths, "git_repo")
	}

	if len(updateMaskPaths) > 0 {
		updateReq.UpdateMask = &fieldmaskpb.FieldMask{
			Paths: updateMaskPaths,
		}

		_, err := tc.UpdateProject(ctx, connect.NewRequest(updateReq))
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Chalk Project",
				fmt.Sprintf("Could not update project: %v", err),
			)
			return
		}
	}

	tflog.Trace(ctx, "updated a chalk_project resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProjectResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         r.client.ApiServer,
			interceptors: []connect.Interceptor{MakeApiServerHeaderInterceptor("x-chalk-server", "go-api")},
		},
	)

	// Create team client with token injection interceptor
	tc := NewTeamClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	})

	archiveReq := &serverv1.ArchiveProjectRequest{
		Id: data.Id.ValueString(),
	}

	_, err := tc.ArchiveProject(ctx, connect.NewRequest(archiveReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Archiving Chalk Project",
			fmt.Sprintf("Could not archive project %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "archived chalk_project resource")
}

func (r *ProjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
