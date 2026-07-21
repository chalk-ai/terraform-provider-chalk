package provider

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/gen/chalk/server/v1/serverv1connect"
	"github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// vpcPollInterval is how often we poll the server while waiting for a managed
// VPC to be applied or deleted. vpcPollTimeout bounds how long we wait before
// giving up. They are vars (not consts) so tests can shorten them.
//
// vpcPollTimeout must be strictly longer than the server's vpcDeploymentTimeout
// (30m, see go-api-server/cloudcomponents/lifecycle.go): the server lazily
// flips a stuck deployment to FAILED once that deadline passes, so we keep
// polling past it to observe the server's terminal status instead of timing out
// first and reporting a less useful error.
var (
	vpcPollInterval = 10 * time.Second
	vpcPollTimeout  = 35 * time.Minute
)

var _ resource.Resource = &ManagedAWSVPCResource{}
var _ resource.ResourceWithImportState = &ManagedAWSVPCResource{}

func NewManagedAWSVPCResource() resource.Resource {
	return &ManagedAWSVPCResource{}
}

type ManagedAWSVPCResource struct {
	client *client.Manager
}

type ManagedAWSVPCResourceModel struct {
	Id                      types.String `tfsdk:"id"`
	CloudCredentialId       types.String `tfsdk:"cloud_credential_id"`
	CidrBlock               types.String `tfsdk:"cidr_block"`
	AdditionalCidrBlocks    types.List   `tfsdk:"additional_cidr_blocks"`
	Subnets                 types.List   `tfsdk:"subnets"`
	AdditionalPublicRoutes  types.List   `tfsdk:"additional_public_routes"`
	AdditionalPrivateRoutes types.List   `tfsdk:"additional_private_routes"`
}

type SubnetModel struct {
	Name             types.String `tfsdk:"name"`
	PrivateCidrBlock types.String `tfsdk:"private_cidr_block"`
	PublicCidrBlock  types.String `tfsdk:"public_cidr_block"`
	AvailabilityZone types.String `tfsdk:"availability_zone"`
}

type RouteModel struct {
	Name                 types.String `tfsdk:"name"`
	DestinationCidrBlock types.String `tfsdk:"destination_cidr_block"`
	PeerId               types.String `tfsdk:"peer_id"`
}

func (r *ManagedAWSVPCResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_aws_vpc"
}

func (r *ManagedAWSVPCResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk managed AWS VPC resource. Creates a fully managed VPC using the provided cloud credentials.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "VPC identifier",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cloud_credential_id": schema.StringAttribute{
				MarkdownDescription: "ID of the cloud credential to use for the managed VPC",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cidr_block": schema.StringAttribute{
				MarkdownDescription: "The IPv4 CIDR block for the VPC.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"additional_cidr_blocks": schema.ListAttribute{
				MarkdownDescription: "A list of additional IPv4 CIDR blocks for the VPC.",
				ElementType:         types.StringType,
				Optional:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"subnets": schema.ListNestedAttribute{
				MarkdownDescription: "A list of subnets for the VPC.",
				Required:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required: true,
						},
						"private_cidr_block": schema.StringAttribute{
							Required: true,
						},
						"public_cidr_block": schema.StringAttribute{
							Required: true,
						},
						"availability_zone": schema.StringAttribute{
							Required: true,
						},
					},
				},
			},
			"additional_public_routes": schema.ListNestedAttribute{
				MarkdownDescription: "A list of additional public routes for the VPC.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required: true,
						},
						"destination_cidr_block": schema.StringAttribute{
							Required: true,
						},
						"peer_id": schema.StringAttribute{
							Required: true,
						},
					},
				},
			},
			"additional_private_routes": schema.ListNestedAttribute{
				MarkdownDescription: "A list of additional private routes for the VPC.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required: true,
						},
						"destination_cidr_block": schema.StringAttribute{
							Required: true,
						},
						"peer_id": schema.StringAttribute{
							Required: true,
						},
					},
				},
			},
		},
	}
}

