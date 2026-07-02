package provider

import (
	"context"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// clusterCloudStorageBindingBase provides the client wiring and role-independent
// methods (Configure/Update/ImportState) shared by every cluster binding resource.
type clusterCloudStorageBindingBase struct {
	client *client.Manager
}

func (b *clusterCloudStorageBindingBase) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	b.client = configureCloudManager(req, resp)
}

func (b *clusterCloudStorageBindingBase) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Cluster cloud storage bindings cannot be updated. They must be deleted and recreated.",
	)
}

// ImportState imports by the target key alone (`<cluster_id>`); the role is fixed by
// the resource type, so it is not part of the import ID.
func (b *clusterCloudStorageBindingBase) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("cluster_id"), req, resp)
}

// --- DATASET ---

var (
	_ resource.Resource                = &ClusterDatasetCloudStorageBindingResource{}
	_ resource.ResourceWithImportState = &ClusterDatasetCloudStorageBindingResource{}
)

func NewClusterDatasetCloudStorageBindingResource() resource.Resource {
	return &ClusterDatasetCloudStorageBindingResource{}
}

type ClusterDatasetCloudStorageBindingResource struct {
	clusterCloudStorageBindingBase
}

func (r *ClusterDatasetCloudStorageBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_dataset_cloud_storage_binding"
}

func (r *ClusterDatasetCloudStorageBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = cloudStorageBindingSchema("cluster_id", "cluster", "DATASET")
}

func (r *ClusterDatasetCloudStorageBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m clusterCloudStorageBindingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).CreateBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.CreateBindingClusterCloudStorageRequest{
		ClusterId:      m.ClusterId.ValueString(),
		CloudStorageId: m.CloudStorageId.ValueString(),
		StorageRole:    serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_DATASET,
	}))
	finishClusterBindingCreate(ctx, resp, &m, clusterBinding(out), err, "DATASET")
}

func (r *ClusterDatasetCloudStorageBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m clusterCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).GetBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.GetBindingClusterCloudStorageRequest{
		ClusterId:   m.ClusterId.ValueString(),
		StorageRole: serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_DATASET,
	}))
	finishClusterBindingRead(ctx, resp, &m, clusterGetBinding(out), err)
}

func (r *ClusterDatasetCloudStorageBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m clusterCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.NewCloudComponentsClient(ctx).DeleteBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.DeleteBindingClusterCloudStorageRequest{
		ClusterId:   m.ClusterId.ValueString(),
		StorageRole: serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_DATASET,
	}))
	handleBindingDelete(resp, err, "cluster")
}

// --- PLAN_STAGES ---

var (
	_ resource.Resource                = &ClusterPlanStagesCloudStorageBindingResource{}
	_ resource.ResourceWithImportState = &ClusterPlanStagesCloudStorageBindingResource{}
)

func NewClusterPlanStagesCloudStorageBindingResource() resource.Resource {
	return &ClusterPlanStagesCloudStorageBindingResource{}
}

type ClusterPlanStagesCloudStorageBindingResource struct {
	clusterCloudStorageBindingBase
}

func (r *ClusterPlanStagesCloudStorageBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_plan_stages_cloud_storage_binding"
}

func (r *ClusterPlanStagesCloudStorageBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = cloudStorageBindingSchema("cluster_id", "cluster", "PLAN_STAGES")
}

func (r *ClusterPlanStagesCloudStorageBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m clusterCloudStorageBindingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).CreateBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.CreateBindingClusterCloudStorageRequest{
		ClusterId:      m.ClusterId.ValueString(),
		CloudStorageId: m.CloudStorageId.ValueString(),
		StorageRole:    serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_PLAN_STAGES,
	}))
	finishClusterBindingCreate(ctx, resp, &m, clusterBinding(out), err, "PLAN_STAGES")
}

func (r *ClusterPlanStagesCloudStorageBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m clusterCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).GetBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.GetBindingClusterCloudStorageRequest{
		ClusterId:   m.ClusterId.ValueString(),
		StorageRole: serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_PLAN_STAGES,
	}))
	finishClusterBindingRead(ctx, resp, &m, clusterGetBinding(out), err)
}

func (r *ClusterPlanStagesCloudStorageBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m clusterCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.NewCloudComponentsClient(ctx).DeleteBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.DeleteBindingClusterCloudStorageRequest{
		ClusterId:   m.ClusterId.ValueString(),
		StorageRole: serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_PLAN_STAGES,
	}))
	handleBindingDelete(resp, err, "cluster")
}

// --- SOURCE_BUNDLE ---

var (
	_ resource.Resource                = &ClusterSourceBundleCloudStorageBindingResource{}
	_ resource.ResourceWithImportState = &ClusterSourceBundleCloudStorageBindingResource{}
)

func NewClusterSourceBundleCloudStorageBindingResource() resource.Resource {
	return &ClusterSourceBundleCloudStorageBindingResource{}
}

type ClusterSourceBundleCloudStorageBindingResource struct {
	clusterCloudStorageBindingBase
}

func (r *ClusterSourceBundleCloudStorageBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_source_bundle_cloud_storage_binding"
}

func (r *ClusterSourceBundleCloudStorageBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = cloudStorageBindingSchema("cluster_id", "cluster", "SOURCE_BUNDLE")
}

func (r *ClusterSourceBundleCloudStorageBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m clusterCloudStorageBindingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).CreateBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.CreateBindingClusterCloudStorageRequest{
		ClusterId:      m.ClusterId.ValueString(),
		CloudStorageId: m.CloudStorageId.ValueString(),
		StorageRole:    serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_SOURCE_BUNDLE,
	}))
	finishClusterBindingCreate(ctx, resp, &m, clusterBinding(out), err, "SOURCE_BUNDLE")
}

func (r *ClusterSourceBundleCloudStorageBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m clusterCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).GetBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.GetBindingClusterCloudStorageRequest{
		ClusterId:   m.ClusterId.ValueString(),
		StorageRole: serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_SOURCE_BUNDLE,
	}))
	finishClusterBindingRead(ctx, resp, &m, clusterGetBinding(out), err)
}

func (r *ClusterSourceBundleCloudStorageBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m clusterCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.NewCloudComponentsClient(ctx).DeleteBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.DeleteBindingClusterCloudStorageRequest{
		ClusterId:   m.ClusterId.ValueString(),
		StorageRole: serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_SOURCE_BUNDLE,
	}))
	handleBindingDelete(resp, err, "cluster")
}

// --- MODEL_REGISTRY ---

var (
	_ resource.Resource                = &ClusterModelRegistryCloudStorageBindingResource{}
	_ resource.ResourceWithImportState = &ClusterModelRegistryCloudStorageBindingResource{}
)

func NewClusterModelRegistryCloudStorageBindingResource() resource.Resource {
	return &ClusterModelRegistryCloudStorageBindingResource{}
}

type ClusterModelRegistryCloudStorageBindingResource struct {
	clusterCloudStorageBindingBase
}

func (r *ClusterModelRegistryCloudStorageBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_model_registry_cloud_storage_binding"
}

func (r *ClusterModelRegistryCloudStorageBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = cloudStorageBindingSchema("cluster_id", "cluster", "MODEL_REGISTRY")
}

func (r *ClusterModelRegistryCloudStorageBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m clusterCloudStorageBindingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).CreateBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.CreateBindingClusterCloudStorageRequest{
		ClusterId:      m.ClusterId.ValueString(),
		CloudStorageId: m.CloudStorageId.ValueString(),
		StorageRole:    serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_MODEL_REGISTRY,
	}))
	finishClusterBindingCreate(ctx, resp, &m, clusterBinding(out), err, "MODEL_REGISTRY")
}

func (r *ClusterModelRegistryCloudStorageBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m clusterCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).GetBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.GetBindingClusterCloudStorageRequest{
		ClusterId:   m.ClusterId.ValueString(),
		StorageRole: serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_MODEL_REGISTRY,
	}))
	finishClusterBindingRead(ctx, resp, &m, clusterGetBinding(out), err)
}

func (r *ClusterModelRegistryCloudStorageBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m clusterCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.NewCloudComponentsClient(ctx).DeleteBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.DeleteBindingClusterCloudStorageRequest{
		ClusterId:   m.ClusterId.ValueString(),
		StorageRole: serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_MODEL_REGISTRY,
	}))
	handleBindingDelete(resp, err, "cluster")
}

// --- VOLUME (cluster only) ---

var (
	_ resource.Resource                = &ClusterVolumeCloudStorageBindingResource{}
	_ resource.ResourceWithImportState = &ClusterVolumeCloudStorageBindingResource{}
)

func NewClusterVolumeCloudStorageBindingResource() resource.Resource {
	return &ClusterVolumeCloudStorageBindingResource{}
}

type ClusterVolumeCloudStorageBindingResource struct {
	clusterCloudStorageBindingBase
}

func (r *ClusterVolumeCloudStorageBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_volume_cloud_storage_binding"
}

func (r *ClusterVolumeCloudStorageBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = cloudStorageBindingSchema("cluster_id", "cluster", "VOLUME")
}

func (r *ClusterVolumeCloudStorageBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m clusterCloudStorageBindingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).CreateBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.CreateBindingClusterCloudStorageRequest{
		ClusterId:      m.ClusterId.ValueString(),
		CloudStorageId: m.CloudStorageId.ValueString(),
		StorageRole:    serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_VOLUME,
	}))
	finishClusterBindingCreate(ctx, resp, &m, clusterBinding(out), err, "VOLUME")
}

func (r *ClusterVolumeCloudStorageBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m clusterCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.NewCloudComponentsClient(ctx).GetBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.GetBindingClusterCloudStorageRequest{
		ClusterId:   m.ClusterId.ValueString(),
		StorageRole: serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_VOLUME,
	}))
	finishClusterBindingRead(ctx, resp, &m, clusterGetBinding(out), err)
}

func (r *ClusterVolumeCloudStorageBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m clusterCloudStorageBindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.NewCloudComponentsClient(ctx).DeleteBindingClusterCloudStorage(ctx, connect.NewRequest(&serverv1.DeleteBindingClusterCloudStorageRequest{
		ClusterId:   m.ClusterId.ValueString(),
		StorageRole: serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_VOLUME,
	}))
	handleBindingDelete(resp, err, "cluster")
}

// clusterBinding / clusterGetBinding extract the binding from a (possibly nil) response.
func clusterBinding(out *connect.Response[serverv1.CreateBindingClusterCloudStorageResponse]) *serverv1.ClusterCloudStorageBinding {
	if out == nil {
		return nil
	}
	return out.Msg.GetBinding()
}

func clusterGetBinding(out *connect.Response[serverv1.GetBindingClusterCloudStorageResponse]) *serverv1.ClusterCloudStorageBinding {
	if out == nil {
		return nil
	}
	return out.Msg.GetBinding()
}
