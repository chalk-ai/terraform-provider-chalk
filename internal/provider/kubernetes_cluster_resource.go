package provider

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"net/http"
)

// Custom validator to require field when managed=true
type requiredWhenManagedValidator struct{}

func (v requiredWhenManagedValidator) Description(ctx context.Context) string {
	return "field is required when managed is true"
}

func (v requiredWhenManagedValidator) MarkdownDescription(ctx context.Context) string {
	return "field is required when `managed` is true"
}

func (v requiredWhenManagedValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	// Skip validation if the current field is unknown or null (not set)
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		// Check if managed is true
		var managedValue types.Bool
		diags := req.Config.GetAttribute(ctx, path.Root("managed"), &managedValue)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		// If managed is true and this field is null/empty, add error
		if !managedValue.IsNull() && !managedValue.IsUnknown() && managedValue.ValueBool() {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Missing required field",
				"This field is required when managed is true",
			)
		}
	}
}

func RequiredWhenManaged() validator.String {
	return requiredWhenManagedValidator{}
}

var _ resource.Resource = &KubernetesClusterResource{}
var _ resource.ResourceWithImportState = &KubernetesClusterResource{}

func NewKubernetesClusterResource() resource.Resource {
	return &KubernetesClusterResource{}
}

type KubernetesClusterResource struct {
	client *ChalkClient
}

type KubernetesClusterResourceModel struct {
	Id                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Designator        types.String `tfsdk:"designator"`
	Kind              types.String `tfsdk:"kind"`
	KubernetesVersion types.String `tfsdk:"kubernetes_version"`
	Managed           types.Bool   `tfsdk:"managed"`
	CloudCredentialId types.String `tfsdk:"cloud_credential_id"`
	VpcId             types.String `tfsdk:"vpc_id"`
	TeamId            types.String `tfsdk:"team_id"`
}

func (r *KubernetesClusterResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_cluster"
}

