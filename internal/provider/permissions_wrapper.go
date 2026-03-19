package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// applyResourcePermDocs wraps each resource constructor so that the resource's
// Schema MarkdownDescription is appended with its required-permissions text
// (sourced from the generated resourcePermissionsMarkdown and datasourcePermissionsMarkdown maps).
func applyResourcePermDocs(ctors []func() resource.Resource) []func() resource.Resource {
	wrapped := make([]func() resource.Resource, len(ctors))
	for i, ctor := range ctors {
		wrapped[i] = func() resource.Resource {
			inner := ctor()
			var meta resource.MetadataResponse
			inner.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "chalk"}, &meta)
			if doc := resourcePermissionsMarkdown[meta.TypeName]; doc != "" {
				return &resourceWithPermDoc{inner: inner, permDoc: doc}
			}
			return inner
		}
	}
	return wrapped
}

// applyDatasourcePermDocs wraps each data-source constructor in the same way.
func applyDatasourcePermDocs(ctors []func() datasource.DataSource) []func() datasource.DataSource {
	wrapped := make([]func() datasource.DataSource, len(ctors))
	for i, ctor := range ctors {
		wrapped[i] = func() datasource.DataSource {
			inner := ctor()
			var meta datasource.MetadataResponse
			inner.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "chalk"}, &meta)
			if doc := datasourcePermissionsMarkdown[meta.TypeName]; doc != "" {
				return &datasourceWithPermDoc{inner: inner, permDoc: doc}
			}
			return inner
		}
	}
	return wrapped
}

// resourceWithPermDoc delegates all resource.Resource methods to inner, and
// appends permDoc to the schema's MarkdownDescription. It also conditionally
// forwards every optional framework interface so that capabilities declared by
// the inner resource are not silently dropped.
//
// NOTE: if terraform-plugin-framework adds a new optional interface and an inner
// resource implements it, this wrapper must be updated to forward it — otherwise
// the framework won't detect the capability and that behavior will be silently lost.
type resourceWithPermDoc struct {
	inner   resource.Resource
	permDoc string
}

func (w *resourceWithPermDoc) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	w.inner.Metadata(ctx, req, resp)
}

func (w *resourceWithPermDoc) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	w.inner.Schema(ctx, req, resp)
	resp.Schema.MarkdownDescription += "\n\n" + w.permDoc
}

func (w *resourceWithPermDoc) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	w.inner.Create(ctx, req, resp)
}

func (w *resourceWithPermDoc) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	w.inner.Read(ctx, req, resp)
}

func (w *resourceWithPermDoc) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	w.inner.Update(ctx, req, resp)
}

func (w *resourceWithPermDoc) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	w.inner.Delete(ctx, req, resp)
}

// ResourceWithConfigure — forwards to inner if supported.
func (w *resourceWithPermDoc) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c, ok := w.inner.(resource.ResourceWithConfigure); ok {
		c.Configure(ctx, req, resp)
	}
}

// ResourceWithImportState — forwards to inner if supported, otherwise errors.
func (w *resourceWithPermDoc) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if i, ok := w.inner.(resource.ResourceWithImportState); ok {
		i.ImportState(ctx, req, resp)
	} else {
		resp.Diagnostics.AddError("Import Not Supported", "This resource does not support import.")
	}
}

// ResourceWithValidateConfig — forwards to inner if supported.
func (w *resourceWithPermDoc) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	if v, ok := w.inner.(resource.ResourceWithValidateConfig); ok {
		v.ValidateConfig(ctx, req, resp)
	}
}

// ResourceWithConfigValidators — forwards to inner if supported.
func (w *resourceWithPermDoc) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	if v, ok := w.inner.(resource.ResourceWithConfigValidators); ok {
		return v.ConfigValidators(ctx)
	}
	return nil
}

// ResourceWithModifyPlan — forwards to inner if supported.
func (w *resourceWithPermDoc) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if m, ok := w.inner.(resource.ResourceWithModifyPlan); ok {
		m.ModifyPlan(ctx, req, resp)
	}
}

// datasourceWithPermDoc is the data-source equivalent of resourceWithPermDoc.
type datasourceWithPermDoc struct {
	inner   datasource.DataSource
	permDoc string
}

func (w *datasourceWithPermDoc) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	w.inner.Metadata(ctx, req, resp)
}

func (w *datasourceWithPermDoc) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	w.inner.Schema(ctx, req, resp)
	resp.Schema.MarkdownDescription += "\n\n" + w.permDoc
}

func (w *datasourceWithPermDoc) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	w.inner.Read(ctx, req, resp)
}

// DataSourceWithConfigure — forwards to inner if supported.
func (w *datasourceWithPermDoc) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c, ok := w.inner.(datasource.DataSourceWithConfigure); ok {
		c.Configure(ctx, req, resp)
	}
}
