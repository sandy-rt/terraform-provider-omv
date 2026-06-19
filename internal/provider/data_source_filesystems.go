package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &filesystemsDataSource{}

type filesystemsDataSource struct {
	data *providerData
}

type filesystemsModel struct {
	Filesystems []filesystemModel `tfsdk:"filesystems"`
}

type filesystemModel struct {
	UUID        types.String `tfsdk:"uuid"`
	Description types.String `tfsdk:"description"`
}

type omvCandidate struct {
	UUID        string `json:"uuid"`
	Description string `json:"description"`
}

func NewFilesystemsDataSource() datasource.DataSource { return &filesystemsDataSource{} }

func (d *filesystemsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_filesystems"
}

func (d *filesystemsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Filesystems available to host shared folders (ShareMgmt.getCandidates).",
		Attributes: map[string]schema.Attribute{
			"filesystems": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"uuid": schema.StringAttribute{
							Computed:    true,
							Description: "Filesystem mntent UUID (use as shared folder mntentref).",
						},
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "Human-readable description, e.g. '/dev/md0 [data]'.",
						},
					},
				},
			},
		},
	}
}

func (d *filesystemsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.data = req.ProviderData.(*providerData)
}

func (d *filesystemsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	raw, err := d.data.client.Call("ShareMgmt", "getCandidates", nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list filesystem candidates", err.Error())
		return
	}
	var candidates []omvCandidate
	if err := json.Unmarshal(raw, &candidates); err != nil {
		resp.Diagnostics.AddError("Failed to parse candidates", err.Error())
		return
	}
	var state filesystemsModel
	for _, c := range candidates {
		state.Filesystems = append(state.Filesystems, filesystemModel{
			UUID:        types.StringValue(c.UUID),
			Description: types.StringValue(c.Description),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