func (r *ManagedAWSVPCResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ManagedAWSVPCResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ManagedAWSVPCResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cc := r.client.NewCloudComponentsClient(ctx)
	credentialId := data.CloudCredentialId.ValueString()

	awsVpcConfig, diags := r.buildAwsVpcConfig(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudVpcConfig := &serverv1.CloudVpcConfig{
		Config: &serverv1.CloudVpcConfig_Aws{
			Aws: awsVpcConfig,
		},
	}

	cloudComponentVpc := &serverv1.CloudComponentVpc{
		Config: cloudVpcConfig,
	}

	vpcRequest := &serverv1.CloudComponentVpcRequest{
		Kind:              "aws",
		Spec:              cloudComponentVpc,
		Managed:           true,
		CloudCredentialId: &credentialId,
	}

	createReq := &serverv1.CreateCloudComponentVpcRequest{
		Vpc: vpcRequest,
	}

	vpc, err := cc.CreateCloudComponentVpc(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Managed VPC", fmt.Sprintf("Could not create managed VPC: %v", err))
		return
	}

	// The VPC is provisioned asynchronously. Poll until it reaches a terminal
	// lifecycle status: ACTIVE on success, FAILED otherwise.
	created := vpc.Msg.Vpc
	finalVpc, waitErr := waitForCloudVpcActive(ctx, cc, created.GetId())
	if finalVpc == nil {
		// We never observed a fresh status (e.g. transport error or timeout);
		// fall back to the create response so the resource is still tracked.
		finalVpc = created
	}

	diags = r.updateModelFromProto(ctx, &data, finalVpc)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Persist state before returning any wait error so that the partially
	// created VPC is recorded and Terraform taints it (replacing it on the next
	// apply) instead of leaking an untracked resource.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if waitErr != nil {
		resp.Diagnostics.AddError("Error Waiting for Managed VPC", fmt.Sprintf("Managed VPC %s did not become active: %v", created.GetId(), waitErr))
		return
	}

	tflog.Trace(ctx, "created a chalk_managed_aws_vpc resource")
}

