package provider

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"net/http"
)

var (
	_ resource.Resource                = &ClusterBackgroundPersistenceDeploymentBindingResource{}
	_ resource.ResourceWithImportState = &ClusterBackgroundPersistenceDeploymentBindingResource{}
)

func NewClusterBackgroundPersistenceDeploymentBindingResource() resource.Resource {
	return &ClusterBackgroundPersistenceDeploymentBindingResource{}
}

type ClusterBackgroundPersistenceDeploymentBindingResource struct {
	client *ChalkClient
}

type ClusterBackgroundPersistenceDeploymentBindingResourceModel struct {
	ClusterID                         types.String `tfsdk:"cluster_id"`
	BackgroundPersistenceDeploymentID types.String `tfsdk:"background_persistence_deployment_id"`
}

func (r *ClusterBackgroundPersistenceDeploymentBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_background_persistence_deployment_binding"
}

func (r *ClusterBackgroundPersistenceDeploymentBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a binding between a Chalk cluster and a background persistence deployment.",
		Attributes: map[string]schema.Attribute{
			"cluster_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the cluster to bind to the background persistence deployment.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"background_persistence_deployment_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the background persistence deployment to bind to the cluster.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *ClusterBackgroundPersistenceDeploymentBindingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ChalkClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ChalkClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *ClusterBackgroundPersistenceDeploymentBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ClusterBackgroundPersistenceDeploymentBindingResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         r.client.ApiServer,
			interceptors: []connect.Interceptor{MakeApiServerHeaderInterceptor("x-chalk-server", "go-api")},
		},
	)

	grpcClientOptions := &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	}

	cloudComponentsClient := NewCloudComponentsClient(ctx, grpcClientOptions)

	createRequest := &serverv1.CreateBindingClusterBackgroundPersistenceDeploymentRequest{
		ClusterId:                         data.ClusterID.ValueString(),
		BackgroundPersistenceDeploymentId: data.BackgroundPersistenceDeploymentID.ValueString(),
	}

	_, err := cloudComponentsClient.CreateBindingClusterBackgroundPersistenceDeployment(ctx, connect.NewRequest(createRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating cluster background persistence deployment binding",
			fmt.Sprintf("Could not create cluster background persistence deployment binding: %s", err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterBackgroundPersistenceDeploymentBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ClusterBackgroundPersistenceDeploymentBindingResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         r.client.ApiServer,
			interceptors: []connect.Interceptor{MakeApiServerHeaderInterceptor("x-chalk-server", "go-api")},
		},
	)

	grpcClientOptions := &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	}

	cloudComponentsClient := NewCloudComponentsClient(ctx, grpcClientOptions)

	getRequest := &serverv1.GetBindingClusterBackgroundPersistenceDeploymentRequest{
		ClusterId: data.ClusterID.ValueString(),
	}

	response, err := cloudComponentsClient.GetBindingClusterBackgroundPersistenceDeployment(ctx, connect.NewRequest(getRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading cluster background persistence deployment binding",
			fmt.Sprintf("Could not read cluster background persistence deployment binding: %s", err.Error()),
		)
		return
	}

	data.ClusterID = types.StringValue(response.Msg.ClusterId)
	data.BackgroundPersistenceDeploymentID = types.StringValue(response.Msg.BackgroundPersistenceDeploymentId)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterBackgroundPersistenceDeploymentBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Cluster background persistence deployment bindings cannot be updated. They must be deleted and recreated.",
	)
}

func (r *ClusterBackgroundPersistenceDeploymentBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ClusterBackgroundPersistenceDeploymentBindingResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         r.client.ApiServer,
			interceptors: []connect.Interceptor{MakeApiServerHeaderInterceptor("x-chalk-server", "go-api")},
		},
	)

	grpcClientOptions := &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	}

	cloudComponentsClient := NewCloudComponentsClient(ctx, grpcClientOptions)

	deleteRequest := &serverv1.DeleteBindingClusterBackgroundPersistenceDeploymentRequest{
		ClusterId: data.ClusterID.ValueString(),
	}

	_, err := cloudComponentsClient.DeleteBindingClusterBackgroundPersistenceDeployment(ctx, connect.NewRequest(deleteRequest))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting cluster background persistence deployment binding",
			fmt.Sprintf("Could not delete cluster background persistence deployment binding: %s", err.Error()),
		)
		return
	}
}

func (r *ClusterBackgroundPersistenceDeploymentBindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("cluster_id"), req, resp)
}
