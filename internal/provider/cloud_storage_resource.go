package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	_ resource.Resource                   = &CloudStorageResource{}
	_ resource.ResourceWithImportState    = &CloudStorageResource{}
	_ resource.ResourceWithValidateConfig = &CloudStorageResource{}
)

// Cloud storage kinds. A storage's kind selects the cloud provider, which in turn
// determines both the URI scheme and the credential type, so the two must agree.
const (
	cloudStorageKindGCS   = "gcs"
	cloudStorageKindS3    = "s3"
	cloudStorageKindAzure = "abs"
	cloudStorageKindMock  = "mock"
)

// Storage URI patterns, kept in lockstep with the server-side validation in
// go-api-server/cloudcomponents/cloud_storage.go so plan-time validation never
// disagrees with what the server will accept at apply time.
var (
	gcsStorageURIRegex            = regexp.MustCompile(`^gs://[a-z0-9][a-z0-9._-]*(?:/.*)?$`)
	s3StorageURIRegex             = regexp.MustCompile(`^s3://[a-z0-9][a-z0-9.-]*(?:/.*)?$`)
	azureBlobHTTPSStorageURIRegex = regexp.MustCompile(`^https://[a-z0-9]+\.blob\.core\.windows\.net/[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:/.*)?$`)
	azureABFSStorageURIRegex      = regexp.MustCompile(`^abfss?://[a-z0-9](?:[a-z0-9-]*[a-z0-9])?@[a-z0-9]+\.dfs\.core\.windows\.net(?:/.*)?$`)
	mockStorageURIRegex           = regexp.MustCompile(`^mock://[a-z0-9][a-z0-9._-]*(?:/.*)?$`)
)

// validateStorageURIForKind enforces that the URI scheme matches the declared kind,
// returning a human-readable reason on mismatch. It mirrors the server-side rules.
func validateStorageURIForKind(kind, uri string) (ok bool, reason string) {
	trimmed := strings.TrimSpace(uri)
	switch kind {
	case cloudStorageKindGCS:
		return gcsStorageURIRegex.MatchString(trimmed), "gcs storage uri must look like gs://bucket[/path]"
	case cloudStorageKindS3:
		return s3StorageURIRegex.MatchString(trimmed), "s3 storage uri must look like s3://bucket[/path]"
	case cloudStorageKindAzure:
		return azureBlobHTTPSStorageURIRegex.MatchString(trimmed) || azureABFSStorageURIRegex.MatchString(trimmed),
			"abs storage uri must look like https://<account>.blob.core.windows.net/<container>[/path] or abfss://<container>@<account>.dfs.core.windows.net[/path]"
	case cloudStorageKindMock:
		return mockStorageURIRegex.MatchString(trimmed), "mock storage uri must look like mock://bucket[/path]"
	default:
		return false, fmt.Sprintf("unsupported storage kind %q (expected one of gcs, s3, abs, mock)", kind)
	}
}

func NewCloudStorageResource() resource.Resource {
	return &CloudStorageResource{}
}

type CloudStorageResource struct {
	client *client.Manager
}