func (r *ManagedAWSVPCResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ManagedAWSVPCResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cc := r.client.NewCloudComponentsClient(ctx)
	vpc, err := cc.GetCloudComponentVpc(ctx, connect.NewRequest(&serverv1.GetCloudComponentVpcRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Managed VPC", fmt.Sprintf("Could not read managed VPC %s: %v", data.Id.ValueString(), err))
		return
	}

	diags := r.updateModelFromProto(ctx, &data, vpc.Msg.Vpc)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ManagedAWSVPCResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddWarning(
		"VPC update not supported",
		"Updating a managed VPC is not supported by the underlying API. The provider will refresh the state from the server, which may overwrite your changes.",
	)

	var data ManagedAWSVPCResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cc := r.client.NewCloudComponentsClient(ctx)
	vpc, err := cc.GetCloudComponentVpc(ctx, connect.NewRequest(&serverv1.GetCloudComponentVpcRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Managed VPC", fmt.Sprintf("Could not read managed VPC %s: %v", data.Id.ValueString(), err))
		return
	}
	diags := r.updateModelFromProto(ctx, &data, vpc.Msg.Vpc)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Trace(ctx, "updated chalk_managed_aws_vpc resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ManagedAWSVPCResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ManagedAWSVPCResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cc := r.client.NewCloudComponentsClient(ctx)
	id := data.Id.ValueString()
	_, err := cc.DeleteCloudComponentVpc(ctx, connect.NewRequest(&serverv1.DeleteCloudComponentVpcRequest{
		Id: id,
	}))
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting Managed VPC", fmt.Sprintf("Could not delete managed VPC %s: %v", id, err))
		return
	}

	// Deletion is asynchronous: the server keeps reporting the VPC with a
	// DELETING status until teardown is confirmed. Poll until it reaches the
	// terminal DELETED status or can no longer be fetched so that Delete only
	// returns once the VPC is actually gone.
	err = pollUntilDeleted(ctx, vpcPollInterval, vpcPollTimeout, func(ctx context.Context) (componentStatus, error) {
		vpc, err := cc.GetCloudComponentVpc(ctx, connect.NewRequest(&serverv1.GetCloudComponentVpcRequest{
			Id: id,
		}))
		if err != nil {
			if isNotFoundErr(err) {
				return componentStatus{found: false}, nil
			}
			return componentStatus{}, err
		}
		return componentStatus{found: true, status: vpc.Msg.Vpc.GetStatus(), statusError: vpc.Msg.Vpc.GetStatusError()}, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Error Waiting for Managed VPC Deletion", fmt.Sprintf("Managed VPC %s was deleted but did not disappear: %v", id, err))
		return
	}

	tflog.Trace(ctx, "deleted chalk_managed_aws_vpc resource")
}

func (r *ManagedAWSVPCResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// waitForCloudVpcActive polls GetCloudComponentVpc until the VPC reaches a
// terminal lifecycle status. It returns the latest VPC response together with a
// nil error once the status is ACTIVE, or the response and a non-nil error when
// the status is FAILED. On a transport error or timeout it returns a nil
// response and the error. It is shared by the AWS and GCP managed VPC resources,
// which use the same CloudComponents endpoints and response type.
func waitForCloudVpcActive(
	ctx context.Context,
	cc serverv1connect.CloudComponentsServiceClient,
	id string,
) (*serverv1.CloudComponentVpcResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, vpcPollTimeout)
	defer cancel()

	ticker := time.NewTicker(vpcPollInterval)
	defer ticker.Stop()

	for {
		resp, err := cc.GetCloudComponentVpc(ctx, connect.NewRequest(&serverv1.GetCloudComponentVpcRequest{
			Id: id,
		}))
		if err != nil {
			return nil, err
		}
		vpc := resp.Msg.Vpc

		if terminal, failure := terminalStatus(vpc.GetStatus(), vpc.GetStatusError()); terminal {
			return vpc, failure
		}

		tflog.Trace(ctx, "waiting for managed VPC to become active", map[string]any{
			"id":     id,
			"status": vpc.GetStatus(),
		})

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out after %s waiting for status %s: %w", vpcPollTimeout, cloudComponentStatusActive, ctx.Err())
		case <-ticker.C:
		}
	}
}

// componentStatus is the lifecycle snapshot fetched while polling a managed
// cloud component during deletion.
type componentStatus struct {
	// found reports whether the component could still be fetched (Get did not
	// return not-found).
	found bool
	// status / statusError are the lifecycle status and failure detail when
	// found is true.
	status      string
	statusError string
}

// pollUntilDeleted repeatedly invokes get until the component has finished
// tearing down. Deletion is only complete once the component can no longer be
// fetched OR it reports the terminal DELETED status; a DELETING (or any other
// in-flight) status keeps polling. This matters because the server keeps the
// metadata record around — reporting DELETING — until the deployer confirms
// teardown, so we must not treat a still-deleting cluster as gone. A FAILED
// status or a transport error aborts the wait. It gives up once timeout elapses.
func pollUntilDeleted(ctx context.Context, interval, timeout time.Duration, get func(context.Context) (componentStatus, error)) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		cur, err := get(ctx)
		if err != nil {
			return err
		}

		if done, failure := deletionComplete(cur.found, cur.status, cur.statusError); done {
			return failure
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out after %s waiting for resource to be deleted (last status %q): %w", timeout, cur.status, ctx.Err())
		case <-ticker.C:
		}
	}
}

// isNotFoundErr reports whether err is a connect not-found error.
func isNotFoundErr(err error) bool {
	return err != nil && connect.CodeOf(err) == connect.CodeNotFound
}

