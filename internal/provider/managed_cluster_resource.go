package provider

import (
	"context"
	"fmt"
	"maps"
	"time"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/gen/chalk/server/v1/serverv1connect"
	"github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// clusterPollInterval is how often we poll the server while waiting for a
// managed cluster to be applied or deleted. clusterPollTimeout bounds how long
// we wait before giving up. They are vars (not consts) so tests can shorten
// them.
//
// clusterPollTimeout must be strictly longer than the server's
// clusterDeploymentTimeout (90m, see go-api-server/cloudcomponents/lifecycle.go):
// the server lazily flips a stuck deployment to FAILED once that deadline
// passes, so we keep polling past it to observe the server's terminal status
// instead of timing out first and reporting a less useful error.
var (
	clusterPollInterval = 10 * time.Second
	clusterPollTimeout  = 95 * time.Minute
)

var _ resource.Resource = &ManagedClusterResource{}
var _ resource.ResourceWithImportState = &ManagedClusterResource{}

func NewManagedClusterResource() resource.Resource {
	return &ManagedClusterResource{}
}

type ManagedClusterResource struct {
	client *client.Manager
}

type ManagedClusterResourceModel struct {
	Id                  types.String              `tfsdk:"id"`
	CloudCredentialId   types.String              `tfsdk:"cloud_credential_id"`
	VpcId               types.String              `tfsdk:"vpc_id"`
	MaintenanceWindow   *maintenanceWindowModel   `tfsdk:"maintenance_window"`
	DataPlaneRedis      *dataPlaneRedisModel      `tfsdk:"data_plane_redis"`
	DataPlaneController *dataPlaneControllerModel `tfsdk:"data_plane_controller"`
}

func (r *ManagedClusterResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_cluster"
}

func (r *ManagedClusterResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk managed Kubernetes cluster resource. Creates a fully managed cluster using the provided cloud credentials.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cluster identifier",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cloud_credential_id": schema.StringAttribute{
				MarkdownDescription: "ID of the cloud credential to use for the managed cluster",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"vpc_id": schema.StringAttribute{
				MarkdownDescription: "ID of the VPC to use for the cluster",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}

	maps.Copy(resp.Schema.Attributes, clusterConfigSchemaAttributes())
}

func (r *ManagedClusterResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ManagedClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ManagedClusterResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create cloud components client
	cc := r.client.NewCloudComponentsClient(ctx)

	credentialId := data.CloudCredentialId.ValueString()
	vpcId := data.VpcId.ValueString()

	createReq := &serverv1.CreateCloudComponentClusterRequest{
		Cluster: &serverv1.CloudComponentClusterRequest{
			Managed:           true,
			CloudCredentialId: &credentialId,
			VpcId:             &vpcId,
			// The server generates name/kind/designator/dns for managed clusters,
			// but still reads the config blocks off the spec.
			Spec: &serverv1.CloudComponentCluster{},
		},
	}

	applyClusterConfigToSpec(createReq.Cluster.Spec, data.MaintenanceWindow, data.DataPlaneRedis, data.DataPlaneController)

	cluster, err := cc.CreateCloudComponentCluster(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Managed Cluster",
			fmt.Sprintf("Could not create managed cluster: %v", err),
		)
		return
	}

	// The cluster is provisioned asynchronously. Poll until it reaches a
	// terminal lifecycle status: ACTIVE on success, FAILED otherwise.
	created := cluster.Msg.Cluster
	finalCluster, waitErr := r.waitForClusterActive(ctx, cc, created.GetId())
	if finalCluster == nil {
		// We never observed a fresh status (e.g. transport error or timeout);
		// fall back to the create response so the resource is still tracked.
		finalCluster = created
	}

	// Update with the latest values.
	r.updateModelFromProto(&data, finalCluster)

	// Persist state before returning any wait error so that the partially
	// created cluster is recorded and Terraform taints it (replacing it on the
	// next apply) instead of leaking an untracked resource.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if waitErr != nil {
		resp.Diagnostics.AddError(
			"Error Waiting for Managed Cluster",
			fmt.Sprintf("Managed cluster %s did not become active: %v", created.GetId(), waitErr),
		)
		return
	}

	tflog.Trace(ctx, "created a chalk_managed_cluster resource")
}

