package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/sandy-rt/terraform-provider-omv/internal/client"
)

var (
	_ resource.Resource                = &sharedFolderResource{}
	_ resource.ResourceWithImportState = &sharedFolderResource{}
)

type sharedFolderResource struct {
	data *providerData
}

type sharedFolderModel struct {
	UUID       types.String `tfsdk:"uuid"`
	Name       types.String `tfsdk:"name"`
	Comment    types.String `tfsdk:"comment"`
	MntEntRef  types.String `tfsdk:"mntentref"`
	RelDirPath types.String `tfsdk:"reldirpath"`
	MountPoint types.String `tfsdk:"mountpoint"`
}

// omvSharedFolder mirrors ShareMgmt.get / ShareMgmt.set response fields.
type omvSharedFolder struct {
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	Comment    string `json:"comment"`
	MntEntRef  string `json:"mntentref"`
	RelDirPath string `json:"reldirpath"`
	MountPoint string `json:"mountpoint"`
}

func NewSharedFolderResource() resource.Resource { return &sharedFolderResource{} }

func (r *sharedFolderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_shared_folder"
}

func (r *sharedFolderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An OpenMediaVault shared folder (ShareMgmt).",
		Attributes: map[string]schema.Attribute{
			"uuid": schema.StringAttribute{
				Computed:      true,
				Description:   "OMV-assigned shared folder UUID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Shared folder name.",
			},
			"comment": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Comment. Defaults to the name.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"mntentref": schema.StringAttribute{
				Required:    true,
				Description: "UUID of the filesystem (mntent) the folder lives on. See the omv_filesystems data source.",
			},
			"reldirpath": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Relative directory path. Defaults to <name>/.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"mountpoint": schema.StringAttribute{
				Computed:      true,
				Description:   "Absolute mountpoint of the filesystem.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *sharedFolderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.data = req.ProviderData.(*providerData)
}

func (r *sharedFolderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sharedFolderModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	comment := plan.Comment.ValueString()
	if comment == "" {
		comment = name
	}
	reldir := plan.RelDirPath.ValueString()
	if reldir == "" {
		reldir = name + "/"
	}

	params := map[string]interface{}{
		"uuid":       client.NewObjectUUID,
		"name":       name,
		"comment":    comment,
		"mntentref":  plan.MntEntRef.ValueString(),
		"reldirpath": reldir,
	}
	raw, err := r.data.client.Call("ShareMgmt", "set", params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create shared folder", err.Error())
		return
	}
	// Changes are only staged here; deployment happens via the omv_apply resource.

	var folder omvSharedFolder
	if err := json.Unmarshal(raw, &folder); err != nil || folder.UUID == "" {
		resp.Diagnostics.AddError("Unexpected create response", fmt.Sprintf("could not read uuid from: %s", string(raw)))
		return
	}
	// Re-read to populate computed fields (mountpoint etc.).
	r.readInto(ctx, folder.UUID, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sharedFolderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sharedFolderModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found := r.readInto(ctx, state.UUID.ValueString(), &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sharedFolderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state sharedFolderModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	comment := plan.Comment.ValueString()
	if comment == "" {
		comment = plan.Name.ValueString()
	}
	reldir := plan.RelDirPath.ValueString()
	if reldir == "" {
		reldir = plan.Name.ValueString() + "/"
	}

	params := map[string]interface{}{
		"uuid":       state.UUID.ValueString(),
		"name":       plan.Name.ValueString(),
		"comment":    comment,
		"mntentref":  plan.MntEntRef.ValueString(),
		"reldirpath": reldir,
	}
	if _, err := r.data.client.Call("ShareMgmt", "set", params); err != nil {
		resp.Diagnostics.AddError("Failed to update shared folder", err.Error())
		return
	}
	// Staged only; deployment happens via the omv_apply resource.
	r.readInto(ctx, state.UUID.ValueString(), &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sharedFolderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.data.allowDestroy {
		resp.Diagnostics.AddError(
			"Destroy is disabled (safety guard)",
			"The provider has allow_destroy=false, so it will not delete shared folders. "+
				"Set allow_destroy=true in the provider block only if you really intend to delete this share.",
		)
		return
	}
	var state sharedFolderModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.data.client.Call("ShareMgmt", "delete", map[string]interface{}{"uuid": state.UUID.ValueString()}); err != nil {
		resp.Diagnostics.AddError("Failed to delete shared folder", err.Error())
		return
	}
	// Staged only; deployment happens via the omv_apply resource.
}

func (r *sharedFolderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("uuid"), req, resp)
}

// readInto loads the folder from OMV into model. Returns false if not found.
func (r *sharedFolderResource) readInto(_ context.Context, uuid string, model *sharedFolderModel, diags *diag.Diagnostics) bool {
	raw, err := r.data.client.Call("ShareMgmt", "get", map[string]interface{}{"uuid": uuid})
	if err != nil {
		// Treat a lookup failure as "gone" so Terraform can recreate/cleanup state.
		return false
	}
	var folder omvSharedFolder
	if err := json.Unmarshal(raw, &folder); err != nil {
		diags.AddError("Failed to parse shared folder", err.Error())
		return false
	}
	model.UUID = types.StringValue(folder.UUID)
	model.Name = types.StringValue(folder.Name)
	model.Comment = types.StringValue(folder.Comment)
	model.MntEntRef = types.StringValue(folder.MntEntRef)
	model.RelDirPath = types.StringValue(folder.RelDirPath)
	model.MountPoint = types.StringValue(folder.MountPoint)
	return true
}
