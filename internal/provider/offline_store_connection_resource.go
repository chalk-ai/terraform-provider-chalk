package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/chalk-ai/terraform-provider-chalk/client"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

var _ resource.Resource = &OfflineStoreConnectionResource{}
var _ resource.ResourceWithImportState = &OfflineStoreConnectionResource{}
var _ resource.ResourceWithConfigValidators = &OfflineStoreConnectionResource{}
var _ resource.ResourceWithModifyPlan = &OfflineStoreConnectionResource{}

func NewOfflineStoreConnectionResource() resource.Resource {
	return &OfflineStoreConnectionResource{}
}

type OfflineStoreConnectionResource struct {
	client *client.Manager
}

type OfflineStoreConnectionResourceModel struct {
	Id            types.String              `tfsdk:"id"`
	EnvironmentId types.String              `tfsdk:"environment_id"`
	Name          types.String              `tfsdk:"name"`
	Snowflake     *SnowflakeConnectionModel `tfsdk:"snowflake"`
	BigQuery      *BigQueryConnectionModel  `tfsdk:"bigquery"`
	Iceberg       *IcebergConnectionModel   `tfsdk:"iceberg"`
}

type SnowflakeConnectionModel struct {
	Credentials            SnowflakeCredentialsModel `tfsdk:"credentials"`
	StorageIntegrationName types.String              `tfsdk:"storage_integration_name"`
}

type SnowflakeCredentialsModel struct {
	Account    types.String `tfsdk:"account"`
	Username   types.String `tfsdk:"username"`
	Password   types.String `tfsdk:"password"`
	PrivateKey types.String `tfsdk:"private_key"`
	Warehouse  types.String `tfsdk:"warehouse"`
	Database   types.String `tfsdk:"database"`
	Schema     types.String `tfsdk:"schema"`
	Role       types.String `tfsdk:"role"`
}

type BigQueryConnectionModel struct {
	ProjectId types.String `tfsdk:"project_id"`
	DatasetId types.String `tfsdk:"dataset_id"`
}

type IcebergConnectionModel struct {
	GlueS3 *IcebergGlueS3Model `tfsdk:"glue_s3"`
}

type IcebergGlueS3Model struct {
	S3Bucket         types.String `tfsdk:"s3_bucket"`
	GlueDatabaseName types.String `tfsdk:"glue_database_name"`
}

func (r *OfflineStoreConnectionResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("snowflake"),
			path.MatchRoot("bigquery"),
			path.MatchRoot("iceberg"),
		),
	}
}

func (r *OfflineStoreConnectionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_offline_store_connection"
}