type CloudStorageResourceModel struct {
	Id                types.String `tfsdk:"id"`
	Kind              types.String `tfsdk:"kind"`
	Uri               types.String `tfsdk:"uri"`
	CloudCredentialId types.String `tfsdk:"cloud_credential_id"`
	Managed           types.Bool   `tfsdk:"managed"`

	// Computed
	Name       types.String `tfsdk:"name"`
	Designator types.String `tfsdk:"designator"`
	TeamId     types.String `tfsdk:"team_id"`
	AppliedAt  types.String `tfsdk:"applied_at"`
	CreatedAt  types.String `tfsdk:"created_at"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
}

func (r *CloudStorageResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_storage"
}

func (r *CloudStorageResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Registers a reference to an existing cloud storage bucket (plus the cloud credential used to reach it) with Chalk.\n\n" +
			"A `chalk_cloud_storage` is a *reference* to a bucket, not a bucket Chalk provisions. " +
			"Every attribute is replace-only: there is no update RPC, and the server only validates the URI/kind pairing and bucket access at create time, so any change forces the storage to be recreated.\n\n" +
			"**Create-time bucket access check:** creating this resource performs a live `Head` against the bucket using the referenced `cloud_credential_id`. " +
			"Apply fails with a permission error unless that credential can already reach the bucket, so the referenced `chalk_*_cloud_credentials` resource (and the bucket's real IAM grants) must exist first.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud storage identifier.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Cloud storage kind. One of `gcs`, `s3`, `abs` (Azure Blob Storage), or `mock`. Determines both the required `uri` scheme and the credential type. Changing this forces a new resource.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(cloudStorageKindGCS, cloudStorageKindS3, cloudStorageKindAzure, cloudStorageKindMock),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"uri": schema.StringAttribute{
				MarkdownDescription: "URI of the existing bucket (and optional path prefix). Must match `kind`: `gs://bucket[/path]` for gcs, `s3://bucket[/path]` for s3, " +
					"`https://<account>.blob.core.windows.net/<container>[/path]` or `abfss://<container>@<account>.dfs.core.windows.net[/path]` for abs, and `mock://bucket[/path]` for mock. Changing this forces a new resource.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cloud_credential_id": schema.StringAttribute{
				MarkdownDescription: "ID of the cloud credential (e.g. a `chalk_aws_cloud_credentials`/`chalk_gcp_cloud_credentials`/`chalk_azure_cloud_credentials` resource) used to access the bucket. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"managed": schema.BoolAttribute{
				MarkdownDescription: "Whether the storage is managed by Chalk. Defaults to `false`. Changing this forces a new resource.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Cloud storage name. Set by the server to the storage `uri`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"designator": schema.StringAttribute{
				MarkdownDescription: "Server-assigned designator. Only populated for managed storages.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"team_id": schema.StringAttribute{
				MarkdownDescription: "ID of the team that owns the storage.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"applied_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp at which the storage was last applied, if any.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp at which the storage was created.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp at which the storage was last updated.",
				Computed:            true,
			},
		},
	}
}

// ValidateConfig enforces the URI-vs-kind pairing at plan time, mirroring the
// server-side check so misconfigurations surface before apply rather than as a
// create-time InvalidArgument.
func (r *CloudStorageResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data CloudStorageResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Skip when either value is unknown/null (e.g. sourced from another resource);
	// the pairing is re-checked at apply and server-side regardless.
	if data.Kind.IsNull() || data.Kind.IsUnknown() || data.Uri.IsNull() || data.Uri.IsUnknown() {
		return
	}

	if ok, reason := validateStorageURIForKind(data.Kind.ValueString(), data.Uri.ValueString()); !ok {
		resp.Diagnostics.AddAttributeError(
			path.Root("uri"),
			"Invalid storage URI for kind",
			fmt.Sprintf("%s, got %q", reason, data.Uri.ValueString()),
		)
	}
}

func (r *CloudStorageResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CloudStorageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CloudStorageResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	credId := data.CloudCredentialId.ValueString()
	createRequest := &serverv1.CreateCloudComponentStorageRequest{
		Storage: &serverv1.CloudComponentStorageRequest{
			Kind: data.Kind.ValueString(),
			Spec: &serverv1.CloudComponentStorage{
				Uri: data.Uri.ValueString(),
			},
			Managed:           data.Managed.ValueBool(),
			CloudCredentialId: &credId,
		},
	}

	response, err := cloudComponentsClient.CreateCloudComponentStorage(ctx, connect.NewRequest(createRequest))
	if err != nil {
		summary, detail := describeCloudStorageCreateError(err)
		resp.Diagnostics.AddError(summary, detail)
		return
	}

	// Guard against an empty success response: without a storage to read computed
	// fields from, the state would keep unknown values and Terraform would abort
	// with an opaque "inconsistent result after apply" error. Surface it clearly.
	if response.Msg.GetStorage() == nil {
		resp.Diagnostics.AddError(
			"Empty create response",
			"The server returned no storage in the create response. This is unexpected; please report it to the provider developers.",
		)
		return
	}

	setCloudStorageState(&data, response.Msg.GetStorage())

	tflog.Trace(ctx, "created a chalk_cloud_storage resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudStorageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CloudStorageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	response, err := cloudComponentsClient.GetCloudComponentStorage(ctx, connect.NewRequest(&serverv1.GetCloudComponentStorageRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading cloud storage",
			fmt.Sprintf("Could not read cloud storage %s: %s", data.Id.ValueString(), err.Error()),
		)
		return
	}

	setCloudStorageState(&data, response.Msg.GetStorage())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudStorageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// No update RPC exists and every attribute is RequiresReplace, so this should
	// never be called. Guard against it explicitly.
	resp.Diagnostics.AddError(
		"Update not supported",
		"Cloud storages cannot be updated. They must be deleted and recreated.",
	)
}

