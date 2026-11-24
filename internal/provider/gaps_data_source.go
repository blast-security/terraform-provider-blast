package provider

import (
	"context"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &gapsDataSource{}

func NewGapsDataSource() datasource.DataSource { return &gapsDataSource{} }

type gapsDataSource struct{ client *client.Client }

var gapSummaryAttrTypes = map[string]attr.Type{
	"id":             types.StringType,
	"project_id":     types.StringType,
	"external_key":   types.StringType,
	"name":           types.StringType,
	"category":       types.StringType,
	"type":           types.StringType,
	"priority_score": types.Float64Type,
	"priority_level": types.StringType,
	"is_quick_win":   types.BoolType,
	"status":         types.StringType,
	"gain_points":    types.Float64Type,
	"created_at":     types.StringType,
	"updated_at":     types.StringType,
}

type gapsDataSourceModel struct {
	ProjectID     types.String `tfsdk:"project_id"`
	Tab           types.String `tfsdk:"tab"`
	Category      types.List   `tfsdk:"category"`
	Type          types.List   `tfsdk:"type"`
	PriorityLevel types.List   `tfsdk:"priority_level"`
	Status        types.List   `tfsdk:"status"`
	IsQuickWin    types.Bool   `tfsdk:"is_quick_win"`
	ExternalKey   types.String `tfsdk:"external_key"`
	Name          types.String `tfsdk:"name"`
	Gaps          types.List   `tfsdk:"gaps"`
}

func (d *gapsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gaps"
}

func (d *gapsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List defense gaps, with optional filters. Returns summary fields for each gap; use blast_gap for full detail including mitigation and priority indicators.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Optional:    true,
				Description: "Blast project ID. Gaps are project-scoped; this field is required (the default all-units scope is rejected by the API).",
			},
			"tab": schema.StringAttribute{
				Optional:    true,
				Description: "Status group: open (open + in_progress), history (resolved, closed, ignored), or all (no status filter). Defaults to open when unset.",
			},
			"category": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Filter by one or more gap categories.",
			},
			"type": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Filter by one or more gap types.",
			},
			"priority_level": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Filter by one or more priority levels (critical, high, medium, low).",
			},
			"status": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Filter by one or more status values (open, in_progress, resolved, closed, ignored).",
			},
			"is_quick_win": schema.BoolAttribute{
				Optional:    true,
				Description: "Only quick-win (true) or non-quick-win (false) gaps. Unset returns both.",
			},
			"external_key": schema.StringAttribute{
				Optional:    true,
				Description: "Exact match on external_key. Useful for a sync to look up the gap it owns.",
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Description: "Partial match on gap name.",
			},
			"gaps": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of gaps matching the filters.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":             schema.StringAttribute{Computed: true},
						"project_id":     schema.StringAttribute{Computed: true},
						"external_key":   schema.StringAttribute{Computed: true},
						"name":           schema.StringAttribute{Computed: true},
						"category":       schema.StringAttribute{Computed: true},
						"type":           schema.StringAttribute{Computed: true},
						"priority_score": schema.Float64Attribute{Computed: true},
						"priority_level": schema.StringAttribute{Computed: true},
						"is_quick_win":   schema.BoolAttribute{Computed: true},
						"status":         schema.StringAttribute{Computed: true},
						"gain_points":    schema.Float64Attribute{Computed: true},
						"created_at":     schema.StringAttribute{Computed: true},
						"updated_at":     schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *gapsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, diags := providerClient(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.client = c
}

func (d *gapsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m gapsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	filters := client.GapListFilters{
		Tab:         m.Tab.ValueString(),
		ExternalKey: m.ExternalKey.ValueString(),
		Name:        m.Name.ValueString(),
	}
	if !m.IsQuickWin.IsNull() && !m.IsQuickWin.IsUnknown() {
		v := m.IsQuickWin.ValueBool()
		filters.IsQuickWin = &v
	}
	if !m.Category.IsNull() && !m.Category.IsUnknown() {
		resp.Diagnostics.Append(m.Category.ElementsAs(ctx, &filters.Category, false)...)
	}
	if !m.Type.IsNull() && !m.Type.IsUnknown() {
		resp.Diagnostics.Append(m.Type.ElementsAs(ctx, &filters.Type, false)...)
	}
	if !m.PriorityLevel.IsNull() && !m.PriorityLevel.IsUnknown() {
		resp.Diagnostics.Append(m.PriorityLevel.ElementsAs(ctx, &filters.PriorityLevel, false)...)
	}
	if !m.Status.IsNull() && !m.Status.IsUnknown() {
		resp.Diagnostics.Append(m.Status.ElementsAs(ctx, &filters.Status, false)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	list, err := d.client.ListGaps(ctx, m.ProjectID.ValueString(), filters)
	if err != nil {
		resp.Diagnostics.AddError("Error listing blast_gaps", err.Error())
		return
	}

	objType := types.ObjectType{AttrTypes: gapSummaryAttrTypes}
	elems := make([]attr.Value, len(list))
	for i, g := range list {
		elems[i] = types.ObjectValueMust(gapSummaryAttrTypes, map[string]attr.Value{
			"id":             types.StringValue(g.ID),
			"project_id":     types.StringValue(g.ProjectID),
			"external_key":   types.StringValue(g.ExternalKey),
			"name":           types.StringValue(g.Name),
			"category":       types.StringValue(g.Category),
			"type":           types.StringValue(g.Type),
			"priority_score": types.Float64Value(g.PriorityScore),
			"priority_level": types.StringValue(g.PriorityLevel),
			"is_quick_win":   types.BoolValue(g.IsQuickWin),
			"status":         types.StringValue(g.Status),
			"gain_points":    types.Float64Value(g.GainPoints),
			"created_at":     types.StringValue(g.CreatedAt),
			"updated_at":     types.StringValue(g.UpdatedAt),
		})
	}

	var d2 diag.Diagnostics
	m.Gaps, d2 = types.ListValue(objType, elems)
	resp.Diagnostics.Append(d2...)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}
