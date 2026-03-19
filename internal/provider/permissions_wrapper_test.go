package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// TestPermDocWrapperForwardsResourceInterfaces verifies that resourceWithPermDoc
// forwards every optional framework interface that its inner resource implements.
//
// If terraform-plugin-framework adds a new optional resource interface, add it
// to resourceOptionalIfaces below AND implement forwarding in permissions_wrapper.go.
func TestPermDocWrapperForwardsResourceInterfaces(t *testing.T) {
	t.Parallel()

	type optIface struct {
		name  string
		inner func(resource.Resource) bool
		outer func(resource.Resource) bool
	}
	resourceOptionalIfaces := []optIface{
		{
			"ResourceWithConfigure",
			func(r resource.Resource) bool { _, ok := r.(resource.ResourceWithConfigure); return ok },
			func(r resource.Resource) bool { _, ok := r.(resource.ResourceWithConfigure); return ok },
		},
		{
			"ResourceWithImportState",
			func(r resource.Resource) bool { _, ok := r.(resource.ResourceWithImportState); return ok },
			func(r resource.Resource) bool { _, ok := r.(resource.ResourceWithImportState); return ok },
		},
		{
			"ResourceWithValidateConfig",
			func(r resource.Resource) bool { _, ok := r.(resource.ResourceWithValidateConfig); return ok },
			func(r resource.Resource) bool { _, ok := r.(resource.ResourceWithValidateConfig); return ok },
		},
		{
			"ResourceWithConfigValidators",
			func(r resource.Resource) bool { _, ok := r.(resource.ResourceWithConfigValidators); return ok },
			func(r resource.Resource) bool { _, ok := r.(resource.ResourceWithConfigValidators); return ok },
		},
		{
			"ResourceWithModifyPlan",
			func(r resource.Resource) bool { _, ok := r.(resource.ResourceWithModifyPlan); return ok },
			func(r resource.Resource) bool { _, ok := r.(resource.ResourceWithModifyPlan); return ok },
		},
	}

	for _, ctor := range allResourceCtors {
		inner := ctor()
		wrapper := &resourceWithPermDoc{inner: inner, permDoc: "test"}

		var meta resource.MetadataResponse
		inner.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "chalk"}, &meta)

		for _, iface := range resourceOptionalIfaces {
			if iface.inner(inner) && !iface.outer(wrapper) {
				t.Errorf("resource %s: inner implements %s but wrapper does not — update permissions_wrapper.go", meta.TypeName, iface.name)
			}
		}
	}
}

// TestPermDocWrapperForwardsDatasourceInterfaces is the data-source equivalent
// of TestPermDocWrapperForwardsResourceInterfaces.
//
// If terraform-plugin-framework adds a new optional data-source interface, add
// it to datasourceOptionalIfaces below AND implement forwarding in permissions_wrapper.go.
func TestPermDocWrapperForwardsDatasourceInterfaces(t *testing.T) {
	t.Parallel()

	type optIface struct {
		name  string
		inner func(datasource.DataSource) bool
		outer func(datasource.DataSource) bool
	}
	datasourceOptionalIfaces := []optIface{
		{
			"DataSourceWithConfigure",
			func(d datasource.DataSource) bool { _, ok := d.(datasource.DataSourceWithConfigure); return ok },
			func(d datasource.DataSource) bool { _, ok := d.(datasource.DataSourceWithConfigure); return ok },
		},
	}

	for _, ctor := range allDatasourceCtors {
		inner := ctor()
		wrapper := &datasourceWithPermDoc{inner: inner, permDoc: "test"}

		var meta datasource.MetadataResponse
		inner.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "chalk"}, &meta)

		for _, iface := range datasourceOptionalIfaces {
			if iface.inner(inner) && !iface.outer(wrapper) {
				t.Errorf("datasource %s: inner implements %s but wrapper does not — update permissions_wrapper.go", meta.TypeName, iface.name)
			}
		}
	}
}
