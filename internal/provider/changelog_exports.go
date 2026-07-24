package provider

// ResourcePermissionsMarkdown returns the generated permission documentation
// for a Terraform resource type. It is exported for repository tooling.
func ResourcePermissionsMarkdown(typeName string) string {
	return resourcePermissionsMarkdown[typeName]
}

// DataSourcePermissionsMarkdown returns the generated permission documentation
// for a Terraform data-source type. It is exported for repository tooling.
func DataSourcePermissionsMarkdown(typeName string) string {
	return datasourcePermissionsMarkdown[typeName]
}
