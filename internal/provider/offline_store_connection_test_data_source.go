package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &OfflineStoreConnectionTestDataSource{}

func NewOfflineStoreConnectionTestDataSource() datasource.DataSource {
	return &OfflineStoreConnectionTestDataSource{}
}

type OfflineStoreConnectionTestDataSource struct {
	client *ClientManager
}

type OfflineStoreConnectionTestDataSourceModel struct {
	EnvironmentId            types.String `tfsdk:"environment_id"`
	OfflineStoreConnectionId types.String `tfsdk:"offline_store_connection_id"`
	Success                  types.Bool   `tfsdk:"success"`
	ErrorMessage             types.String `tfsdk:"error_message"`
}

func (d *OfflineStoreConnectionTestDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_offline_store_connection_test"
}

func (d *OfflineStoreConnectionTestDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Tests an offline store connection by calling the Chalk API. Intended for use in lifecycle preconditions on `chalk_environment_offline_store_connection_binding`.",
		Attributes: map[string]schema.Attribute{
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the environment the connection is scoped to.",
				Required:            true,
			},
			"offline_store_connection_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the offline store connection to test.",
				Required:            true,
			},
			"success": schema.BoolAttribute{
				MarkdownDescription: "Whether the connection test passed.",
				Computed:            true,
			},
			"error_message": schema.StringAttribute{
				MarkdownDescription: "Error message if the connection test failed, empty otherwise.",
				Computed:            true,
			},
		},
	}
}

func (d *OfflineStoreConnectionTestDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*ClientManager)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ClientManager, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *OfflineStoreConnectionTestDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OfflineStoreConnectionTestDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	osc := d.client.NewOfflineStoreConnectionClient(ctx, data.EnvironmentId.ValueString())
	testResp, err := osc.TestOfflineStoreConnection(ctx, connect.NewRequest(&serverv1.TestOfflineStoreConnectionRequest{
		Connection: &serverv1.TestOfflineStoreConnectionRequest_Id{Id: data.OfflineStoreConnectionId.ValueString()},
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error testing offline store connection",
			fmt.Sprintf("Could not test offline store connection: %s", err.Error()),
		)
		return
	}

	data.Success = types.BoolValue(testResp.Msg.Success)
	if testResp.Msg.Success {
		data.ErrorMessage = types.StringValue("")
	} else {
		msg := testResp.Msg.Error
		if msg == "" {
			msg = testResp.Msg.Message
		}
		data.ErrorMessage = types.StringValue(msg)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