func (r *CloudStorageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CloudStorageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentsClient := r.client.NewCloudComponentsClient(ctx)

	_, err := cloudComponentsClient.DeleteCloudComponentStorage(ctx, connect.NewRequest(&serverv1.DeleteCloudComponentStorageRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			// Already gone server-side; treat as a successful delete.
			return
		}
		resp.Diagnostics.AddError(
			"Error deleting cloud storage",
			fmt.Sprintf("Could not delete cloud storage %s: %s", data.Id.ValueString(), err.Error()),
		)
		return
	}

	tflog.Trace(ctx, "deleted a chalk_cloud_storage resource")
}

func (r *CloudStorageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// setCloudStorageState populates the model from a storage response. cloud_credential_id
// and kind are preserved from the plan/state (they are inputs), but we prefer the
// server's values when present to keep state authoritative.
func setCloudStorageState(data *CloudStorageResourceModel, storage *serverv1.CloudComponentStorageResponse) {
	if storage == nil {
		return
	}
	data.Id = types.StringValue(storage.GetId())
	data.Name = types.StringValue(storage.GetName())
	// kind is a RequiresReplace input; only overwrite it when the server echoes a
	// value, so a response that omits it can't trigger a spurious replace (mirrors
	// the uri/cloud_credential_id guards below).
	if storage.GetKind() != "" {
		data.Kind = types.StringValue(storage.GetKind())
	}
	data.Managed = types.BoolValue(storage.GetManaged())

	if spec := storage.GetSpec(); spec != nil {
		data.Uri = types.StringValue(spec.GetUri())
	}

	if storage.CloudCredentialId != nil {
		data.CloudCredentialId = types.StringValue(storage.GetCloudCredentialId())
	}

	if storage.Designator != nil {
		data.Designator = types.StringValue(storage.GetDesignator())
	} else {
		data.Designator = types.StringNull()
	}

	data.TeamId = types.StringValue(storage.GetTeamId())
	data.AppliedAt = timestampToStringValue(storage.GetAppliedAt())
	data.CreatedAt = timestampToStringValue(storage.GetCreatedAt())
	data.UpdatedAt = timestampToStringValue(storage.GetUpdatedAt())
}

// timestampToStringValue renders a protobuf timestamp as an RFC3339 string, or a
// null string when the timestamp is unset.
func timestampToStringValue(ts *timestamppb.Timestamp) types.String {
	if ts == nil || !ts.IsValid() || (ts.GetSeconds() == 0 && ts.GetNanos() == 0) {
		return types.StringNull()
	}
	return types.StringValue(ts.AsTime().Format(time.RFC3339))
}

// describeCloudStorageCreateError maps the well-known create-time failure codes to
// actionable diagnostics.
func describeCloudStorageCreateError(err error) (summary, detail string) {
	switch connect.CodeOf(err) {
	case connect.CodePermissionDenied:
		return "Cloud storage bucket is not reachable",
			fmt.Sprintf("Chalk performs a live access check against the bucket at create time using the referenced cloud_credential_id. "+
				"The credential could not reach the bucket: %s", err.Error())
	case connect.CodeInvalidArgument:
		return "Invalid cloud storage configuration",
			fmt.Sprintf("The server rejected the storage configuration (check that uri matches kind and cloud_credential_id is set): %s", err.Error())
	case connect.CodeAlreadyExists:
		return "Cloud storage already exists",
			fmt.Sprintf("A cloud storage already exists for this uri: %s", err.Error())
	default:
		return "Error creating cloud storage",
			fmt.Sprintf("Could not create cloud storage: %s", err.Error())
	}
}