func (r *KubernetesClusterResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk Kubernetes cluster resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cluster identifier",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Cluster name",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"designator": schema.StringAttribute{
				MarkdownDescription: "Cluster designator",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Cloud provider kind (e.g., 'EKS_STANDARD', 'EKS_AUTOPILOT', 'GKE_STANDARD', 'GKE_AUTOPILOT')",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kubernetes_version": schema.StringAttribute{
				MarkdownDescription: "Kubernetes version",
				Required:            true,
			},
			"managed": schema.BoolAttribute{
				MarkdownDescription: "Whether the cluster is managed by Chalk",
				Required:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"cloud_credential_id": schema.StringAttribute{
				MarkdownDescription: "ID of the cloud credential to use for the cluster",
				Optional:            true,
				Validators: []validator.String{
					RequiredWhenManaged(),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"vpc_id": schema.StringAttribute{
				MarkdownDescription: "ID of the VPC to use for the cluster",
				Optional:            true,
				Validators: []validator.String{
					RequiredWhenManaged(),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"team_id": schema.StringAttribute{
				MarkdownDescription: "Team ID",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *KubernetesClusterResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *KubernetesClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data KubernetesClusterResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         r.client.ApiServer,
			interceptors: []connect.Interceptor{MakeApiServerHeaderInterceptor("x-chalk-server", "go-api")},
		},
	)

	// Create cloud components client with token injection interceptor
	cc := NewCloudComponentsClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	})

	createReq := &serverv1.CreateCloudComponentClusterRequest{
		Cluster: &serverv1.CloudComponentClusterRequest{
			Kind: data.Kind.ValueString(),
			Spec: &serverv1.CloudComponentCluster{
				Name:              data.Name.ValueString(),
				KubernetesVersion: data.KubernetesVersion.ValueString(),
			},
			Managed: data.Managed.ValueBool(),
		},
	}

	if !data.CloudCredentialId.IsNull() {
		credentialId := data.CloudCredentialId.ValueString()
		createReq.Cluster.CloudCredentialId = &credentialId
	}

	if !data.VpcId.IsNull() {
		vpcId := data.VpcId.ValueString()
		createReq.Cluster.VpcId = &vpcId
	}

	cluster, err := cc.CreateCloudComponentCluster(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Kubernetes Cluster",
			fmt.Sprintf("Could not create cluster: %v", err),
		)
		return
	}

	// Update with created values
	r.updateModelFromProto(&data, cluster.Msg.Cluster)

	tflog.Trace(ctx, "created a chalk_kubernetes_cluster resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KubernetesClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data KubernetesClusterResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         r.client.ApiServer,
			interceptors: []connect.Interceptor{MakeApiServerHeaderInterceptor("x-chalk-server", "go-api")},
		},
	)

	// Create cloud components client with token injection interceptor
	cc := NewCloudComponentsClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	})

	cluster, err := cc.GetCloudComponentCluster(ctx, connect.NewRequest(&serverv1.GetCloudComponentClusterRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Kubernetes Cluster",
			fmt.Sprintf("Could not read cluster %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Update the model with the fetched data
	r.updateModelFromProto(&data, cluster.Msg.Cluster)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KubernetesClusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data KubernetesClusterResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         r.client.ApiServer,
			interceptors: []connect.Interceptor{MakeApiServerHeaderInterceptor("x-chalk-server", "go-api")},
		},
	)

	// Create cloud components client with token injection interceptor
	cc := NewCloudComponentsClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	})

	updateReq := &serverv1.UpdateCloudComponentClusterRequest{
		Id: data.Id.ValueString(),
		Cluster: &serverv1.CloudComponentClusterRequest{
			Kind: data.Kind.ValueString(),
			Spec: &serverv1.CloudComponentCluster{
				Name:              data.Name.ValueString(),
				KubernetesVersion: data.KubernetesVersion.ValueString(),
			},
			Managed: data.Managed.ValueBool(),
		},
	}

	if !data.CloudCredentialId.IsNull() {
		credentialId := data.CloudCredentialId.ValueString()
		updateReq.Cluster.CloudCredentialId = &credentialId
	}

	if !data.VpcId.IsNull() {
		vpcId := data.VpcId.ValueString()
		updateReq.Cluster.VpcId = &vpcId
	}

	cluster, err := cc.UpdateCloudComponentCluster(ctx, connect.NewRequest(updateReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Kubernetes Cluster",
			fmt.Sprintf("Could not update cluster: %v", err),
		)
		return
	}

	// Update the model with the updated data
	r.updateModelFromProto(&data, cluster.Msg.Cluster)

	tflog.Trace(ctx, "updated a chalk_kubernetes_cluster resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KubernetesClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data KubernetesClusterResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create auth client first
	authClient := NewAuthClient(
		ctx,
		&GrpcClientOptions{
			httpClient:   &http.Client{},
			host:         r.client.ApiServer,
			interceptors: []connect.Interceptor{MakeApiServerHeaderInterceptor("x-chalk-server", "go-api")},
		},
	)

	// Create cloud components client with token injection interceptor
	cc := NewCloudComponentsClient(ctx, &GrpcClientOptions{
		httpClient: &http.Client{},
		host:       r.client.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			MakeTokenInjectionInterceptor(authClient, r.client.ClientID, r.client.ClientSecret),
		},
	})

	deleteReq := &serverv1.DeleteCloudComponentClusterRequest{
		Id: data.Id.ValueString(),
	}

	_, err := cc.DeleteCloudComponentCluster(ctx, connect.NewRequest(deleteReq))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Kubernetes Cluster",
			fmt.Sprintf("Could not delete cluster %s: %v", data.Id.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "deleted chalk_kubernetes_cluster resource")
}

func (r *KubernetesClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *KubernetesClusterResource) updateModelFromProto(model *KubernetesClusterResourceModel, cluster *serverv1.CloudComponentClusterResponse) {
	model.Id = types.StringValue(cluster.Id)
	model.Name = types.StringValue(cluster.Spec.Name)
	model.Kind = types.StringValue(cluster.Kind)
	model.KubernetesVersion = types.StringValue(cluster.Spec.KubernetesVersion)
	model.Managed = types.BoolValue(cluster.Managed)
	model.TeamId = types.StringValue(cluster.TeamId)

	if cluster.Spec.Designator != nil {
		model.Designator = types.StringValue(*cluster.Spec.Designator)
	} else {
		model.Designator = types.StringNull()
	}

	if cluster.CloudCredentialId != nil {
		model.CloudCredentialId = types.StringValue(*cluster.CloudCredentialId)
	} else {
		model.CloudCredentialId = types.StringNull()
	}

	if cluster.VpcId != nil {
		model.VpcId = types.StringValue(*cluster.VpcId)
	} else {
		model.VpcId = types.StringNull()
	}

}
