package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

var _ resource.Resource = &ManagedAzureVPCResource{}

func NewManagedAzureVPCResource() resource.Resource {
	return &ManagedAzureVPCResource{}
}

type ManagedAzureVPCResource struct{}

func (r *ManagedAzureVPCResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_azure_vpc"
}

func (r *ManagedAzureVPCResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk managed Azure VPC resource. Not yet implemented.",
		Attributes:          map[string]schema.Attribute{},
	}
}

func (r *ManagedAzureVPCResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
}

func (r *ManagedAzureVPCResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError("Not Implemented", "This resource is not yet implemented.")
}

func (r *ManagedAzureVPCResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.Diagnostics.AddError("Not Implemented", "This resource is not yet implemented.")
}

func (r *ManagedAzureVPCResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Not Implemented", "This resource is not yet implemented.")
}

func (r *ManagedAzureVPCResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddError("Not Implemented", "This resource is not yet implemented.")
}