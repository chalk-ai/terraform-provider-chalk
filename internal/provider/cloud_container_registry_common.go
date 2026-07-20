package provider

import (
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// cloudContainerRegistryResourceModel is the state model for the unmanaged
// container registry resource: its inputs plus the server-assigned id. The managed
// variant has no `name` attribute (Chalk owns the registry and derives its path),
// so it uses its own model; CRUD handling is otherwise identical. The server
// derives the registry kind and all addressing from `name`, so there is no
// separate kind input or config block.
type cloudContainerRegistryResourceModel struct {
	Id                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	CloudCredentialId types.String `tfsdk:"cloud_credential_id"`
}

// managedCloudContainerRegistryResourceModel is the state model for the managed
// container registry resource.
type managedCloudContainerRegistryResourceModel struct {
	Id                types.String `tfsdk:"id"`
	CloudCredentialId types.String `tfsdk:"cloud_credential_id"`
}

// cloudContainerRegistrySchema builds the schema shared by the managed and
// unmanaged container registry resources. Only inputs and the resource id are
// exposed; server-derived metadata deliberately has no attributes. When managed is
// true, Chalk owns the registry and derives its path, so there is no `name`
// attribute at all; otherwise it is a Required, replace-only input. Every input is
// replace-only: there is no update RPC exposed, so any change forces recreation.
func cloudContainerRegistrySchema(managed bool) schema.Schema {
	var markdown string
	if managed {
		markdown = "Registers a Chalk-managed container registry: Chalk owns the registry and derives its path, so you only supply the cloud credential.\n\n" +
			"Every attribute is replace-only (there is no update RPC). **Create-time access check:** creating this resource performs a live access check using the referenced `cloud_credential_id`; apply fails unless that credential can reach the registry, so the credential must exist first."
	} else {
		markdown = "Registers a reference to an existing (unmanaged) cloud container registry plus the cloud credential used to reach it. Chalk does not provision the registry.\n\n" +
			"The registry kind (GAR, ECR, or ACR) is derived by the server from `name`. " +
			"Every attribute is replace-only (there is no update RPC). **Create-time access check:** creating this resource performs a live access check against the registry using the referenced `cloud_credential_id`; apply fails unless that credential can already reach the registry, so the credential must exist first."
	}

	attributes := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			MarkdownDescription: "Cloud container registry identifier.",
			Computed:            true,
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"cloud_credential_id": schema.StringAttribute{
			MarkdownDescription: "ID of the cloud credential (e.g. a `chalk_aws_cloud_credentials`/`chalk_gcp_cloud_credentials`/`chalk_azure_cloud_credentials` resource) used to access the registry. Its cloud provider must match the registry kind. Changing this forces a new resource.",
			Required:            true,
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
	}
	if !managed {
		attributes["name"] = schema.StringAttribute{
			MarkdownDescription: "Fully-qualified registry path, e.g. `us-docker.pkg.dev/<project>/<repository>` (GAR), " +
				"`<account-id>.dkr.ecr.<region>.amazonaws.com/<repository>` (ECR), or `<registry>.azurecr.io/<repository>` (ACR). " +
				"The server derives the registry kind from this. Changing this forces a new resource.",
			Required: true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		}
	}

	return schema.Schema{
		MarkdownDescription: markdown,
		Attributes:          attributes,
	}
}

// setCloudContainerRegistryState populates the unmanaged model from a registry
// response. The replace-only input (cloud_credential_id) is only overwritten when
// the server echoes a value, so a response that omits it cannot trigger a spurious
// replace.
func setCloudContainerRegistryState(data *cloudContainerRegistryResourceModel, registry *serverv1.CloudComponentContainerRegistryResponse) {
	if registry == nil {
		return
	}
	data.Id = types.StringValue(registry.GetId())
	data.Name = types.StringValue(registry.GetName())

	if registry.CloudCredentialId != nil {
		data.CloudCredentialId = types.StringValue(registry.GetCloudCredentialId())
	}
}

// setManagedCloudContainerRegistryState is the managed-variant twin of
// setCloudContainerRegistryState (the managed model has no name).
func setManagedCloudContainerRegistryState(data *managedCloudContainerRegistryResourceModel, registry *serverv1.CloudComponentContainerRegistryResponse) {
	if registry == nil {
		return
	}
	data.Id = types.StringValue(registry.GetId())

	if registry.CloudCredentialId != nil {
		data.CloudCredentialId = types.StringValue(registry.GetCloudCredentialId())
	}
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
			fmt.Sprintf("The server rejected the registry configuration (check that name is a valid registry path and the credential's cloud provider matches): %s", err.Error())
	case connect.CodeAlreadyExists:
		return "Cloud container registry already exists",
			fmt.Sprintf("A cloud container registry already exists for this name: %s", err.Error())
	default:
		return "Error creating cloud container registry",
			fmt.Sprintf("Could not create cloud container registry: %s", err.Error())
	}
}
