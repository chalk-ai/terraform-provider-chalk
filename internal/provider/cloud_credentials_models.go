package provider

import "github.com/hashicorp/terraform-plugin-framework/types"

// Shared model types for cloud credentials resources

type DockerBuildConfigModel struct {
	Builder                   types.String `tfsdk:"builder"`
	PushRegistryType          types.String `tfsdk:"push_registry_type"`
	PushRegistryTagPrefix     types.String `tfsdk:"push_registry_tag_prefix"`
	RegistryCredentialsSecret types.String `tfsdk:"registry_credentials_secret_id"`
	NotificationTopic         types.String `tfsdk:"notification_topic"`
}

type GCPWorkloadIdentityModel struct {
	ProjectNumber  types.String `tfsdk:"project_number"`
	ServiceAccount types.String `tfsdk:"service_account"`
	PoolId         types.String `tfsdk:"pool_id"`
	ProviderId     types.String `tfsdk:"provider_id"`
}
