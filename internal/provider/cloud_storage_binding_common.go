package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The cloud-storage binding resources come one per (target, role) pair: the role is
// encoded in the resource type rather than an attribute, so each type pins a fixed
// serverv1.CloudStorageRole. Datasets, plan stages, etc. have materially different
// data-sensitivity profiles (e.g. dataset buckets may hold PII), so keeping them as
// distinct resource types makes intent explicit in configuration and state.

// environmentCloudStorageBindingModel is the state model for every environment
// binding resource. environment_id + the fixed role form the real key.
type environmentCloudStorageBindingModel struct {
	Id             types.String `tfsdk:"id"`
	EnvironmentId  types.String `tfsdk:"environment_id"`
	CloudStorageId types.String `tfsdk:"cloud_storage_id"`
}

// clusterCloudStorageBindingModel is the state model for every cluster binding resource.
type clusterCloudStorageBindingModel struct {
	Id             types.String `tfsdk:"id"`
	ClusterId      types.String `tfsdk:"cluster_id"`
	CloudStorageId types.String `tfsdk:"cloud_storage_id"`
}

// cloudStorageBindingSchema builds the schema shared by all binding resources.
// targetAttr is "environment_id" or "cluster_id"; targetLabel and roleLabel are used
// only in the human-readable description.
func cloudStorageBindingSchema(targetAttr, targetLabel, roleLabel string) schema.Schema {
	return schema.Schema{
		MarkdownDescription: fmt.Sprintf("Binds a cloud storage to %s for the **%s** role.\n\n"+
			"The role is fixed by the resource type. Bindings are keyed by `(%s, role)` — the %s has at most one storage for this role — so this resource is identified by the %s, not by `cloud_storage_id`. "+
			"Every attribute is replace-only (there is no update RPC).\n\n"+
			"~> **Delete removes whatever occupies the (%s, %s) slot.** Deletes target `(%s, role)` (not a binding id), so a stale plan can delete a binding created out-of-band for the same %s and role. "+
			"If the storage in the slot is reassigned outside Terraform, the next plan reconciles `cloud_storage_id` and schedules a recreate.",
			targetLabel, roleLabel, targetAttr, targetLabel, targetAttr, targetLabel, roleLabel, targetAttr, targetLabel),
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Binding identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			targetAttr: schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("The ID of the %s to bind the cloud storage to. Changing this forces a new resource.", targetLabel),
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cloud_storage_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the cloud storage to bind. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func setEnvBindingState(m *environmentCloudStorageBindingModel, b *serverv1.EnvironmentCloudStorageBinding) {
	m.Id = types.StringValue(b.GetId())
	if b.GetEnvironmentId() != "" {
		m.EnvironmentId = types.StringValue(b.GetEnvironmentId())
	}
	m.CloudStorageId = types.StringValue(b.GetCloudStorageId())
}

func setClusterBindingState(m *clusterCloudStorageBindingModel, b *serverv1.ClusterCloudStorageBinding) {
	m.Id = types.StringValue(b.GetId())
	if b.GetClusterId() != "" {
		m.ClusterId = types.StringValue(b.GetClusterId())
	}
	m.CloudStorageId = types.StringValue(b.GetCloudStorageId())
}

// finishEnvBindingCreate applies the create response to state, mapping AlreadyExists
// and empty-response failures to clear diagnostics.
func finishEnvBindingCreate(ctx context.Context, resp *resource.CreateResponse, m *environmentCloudStorageBindingModel, binding *serverv1.EnvironmentCloudStorageBinding, err error, roleLabel string) {
	if err != nil {
		if connect.CodeOf(err) == connect.CodeAlreadyExists {
			resp.Diagnostics.AddError(
				"Environment already has a binding for this role",
				fmt.Sprintf("Environment %q already has a %s cloud storage binding. Remove the existing binding first, or import it. (%s)", m.EnvironmentId.ValueString(), roleLabel, err.Error()),
			)
			return
		}
		resp.Diagnostics.AddError("Error creating environment cloud storage binding", fmt.Sprintf("Could not create binding: %s", err.Error()))
		return
	}
	if binding == nil {
		resp.Diagnostics.AddError("Empty create response", "The server returned no binding in the create response. This is unexpected; please report it to the provider developers.")
		return
	}
	setEnvBindingState(m, binding)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func finishClusterBindingCreate(ctx context.Context, resp *resource.CreateResponse, m *clusterCloudStorageBindingModel, binding *serverv1.ClusterCloudStorageBinding, err error, roleLabel string) {
	if err != nil {
		if connect.CodeOf(err) == connect.CodeAlreadyExists {
			resp.Diagnostics.AddError(
				"Cluster already has a binding for this role",
				fmt.Sprintf("Cluster %q already has a %s cloud storage binding. Remove the existing binding first, or import it. (%s)", m.ClusterId.ValueString(), roleLabel, err.Error()),
			)
			return
		}
		resp.Diagnostics.AddError("Error creating cluster cloud storage binding", fmt.Sprintf("Could not create binding: %s", err.Error()))
		return
	}
	if binding == nil {
		resp.Diagnostics.AddError("Empty create response", "The server returned no binding in the create response. This is unexpected; please report it to the provider developers.")
		return
	}
	setClusterBindingState(m, binding)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

// finishEnvBindingRead applies the get response to state. NotFound removes the
// resource; a differing cloud_storage_id (the slot was reassigned out-of-band) is
// reflected so the next plan recreates.
func finishEnvBindingRead(ctx context.Context, resp *resource.ReadResponse, m *environmentCloudStorageBindingModel, binding *serverv1.EnvironmentCloudStorageBinding, err error) {
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading environment cloud storage binding", fmt.Sprintf("Could not read binding: %s", err.Error()))
		return
	}
	if binding == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	setEnvBindingState(m, binding)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func finishClusterBindingRead(ctx context.Context, resp *resource.ReadResponse, m *clusterCloudStorageBindingModel, binding *serverv1.ClusterCloudStorageBinding, err error) {
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading cluster cloud storage binding", fmt.Sprintf("Could not read binding: %s", err.Error()))
		return
	}
	if binding == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	setClusterBindingState(m, binding)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

// handleBindingDelete records a diagnostic on a real delete failure. A NotFound is
// treated as an already-deleted success.
func handleBindingDelete(resp *resource.DeleteResponse, err error, targetLabel string) {
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error deleting %s cloud storage binding", targetLabel),
			fmt.Sprintf("Could not delete binding: %s", err.Error()),
		)
	}
}
