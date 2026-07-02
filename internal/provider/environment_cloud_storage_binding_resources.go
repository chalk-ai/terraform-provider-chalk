package provider

import (
	"context"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// environmentCloudStorageBindingBase provides the client wiring and the
// role-independent methods (Configure/Update/ImportState) shared by every
// environment binding resource. Each concrete resource embeds it and supplies its
// own Metadata (a literal type name) plus Create/Read/Delete that pin a fixed role.
type environmentCloudStorageBindingBase struct {
	client *client.Manager
}

func (b *environmentCloudStorageBindingBase) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	b.client = configureCloudManager(req, resp)
}

func (b *environmentCloudStorageBindingBase) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Environment cloud storage bindings cannot be updated. They must be deleted and recreated.",
	)
}

// ImportState imports by the target key alone (`<environment_id>`); the role is
// fixed by the resource type, so it is not part of the import ID.
func (b *environmentCloudStorageBindingBase) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("environment_id"), req, resp)
}

// --- DATASET ---

var (
	_ resource.Resource                = &EnvironmentDatasetCloudStorageBindingResource{}
	_ resource.ResourceWithImportState = &EnvironmentDatasetCloudStorageBindingResource{}
)

func NewEnvironmentDatasetCloudStorageBindingResource() resource.Resource {
	return &EnvironmentDatasetCloudStorageBindingResource{}
}

type EnvironmentDatasetCloudStorageBindingResource struct {
	environmentCloudStorageBindingBase
}

func (r *EnvironmentDatasetCloudStorageBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_dataset_cloud_storage_binding"
}

func (r *EnvironmentDatasetCloudStorageBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = cloudStorageBindingSchema("environment_id", "environment", "DATASET")
}

func (r *EnvironmentDatasetCloudStorageBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m environmentCloudStorageBindingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).CreateBindingEnvironmentCloudStorage(ctx, connect.NewRequest(&serverv1.CreateBindingEnvironmentCloudStorageRequest{
		EnvironmentId:  m.EnvironmentId.ValueString(),
		CloudStorageId: m.CloudStorageId.ValueString(),
		StorageRole:    serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_DATASET,
	}))
	finishEnvBindingCreate(ctx, resp, &m, envBinding(out), err, "DATASET")
}

func (r *EnvironmentDatasetCloudStorageBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m environmentCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).GetBindingEnvironmentCloudStorage(ctx, connect.NewRequest(&serverv1.GetBindingEnvironmentCloudStorageRequest{
		EnvironmentId: m.EnvironmentId.ValueString(),
		StorageRole:   serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_DATASET,
	}))
	finishEnvBindingRead(ctx, resp, &m, envGetBinding(out), err)
}

func (r *EnvironmentDatasetCloudStorageBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m environmentCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.NewCloudComponentsClient(ctx).DeleteBindingEnvironmentCloudStorage(ctx, connect.NewRequest(&serverv1.DeleteBindingEnvironmentCloudStorageRequest{
		EnvironmentId: m.EnvironmentId.ValueString(),
		StorageRole:   serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_DATASET,
	}))
	handleBindingDelete(resp, err, "environment")
}

// --- PLAN_STAGES ---

var (
	_ resource.Resource                = &EnvironmentPlanStagesCloudStorageBindingResource{}
	_ resource.ResourceWithImportState = &EnvironmentPlanStagesCloudStorageBindingResource{}
)

func NewEnvironmentPlanStagesCloudStorageBindingResource() resource.Resource {
	return &EnvironmentPlanStagesCloudStorageBindingResource{}
}

type EnvironmentPlanStagesCloudStorageBindingResource struct {
	environmentCloudStorageBindingBase
}

func (r *EnvironmentPlanStagesCloudStorageBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_plan_stages_cloud_storage_binding"
}

func (r *EnvironmentPlanStagesCloudStorageBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = cloudStorageBindingSchema("environment_id", "environment", "PLAN_STAGES")
}

func (r *EnvironmentPlanStagesCloudStorageBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m environmentCloudStorageBindingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).CreateBindingEnvironmentCloudStorage(ctx, connect.NewRequest(&serverv1.CreateBindingEnvironmentCloudStorageRequest{
		EnvironmentId:  m.EnvironmentId.ValueString(),
		CloudStorageId: m.CloudStorageId.ValueString(),
		StorageRole:    serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_PLAN_STAGES,
	}))
	finishEnvBindingCreate(ctx, resp, &m, envBinding(out), err, "PLAN_STAGES")
}

func (r *EnvironmentPlanStagesCloudStorageBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m environmentCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).GetBindingEnvironmentCloudStorage(ctx, connect.NewRequest(&serverv1.GetBindingEnvironmentCloudStorageRequest{
		EnvironmentId: m.EnvironmentId.ValueString(),
		StorageRole:   serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_PLAN_STAGES,
	}))
	finishEnvBindingRead(ctx, resp, &m, envGetBinding(out), err)
}

func (r *EnvironmentPlanStagesCloudStorageBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m environmentCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.NewCloudComponentsClient(ctx).DeleteBindingEnvironmentCloudStorage(ctx, connect.NewRequest(&serverv1.DeleteBindingEnvironmentCloudStorageRequest{
		EnvironmentId: m.EnvironmentId.ValueString(),
		StorageRole:   serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_PLAN_STAGES,
	}))
	handleBindingDelete(resp, err, "environment")
}

