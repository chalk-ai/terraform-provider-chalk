package provider

// ResourcePermissionsMarkdown returns generated resource permission docs.
func ResourcePermissionsMarkdown(typeName string) string {
	return resourcePermissionsMarkdown[typeName]
}

// DataSourcePermissionsMarkdown returns generated data-source permission docs.
func DataSourcePermissionsMarkdown(typeName string) string {
	return datasourcePermissionsMarkdown[typeName]
}
