package provider

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var controlRefAttrTypes = map[string]attr.Type{
	"id": types.StringType, "name": types.StringType, "type": types.StringType, "status": types.StringType,
}

var guardrailAttrTypes = map[string]attr.Type{
	"id":                     types.StringType,
	"catalog_id":             types.StringType,
	"name":                   types.StringType,
	"description":            types.StringType,
	"severity":               types.StringType,
	"status":                 types.StringType,
	"coverage":               types.Float64Type,
	"defense_score":          types.Float64Type,
	"has_violations":         types.BoolType,
	"has_effective_exposure": types.BoolType,
	"simulation_results":     types.StringType,
	"risks":                  types.ListType{ElemType: types.StringType},
	"platform_type":          types.StringType,
	"platform_service":       types.StringType,
	"guardrail_type":         types.StringType,
	"security_frameworks":    types.ListType{ElemType: types.StringType},
	"policy_id":              types.StringType,
	"security_controls":      types.ListType{ElemType: types.ObjectType{AttrTypes: controlRefAttrTypes}},
}

func guardrailDSAttrs() map[string]schema.Attribute {
	s := func(d string) schema.StringAttribute { return schema.StringAttribute{Computed: true, Description: d} }
	return map[string]schema.Attribute{
		"id":                     s("Internal guardrail ID."),
		"catalog_id":             s("Catalog ID, e.g. g-aws-s3-73735b0c. Use this in exclusions and deployments."),
		"name":                   s(""),
		"description":            s("Only populated by the single-guardrail lookup."),
		"severity":               s(""),
		"status":                 s("Active / Inactive."),
		"coverage":               schema.Float64Attribute{Computed: true},
		"defense_score":          schema.Float64Attribute{Computed: true},
		"has_violations":         schema.BoolAttribute{Computed: true},
		"has_effective_exposure": schema.BoolAttribute{Computed: true},
		"simulation_results":     s(""),
		"risks":                  schema.ListAttribute{Computed: true, ElementType: types.StringType},
		"platform_type":          s(""),
		"platform_service":       s(""),
		"guardrail_type":         s(""),
		"security_frameworks":    schema.ListAttribute{Computed: true, ElementType: types.StringType},
		"policy_id":              s(""),
		"security_controls": schema.ListNestedAttribute{
			Computed:    true,
			Description: "Security controls currently enforcing this guardrail (single lookup only).",
			NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"id": s(""), "name": s(""), "type": s(""), "status": s(""),
			}},
		},
	}
}

func optFloat(f *float64) types.Float64 {
	if f == nil {
		return types.Float64Null()
	}
	return types.Float64Value(*f)
}

func guardrailValues(g *client.Guardrail) map[string]attr.Value {
	ctrls := make([]attr.Value, len(g.SecurityControls))
	for i, c := range g.SecurityControls {
		ctrls[i] = types.ObjectValueMust(controlRefAttrTypes, map[string]attr.Value{
			"id": types.StringValue(c.ID), "name": types.StringValue(c.Name),
			"type": types.StringValue(c.Type), "status": types.StringValue(c.Status),
		})
	}
	return map[string]attr.Value{
		"id":                     types.StringValue(g.ID),
		"catalog_id":             types.StringValue(g.CatalogID),
		"name":                   types.StringValue(g.Name),
		"description":            types.StringValue(g.Description),
		"severity":               types.StringValue(g.Severity),
		"status":                 types.StringValue(g.Status),
		"coverage":               optFloat(g.Coverage),
		"defense_score":          optFloat(g.DefenseScore),
		"has_violations":         types.BoolValue(g.HasViolations),
		"has_effective_exposure": types.BoolValue(g.HasEffectiveExposure),
		"simulation_results":     types.StringValue(g.SimulationResults),
		"risks":                  toList(g.Risks),
		"platform_type":          types.StringValue(g.PlatformType),
		"platform_service":       types.StringValue(g.PlatformService),
		"guardrail_type":         types.StringValue(g.GuardrailType),
		"security_frameworks":    toList(g.SecurityFrameworks),
		"policy_id":              types.StringValue(g.PolicyID),
		"security_controls":      types.ListValueMust(types.ObjectType{AttrTypes: controlRefAttrTypes}, ctrls),
	}
}

// ---------- blast_guardrail ----------

func NewGuardrailDataSource() datasource.DataSource { return &guardrailDataSource{} }

type guardrailDataSource struct{ client *client.Client }

func (d *guardrailDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_guardrail"
}

func (d *guardrailDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := guardrailDSAttrs()
	attrs["catalog_id"] = schema.StringAttribute{Required: true, Description: "Guardrail catalog ID, e.g. g-aws-s3-73735b0c."}
	attrs["project_id"] = schema.StringAttribute{Optional: true, Description: "Blast project ID. Overrides the provider default."}
	resp.Schema = schema.Schema{Description: "Look up a guardrail by catalog ID.", Attributes: attrs}
}

