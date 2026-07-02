package provider

import (
	"fmt"
	"regexp"
	"strings"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Cloud container registry kinds. The kind selects the cloud provider (and thus
// the credential type and the registry-path format), so it is derived from which
// config block the user sets rather than being a separate, redundant input.
const (
	containerRegistryKindGAR = "gar" // Google Artifact Registry
	containerRegistryKindECR = "ecr" // AWS Elastic Container Registry
	containerRegistryKindACR = "acr" // Azure Container Registry
)

// Registry-path patterns, kept in lockstep with the server-side validation in
// go-api-server/builder/container_registry_paths.go so plan-time validation never
// disagrees with what the server will accept at apply time.
var (
	garRegistryPathRegex = regexp.MustCompile(`^[a-z0-9-]+-docker\.pkg\.dev/[a-z0-9][a-z0-9-]*/[a-z0-9][a-z0-9._-]*(?:/[a-z0-9][a-z0-9._/-]*)?$`)
	ecrRegistryPathRegex = regexp.MustCompile(`^\d{12}\.dkr\.ecr\.[a-z0-9-]+\.amazonaws\.com/[a-z0-9]+(?:[._/-][a-z0-9]+)*$`)
	acrRegistryPathRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.azurecr\.io(?:/[a-z0-9]+(?:[._/-][a-z0-9]+)*)?$`)
)

// validateRegistryPathForKind enforces that the registry `name` path matches the
// format implied by the (derived) kind, returning a human-readable reason on
// mismatch. It mirrors the server-side path parsers.
func validateRegistryPathForKind(kind, name string) (ok bool, reason string) {
	trimmed := strings.TrimSpace(name)
	switch kind {
	case containerRegistryKindGAR:
		return garRegistryPathRegex.MatchString(trimmed),
			"gar registry name must look like <location>-docker.pkg.dev/<project>/<repository>[/<package>]"
	case containerRegistryKindECR:
		return ecrRegistryPathRegex.MatchString(trimmed),
			"ecr registry name must look like <account-id>.dkr.ecr.<region>.amazonaws.com/<repository>"
	case containerRegistryKindACR:
		return acrRegistryPathRegex.MatchString(trimmed),
			"acr registry name must look like <registry>.azurecr.io[/<repository>]"
	default:
		return false, fmt.Sprintf("unsupported container registry kind %q (expected one of gar, ecr, acr)", kind)
	}
}

// garConfigModel is the Google Artifact Registry variant of the config oneof.
type garConfigModel struct {
	RepositoryName types.String `tfsdk:"repository_name"`
}

// ecrConfigModel is the AWS ECR variant of the config oneof.
type ecrConfigModel struct {
	RegistryId     types.String `tfsdk:"registry_id"`
	RepositoryName types.String `tfsdk:"repository_name"`
}

// acrConfigModel is the Azure ACR variant of the config oneof.
type acrConfigModel struct {
	RepositoryName types.String `tfsdk:"repository_name"`
}

// registryConfigModel mirrors the CloudContainerRegistryConfig oneof: exactly one
// of gar/ecr/acr is set, and which one is set determines the registry kind.
type registryConfigModel struct {
	Gar *garConfigModel `tfsdk:"gar"`
	Ecr *ecrConfigModel `tfsdk:"ecr"`
	Acr *acrConfigModel `tfsdk:"acr"`
}

// cloudContainerRegistryResourceModel is the state/plan model for the container
// registry resource. `name`, `cloud_credential_id`, and `config` are the
// replace-only inputs; everything else is server-derived.
type cloudContainerRegistryResourceModel struct {
	Id                types.String         `tfsdk:"id"`
	Kind              types.String         `tfsdk:"kind"`
	Name              types.String         `tfsdk:"name"`
	CloudCredentialId types.String         `tfsdk:"cloud_credential_id"`
	Designator        types.String         `tfsdk:"designator"`
	Config            *registryConfigModel `tfsdk:"config"`
	Managed           types.Bool           `tfsdk:"managed"`
	TeamId            types.String         `tfsdk:"team_id"`
	AppliedAt         types.String         `tfsdk:"applied_at"`
	CreatedAt         types.String         `tfsdk:"created_at"`
	UpdatedAt         types.String         `tfsdk:"updated_at"`
}

// registryKindFromConfig derives the kind ("gar"/"ecr"/"acr") from whichever
// config block is set. Returns "" when the config is nil or none is set; the
// caller reports that as a validation error. It mirrors the server-side
// registryKindFromSpec.
func registryKindFromConfig(cfg *registryConfigModel) string {
	if cfg == nil {
		return ""
	}
	switch {
	case cfg.Gar != nil:
		return containerRegistryKindGAR
	case cfg.Ecr != nil:
		return containerRegistryKindECR
	case cfg.Acr != nil:
		return containerRegistryKindACR
	default:
		return ""
	}
}

// cloudContainerRegistrySchema builds the schema for the container registry
// resource. Every input is replace-only: there is no update RPC exposed, so any
// change forces recreation.
func cloudContainerRegistrySchema() schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Registers a reference to a cloud container registry (Google Artifact Registry, AWS ECR, or Azure ACR) plus the cloud credential used to reach it.\n\n" +
			"The registry kind is inferred from which `config` block you set (`gar`, `ecr`, or `acr`); exactly one must be provided. " +
			"Every attribute is replace-only (there is no update RPC). **Create-time access check:** creating this resource performs a live access check against the registry using the referenced `cloud_credential_id`; apply fails unless that credential can already reach the registry, so the credential must exist first.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud container registry identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Container registry kind. One of `gar`, `ecr`, or `acr`. Derived from the `config` block you set.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Fully-qualified registry path. For `gar`: `<location>-docker.pkg.dev/<project>/<repository>`; " +
					"for `ecr`: `<account-id>.dkr.ecr.<region>.amazonaws.com/<repository>`; for `acr`: `<registry>.azurecr.io`. " +
					"Must match the kind implied by the `config` block. Changing this forces a new resource.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cloud_credential_id": schema.StringAttribute{
				MarkdownDescription: "ID of the cloud credential (e.g. a `chalk_aws_cloud_credentials`/`chalk_gcp_cloud_credentials`/`chalk_azure_cloud_credentials` resource) used to access the registry. Its cloud provider must match the registry kind. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"designator": schema.StringAttribute{
				MarkdownDescription: "Optional server-side designator for the registry. Changing this forces a new resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"config": schema.SingleNestedAttribute{
				MarkdownDescription: "Registry configuration. Set exactly one of `gar`, `ecr`, or `acr`; the one you set determines the registry `kind`. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers:       []planmodifier.Object{objectplanmodifier.RequiresReplace()},
				Attributes: map[string]schema.Attribute{
					"gar": schema.SingleNestedAttribute{
						MarkdownDescription: "Google Artifact Registry configuration.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"repository_name": schema.StringAttribute{
								MarkdownDescription: "Name of the Artifact Registry repository to push customer-built images to.",
								Required:            true,
								Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
							},
						},
					},
					"ecr": schema.SingleNestedAttribute{
						MarkdownDescription: "AWS Elastic Container Registry configuration.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"registry_id": schema.StringAttribute{
								MarkdownDescription: "AWS account ID that owns the ECR registry. Optional; defaults to the account in `name`.",
								Optional:            true,
							},
							"repository_name": schema.StringAttribute{
								MarkdownDescription: "Name of the ECR repository to push customer-built images to.",
								Required:            true,
								Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
							},
						},
					},
					"acr": schema.SingleNestedAttribute{
						MarkdownDescription: "Azure Container Registry configuration.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"repository_name": schema.StringAttribute{
								MarkdownDescription: "Name of the ACR repository to push customer-built images to.",
								Optional:            true,
							},
						},
					},
				},
			},
			"managed": schema.BoolAttribute{
				MarkdownDescription: "Whether the registry is managed by Chalk. Always false for this resource.",
				Computed:            true,
			},
			"team_id": schema.StringAttribute{
				MarkdownDescription: "ID of the team that owns the registry.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"applied_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp at which the registry was last applied, if any.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp at which the registry was created.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp at which the registry was last updated.",
				Computed:            true,
			},
		},
	}
}

// registryConfigToProto converts the TF config model into the proto oneof. The
// caller has already validated that exactly one block is set.
func registryConfigToProto(cfg *registryConfigModel) *serverv1.CloudContainerRegistryConfig {
	if cfg == nil {
		return nil
	}
	switch {
	case cfg.Gar != nil:
		return &serverv1.CloudContainerRegistryConfig{
			Config: &serverv1.CloudContainerRegistryConfig_Gar{
				Gar: &serverv1.GarContainerRegistryConfig{
					RepositoryName: cfg.Gar.RepositoryName.ValueString(),
				},
			},
		}
	case cfg.Ecr != nil:
		return &serverv1.CloudContainerRegistryConfig{
			Config: &serverv1.CloudContainerRegistryConfig_Ecr{
				Ecr: &serverv1.EcrContainerRegistryConfig{
					RegistryId:     cfg.Ecr.RegistryId.ValueString(),
					RepositoryName: cfg.Ecr.RepositoryName.ValueString(),
				},
			},
		}
	case cfg.Acr != nil:
		return &serverv1.CloudContainerRegistryConfig{
			Config: &serverv1.CloudContainerRegistryConfig_Acr{
				Acr: &serverv1.AcrContainerRegistryConfig{
					RepositoryName: cfg.Acr.RepositoryName.ValueString(),
				},
			},
		}
	default:
		return nil
	}
}

// registryConfigFromProto converts the proto oneof back into the TF config model,
// building exactly the one block the server echoes. A nil or unset config yields
// nil so that state matches an equivalent config exactly.
func registryConfigFromProto(cfg *serverv1.CloudContainerRegistryConfig) *registryConfigModel {
	if cfg == nil {
		return nil
	}
	switch cfg.GetConfig().(type) {
	case *serverv1.CloudContainerRegistryConfig_Gar:
		return &registryConfigModel{Gar: &garConfigModel{
			RepositoryName: types.StringValue(cfg.GetGar().GetRepositoryName()),
		}}
	case *serverv1.CloudContainerRegistryConfig_Ecr:
		ecr := &ecrConfigModel{
			RegistryId:     types.StringNull(),
			RepositoryName: types.StringValue(cfg.GetEcr().GetRepositoryName()),
		}
		if cfg.GetEcr().GetRegistryId() != "" {
			ecr.RegistryId = types.StringValue(cfg.GetEcr().GetRegistryId())
		}
		return &registryConfigModel{Ecr: ecr}
	case *serverv1.CloudContainerRegistryConfig_Acr:
		acr := &acrConfigModel{RepositoryName: types.StringNull()}
		if cfg.GetAcr().GetRepositoryName() != "" {
			acr.RepositoryName = types.StringValue(cfg.GetAcr().GetRepositoryName())
		}
		return &registryConfigModel{Acr: acr}
	default:
		return nil
	}
}

// setCloudContainerRegistryState populates the model from a registry response.
// The replace-only inputs (name, cloud_credential_id, config) are echoed verbatim
// by the server, so overwriting them here keeps state faithful without triggering
// a spurious replace.
func setCloudContainerRegistryState(data *cloudContainerRegistryResourceModel, registry *serverv1.CloudComponentContainerRegistryResponse) {
	if registry == nil {
		return
	}
	data.Id = types.StringValue(registry.GetId())
	data.Name = types.StringValue(registry.GetName())
	if registry.GetKind() != "" {
		data.Kind = types.StringValue(registry.GetKind())
	}
	data.Managed = types.BoolValue(registry.GetManaged())

	if spec := registry.GetSpec(); spec != nil {
		if cfg := registryConfigFromProto(spec.GetConfig()); cfg != nil {
			data.Config = cfg
		}
	}

	if registry.CloudCredentialId != nil {
		data.CloudCredentialId = types.StringValue(registry.GetCloudCredentialId())
	}

	if registry.Designator != nil {
		data.Designator = types.StringValue(registry.GetDesignator())
	} else {
		data.Designator = types.StringNull()
	}

	data.TeamId = types.StringValue(registry.GetTeamId())
	data.AppliedAt = timestampToStringValue(registry.GetAppliedAt())
	data.CreatedAt = timestampToStringValue(registry.GetCreatedAt())
	data.UpdatedAt = timestampToStringValue(registry.GetUpdatedAt())
}

// describeCloudContainerRegistryCreateError maps well-known create-time failure
// codes to actionable diagnostics.
func describeCloudContainerRegistryCreateError(err error) (summary, detail string) {
	switch connect.CodeOf(err) {
	case connect.CodeFailedPrecondition:
		return "Cloud container registry is not reachable",
			fmt.Sprintf("Chalk performs a live access check against the registry at create time using the referenced cloud_credential_id. "+
				"The credential could not reach the registry: %s", err.Error())
	case connect.CodeNotFound:
		return "Cloud credential not found",
			fmt.Sprintf("The referenced cloud_credential_id does not exist: %s", err.Error())
	case connect.CodeInvalidArgument:
		return "Invalid cloud container registry configuration",
			fmt.Sprintf("The server rejected the registry configuration (check that name matches the config kind and the credential's cloud provider): %s", err.Error())
	case connect.CodeAlreadyExists:
		return "Cloud container registry already exists",
			fmt.Sprintf("A cloud container registry already exists for this name: %s", err.Error())
	default:
		return "Error creating cloud container registry",
			fmt.Sprintf("Could not create cloud container registry: %s", err.Error())
	}
}
