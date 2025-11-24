package provider

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	authv1 "github.com/chalk-ai/chalk-go/gen/chalk/auth/v1"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &ServiceTokenResource{}
var _ resource.ResourceWithImportState = &ServiceTokenResource{}

func NewServiceTokenResource() resource.Resource {
	return &ServiceTokenResource{}
}

type ServiceTokenResource struct {
	client *client.Manager
}

type ServiceTokenResourceModel struct {
	Id                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	ClientId               types.String `tfsdk:"client_id"`
	ClientSecret           types.String `tfsdk:"client_secret"`
	Permissions            types.List   `tfsdk:"permissions"`
	CustomClaims           types.Map    `tfsdk:"custom_claims"`
	FeatureTagToPermission types.Map    `tfsdk:"feature_tag_to_permission"`
	DefaultPermission      types.String `tfsdk:"default_permission"`
}

func (r *ServiceTokenResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_token"
}

func (r *ServiceTokenResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk service token resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Service token identifier",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Service token name",
				Required:            true,
			},
			"client_id": schema.StringAttribute{
				MarkdownDescription: "Client ID for the service token",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"client_secret": schema.StringAttribute{
				MarkdownDescription: "Client secret for the service token",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"permissions": schema.ListAttribute{
				MarkdownDescription: "List of permissions for the service token",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"custom_claims": schema.MapAttribute{
				MarkdownDescription: "Custom claims for the service token (key-value pairs where values are comma-separated lists)",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"feature_tag_to_permission": schema.MapAttribute{
				MarkdownDescription: "Map of feature tags to permissions",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"default_permission": schema.StringAttribute{
				MarkdownDescription: "Default permission for features",
				Optional:            true,
			},
		},
	}
}

func (r *ServiceTokenResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ServiceTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceTokenResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	// Create team client
	tc, err := r.client.NewTeamClient(ctx, "")
	if err != nil {
		resp.Diagnostics.AddError("Team Client", err.Error())
		return
	}

	createReq := &serverv1.CreateServiceTokenRequest{
		Name: data.Name.ValueString(),
	}

	// Handle permissions
	if !data.Permissions.IsNull() {
		var permissions []string
		diags := data.Permissions.ElementsAs(ctx, &permissions, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		for _, perm := range permissions {
			// Convert string to Permission enum
			if permValue, ok := authv1.Permission_value[perm]; ok {
				createReq.Permissions = append(createReq.Permissions, authv1.Permission(permValue))
			} else {
				resp.Diagnostics.AddError(
					"Invalid Permission",
					fmt.Sprintf("Permission %s is not a valid permission value", perm),
				)
				return
			}
		}
	}

	// Handle custom claims
	if !data.CustomClaims.IsNull() {
		claims := make(map[string]string)
		diags := data.CustomClaims.ElementsAs(ctx, &claims, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		for key, valuesStr := range claims {
			// Split comma-separated values
			values := []string{valuesStr}
			if valuesStr != "" {
				// Could parse comma-separated values if needed
				// values = strings.Split(valuesStr, ",")
			}
			createReq.CustomerClaims = append(createReq.CustomerClaims, &authv1.CustomClaim{
				Key:    key,
				Values: values,
			})
		}
	}

	// Handle feature tag to permission mapping
	if !data.FeatureTagToPermission.IsNull() {
		featurePerms := make(map[string]string)
		diags := data.FeatureTagToPermission.ElementsAs(ctx, &featurePerms, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		createReq.FeatureTagToPermission = make(map[string]authv1.FeaturePermission)
		for tag, perm := range featurePerms {
			if permValue, ok := authv1.FeaturePermission_value[perm]; ok {
				createReq.FeatureTagToPermission[tag] = authv1.FeaturePermission(permValue)
			} else {
				resp.Diagnostics.AddError(
					"Invalid Feature Permission",
					fmt.Sprintf("Feature permission %s is not a valid value", perm),
				)
				return
			}
		}
	}

	// Handle default permission
	if !data.DefaultPermission.IsNull() {
		if permValue, ok := authv1.FeaturePermission_value[data.DefaultPermission.ValueString()]; ok {
			createReq.DefaultPermission = authv1.FeaturePermission(permValue)
		} else {
			resp.Diagnostics.AddError(
				"Invalid Default Permission",
				fmt.Sprintf("Default permission %s is not a valid value", data.DefaultPermission.ValueString()),
			)
			return
		}
	}

	tokenResp, err := tc.CreateServiceToken(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Chalk Service Token",
			fmt.Sprintf("Could not create service token: %v", err),
		)
		return
	}

	// Update with created values
	data.Id = types.StringValue(tokenResp.Msg.Agent.Id)
	data.ClientId = types.StringValue(tokenResp.Msg.Agent.ClientId)
	data.ClientSecret = types.StringValue(tokenResp.Msg.ClientSecret)

	tflog.Trace(ctx, "created a chalk_service_token resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceTokenResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	// Create team client
	tc, err := r.client.NewTeamClient(ctx, "")
	if err != nil {
		resp.Diagnostics.AddError("Team Client", err.Error())
		return
	}

	listResp, err := tc.ListServiceTokens(ctx, connect.NewRequest(&serverv1.ListServiceTokensRequest{}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Chalk Service Tokens",
			fmt.Sprintf("Could not list service tokens: %v", err),
		)
		return
	}

	// Find the token by client_id
	var foundToken *authv1.DisplayServiceTokenAgent
	for _, agent := range listResp.Msg.Agents {
		if agent.ClientId == data.ClientId.ValueString() {
			foundToken = agent
			break
		}
	}

	if foundToken == nil {
		resp.Diagnostics.AddError(
			"Service Token Not Found",
			fmt.Sprintf("Could not find service token with client_id %s", data.ClientId.ValueString()),
		)
		return
	}

	// Update the model with the fetched data
	data.Name = types.StringValue(foundToken.Name)

	// Update permissions
	if len(foundToken.Permissions) > 0 {
		permissionStrings := []attr.Value{}
		for _, perm := range foundToken.Permissions {
			permissionStrings = append(permissionStrings, types.StringValue(perm.Permission.String()))
		}
		data.Permissions = types.ListValueMust(types.StringType, permissionStrings)
	}

	// Update custom claims
	if len(foundToken.CustomerClaims) > 0 {
		claims := make(map[string]attr.Value)
		for _, claim := range foundToken.CustomerClaims {
			// Join values with comma if multiple
			valueStr := ""
			if len(claim.Values) > 0 {
				valueStr = claim.Values[0]
				// Could join multiple values if needed
				// valueStr = strings.Join(claim.Values, ",")
			}
			claims[claim.Key] = types.StringValue(valueStr)
		}
		data.CustomClaims = types.MapValueMust(types.StringType, claims)
	}

	// Update feature permissions
	if foundToken.FeaturePermissions != nil && len(foundToken.FeaturePermissions.Tags) > 0 {
		featurePerms := make(map[string]attr.Value)
		for tag, perm := range foundToken.FeaturePermissions.Tags {
			featurePerms[tag] = types.StringValue(perm.String())
		}
		data.FeatureTagToPermission = types.MapValueMust(types.StringType, featurePerms)
	}

	if foundToken.FeaturePermissions != nil && foundToken.FeaturePermissions.DefaultPermission != nil && *foundToken.FeaturePermissions.DefaultPermission != authv1.FeaturePermission_FEATURE_PERMISSION_UNSPECIFIED {
		data.DefaultPermission = types.StringValue((*foundToken.FeaturePermissions.DefaultPermission).String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ServiceTokenResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	// Create team client
	tc, err := r.client.NewTeamClient(ctx, "")
	if err != nil {
		resp.Diagnostics.AddError("Team Client", err.Error())
		return
	}

	updateReq := &serverv1.UpdateServiceTokenRequest{
		ClientId: data.ClientId.ValueString(),
		Name:     data.Name.ValueString(),
	}

	// Handle permissions
	if !data.Permissions.IsNull() {
		var permissions []string
		diags := data.Permissions.ElementsAs(ctx, &permissions, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		for _, perm := range permissions {
			if permValue, ok := authv1.Permission_value[perm]; ok {
				updateReq.Permissions = append(updateReq.Permissions, authv1.Permission(permValue))
			} else {
				resp.Diagnostics.AddError(
					"Invalid Permission",
					fmt.Sprintf("Permission %s is not a valid permission value", perm),
				)
				return
			}
		}
	}

	// Handle custom claims
	if !data.CustomClaims.IsNull() {
		claims := make(map[string]string)
		diags := data.CustomClaims.ElementsAs(ctx, &claims, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		for key, valuesStr := range claims {
			values := []string{valuesStr}
			updateReq.CustomerClaims = append(updateReq.CustomerClaims, &authv1.CustomClaim{
				Key:    key,
				Values: values,
			})
		}
	}

	// Handle feature tag to permission mapping
	if !data.FeatureTagToPermission.IsNull() {
		featurePerms := make(map[string]string)
		diags := data.FeatureTagToPermission.ElementsAs(ctx, &featurePerms, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		updateReq.FeatureTagToPermission = make(map[string]authv1.FeaturePermission)
		for tag, perm := range featurePerms {
			if permValue, ok := authv1.FeaturePermission_value[perm]; ok {
				updateReq.FeatureTagToPermission[tag] = authv1.FeaturePermission(permValue)
			} else {
				resp.Diagnostics.AddError(
					"Invalid Feature Permission",
					fmt.Sprintf("Feature permission %s is not a valid value", perm),
				)
				return
			}
		}
	}

	// Handle default permission
	if !data.DefaultPermission.IsNull() {
		if permValue, ok := authv1.FeaturePermission_value[data.DefaultPermission.ValueString()]; ok {
			updateReq.DefaultPermission = authv1.FeaturePermission(permValue)
		} else {
			resp.Diagnostics.AddError(
				"Invalid Default Permission",
				fmt.Sprintf("Default permission %s is not a valid value", data.DefaultPermission.ValueString()),
			)
			return
		}
	}
	_, err = tc.UpdateServiceToken(ctx, connect.NewRequest(updateReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Chalk Service Token",
			fmt.Sprintf("Could not update service token: %v", err),
		)
		return
	}

	tflog.Trace(ctx, "updated a chalk_service_token resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceTokenResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	// Create team client
	tc, err := r.client.NewTeamClient(ctx, "")
	if err != nil {
		resp.Diagnostics.AddError("Team Client", err.Error())
		return
	}

	deleteReq := &serverv1.DeleteServiceTokenRequest{
		Id: data.Id.ValueString(),
	}
	_, err = tc.DeleteServiceToken(ctx, connect.NewRequest(deleteReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Chalk Service Token",
			fmt.Sprintf("Could not delete service token %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "deleted chalk_service_token resource")
}

func (r *ServiceTokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("client_id"), req, resp)
}
