package provider

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &EnvironmentDataSource{}

func NewEnvironmentDataSource() datasource.DataSource {
	return &EnvironmentDataSource{}
}

type EnvironmentDataSource struct {
	client *ChalkClient
}

type EnvironmentDataSourceModel struct {
	Id                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	CloudAccountLocator types.String `tfsdk:"cloud_account_locator"`
	IsDefault           types.Bool   `tfsdk:"is_default"`
	CreatedAt           types.String `tfsdk:"created_at"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
	OnlineStoreKind     types.String `tfsdk:"online_store_kind"`
}

func (d *EnvironmentDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (d *EnvironmentDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk environment data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Environment identifier",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Environment name",
				Required:            true,
			},
			"cloud_account_locator": schema.StringAttribute{
				MarkdownDescription: "Cloud Account Locator",
				Computed:            true,
			},
			"online_store_kind": schema.StringAttribute{
				MarkdownDescription: "Online store kind for the environment",
				Optional:            true,
			},
			"is_default": schema.BoolAttribute{
				MarkdownDescription: "Whether this is the default environment",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the environment was created",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the environment was last updated",
				Computed:            true,
			},
		},
	}
}

func (d *EnvironmentDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ChalkClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ChalkClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

func (d *EnvironmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EnvironmentDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "read chalk_environment data source", map[string]interface{}{
		"name": data.Name.ValueString(),
	})

	// Create auth client first
	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         d.client.ApiServer,
			interceptors: []connect.Interceptor{MakeApiServerHeaderInterceptor("x-chalk-server", "go-api")},
		},
	)

	// Create team client with token injection interceptor
	tc := NewTeamClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       d.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-env-id", data.Name.ValueString()),
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, d.client.ClientID, d.client.ClientSecret),
		},
	})

	env, err := tc.GetEnv(ctx, connect.NewRequest(&serverv1.GetEnvRequest{}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Chalk Environment",
			fmt.Sprintf("Could not read environment %s: %v", data.Name.ValueString(), err),
		)
		return
	}

	// Placeholder data for now
	data.Id = types.StringValue("env-" + data.Name.ValueString())
	data.OnlineStoreKind = types.StringValue(*env.Msg.Environment.OnlineStoreKind)
	data.CloudAccountLocator = types.StringValue(env.Msg.Environment.GetCloudAccountLocator())

	tflog.Trace(ctx, "read chalk_environment data source complete", map[string]interface{}{
		"id": data.Id.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