func (d *guardrailDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, diags := providerClient(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.client = c
}

func (d *guardrailDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var catalogID, project types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, pathRoot("catalog_id"), &catalogID)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, pathRoot("project_id"), &project)...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := d.client.GetGuardrail(ctx, project.ValueString(), catalogID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading guardrail", err.Error())
		return
	}
	vals := guardrailValues(g)
	if g.CatalogID == "" {
		vals["catalog_id"] = catalogID
	}
	resp.Diagnostics.Append(setAll(ctx, &resp.State, vals)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, pathRoot("project_id"), project)...)
}

// ---------- blast_guardrails ----------

func NewGuardrailsDataSource() datasource.DataSource { return &guardrailsDataSource{} }

type guardrailsDataSource struct{ client *client.Client }

type guardrailsModel struct {
	ProjectID         types.String `tfsdk:"project_id"`
	Name              types.String `tfsdk:"name"`
	Severity          types.List   `tfsdk:"severity"`
	Status            types.String `tfsdk:"status"`
	PlatformType      types.List   `tfsdk:"platform_type"`
	PlatformService   types.List   `tfsdk:"platform_service"`
	SecurityFramework types.List   `tfsdk:"security_framework"`
	GuardrailType     types.List   `tfsdk:"guardrail_type"`
	Risk              types.List   `tfsdk:"risk"`
	ProgramName       types.String `tfsdk:"program_name"`
	HasViolations     types.Bool   `tfsdk:"has_violations"`
	CatalogIDs        types.List   `tfsdk:"catalog_ids"`
	Guardrails        types.List   `tfsdk:"guardrails"`
}

func (d *guardrailsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_guardrails"
}

func (d *guardrailsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	multi := func(desc string) schema.ListAttribute {
		return schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: desc}
	}
	attrs := guardrailDSAttrs()
	resp.Schema = schema.Schema{
		Description: "Search the guardrail catalog. All filters are optional and combined with AND; list filters match any value.",
		Attributes: map[string]schema.Attribute{
			"project_id":         schema.StringAttribute{Optional: true, Description: "Blast project ID. Overrides the provider default."},
			"name":               schema.StringAttribute{Optional: true, Description: "Text search by name."},
			"severity":           multi("Low, Medium, High, Critical, Info."),
			"status":             schema.StringAttribute{Optional: true, Description: "Active or Inactive.", Validators: []validator.String{stringvalidator.OneOfCaseInsensitive("Active", "Inactive")}},
			"platform_type":      multi("Amazon Web Services, Azure Cloud, Google Cloud Platform."),
			"platform_service":   multi("e.g. IAM, S3."),
			"security_framework": multi("e.g. CIS AWS Foundations 4.0, PCI DSS v4.0.1."),
			"guardrail_type":     multi("Access Restriction, Configuration Restriction, Defense Hardening."),
			"risk":               multi("e.g. Privilege Escalation, Data Leakage."),
			"program_name":       schema.StringAttribute{Optional: true, Description: "Defense program name."},
			"has_violations":     schema.BoolAttribute{Optional: true},
			"catalog_ids":        schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Catalog IDs of all matches (handy for blast_guardrail_deployment)."},
			"guardrails": schema.ListNestedAttribute{
				Computed:     true,
				NestedObject: schema.NestedAttributeObject{Attributes: attrs},
			},
		},
	}
}

func (d *guardrailsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, diags := providerClient(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.client = c
}

func (d *guardrailsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m guardrailsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	q := url.Values{}
	addStr := func(k string, v types.String) {
		if !v.IsNull() && v.ValueString() != "" {
			q.Set(k, v.ValueString())
		}
	}
	addList := func(k string, l types.List) {
		vals, diags := strList(ctx, l)
		resp.Diagnostics.Append(diags...)
		if len(vals) > 0 {
			q.Set(k, strings.Join(vals, ","))
		}
	}
	addStr("name", m.Name)
	addList("severity", m.Severity)
	addStr("status", m.Status)
	addList("platformType", m.PlatformType)
	addList("platformService", m.PlatformService)
	addList("securityFramework", m.SecurityFramework)
	addList("guardrailType", m.GuardrailType)
	addList("risk", m.Risk)
	addStr("programName", m.ProgramName)
	if !m.HasViolations.IsNull() {
		q.Set("hasViolations", strconv.FormatBool(m.HasViolations.ValueBool()))
	}
	if resp.Diagnostics.HasError() {
		return
	}

	list, err := d.client.ListGuardrails(ctx, m.ProjectID.ValueString(), q)
	if err != nil {
		resp.Diagnostics.AddError("Error listing guardrails", err.Error())
		return
	}
	elems := make([]attr.Value, len(list))
	ids := make([]string, len(list))
	for i := range list {
		elems[i] = types.ObjectValueMust(guardrailAttrTypes, guardrailValues(&list[i]))
		ids[i] = list[i].CatalogID
	}
	m.Guardrails = types.ListValueMust(types.ObjectType{AttrTypes: guardrailAttrTypes}, elems)
	m.CatalogIDs = toList(ids)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}
