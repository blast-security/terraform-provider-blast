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

var _ datasource.DataSource = &automationRulesDataSource{}

func NewAutomationRulesDataSource() datasource.DataSource { return &automationRulesDataSource{} }

type automationRulesDataSource struct{ client *client.Client }

var automationRuleSummaryAttrTypes = map[string]attr.Type{
	"id":           types.StringType,
	"name":         types.StringType,
	"trigger":      types.StringType,
	"active":       types.BoolType,
	"destinations": types.ListType{ElemType: types.StringType},
	"modified_by":  types.StringType,
	"modified_at":  types.StringType,
}

type automationRulesDSModel struct {
	ProjectID types.String `tfsdk:"project_id"`
	Trigger   types.String `tfsdk:"trigger"`
	Active    types.Bool   `tfsdk:"active"`
	Rules     types.List   `tfsdk:"rules"`
}

func (d *automationRulesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_automation_rules"
}

func (d *automationRulesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List automation rules, optionally filtered by trigger or active state.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{Optional: true, Description: "Blast project ID. Overrides the provider default."},
			"trigger":    schema.StringAttribute{Optional: true, Description: "Filter by trigger type (e.g. BLOCKED_ACTIVITY)."},
			"active":     schema.BoolAttribute{Optional: true, Description: "Only active (true) or disabled (false) rules. Unset returns both."},
			"rules": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":           schema.StringAttribute{Computed: true},
						"name":         schema.StringAttribute{Computed: true},
						"trigger":      schema.StringAttribute{Computed: true},
						"active":       schema.BoolAttribute{Computed: true},
						"destinations": schema.ListAttribute{Computed: true, ElementType: types.StringType},
						"modified_by":  schema.StringAttribute{Computed: true},
						"modified_at":  schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *automationRulesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, diags := providerClient(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.client = c
}

func (d *automationRulesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m automationRulesDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Unset (null) means no filter; false is a real filter for disabled rules.
	var active *bool
	if !m.Active.IsNull() && !m.Active.IsUnknown() {
		v := m.Active.ValueBool()
		active = &v
	}

	list, err := d.client.ListAutomationRules(ctx, m.ProjectID.ValueString(), m.Trigger.ValueString(), active)
	if err != nil {
		resp.Diagnostics.AddError("Error listing automation rules", err.Error())
		return
	}

	objType := types.ObjectType{AttrTypes: automationRuleSummaryAttrTypes}
	elems := make([]attr.Value, len(list))
	for i, r := range list {
		elems[i] = types.ObjectValueMust(automationRuleSummaryAttrTypes, map[string]attr.Value{
			"id":           types.StringValue(r.ID),
			"name":         types.StringValue(r.Name),
			"trigger":      types.StringValue(r.Trigger),
			"active":       types.BoolValue(r.Active),
			"destinations": toList(r.Destinations),
			"modified_by":  types.StringValue(r.ModifiedBy),
			"modified_at":  types.StringValue(r.ModifiedAt),
		})
	}

	var d2 diag.Diagnostics
	m.Rules, d2 = types.ListValue(objType, elems)
	resp.Diagnostics.Append(d2...)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}