func (r *OfflineStoreConnectionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Chalk offline store connection (Snowflake, BigQuery, or Iceberg).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the offline store connection.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The environment ID this connection is scoped to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the offline store connection.",
				Required:            true,
			},
			"snowflake": schema.SingleNestedAttribute{
				MarkdownDescription: "Snowflake offline store connection configuration.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"storage_integration_name": schema.StringAttribute{
						MarkdownDescription: "Name of the Snowflake storage integration to use for bulk data operations (e.g. `MY_SNOWFLAKE_INTEGRATION`). Optional but recommended for large datasets.",
						Optional:            true,
					},
					"credentials": schema.SingleNestedAttribute{
						MarkdownDescription: "Snowflake credentials.",
						Required:            true,
						Attributes: map[string]schema.Attribute{
							"account": schema.StringAttribute{
								MarkdownDescription: "Snowflake account identifier.",
								Required:            true,
							},
							"username": schema.StringAttribute{
								MarkdownDescription: "Snowflake username.",
								Required:            true,
							},
							"password": schema.StringAttribute{
								MarkdownDescription: "Snowflake password. Exactly one of password or private_key must be provided. " +
									"Stored as sensitive in state. After importing this resource, this field will be null; run terraform apply to restore it.",
								Optional:  true,
								Sensitive: true,
								Validators: []validator.String{
									stringvalidator.ExactlyOneOf(
										path.MatchRelative().AtParent().AtName("private_key"),
									),
								},
							},
							"private_key": schema.StringAttribute{
								MarkdownDescription: "Snowflake private key. Exactly one of password or private_key must be provided. " +
									"Stored as sensitive in state. After importing this resource, this field will be null; run terraform apply to restore it.",
								Optional:  true,
								Sensitive: true,
								Validators: []validator.String{
									stringvalidator.ExactlyOneOf(
										path.MatchRelative().AtParent().AtName("password"),
									),
								},
							},
							"warehouse": schema.StringAttribute{
								MarkdownDescription: "Snowflake warehouse.",
								Required:            true,
							},
							"database": schema.StringAttribute{
								MarkdownDescription: "Snowflake database.",
								Required:            true,
							},
							"schema": schema.StringAttribute{
								MarkdownDescription: "Snowflake schema.",
								Required:            true,
							},
							"role": schema.StringAttribute{
								MarkdownDescription: "Snowflake role.",
								Required:            true,
							},
						},
					},
				},
			},
			"bigquery": schema.SingleNestedAttribute{
				MarkdownDescription: "BigQuery offline store connection configuration.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"project_id": schema.StringAttribute{
						MarkdownDescription: "BigQuery project ID.",
						Required:            true,
					},
					"dataset_id": schema.StringAttribute{
						MarkdownDescription: "BigQuery dataset ID.",
						Required:            true,
					},
				},
			},
			"iceberg": schema.SingleNestedAttribute{
				MarkdownDescription: "Iceberg offline store connection configuration using AWS Glue catalog and S3 storage.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"glue_s3": schema.SingleNestedAttribute{
						MarkdownDescription: "Iceberg configuration with AWS Glue catalog and S3 storage.",
						Required:            true,
						Attributes: map[string]schema.Attribute{
							"s3_bucket": schema.StringAttribute{
								MarkdownDescription: "Name of the S3 bucket where Iceberg data files will be stored.",
								Required:            true,
							},
							"glue_database_name": schema.StringAttribute{
								MarkdownDescription: "Name of the AWS Glue database to use as the Iceberg catalog.",
								Required:            true,
							},
						},
					},
				},
			},
		},
	}
}

