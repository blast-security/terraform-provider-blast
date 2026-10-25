package provider

import (
	"context"
	"net/url"
	"strings"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var platformResourceAttrTypes = map[string]attr.Type{
	"resource_id":            types.StringType,
	"resource_name":          types.StringType,
	"resource_type":          types.StringType,
	"platform_type":          types.StringType,
	"attached_effectiveness": types.BoolType,
	"monitored":              types.BoolType,
}

var securityControlAttrTypes = map[string]attr.Type{
	"id":                  types.StringType,
	"name":                types.StringType,
	"type":                types.StringType,
	"cloud_provider":      types.StringType,
	"status":              types.StringType,
	"attached_guardrails": types.ListType{ElemType: types.StringType},
	"control_id":          types.StringType,
	"resource_type":       types.StringType,
	"open_action_count":   types.Float64Type,
	"actions_disabled":    types.BoolType,
	"platform_resources":  types.ListType{ElemType: types.ObjectType{AttrTypes: platformResourceAttrTypes}},
}

func securityControlDSAttrs() map[string]schema.Attribute {
	s := func(d string) schema.StringAttribute { return schema.StringAttribute{Computed: true, Description: d} }
	return map[string]schema.Attribute{
		"id":                  s("Blast security control ID."),
		"name":                s(""),
		"type":                s("e.g. Service Control Policy."),
		"cloud_provider":      s(""),
		"status":              s(""),
		"attached_guardrails": schema.ListAttribute{Computed: true, ElementType: types.StringType},
		"control_id":          s("Platform-native ID (e.g. the SCP ARN)."),
		"resource_type":       s(""),
		"open_action_count":   schema.Float64Attribute{Computed: true},
		"actions_disabled":    schema.BoolAttribute{Computed: true},
		"platform_resources": schema.ListNestedAttribute{
			Computed:    true,
			Description: "Org units / accounts this control is attached to.",
			NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"resource_id":            s(""),
				"resource_name":          s(""),
				"resource_type":          s(""),
				"platform_type":          s(""),
				"attached_effectiveness": schema.BoolAttribute{Computed: true},
				"monitored":              schema.BoolAttribute{Computed: true},
			}},
		},
	}
}

func securityControlValues(c *client.SecurityControl) map[string]attr.Value {
	res := make([]attr.Value, len(c.PlatformResources))
	for i, r := range c.PlatformResources {
		res[i] = types.ObjectValueMust(platformResourceAttrTypes, map[string]attr.Value{
			"resource_id":            types.StringValue(r.ResourceID),
			"resource_name":          types.StringValue(r.ResourceName),
			"resource_type":          types.StringValue(r.ResourceType),
			"platform_type":          types.StringValue(r.PlatformType),
			"attached_effectiveness": types.BoolValue(r.AttachedEffectiveness),
			"monitored":              types.BoolValue(r.Monitored),
		})
	}
	return map[string]attr.Value{
		"id":                  types.StringValue(c.ID),
		"name":                types.StringValue(c.Name),
		"type":                types.StringValue(c.Type),
		"cloud_provider":      types.StringValue(c.CloudProvider),
		"status":              types.StringValue(c.Status),
		"attached_guardrails": toList(c.AttachedGuardrails),
		"control_id":          types.StringValue(c.ControlID),
		"resource_type":       types.StringValue(c.ResourceType),
		"open_action_count":   optFloat(c.OpenActionCount),
		"actions_disabled":    types.BoolValue(c.ActionsDisabled),
		"platform_resources":  types.ListValueMust(types.ObjectType{AttrTypes: platformResourceAttrTypes}, res),
	}
}

// ---------- blast_security_control ----------

func NewSecurityControlDataSource() datasource.DataSource { return &securityControlDataSource{} }

type securityControlDataSource struct{ client *client.Client }

func (d *securityControlDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security_control"
}

func (d *securityControlDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := securityControlDSAttrs()
	attrs["id"] = schema.StringAttribute{Required: true, Description: "Blast security control ID."}
	attrs["project_id"] = schema.StringAttribute{Optional: true, Description: "Blast project ID. Overrides the provider default."}
	attrs["include_policy"] = schema.BoolAttribute{Optional: true, Description: "Also fetch the deployed policy document into `policy`. Defaults to true."}
	attrs["include_optimized_policy"] = schema.BoolAttribute{Optional: true, Description: "Also fetch Blast's optimized policy into `optimized_policy`. Defaults to false."}
	attrs["policy"] = schema.StringAttribute{Computed: true, Description: "Currently deployed policy document (JSON). Null when the control type has no policy document."}
	attrs["optimized_policy"] = schema.StringAttribute{Computed: true, Description: "Blast-suggested optimized policy (JSON)."}
	resp.Schema = schema.Schema{Description: "Look up a security control (SCP, RCP, etc.) and its policy document.", Attributes: attrs}
}

