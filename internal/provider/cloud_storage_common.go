package provider

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	azureABFSStorageURIRegex      = regexp.MustCompile(`^abfss?://(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?@[a-z0-9]+\.dfs\.core\.windows\.net(?:/.*)?|[a-z0-9]+\.blob\.core\.windows\.net/[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:/.*)?)$`)
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
		return azureBlobHTTPSStorageURIRegex.MatchString(trimmed) ||
				azureABFSStorageURIRegex.MatchString(trimmed),
			"abs storage uri must look like https://<account>.blob.core.windows.net/<container>[/path], abfs://<account>.blob.core.windows.net/<container>[/path], or abfss://<container>@<account>.dfs.core.windows.net[/path]"
	case cloudStorageKindMock:
		return mockStorageURIRegex.MatchString(trimmed), "mock storage uri must look like mock://bucket[/path]"
	default:
		return false, fmt.Sprintf("unsupported storage kind %q (expected one of gcs, s3, abs, mock)", kind)
	}
}

// cloudStorageResourceModel is shared by the managed and unmanaged storage
// resources. The two differ only in how `uri` is surfaced in the schema (Required
// for unmanaged, Computed for managed); the underlying model and CRUD handling are
// identical.
type cloudStorageResourceModel struct {
	Id                types.String `tfsdk:"id"`
	Kind              types.String `tfsdk:"kind"`
	Uri               types.String `tfsdk:"uri"`
	CloudCredentialId types.String `tfsdk:"cloud_credential_id"`
	Managed           types.Bool   `tfsdk:"managed"`
	Name              types.String `tfsdk:"name"`
	Designator        types.String `tfsdk:"designator"`
	TeamId            types.String `tfsdk:"team_id"`
	AppliedAt         types.String `tfsdk:"applied_at"`
	CreatedAt         types.String `tfsdk:"created_at"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
}

// cloudStorageSchema builds the schema shared by the managed and unmanaged storage
// resources. When managed is true, `uri` is Computed (Chalk owns the bucket);
// otherwise it is a Required, replace-only input.
func cloudStorageSchema(managed bool) schema.Schema {
	var markdown string
	var uriAttr schema.StringAttribute
	if managed {
		markdown = "Registers a Chalk-managed cloud storage: Chalk owns the bucket and derives its `uri`, so you only supply the cloud credential.\n\n" +
			"Every attribute is replace-only (there is no update RPC). **Create-time bucket access check:** creating this resource performs a live access check using the referenced `cloud_credential_id`; apply fails unless that credential can reach the storage, so the credential must exist first."
		uriAttr = schema.StringAttribute{
			MarkdownDescription: "URI of the managed bucket. Derived and set by Chalk.",
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		}
	} else {
		markdown = "Registers a reference to an existing (unmanaged) cloud storage bucket plus the cloud credential used to reach it. Chalk does not provision the bucket.\n\n" +
			"Every attribute is replace-only (there is no update RPC). **Create-time bucket access check:** creating this resource performs a live `Head` against the bucket using the referenced `cloud_credential_id`; apply fails unless that credential can already reach the bucket, so the credential (and the bucket's real IAM grants) must exist first."
		uriAttr = schema.StringAttribute{
			MarkdownDescription: "URI of the existing bucket (and optional path prefix), e.g. `s3://bucket/prefix`, `gs://bucket/prefix`, " +
				"`https://<account>.blob.core.windows.net/<container>[/path]`, `abfs://<account>.blob.core.windows.net/<container>[/path]`, " +
				"`abfss://<container>@<account>.dfs.core.windows.net[/path]`, or `mock://bucket`. " +
				"When `kind` is set, the scheme must match it. Changing this forces a new resource.",
			Required: true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		}
	}

	return schema.Schema{
		MarkdownDescription: markdown,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud storage identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Cloud storage kind. One of `gcs`, `s3`, `abs` (Azure Blob Storage), or `mock`. " +
					"Optional: when omitted, Chalk infers it from the cloud credential. Changing this forces a new resource.",
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(cloudStorageKindGCS, cloudStorageKindS3, cloudStorageKindAzure, cloudStorageKindMock),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"uri": uriAttr,
			"cloud_credential_id": schema.StringAttribute{
				MarkdownDescription: "ID of the cloud credential (e.g. a `chalk_aws_cloud_credentials`/`chalk_gcp_cloud_credentials`/`chalk_azure_cloud_credentials` resource) used to access the bucket. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"managed": schema.BoolAttribute{
				MarkdownDescription: "Whether the storage is managed by Chalk. Determined by the resource type.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Cloud storage name. Set by the server to the storage `uri`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"designator": schema.StringAttribute{
				MarkdownDescription: "Server-assigned designator. Only populated for managed storages.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"team_id": schema.StringAttribute{
				MarkdownDescription: "ID of the team that owns the storage.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"applied_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp at which the storage was last applied, if any.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp at which the storage was created.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp at which the storage was last updated.",
				Computed:            true,
			},
		},
	}
}

// setCloudStorageState populates the model from a storage response. Required,
// replace-only inputs (kind, cloud_credential_id) are only overwritten when the
// server echoes a value, so a response that omits one cannot trigger a spurious
// replace.
func setCloudStorageState(data *cloudStorageResourceModel, storage *serverv1.CloudComponentStorageResponse) {
	if storage == nil {
		return
	}
	data.Id = types.StringValue(storage.GetId())
	data.Name = types.StringValue(storage.GetName())
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

// describeCloudStorageCreateError maps well-known create-time failure codes to
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

// timestampToStringValue renders a protobuf timestamp as an RFC3339 string, or a
// null string when the timestamp is unset.
func timestampToStringValue(ts *timestamppb.Timestamp) types.String {
	if ts == nil || !ts.IsValid() || (ts.GetSeconds() == 0 && ts.GetNanos() == 0) {
		return types.StringNull()
	}
	return types.StringValue(ts.AsTime().Format(time.RFC3339))
}

// configureCloudManager extracts the *client.Manager from a Configure request,
// recording a diagnostic on type mismatch. Shared by the cloud storage and binding
// resources.
func configureCloudManager(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *client.Manager {
	if req.ProviderData == nil {
		return nil
	}
	manager, ok := req.ProviderData.(*client.Manager)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Manager, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return nil
	}
	return manager
}