func (r *OfflineStoreConnectionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Manager)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Manager, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *OfflineStoreConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OfflineStoreConnectionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	osc := r.client.NewOfflineStoreConnectionClient(ctx, data.EnvironmentId.ValueString())

	config, err := modelToConfigInput(&data)
	if err != nil {
		resp.Diagnostics.AddError("Error building offline store connection config", err.Error())
		return
	}

	createResp, err := osc.CreateOfflineStoreConnection(ctx, connect.NewRequest(&serverv1.CreateOfflineStoreConnectionRequest{
		Connection: &serverv1.OfflineStoreConnectionInput{
			Name:   data.Name.ValueString(),
			Config: config,
		},
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Offline Store Connection",
			fmt.Sprintf("Could not create offline store connection: %v", err),
		)
		return
	}

	if createResp.Msg.Connection == nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Offline Store Connection",
			"Server returned an empty connection response.",
		)
		return
	}

	data.Id = types.StringValue(createResp.Msg.Connection.Id)
	tflog.Trace(ctx, "created a chalk_offline_store_connection resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OfflineStoreConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OfflineStoreConnectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	osc := r.client.NewOfflineStoreConnectionClient(ctx, data.EnvironmentId.ValueString())

	getResp, err := osc.GetOfflineStoreConnection(ctx, connect.NewRequest(&serverv1.GetOfflineStoreConnectionRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Chalk Offline Store Connection",
			fmt.Sprintf("Could not read offline store connection %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	conn := getResp.Msg.Connection
	if conn == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Id = types.StringValue(conn.Id)
	data.Name = types.StringValue(conn.Name)
	data.EnvironmentId = types.StringValue(conn.EnvironmentId)

	if conn.Config != nil {
		switch cfg := conn.Config.Config.(type) {
		case *serverv1.OfflineStoreConnectionConfigStored_Snowflake:
			if cfg.Snowflake != nil && cfg.Snowflake.Credentials != nil {
				creds := cfg.Snowflake.Credentials
				var existing SnowflakeCredentialsModel
				if data.Snowflake != nil {
					existing = data.Snowflake.Credentials
				}
				data.Snowflake = &SnowflakeConnectionModel{
					Credentials: SnowflakeCredentialsModel{
						Account:   types.StringValue(creds.Account),
						Username:  types.StringValue(creds.Username),
						Warehouse: types.StringValue(derefString(creds.Warehouse)),
						Database:  types.StringValue(derefString(creds.Database)),
						Schema:    types.StringValue(derefString(creds.Schema)),
						Role:      types.StringValue(derefString(creds.Role)),
						// Preserve sensitive fields from state since server returns only secret IDs
						Password:   existing.Password,
						PrivateKey: existing.PrivateKey,
					},
					StorageIntegrationName: optionalStringValue(cfg.Snowflake.GetStorageIntegration().GetIntegrationName()),
				}
				data.BigQuery = nil
				data.Iceberg = nil
			}
		case *serverv1.OfflineStoreConnectionConfigStored_Bigquery:
			if cfg.Bigquery != nil {
				data.BigQuery = &BigQueryConnectionModel{
					ProjectId: types.StringValue(cfg.Bigquery.ProjectId),
					DatasetId: types.StringValue(cfg.Bigquery.DatasetId),
				}
				data.Snowflake = nil
				data.Iceberg = nil
			}
		case *serverv1.OfflineStoreConnectionConfigStored_Iceberg:
			if cfg.Iceberg != nil && cfg.Iceberg.GetGlueS3() != nil {
				data.Iceberg = &IcebergConnectionModel{
					GlueS3: &IcebergGlueS3Model{
						S3Bucket:         types.StringValue(cfg.Iceberg.GetGlueS3().S3Bucket),
						GlueDatabaseName: types.StringValue(cfg.Iceberg.GetGlueS3().GlueDatabaseName),
					},
				}
				data.Snowflake = nil
				data.BigQuery = nil
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OfflineStoreConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OfflineStoreConnectionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get id from state since it's computed
	var state OfflineStoreConnectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Id = state.Id

	osc := r.client.NewOfflineStoreConnectionClient(ctx, data.EnvironmentId.ValueString())

	config, err := modelToConfigInput(&data)
	if err != nil {
		resp.Diagnostics.AddError("Error building offline store connection config", err.Error())
		return
	}

	_, err = osc.UpdateOfflineStoreConnection(ctx, connect.NewRequest(&serverv1.UpdateOfflineStoreConnectionRequest{
		Id: data.Id.ValueString(),
		Connection: &serverv1.OfflineStoreConnectionInput{
			Name:   data.Name.ValueString(),
			Config: config,
		},
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"name", "config"},
		},
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Chalk Offline Store Connection",
			fmt.Sprintf("Could not update offline store connection %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "updated a chalk_offline_store_connection resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OfflineStoreConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OfflineStoreConnectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	osc := r.client.NewOfflineStoreConnectionClient(ctx, data.EnvironmentId.ValueString())

	_, err := osc.DeleteOfflineStoreConnection(ctx, connect.NewRequest(&serverv1.DeleteOfflineStoreConnectionRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Chalk Offline Store Connection",
			fmt.Sprintf("Could not delete offline store connection %s: %v", data.Id.ValueString(), err),
		)
	}
}

func (r *OfflineStoreConnectionResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Only run during updates (state and plan are both non-null).
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var state, plan OfflineStoreConnectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If the connection type changes, require replacement.
	// The backend does not support in-place type changes.
	stateType := connTypeRoot(&state)
	planType := connTypeRoot(&plan)
	if stateType != planType && planType != "" {
		resp.RequiresReplace = append(resp.RequiresReplace, path.Root(planType))
	}
}

func (r *OfflineStoreConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import ID in the format 'environment_id/connection_id', got: %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// connTypeRoot returns the root attribute name of the active connection type in the model.
func connTypeRoot(m *OfflineStoreConnectionResourceModel) string {
	switch {
	case m.Snowflake != nil:
		return "snowflake"
	case m.BigQuery != nil:
		return "bigquery"
	case m.Iceberg != nil:
		return "iceberg"
	default:
		return ""
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// modelToConfigInput converts the TF model to a proto config input.
func modelToConfigInput(data *OfflineStoreConnectionResourceModel) (*serverv1.OfflineStoreConnectionConfigInput, error) {
	if data.Snowflake != nil {
		creds := &serverv1.SnowflakeCredentialsInput{
			Account:  data.Snowflake.Credentials.Account.ValueString(),
			Username: data.Snowflake.Credentials.Username.ValueString(),
		}
		if !data.Snowflake.Credentials.Password.IsNull() && !data.Snowflake.Credentials.Password.IsUnknown() {
			v := data.Snowflake.Credentials.Password.ValueString()
			creds.Password = &v
		}
		if !data.Snowflake.Credentials.PrivateKey.IsNull() && !data.Snowflake.Credentials.PrivateKey.IsUnknown() {
			v := data.Snowflake.Credentials.PrivateKey.ValueString()
			creds.PrivateKey = &v
		}
		if !data.Snowflake.Credentials.Warehouse.IsNull() && !data.Snowflake.Credentials.Warehouse.IsUnknown() {
			v := data.Snowflake.Credentials.Warehouse.ValueString()
			creds.Warehouse = &v
		}
		if !data.Snowflake.Credentials.Database.IsNull() && !data.Snowflake.Credentials.Database.IsUnknown() {
			v := data.Snowflake.Credentials.Database.ValueString()
			creds.Database = &v
		}
		if !data.Snowflake.Credentials.Schema.IsNull() && !data.Snowflake.Credentials.Schema.IsUnknown() {
			v := data.Snowflake.Credentials.Schema.ValueString()
			creds.Schema = &v
		}
		if !data.Snowflake.Credentials.Role.IsNull() && !data.Snowflake.Credentials.Role.IsUnknown() {
			v := data.Snowflake.Credentials.Role.ValueString()
			creds.Role = &v
		}
		sfInput := &serverv1.SnowflakeOfflineStoreConnectionConfigInput{Credentials: creds}
		if v := data.Snowflake.StorageIntegrationName.ValueString(); v != "" {
			sfInput.StorageIntegration = &serverv1.SnowflakeStorageIntegration{IntegrationName: v}
		}
		return &serverv1.OfflineStoreConnectionConfigInput{
			Config: &serverv1.OfflineStoreConnectionConfigInput_Snowflake{
				Snowflake: sfInput,
			},
		}, nil
	}

	if data.Iceberg != nil && data.Iceberg.GlueS3 != nil {
		return &serverv1.OfflineStoreConnectionConfigInput{
			Config: &serverv1.OfflineStoreConnectionConfigInput_Iceberg{
				Iceberg: &serverv1.IcebergOfflineStoreConnectionConfig{
					Catalog: &serverv1.IcebergOfflineStoreConnectionConfig_GlueS3{
						GlueS3: &serverv1.IcebergGlueS3CatalogConfig{
							S3Bucket:         data.Iceberg.GlueS3.S3Bucket.ValueString(),
							GlueDatabaseName: data.Iceberg.GlueS3.GlueDatabaseName.ValueString(),
						},
					},
				},
			},
		}, nil
	}

	// data.BigQuery != nil is guaranteed by ConfigValidators (ExactlyOneOf snowflake/bigquery/iceberg)
	return &serverv1.OfflineStoreConnectionConfigInput{
		Config: &serverv1.OfflineStoreConnectionConfigInput_Bigquery{
			Bigquery: &serverv1.BigQueryOfflineStoreConnectionConfig{
				ProjectId: data.BigQuery.ProjectId.ValueString(),
				DatasetId: data.BigQuery.DatasetId.ValueString(),
			},
		},
	}, nil
}
