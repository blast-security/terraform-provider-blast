package provider

import (
	"context"
	"strings"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---------- shared shape ----------

var exclusionObjAttrTypes = map[string]attr.Type{
	"id":                    types.StringType,
	"name":                  types.StringType,
	"note":                  types.StringType,
	"platform_type":         types.StringType,
	"guardrail_catalog_ids": types.ListType{ElemType: types.StringType},
	"sources":               types.ListType{ElemType: types.ObjectType{AttrTypes: ruleAttrTypes}},
	"targets":               types.ListType{ElemType: types.ObjectType{AttrTypes: ruleAttrTypes}},
	"units":                 types.ListType{ElemType: types.StringType},
	"expiration_date":       types.StringType,
	"is_expired":            types.BoolType,
	"created_at":            types.StringType,
	"created_by":            types.StringType,
	"updated_at":            types.StringType,
}

func ruleDSAttr(desc string) schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Computed:    true,
		Description: desc,
		NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"identifier": schema.StringAttribute{Computed: true},
			"operator":   schema.StringAttribute{Computed: true},
			"values":     schema.ListAttribute{Computed: true, ElementType: types.StringType},
		}},
	}
}

func exclusionDSAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id":                    schema.StringAttribute{Computed: true},
		"name":                  schema.StringAttribute{Computed: true},
		"note":                  schema.StringAttribute{Computed: true},
		"platform_type":         schema.StringAttribute{Computed: true},
		"guardrail_catalog_ids": schema.ListAttribute{Computed: true, ElementType: types.StringType},
		"sources":               ruleDSAttr("Source match rules."),
		"targets":               ruleDSAttr("Target match rules."),
		"units":                 schema.ListAttribute{Computed: true, ElementType: types.StringType},
		"expiration_date":       schema.StringAttribute{Computed: true},
		"is_expired":            schema.BoolAttribute{Computed: true},
		"created_at":            schema.StringAttribute{Computed: true},
		"created_by":            schema.StringAttribute{Computed: true},
		"updated_at":            schema.StringAttribute{Computed: true},
	}
}

func rulesValue(api []client.ExclusionRule) types.List {
	objType := types.ObjectType{AttrTypes: ruleAttrTypes}
	elems := make([]attr.Value, len(api))
	for i, r := range api {
		elems[i] = types.ObjectValueMust(ruleAttrTypes, map[string]attr.Value{
			"identifier": types.StringValue(r.Identifier),
			"operator":   types.StringValue(r.Operator),
			"values":     toList(r.Values),
		})
	}
	return types.ListValueMust(objType, elems)
}

func exclusionValues(e *client.Exclusion) map[string]attr.Value {
	ids := e.GuardrailCatalogIDs
	if len(ids) == 0 && e.GuardrailID != "" {
		ids = []string{e.GuardrailID}
	}
	return map[string]attr.Value{
		"id":                    types.StringValue(e.ID),
		"name":                  types.StringValue(e.Name),
		"note":                  types.StringValue(e.Note),
		"platform_type":         types.StringValue(e.PlatformType),
		"guardrail_catalog_ids": toList(ids),
		"sources":               rulesValue(e.Sources),
		"targets":               rulesValue(e.Targets),
		"units":                 toList(e.Units),
		"expiration_date":       types.StringValue(e.ExpirationDate),
		"is_expired":            types.BoolValue(e.IsExpired),
		"created_at":            types.StringValue(e.CreatedAt),
		"created_by":            types.StringValue(e.CreatedBy),
		"updated_at":            types.StringValue(e.UpdatedAt),
	}
}

// ---------- blast_exclusion ----------

func NewExclusionDataSource() datasource.DataSource { return &exclusionDataSource{} }

type exclusionDataSource struct{ client *client.Client }

func (d *exclusionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_exclusion"
}

func (d *exclusionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := exclusionDSAttrs()
	attrs["id"] = schema.StringAttribute{Required: true, Description: "Exclusion ID."}
	attrs["project_id"] = schema.StringAttribute{Optional: true, Description: "Blast project ID. Overrides the provider default."}
	resp.Schema = schema.Schema{Description: "Look up a single exclusion rule by ID.", Attributes: attrs}
}

func (d *exclusionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, diags := providerClient(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.client = c
}

func (d *exclusionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id, project types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, pathRoot("id"), &id)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, pathRoot("project_id"), &project)...)
	if resp.Diagnostics.HasError() {
		return
	}
	e, err := d.client.GetExclusion(ctx, project.ValueString(), id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading exclusion", err.Error())
		return
	}
	vals := exclusionValues(e)
	resp.Diagnostics.Append(setAll(ctx, &resp.State, vals)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, pathRoot("project_id"), project)...)
}

// ---------- blast_exclusions ----------

func NewExclusionsDataSource() datasource.DataSource { return &exclusionsDataSource{} }

type exclusionsDataSource struct{ client *client.Client }

type exclusionsModel struct {
	ProjectID    types.String `tfsdk:"project_id"`
	GuardrailID  types.String `tfsdk:"guardrail_id"`
	NameContains types.String `tfsdk:"name_contains"`
	Exclusions   types.List   `tfsdk:"exclusions"`
}

func (d *exclusionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_exclusions"
}

func (d *exclusionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List exclusion rules, optionally for one guardrail.",
		Attributes: map[string]schema.Attribute{
			"project_id":    schema.StringAttribute{Optional: true, Description: "Blast project ID. Overrides the provider default."},
			"guardrail_id":  schema.StringAttribute{Optional: true, Description: "Only exclusions attached to this guardrail."},
			"name_contains": schema.StringAttribute{Optional: true, Description: "Client-side case-insensitive substring filter on name."},
			"exclusions": schema.ListNestedAttribute{
				Computed:     true,
				NestedObject: schema.NestedAttributeObject{Attributes: exclusionDSAttrs()},
			},
		},
	}
}

func (d *exclusionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, diags := providerClient(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.client = c
}

func (d *exclusionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m exclusionsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListExclusions(ctx, m.ProjectID.ValueString(), m.GuardrailID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing exclusions", err.Error())
		return
	}
	needle := strings.ToLower(m.NameContains.ValueString())
	objType := types.ObjectType{AttrTypes: exclusionObjAttrTypes}
	var elems []attr.Value
	for i := range list {
		if needle != "" && !strings.Contains(strings.ToLower(list[i].Name), needle) {
			continue
		}
		obj, diags := types.ObjectValue(exclusionObjAttrTypes, exclusionValues(&list[i]))
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}
	var diags diag.Diagnostics
	m.Exclusions, diags = types.ListValue(objType, elems)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}