var subnetAttrTypes = map[string]attr.Type{
	"name":               types.StringType,
	"private_cidr_block": types.StringType,
	"public_cidr_block":  types.StringType,
	"availability_zone":  types.StringType,
}

var routeAttrTypes = map[string]attr.Type{
	"name":                   types.StringType,
	"destination_cidr_block": types.StringType,
	"peer_id":                types.StringType,
}

func (r *ManagedAWSVPCResource) buildAwsVpcConfig(ctx context.Context, data ManagedAWSVPCResourceModel) (*serverv1.AWSVpcConfig, diag.Diagnostics) {
	var diags diag.Diagnostics

	awsVpcConfig := &serverv1.AWSVpcConfig{
		CidrBlock: data.CidrBlock.ValueString(),
	}

	if !data.AdditionalCidrBlocks.IsNull() && !data.AdditionalCidrBlocks.IsUnknown() {
		cidrs := make([]string, 0, len(data.AdditionalCidrBlocks.Elements()))
		diags.Append(data.AdditionalCidrBlocks.ElementsAs(ctx, &cidrs, false)...)
		awsVpcConfig.AdditionalCidrBlocks = cidrs
	}

	if !data.Subnets.IsNull() && !data.Subnets.IsUnknown() {
		var subnets []SubnetModel
		diags.Append(data.Subnets.ElementsAs(ctx, &subnets, false)...)
		for _, s := range subnets {
			awsVpcConfig.Subnets = append(awsVpcConfig.Subnets, &serverv1.AwsSubnetConfig{
				Name:             s.Name.ValueString(),
				PrivateCidrBlock: s.PrivateCidrBlock.ValueString(),
				PublicCidrBlock:  s.PublicCidrBlock.ValueString(),
				AvailabilityZone: s.AvailabilityZone.ValueString(),
			})
		}
	}

	if !data.AdditionalPublicRoutes.IsNull() && !data.AdditionalPublicRoutes.IsUnknown() {
		var routes []RouteModel
		diags.Append(data.AdditionalPublicRoutes.ElementsAs(ctx, &routes, false)...)
		for _, r := range routes {
			awsVpcConfig.AdditionalPublicRoutes = append(awsVpcConfig.AdditionalPublicRoutes, &serverv1.AWSVpcRoute{
				Name:                 r.Name.ValueString(),
				DestinationCidrBlock: r.DestinationCidrBlock.ValueString(),
				PeerId:               r.PeerId.ValueString(),
			})
		}
	}

	if !data.AdditionalPrivateRoutes.IsNull() && !data.AdditionalPrivateRoutes.IsUnknown() {
		var routes []RouteModel
		diags.Append(data.AdditionalPrivateRoutes.ElementsAs(ctx, &routes, false)...)
		for _, r := range routes {
			awsVpcConfig.AdditionalPrivateRoutes = append(awsVpcConfig.AdditionalPrivateRoutes, &serverv1.AWSVpcRoute{
				Name:                 r.Name.ValueString(),
				DestinationCidrBlock: r.DestinationCidrBlock.ValueString(),
				PeerId:               r.PeerId.ValueString(),
			})
		}
	}

	return awsVpcConfig, diags
}

