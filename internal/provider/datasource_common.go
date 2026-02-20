package provider

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/gen/chalk/server/v1/serverv1connect"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// DatasourceConfigValue represents a single config entry that is either a literal value
// or a reference to an existing Chalk secret.
type DatasourceConfigValue struct {
	Literal  types.String `tfsdk:"literal"`
	SecretId types.String `tfsdk:"secret_id"`
}

// configValueAttributes returns the standard schema attributes for a DatasourceConfigValue
// nested object, including ExactlyOneOf validators.
func configValueAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"literal": schema.StringAttribute{
			MarkdownDescription: "A plain-text value.",
			Optional:            true,
			Sensitive:           true,
			Validators: []validator.String{
				stringvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("secret_id"),
				),
			},
		},
		"secret_id": schema.StringAttribute{
			MarkdownDescription: "The ID of an existing Chalk secret.",
			Optional:            true,
			Validators: []validator.String{
				stringvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("literal"),
				),
			},
		},
	}
}

// parseIntegrationKind converts a user-friendly string like "postgresql" to the proto enum.
func parseIntegrationKind(kindStr string) (serverv1.IntegrationKind, error) {
	enumName := "INTEGRATION_KIND_" + strings.ToUpper(kindStr)
	val, ok := serverv1.IntegrationKind_value[enumName]
	if !ok {
		return serverv1.IntegrationKind_INTEGRATION_KIND_UNSPECIFIED,
			fmt.Errorf("unknown integration kind %q", kindStr)
	}
	return serverv1.IntegrationKind(val), nil
}

// integrationKindToString converts a proto enum to a user-friendly string like "postgresql".
func integrationKindToString(kind serverv1.IntegrationKind) string {
	name := kind.String()
	return strings.ToLower(strings.TrimPrefix(name, "INTEGRATION_KIND_"))
}

// configToProto converts a Terraform config map to the proto Config field.
func configToProto(config map[string]DatasourceConfigValue) map[string]*serverv1.IntegrationConfigValue {
	proto := make(map[string]*serverv1.IntegrationConfigValue, len(config))
	for key, val := range config {
		if !val.Literal.IsNull() && !val.Literal.IsUnknown() {
			proto[key] = &serverv1.IntegrationConfigValue{
				Value: &serverv1.IntegrationConfigValue_Literal{
					Literal: val.Literal.ValueString(),
				},
			}
		} else if !val.SecretId.IsNull() && !val.SecretId.IsUnknown() {
			proto[key] = &serverv1.IntegrationConfigValue{
				Value: &serverv1.IntegrationConfigValue_SecretId{
					SecretId: val.SecretId.ValueString(),
				},
			}
		}
	}
	return proto
}

// refreshConfigKeys refreshes config values for the given keys from the server.
// It uses returnedSecrets for values included in a bulk GetIntegration response, calling
// GetIntegrationValue as a fallback for any keys the server withheld. secret_id entries
// in stateConfig are preserved from state unchanged.
// Returns nil and adds to diags if an error occurs.
func refreshConfigKeys(
	ctx context.Context,
	ic serverv1connect.IntegrationsServiceClient,
	integrationId string,
	returnedSecrets map[string]string,
	stateConfig map[string]DatasourceConfigValue,
	diags *diag.Diagnostics,
) map[string]DatasourceConfigValue {
	newConfig := make(map[string]DatasourceConfigValue, len(stateConfig))
	for key, stateVal := range stateConfig {
		if !stateVal.SecretId.IsNull() && !stateVal.SecretId.IsUnknown() {
			newConfig[key] = stateVal
			continue
		}
		if val, ok := returnedSecrets[key]; ok {
			newConfig[key] = DatasourceConfigValue{
				Literal:  types.StringValue(val),
				SecretId: types.StringNull(),
			}
			continue
		}
		valResp, err := ic.GetIntegrationValue(ctx, connect.NewRequest(&serverv1.GetIntegrationValueRequest{
			IntegrationId: integrationId,
			SecretName:    key,
		}))
		if err != nil {
			diags.AddError(
				"Error Reading Chalk Datasource Secret",
				fmt.Sprintf("Could not read secret %q for datasource %s: %v", key, integrationId, err),
			)
			return nil
		}
		if valResp.Msg.Secretvalue != nil {
			newConfig[key] = DatasourceConfigValue{
				Literal:  types.StringValue(valResp.Msg.Secretvalue.Value),
				SecretId: types.StringNull(),
			}
		} else {
			newConfig[key] = DatasourceConfigValue{
				Literal:  types.StringValue(""),
				SecretId: types.StringNull(),
			}
		}
	}
	return newConfig
}
