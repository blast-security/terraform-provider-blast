package provider

import (
	"context"
	"net/url"
	"strings"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---------- shared ----------

var projectAttrTypes = map[string]attr.Type{
	"id":              types.StringType,
	"name":            types.StringType,
	"description":     types.StringType,
	"status":          types.StringType,
	"default":         types.BoolType,
	"types":           types.ListType{ElemType: types.StringType},
	"platform_types":  types.ListType{ElemType: types.StringType},
	"number_of_units": types.Int64Type,
	"unit_ids":        types.ListType{ElemType: types.StringType},
}

func projectDSAttrs() map[string]schema.Attribute {
	s := func(d string) schema.StringAttribute { return schema.StringAttribute{Computed: true, Description: d} }
	l := func(d string) schema.ListAttribute {
		return schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: d}
	}
	return map[string]schema.Attribute{
		"id":              s("Project ID. Use it as `project_id` on other Blast resources."),
		"name":            s(""),
		"description":     s(""),
		"status":          s("Active or Inactive."),
		"default":         schema.BoolAttribute{Computed: true, Description: "True for the tenant default project (\"All Cloud Units\")."},
		"types":           l("Development, Test, Production (Default for the default project)."),
		"platform_types":  l(""),
		"number_of_units": schema.Int64Attribute{Computed: true},
		"unit_ids":        l("Platform unit IDs in the project (only filled by the single-project lookup)."),
	}
}

func projectValues(p *client.Project) map[string]attr.Value {
	ids := make([]string, len(p.Units))
	for i, u := range p.Units {
		ids[i] = u.ID
	}
	n := int64(len(p.Units))
	if p.NumberOfUnits != nil {
		n = int64(*p.NumberOfUnits)
	}
	return map[string]attr.Value{
		"id":              types.StringValue(p.ID),
		"name":            types.StringValue(p.Name),
		"description":     types.StringValue(p.Description),
		"status":          types.StringValue(p.Status),
		"default":         types.BoolValue(p.Default),
		"types":           toList(p.Types),
		"platform_types":  toList(p.PlatformTypes),
		"number_of_units": types.Int64Value(n),
		"unit_ids":        toList(ids),
	}
}

// ---------- blast_project ----------

func NewProjectDataSource() datasource.DataSource { return &projectDataSource{} }

type projectDataSource struct{ client *client.Client }

func (d *projectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *projectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := projectDSAttrs()
	attrs["id"] = schema.StringAttribute{Optional: true, Computed: true, Description: "Project ID. Set exactly one of id or name."}
	attrs["name"] = schema.StringAttribute{Optional: true, Computed: true, Description: "Exact project name (case-insensitive). Set exactly one of id or name."}
	resp.Schema = schema.Schema{Description: "Look up a Blast project by ID or name.", Attributes: attrs}
}

func (d *projectDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(path.MatchRoot("id"), path.MatchRoot("name")),
	}
}

func (d *projectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, diags := providerClient(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.client = c
}

func (d *projectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id, name types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("name"), &name)...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectID := id.ValueString()
	if projectID == "" {
		list, err := d.client.ListProjects(ctx, url.Values{"search": {name.ValueString()}})
		if err != nil {
			resp.Diagnostics.AddError("Error listing projects", err.Error())
			return
		}
		for _, p := range list {
			if strings.EqualFold(p.Name, name.ValueString()) {
				projectID = p.ID
				break
			}
		}
		if projectID == "" {
			resp.Diagnostics.AddAttributeError(path.Root("name"), "Project not found", "No Blast project is named "+name.ValueString()+".")
			return
		}
	}
	p, err := d.client.GetProject(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading project", err.Error())
		return
	}
	resp.Diagnostics.Append(setAll(ctx, &resp.State, projectValues(p))...)
}

// ---------- blast_projects ----------

func NewProjectsDataSource() datasource.DataSource { return &projectsDataSource{} }

type projectsDataSource struct{ client *client.Client }

type projectsModel struct {
	Search       types.String `tfsdk:"search"`
	Type         types.List   `tfsdk:"type"`
	Status       types.String `tfsdk:"status"`
	PlatformType types.List   `tfsdk:"platform_type"`
	IDs          types.List   `tfsdk:"ids"`
	Projects     types.List   `tfsdk:"projects"`
}

func (d *projectsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_projects"
}

func (d *projectsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	multi := func(desc string) schema.ListAttribute {
		return schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: desc}
	}
	resp.Schema = schema.Schema{
		Description: "List Blast projects.",
		Attributes: map[string]schema.Attribute{
			"search":        schema.StringAttribute{Optional: true, Description: "Text search on the project name."},
			"type":          multi("Development, Test, Production, Default."),
			"status":        schema.StringAttribute{Optional: true, Description: "Active or Inactive."},
			"platform_type": multi("Amazon Web Services, Azure Cloud, Google Cloud Platform."),
			"ids":           schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"projects": schema.ListNestedAttribute{
				Computed:     true,
				NestedObject: schema.NestedAttributeObject{Attributes: projectDSAttrs()},
			},
		},
	}
}

