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

var _ datasource.DataSource = &OfflineStoreConnectionDataSource{}

func NewOfflineStoreConnectionDataSource() datasource.DataSource {
	return &OfflineStoreConnectionDataSource{}
}

type OfflineStoreConnectionDataSource struct {
	client *client.Manager
}

// OfflineStoreConnectionDataSourceModel is the secret-free read view of an
// offline store connection. Unlike the resource, snowflake credentials here
// expose only identity (account, username, warehouse, …) — never password or
// private_key.
type OfflineStoreConnectionDataSourceModel struct {
	Id            types.String                 `tfsdk:"id"`
	EnvironmentId types.String                 `tfsdk:"environment_id"`
	Name          types.String                 `tfsdk:"name"`
	Kind          types.String                 `tfsdk:"kind"`
	Snowflake     *OSCSnowflakeDataSourceModel `tfsdk:"snowflake"`
	BigQuery      *OSCBigQueryDataSourceModel  `tfsdk:"bigquery"`
	Iceberg       *OSCIcebergDataSourceModel   `tfsdk:"iceberg"`
}

type OSCSnowflakeDataSourceModel struct {
	Account                types.String `tfsdk:"account"`
	Username               types.String `tfsdk:"username"`
	Warehouse              types.String `tfsdk:"warehouse"`
	Database               types.String `tfsdk:"database"`
	Schema                 types.String `tfsdk:"schema"`
	Role                   types.String `tfsdk:"role"`
	StorageIntegrationName types.String `tfsdk:"storage_integration_name"`
}

type OSCBigQueryDataSourceModel struct {
	ProjectId types.String `tfsdk:"project_id"`
	DatasetId types.String `tfsdk:"dataset_id"`
}

type OSCIcebergDataSourceModel struct {
	GlueS3 *OSCIcebergGlueS3DataSourceModel `tfsdk:"glue_s3"`
}

type OSCIcebergGlueS3DataSourceModel struct {
	S3Bucket         types.String `tfsdk:"s3_bucket"`
	GlueDatabaseName types.String `tfsdk:"glue_database_name"`
}

func (d *OfflineStoreConnectionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_offline_store_connection"
}

func (d *OfflineStoreConnectionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Chalk offline store connection by ID. **Does not** surface snowflake password / private_key (Sensitive on the resource).",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{MarkdownDescription: "Offline store connection identifier.", Required: true},
			"environment_id": schema.StringAttribute{MarkdownDescription: "Environment ID the connection is scoped to.", Required: true},
			"name":           schema.StringAttribute{Computed: true},
			"kind":           schema.StringAttribute{MarkdownDescription: "Underlying backend: `snowflake`, `bigquery`, or `iceberg`.", Computed: true},
			"snowflake": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"account":                  schema.StringAttribute{Computed: true},
					"username":                 schema.StringAttribute{Computed: true},
					"warehouse":                schema.StringAttribute{Computed: true},
					"database":                 schema.StringAttribute{Computed: true},
					"schema":                   schema.StringAttribute{Computed: true},
					"role":                     schema.StringAttribute{Computed: true},
					"storage_integration_name": schema.StringAttribute{Computed: true},
				},
			},
			"bigquery": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"project_id": schema.StringAttribute{Computed: true},
					"dataset_id": schema.StringAttribute{Computed: true},
				},
			},
			"iceberg": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"glue_s3": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"s3_bucket":          schema.StringAttribute{Computed: true},
							"glue_database_name": schema.StringAttribute{Computed: true},
						},
					},
				},
			},
		},
	}
}

func (d *OfflineStoreConnectionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *OfflineStoreConnectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OfflineStoreConnectionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "read chalk_offline_store_connection data source", map[string]any{"id": data.Id.ValueString()})

	osc := d.client.NewOfflineStoreConnectionClient(ctx, data.EnvironmentId.ValueString())
	connResp, err := osc.GetOfflineStoreConnection(ctx, connect.NewRequest(&serverv1.GetOfflineStoreConnectionRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Chalk Offline Store Connection",
			fmt.Sprintf("Could not read offline store connection %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	conn := connResp.Msg.Connection
	if conn == nil {
		resp.Diagnostics.AddError("Empty Connection Response", fmt.Sprintf("Server returned no connection for %s", data.Id.ValueString()))
		return
	}

	data.Name = types.StringValue(conn.Name)
	data.EnvironmentId = types.StringValue(conn.EnvironmentId)

	if conn.Config == nil {
		data.Kind = types.StringNull()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	switch cfg := conn.Config.Config.(type) {
	case *serverv1.OfflineStoreConnectionConfigStored_Snowflake:
		data.Kind = types.StringValue("snowflake")
		if cfg.Snowflake != nil && cfg.Snowflake.Credentials != nil {
			creds := cfg.Snowflake.Credentials
			data.Snowflake = &OSCSnowflakeDataSourceModel{
				Account:                types.StringValue(creds.Account),
				Username:               types.StringValue(creds.Username),
				Warehouse:              types.StringPointerValue(creds.Warehouse),
				Database:               types.StringPointerValue(creds.Database),
				Schema:                 types.StringPointerValue(creds.Schema),
				Role:                   types.StringPointerValue(creds.Role),
				StorageIntegrationName: stringOrNull(cfg.Snowflake.GetStorageIntegration().GetIntegrationName()),
			}
		}
	case *serverv1.OfflineStoreConnectionConfigStored_Bigquery:
		data.Kind = types.StringValue("bigquery")
		if cfg.Bigquery != nil {
			data.BigQuery = &OSCBigQueryDataSourceModel{
				ProjectId: types.StringValue(cfg.Bigquery.ProjectId),
				DatasetId: types.StringValue(cfg.Bigquery.DatasetId),
			}
		}
	case *serverv1.OfflineStoreConnectionConfigStored_Iceberg:
		data.Kind = types.StringValue("iceberg")
		if cfg.Iceberg != nil && cfg.Iceberg.GetGlueS3() != nil {
			data.Iceberg = &OSCIcebergDataSourceModel{
				GlueS3: &OSCIcebergGlueS3DataSourceModel{
					S3Bucket:         types.StringValue(cfg.Iceberg.GetGlueS3().S3Bucket),
					GlueDatabaseName: types.StringValue(cfg.Iceberg.GetGlueS3().GlueDatabaseName),
				},
			}
		}
	default:
		data.Kind = types.StringValue("unknown")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
