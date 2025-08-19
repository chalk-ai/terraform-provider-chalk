# Chalk Service Token Resource

The `chalk_service_token` resource allows you to create and manage service tokens in Chalk.

## Example Usage

```hcl
resource "chalk_service_token" "example" {
  name = "my-service-token"
  
  permissions = [
    "PERMISSION_QUERY_ONLINE",
    "PERMISSION_QUERY_OFFLINE"
  ]
  
  custom_claims = {
    query_tags = "allowed_tag1,allowed_tag2"
  }
  
  feature_tag_to_permission = {
    "sensitive" = "FEATURE_PERMISSION_DENY"
    "public"    = "FEATURE_PERMISSION_ALLOW"
  }
  
  default_permission = "FEATURE_PERMISSION_ALLOW_INTERNAL"
}
```

## Argument Reference

* `name` - (Required) The name of the service token.
* `permissions` - (Optional) List of permissions for the service token. These should be valid Chalk permission values (e.g., "PERMISSION_QUERY_ONLINE").
* `custom_claims` - (Optional) Map of custom claims for the service token. Values are comma-separated strings.
* `feature_tag_to_permission` - (Optional) Map of feature tags to their corresponding permissions. Valid permission values are:
  * `FEATURE_PERMISSION_ALLOW` - Allow unfettered access to the feature
  * `FEATURE_PERMISSION_ALLOW_INTERNAL` - Allow access only within a query plan
  * `FEATURE_PERMISSION_DENY` - Deny access to the feature
* `default_permission` - (Optional) Default permission for features not explicitly mapped.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the service token.
* `client_id` - The client ID for the service token. Use this for authentication.
* `client_secret` - The client secret for the service token. This is sensitive and only available upon creation.

## Import

Service tokens can be imported using the client_id:

```
terraform import chalk_service_token.example <client_id>
```

Note: The client_secret cannot be recovered after initial creation.