func (r *ManagedClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ManagedClusterResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create cloud components client
	cc := r.client.NewCloudComponentsClient(ctx)

	cluster, err := cc.GetCloudComponentCluster(ctx, connect.NewRequest(&serverv1.GetCloudComponentClusterRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Managed Cluster",
			fmt.Sprintf("Could not read managed cluster %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Update the model with the fetched data
	r.updateModelFromProto(&data, cluster.Msg.Cluster)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ManagedClusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// cloud_credential_id and vpc_id are RequiresReplace, so the only mutable
	// fields are the cluster-level config blocks. Push them via a spec update; the
	// server overlays them onto the stored spec and applies the change
	// synchronously, returning the updated cluster.
	var data ManagedClusterResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create cloud components client
	cc := r.client.NewCloudComponentsClient(ctx)

	updateReq := &serverv1.UpdateCloudComponentClusterRequest{
		Id: data.Id.ValueString(),
		Cluster: &serverv1.CloudComponentClusterRequest{
			Managed: true,
			Spec:    &serverv1.CloudComponentCluster{},
		},
	}
	applyClusterConfigToSpec(updateReq.Cluster.Spec, data.MaintenanceWindow, data.DataPlaneRedis, data.DataPlaneController)

	cluster, err := cc.UpdateCloudComponentCluster(ctx, connect.NewRequest(updateReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Managed Cluster",
			fmt.Sprintf("Could not update managed cluster %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Update the model with the returned data
	r.updateModelFromProto(&data, cluster.Msg.Cluster)

	tflog.Trace(ctx, "updated chalk_managed_cluster resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ManagedClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ManagedClusterResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create cloud components client
	cc := r.client.NewCloudComponentsClient(ctx)

	id := data.Id.ValueString()
	deleteReq := &serverv1.DeleteCloudComponentClusterRequest{
		Id: id,
	}

	_, err := cc.DeleteCloudComponentCluster(ctx, connect.NewRequest(deleteReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Managed Cluster",
			fmt.Sprintf("Could not delete managed cluster %s: %v", id, err),
		)
		return
	}

	// Deletion is asynchronous: the server keeps reporting the cluster with a
	// DELETING status until the deployer confirms teardown. Poll until it
	// reaches the terminal DELETED status or can no longer be fetched so that
	// Delete only returns once the cluster is actually gone (rather than as soon
	// as it enters DELETING).
	err = pollUntilDeleted(ctx, clusterPollInterval, clusterPollTimeout, func(ctx context.Context) (componentStatus, error) {
		cluster, err := cc.GetCloudComponentCluster(ctx, connect.NewRequest(&serverv1.GetCloudComponentClusterRequest{
			Id: id,
		}))
		if err != nil {
			if isNotFoundErr(err) {
				return componentStatus{found: false}, nil
			}
			return componentStatus{}, err
		}
		return componentStatus{found: true, status: cluster.Msg.Cluster.GetStatus(), statusError: cluster.Msg.Cluster.GetStatusError()}, nil
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Waiting for Managed Cluster Deletion",
			fmt.Sprintf("Managed cluster %s was deleted but did not disappear: %v", id, err),
		)
		return
	}

	tflog.Trace(ctx, "deleted chalk_managed_cluster resource")
}

func (r *ManagedClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// waitForClusterActive polls GetCloudComponentCluster until the cluster reaches
// a terminal lifecycle status. It returns the latest cluster response together
// with a nil error once the status is ACTIVE, or the response and a non-nil
// error when the status is FAILED. On a transport error or timeout it returns a
// nil response and the error.
func (r *ManagedClusterResource) waitForClusterActive(
	ctx context.Context,
	cc serverv1connect.CloudComponentsServiceClient,
	id string,
) (*serverv1.CloudComponentClusterResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, clusterPollTimeout)
	defer cancel()

	ticker := time.NewTicker(clusterPollInterval)
	defer ticker.Stop()

	for {
		resp, err := cc.GetCloudComponentCluster(ctx, connect.NewRequest(&serverv1.GetCloudComponentClusterRequest{
			Id: id,
		}))
		if err != nil {
			return nil, err
		}
		cluster := resp.Msg.Cluster

		if terminal, failure := terminalStatus(cluster.GetStatus(), cluster.GetStatusError()); terminal {
			return cluster, failure
		}

		tflog.Trace(ctx, "waiting for managed cluster to become active", map[string]any{
			"id":     id,
			"status": cluster.GetStatus(),
		})

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out after %s waiting for status %s: %w", clusterPollTimeout, cloudComponentStatusActive, ctx.Err())
		case <-ticker.C:
		}
	}
}

func (r *ManagedClusterResource) updateModelFromProto(model *ManagedClusterResourceModel, cluster *serverv1.CloudComponentClusterResponse) {
	model.Id = types.StringValue(cluster.Id)

	if cluster.CloudCredentialId != nil {
		model.CloudCredentialId = types.StringValue(*cluster.CloudCredentialId)
	}

	if cluster.VpcId != nil {
		model.VpcId = types.StringValue(*cluster.VpcId)
	} else {
		model.VpcId = types.StringNull()
	}

	model.MaintenanceWindow = maintenanceWindowFromProto(cluster.Spec.GetMaintenanceWindow())
	model.DataPlaneRedis = dataPlaneRedisFromProto(cluster.Spec.GetDataPlaneRedis())
	model.DataPlaneController = dataPlaneControllerFromProto(cluster.Spec.GetDataplaneController())
}