// --- SOURCE_BUNDLE ---

var (
	_ resource.Resource                = &EnvironmentSourceBundleCloudStorageBindingResource{}
	_ resource.ResourceWithImportState = &EnvironmentSourceBundleCloudStorageBindingResource{}
)

func NewEnvironmentSourceBundleCloudStorageBindingResource() resource.Resource {
	return &EnvironmentSourceBundleCloudStorageBindingResource{}
}

type EnvironmentSourceBundleCloudStorageBindingResource struct {
	environmentCloudStorageBindingBase
}

func (r *EnvironmentSourceBundleCloudStorageBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_source_bundle_cloud_storage_binding"
}

func (r *EnvironmentSourceBundleCloudStorageBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = cloudStorageBindingSchema("environment_id", "environment", "SOURCE_BUNDLE")
}

func (r *EnvironmentSourceBundleCloudStorageBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m environmentCloudStorageBindingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).CreateBindingEnvironmentCloudStorage(ctx, connect.NewRequest(&serverv1.CreateBindingEnvironmentCloudStorageRequest{
		EnvironmentId:  m.EnvironmentId.ValueString(),
		CloudStorageId: m.CloudStorageId.ValueString(),
		StorageRole:    serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_SOURCE_BUNDLE,
	}))
	finishEnvBindingCreate(ctx, resp, &m, envBinding(out), err, "SOURCE_BUNDLE")
}

func (r *EnvironmentSourceBundleCloudStorageBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m environmentCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).GetBindingEnvironmentCloudStorage(ctx, connect.NewRequest(&serverv1.GetBindingEnvironmentCloudStorageRequest{
		EnvironmentId: m.EnvironmentId.ValueString(),
		StorageRole:   serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_SOURCE_BUNDLE,
	}))
	finishEnvBindingRead(ctx, resp, &m, envGetBinding(out), err)
}

func (r *EnvironmentSourceBundleCloudStorageBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m environmentCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.NewCloudComponentsClient(ctx).DeleteBindingEnvironmentCloudStorage(ctx, connect.NewRequest(&serverv1.DeleteBindingEnvironmentCloudStorageRequest{
		EnvironmentId: m.EnvironmentId.ValueString(),
		StorageRole:   serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_SOURCE_BUNDLE,
	}))
	handleBindingDelete(resp, err, "environment")
}

// --- MODEL_REGISTRY ---

var (
	_ resource.Resource                = &EnvironmentModelRegistryCloudStorageBindingResource{}
	_ resource.ResourceWithImportState = &EnvironmentModelRegistryCloudStorageBindingResource{}
)

func NewEnvironmentModelRegistryCloudStorageBindingResource() resource.Resource {
	return &EnvironmentModelRegistryCloudStorageBindingResource{}
}

type EnvironmentModelRegistryCloudStorageBindingResource struct {
	environmentCloudStorageBindingBase
}

func (r *EnvironmentModelRegistryCloudStorageBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_model_registry_cloud_storage_binding"
}

func (r *EnvironmentModelRegistryCloudStorageBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = cloudStorageBindingSchema("environment_id", "environment", "MODEL_REGISTRY")
}

func (r *EnvironmentModelRegistryCloudStorageBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m environmentCloudStorageBindingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).CreateBindingEnvironmentCloudStorage(ctx, connect.NewRequest(&serverv1.CreateBindingEnvironmentCloudStorageRequest{
		EnvironmentId:  m.EnvironmentId.ValueString(),
		CloudStorageId: m.CloudStorageId.ValueString(),
		StorageRole:    serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_MODEL_REGISTRY,
	}))
	finishEnvBindingCreate(ctx, resp, &m, envBinding(out), err, "MODEL_REGISTRY")
}

func (r *EnvironmentModelRegistryCloudStorageBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m environmentCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).GetBindingEnvironmentCloudStorage(ctx, connect.NewRequest(&serverv1.GetBindingEnvironmentCloudStorageRequest{
		EnvironmentId: m.EnvironmentId.ValueString(),
		StorageRole:   serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_MODEL_REGISTRY,
	}))
	finishEnvBindingRead(ctx, resp, &m, envGetBinding(out), err)
}

func (r *EnvironmentModelRegistryCloudStorageBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m environmentCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.NewCloudComponentsClient(ctx).DeleteBindingEnvironmentCloudStorage(ctx, connect.NewRequest(&serverv1.DeleteBindingEnvironmentCloudStorageRequest{
		EnvironmentId: m.EnvironmentId.ValueString(),
		StorageRole:   serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_MODEL_REGISTRY,
	}))
	handleBindingDelete(resp, err, "environment")
}

// envBinding / envGetBinding extract the binding from a (possibly nil) response.
func envBinding(out *connect.Response[serverv1.CreateBindingEnvironmentCloudStorageResponse]) *serverv1.EnvironmentCloudStorageBinding {
	if out == nil {
		return nil
	}
	return out.Msg.GetBinding()
}

func envGetBinding(out *connect.Response[serverv1.GetBindingEnvironmentCloudStorageResponse]) *serverv1.EnvironmentCloudStorageBinding {
	if out == nil {
		return nil
	}
	return out.Msg.GetBinding()
}