func (r *ManagedAWSVPCResource) updateModelFromProto(ctx context.Context, model *ManagedAWSVPCResourceModel, vpc *serverv1.CloudComponentVpcResponse) diag.Diagnostics {
	var diags diag.Diagnostics

	model.Id = types.StringValue(vpc.Id)

	if vpc.CloudCredentialId != nil {
		model.CloudCredentialId = types.StringValue(*vpc.CloudCredentialId)
	} else {
		model.CloudCredentialId = types.StringNull()
	}

	if vpc.Spec != nil && vpc.Spec.Config != nil {
		if config, ok := vpc.Spec.Config.Config.(*serverv1.CloudVpcConfig_Aws); ok {
			if config.Aws != nil {
				model.CidrBlock = types.StringValue(config.Aws.CidrBlock)

				var aditionalCidrsDiags diag.Diagnostics
				model.AdditionalCidrBlocks, aditionalCidrsDiags = types.ListValueFrom(ctx, types.StringType, config.Aws.AdditionalCidrBlocks)
				diags.Append(aditionalCidrsDiags...)

				if len(config.Aws.Subnets) > 0 {
					subnets := make([]SubnetModel, len(config.Aws.Subnets))
					for i, s := range config.Aws.Subnets {
						subnets[i] = SubnetModel{
							Name:             types.StringValue(s.Name),
							PrivateCidrBlock: types.StringValue(s.PrivateCidrBlock),
							PublicCidrBlock:  types.StringValue(s.PublicCidrBlock),
							AvailabilityZone: types.StringValue(s.AvailabilityZone),
						}
					}
					var subnetsDiags diag.Diagnostics
					model.Subnets, subnetsDiags = types.ListValueFrom(ctx, types.ObjectType{AttrTypes: subnetAttrTypes}, subnets)
					diags.Append(subnetsDiags...)
				} else {
					model.Subnets = types.ListNull(types.ObjectType{AttrTypes: subnetAttrTypes})
				}

				routeObjectType := types.ObjectType{AttrTypes: routeAttrTypes}

				if len(config.Aws.AdditionalPublicRoutes) > 0 {
					routes := make([]RouteModel, len(config.Aws.AdditionalPublicRoutes))
					for i, r := range config.Aws.AdditionalPublicRoutes {
						routes[i] = RouteModel{
							Name:                 types.StringValue(r.Name),
							DestinationCidrBlock: types.StringValue(r.DestinationCidrBlock),
							PeerId:               types.StringValue(r.PeerId),
						}
					}
					var routesDiags diag.Diagnostics
					model.AdditionalPublicRoutes, routesDiags = types.ListValueFrom(ctx, routeObjectType, routes)
					diags.Append(routesDiags...)
				} else {
					model.AdditionalPublicRoutes = types.ListNull(routeObjectType)
				}

				if len(config.Aws.AdditionalPrivateRoutes) > 0 {
					routes := make([]RouteModel, len(config.Aws.AdditionalPrivateRoutes))
					for i, r := range config.Aws.AdditionalPrivateRoutes {
						routes[i] = RouteModel{
							Name:                 types.StringValue(r.Name),
							DestinationCidrBlock: types.StringValue(r.DestinationCidrBlock),
							PeerId:               types.StringValue(r.PeerId),
						}
					}
					var routesDiags diag.Diagnostics
					model.AdditionalPrivateRoutes, routesDiags = types.ListValueFrom(ctx, routeObjectType, routes)
					diags.Append(routesDiags...)
				} else {
					model.AdditionalPrivateRoutes = types.ListNull(routeObjectType)
				}

			} else {
				model.CidrBlock = types.StringNull()
				model.AdditionalCidrBlocks = types.ListNull(types.StringType)
				model.Subnets = types.ListNull(types.ObjectType{AttrTypes: subnetAttrTypes})
				model.AdditionalPublicRoutes = types.ListNull(types.ObjectType{AttrTypes: routeAttrTypes})
				model.AdditionalPrivateRoutes = types.ListNull(types.ObjectType{AttrTypes: routeAttrTypes})
			}
		}
	} else {
		model.CidrBlock = types.StringNull()
		model.AdditionalCidrBlocks = types.ListNull(types.StringType)
		model.Subnets = types.ListNull(types.ObjectType{AttrTypes: subnetAttrTypes})
		model.AdditionalPublicRoutes = types.ListNull(types.ObjectType{AttrTypes: routeAttrTypes})
		model.AdditionalPrivateRoutes = types.ListNull(types.ObjectType{AttrTypes: routeAttrTypes})
	}

	return diags
}