func (d *securityControlDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, diags := providerClient(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.client = c
}

func (d *securityControlDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id, project types.String
	var incPolicy, incOpt types.Bool
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, pathRoot("id"), &id)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, pathRoot("project_id"), &project)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, pathRoot("include_policy"), &incPolicy)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, pathRoot("include_optimized_policy"), &incOpt)...)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := d.client.GetSecurityControl(ctx, project.ValueString(), id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading security control", err.Error())
		return
	}
	vals := securityControlValues(c)
	vals["project_id"] = project
	vals["include_policy"] = incPolicy
	vals["include_optimized_policy"] = incOpt
	vals["policy"] = types.StringNull()
	vals["optimized_policy"] = types.StringNull()

	if incPolicy.IsNull() || incPolicy.ValueBool() {
		p, err := d.client.GetSecurityControlPolicy(ctx, project.ValueString(), id.ValueString(), false)
		if err != nil {
			resp.Diagnostics.AddWarning("Policy document not available", err.Error())
		} else {
			vals["policy"] = optString(p)
		}
	}
	if incOpt.ValueBool() {
		p, err := d.client.GetSecurityControlPolicy(ctx, project.ValueString(), id.ValueString(), true)
		if err != nil {
			resp.Diagnostics.AddWarning("Optimized policy not available", err.Error())
		} else {
			vals["optimized_policy"] = optString(p)
		}
	}
	resp.Diagnostics.Append(setAll(ctx, &resp.State, vals)...)
}

// ---------- blast_security_controls ----------

func NewSecurityControlsDataSource() datasource.DataSource { return &securityControlsDataSource{} }

type securityControlsDataSource struct{ client *client.Client }

type securityControlsModel struct {
	ProjectID    types.String `tfsdk:"project_id"`
	Type         types.String `tfsdk:"type"`
	PlatformType types.String `tfsdk:"platform_type"`
	NameContains types.String `tfsdk:"name_contains"`
	IDs          types.List   `tfsdk:"ids"`
	Controls     types.List   `tfsdk:"security_controls"`
}

func (d *securityControlsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security_controls"
}

func (d *securityControlsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List security controls (SCPs, RCPs, etc.).",
		Attributes: map[string]schema.Attribute{
			"project_id":    schema.StringAttribute{Optional: true, Description: "Blast project ID. Overrides the provider default."},
			"type":          schema.StringAttribute{Optional: true, Description: "Security control type, e.g. Service Control Policy.", Validators: []validator.String{stringvalidator.OneOfCaseInsensitive(securityControlTypes...)}},
			"platform_type": schema.StringAttribute{Optional: true, Description: "Amazon Web Services, Azure Cloud or Google Cloud Platform.", Validators: []validator.String{stringvalidator.OneOfCaseInsensitive(platformTypes...)}},
			"name_contains": schema.StringAttribute{Optional: true, Description: "Client-side case-insensitive substring filter on name."},
			"ids":           schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"security_controls": schema.ListNestedAttribute{
				Computed:     true,
				NestedObject: schema.NestedAttributeObject{Attributes: securityControlDSAttrs()},
			},
		},
	}
}

func (d *securityControlsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, diags := providerClient(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.client = c
}

func (d *securityControlsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m securityControlsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	q := url.Values{}
	if v := m.Type.ValueString(); v != "" {
		q.Set("securityControlType", v)
	}
	if v := m.PlatformType.ValueString(); v != "" {
		q.Set("platformType", v)
	}
	list, err := d.client.ListSecurityControls(ctx, m.ProjectID.ValueString(), q)
	if err != nil {
		resp.Diagnostics.AddError("Error listing security controls", err.Error())
		return
	}
	needle := strings.ToLower(m.NameContains.ValueString())
	var elems []attr.Value
	var ids []string
	for i := range list {
		if needle != "" && !strings.Contains(strings.ToLower(list[i].Name), needle) {
			continue
		}
		elems = append(elems, types.ObjectValueMust(securityControlAttrTypes, securityControlValues(&list[i])))
		ids = append(ids, list[i].ID)
	}
	m.Controls = types.ListValueMust(types.ObjectType{AttrTypes: securityControlAttrTypes}, elems)
	m.IDs = toList(ids)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}