func (d *projectsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, diags := providerClient(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.client = c
}

func (d *projectsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m projectsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	q := url.Values{}
	if v := m.Search.ValueString(); v != "" {
		q.Set("search", v)
	}
	if v := m.Status.ValueString(); v != "" {
		q.Set("status", v)
	}
	for k, l := range map[string]types.List{"type": m.Type, "platformType": m.PlatformType} {
		vals, diags := strList(ctx, l)
		resp.Diagnostics.Append(diags...)
		if len(vals) > 0 {
			q.Set(k, strings.Join(vals, ","))
		}
	}
	list, err := d.client.ListProjects(ctx, q)
	if err != nil {
		resp.Diagnostics.AddError("Error listing projects", err.Error())
		return
	}
	elems := make([]attr.Value, len(list))
	ids := make([]string, len(list))
	for i := range list {
		elems[i] = types.ObjectValueMust(projectAttrTypes, projectValues(&list[i]))
		ids[i] = list[i].ID
	}
	m.Projects = types.ListValueMust(types.ObjectType{AttrTypes: projectAttrTypes}, elems)
	m.IDs = toList(ids)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

// ---------- blast_cloud_units ----------

func NewCloudUnitsDataSource() datasource.DataSource { return &cloudUnitsDataSource{} }

type cloudUnitsDataSource struct{ client *client.Client }

type cloudUnitsModel struct {
	ProjectID    types.String `tfsdk:"project_id"`
	Type         types.String `tfsdk:"type"`
	PlatformType types.String `tfsdk:"platform_type"`
	NameContains types.String `tfsdk:"name_contains"`
	ParentID     types.String `tfsdk:"parent_id"`
	IDs          types.List   `tfsdk:"ids"`
	Units        types.List   `tfsdk:"units"`
}

var orgUnitAttrTypes = map[string]attr.Type{
	"id": types.StringType, "name": types.StringType, "type": types.StringType,
	"platform_type": types.StringType, "platform_org_id": types.StringType,
	"monitoring_status": types.StringType, "parent_id": types.StringType,
}

func (d *cloudUnitsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_units"
}

func (d *cloudUnitsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := func() schema.StringAttribute { return schema.StringAttribute{Computed: true} }
	resp.Schema = schema.Schema{
		Description: "List onboarded cloud units (organization roots, OUs, accounts, subscriptions...). Use the IDs as `unit_ids` on blast_project.",
		Attributes: map[string]schema.Attribute{
			"project_id":    schema.StringAttribute{Optional: true, Description: "Only units of this project."},
			"type":          schema.StringAttribute{Optional: true, Description: "Client-side filter on the unit type, e.g. ACCOUNT or ORGANIZATIONAL_UNIT (case-insensitive)."},
			"platform_type": schema.StringAttribute{Optional: true, Description: "Client-side filter on the platform, e.g. Amazon Web Services."},
			"name_contains": schema.StringAttribute{Optional: true, Description: "Client-side case-insensitive substring filter on the unit name."},
			"parent_id":     schema.StringAttribute{Optional: true, Description: "Only direct children of this unit."},
			"ids":           schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"units": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id": s(), "name": s(), "type": s(), "platform_type": s(),
					"platform_org_id": s(), "monitoring_status": s(), "parent_id": s(),
				}},
			},
		},
	}
}

func (d *cloudUnitsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, diags := providerClient(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.client = c
}

func (d *cloudUnitsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m cloudUnitsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListOrgUnits(ctx, m.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing cloud units", err.Error())
		return
	}
	match := func(filter types.String, v string, labels []string) bool {
		f := filter.ValueString()
		return f == "" || strings.EqualFold(f, v) ||
			(labels != nil && strings.EqualFold(canonicalEnum(f, labels), canonicalEnum(v, labels)))
	}
	needle := strings.ToLower(m.NameContains.ValueString())
	var elems []attr.Value
	var ids []string
	for _, u := range list {
		if !match(m.Type, u.Type, nil) || !match(m.PlatformType, u.PlatformType, platformTypes) {
			continue
		}
		if p := m.ParentID.ValueString(); p != "" && u.ParentID != p {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(u.Name), needle) {
			continue
		}
		elems = append(elems, types.ObjectValueMust(orgUnitAttrTypes, map[string]attr.Value{
			"id": types.StringValue(u.ID), "name": types.StringValue(u.Name), "type": types.StringValue(u.Type),
			"platform_type": types.StringValue(u.PlatformType), "platform_org_id": types.StringValue(u.PlatformOrgID),
			"monitoring_status": types.StringValue(u.MonitoringStatus), "parent_id": types.StringValue(u.ParentID),
		}))
		ids = append(ids, u.ID)
	}
	m.Units = types.ListValueMust(types.ObjectType{AttrTypes: orgUnitAttrTypes}, elems)
	m.IDs = toList(ids)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}
