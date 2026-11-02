package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	projectTypes    = []string{"Development", "Test", "Production"}
	projectStatuses = []string{"Active", "Inactive"}
)

var (
	_ resource.Resource                = &projectResource{}
	_ resource.ResourceWithImportState = &projectResource{}
	_ resource.ResourceWithModifyPlan  = &projectResource{}
)

func NewProjectResource() resource.Resource { return &projectResource{} }

type projectResource struct{ client *client.Client }

type projectModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Description   types.String `tfsdk:"description"`
	Types         types.Set    `tfsdk:"types"`
	UnitIDs       types.Set    `tfsdk:"unit_ids"`
	Status        types.String `tfsdk:"status"`
	PlatformTypes types.List   `tfsdk:"platform_types"`
	NumberOfUnits types.Int64  `tfsdk:"number_of_units"`
	Default       types.Bool   `tfsdk:"default"`
	Units         types.List   `tfsdk:"units"`
}

var projectUnitAttrTypes = map[string]attr.Type{
	"id":              types.StringType,
	"name":            types.StringType,
	"platform_type":   types.StringType,
	"platform_org_id": types.StringType,
	"grouping_unit":   types.BoolType,
	"unit_types":      types.ListType{ElemType: types.StringType},
	"regions":         types.ListType{ElemType: types.StringType},
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "A Blast project: a named view over a set of cloud units (accounts, OUs, subscriptions...). " +
			"Use its `id` as `project_id` on other Blast resources and data sources.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Project name. Must be unique within the tenant; checked at plan time.",
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"description": schema.StringAttribute{Optional: true},
			"types": schema.SetAttribute{
				Optional: true, Computed: true, ElementType: types.StringType,
				Description: "Project types: Development, Test and/or Production.",
				Validators: []validator.Set{setvalidator.ValueStringsAre(
					stringvalidator.OneOf(projectTypes...),
				)},
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
			"unit_ids": schema.SetAttribute{
				Optional: true, Computed: true, ElementType: types.StringType,
				Description: "Platform unit IDs in the project (AWS account / OU IDs, Azure subscriptions...). " +
					"Exact membership: units not listed are removed. Find IDs with data.blast_cloud_units. " +
					"Omit to leave membership unmanaged by Terraform.",
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				Optional: true, Computed: true,
				Description:   "Active or Inactive. New projects are Active unless set otherwise.",
				Validators:    []validator.String{stringvalidator.OneOf(projectStatuses...)},
				PlanModifiers: keep,
			},
			"platform_types":  schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Cloud platforms covered by the project."},
			"number_of_units": schema.Int64Attribute{Computed: true},
			"default": schema.BoolAttribute{
				Computed: true, Description: "True for the tenant default project (\"All Cloud Units\"), which cannot be changed or deleted.",
			},
			"units": schema.ListNestedAttribute{
				Computed: true, Description: "Cloud units in the project, as resolved by Blast.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":              schema.StringAttribute{Computed: true},
					"name":            schema.StringAttribute{Computed: true},
					"platform_type":   schema.StringAttribute{Computed: true},
					"platform_org_id": schema.StringAttribute{Computed: true},
					"grouping_unit":   schema.BoolAttribute{Computed: true},
					"unit_types":      schema.ListAttribute{Computed: true, ElementType: types.StringType},
					"regions":         schema.ListAttribute{Computed: true, ElementType: types.StringType},
				}},
			},
		},
	}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	c, d := providerClient(req.ProviderData)
	resp.Diagnostics.Append(d...)
	r.client = c
}

// ModifyPlan catches two API errors at plan time: duplicate names and edits to the default project.
func (r *projectResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if r.client == nil || req.Plan.Raw.IsNull() {
		return // destroy, or provider not configured yet
	}
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || plan.Name.IsUnknown() {
		return
	}
	var state *projectModel
	if !req.State.Raw.IsNull() {
		state = &projectModel{}
		resp.Diagnostics.Append(req.State.Get(ctx, state)...)
		if state.Default.ValueBool() && !req.Plan.Raw.Equal(req.State.Raw) {
			resp.Diagnostics.AddError("The default project cannot be changed",
				"\"All Cloud Units\" is managed by Blast. Remove the changes, or remove it from Terraform with `terraform state rm`.")
			return
		}
	}
	if state == nil || !strings.EqualFold(state.Name.ValueString(), plan.Name.ValueString()) {
		ok, err := r.client.ProjectNameAvailable(ctx, plan.Name.ValueString())
		if err != nil {
			resp.Diagnostics.AddWarning("Could not check project name", err.Error())
			return
		}
		if !ok {
			resp.Diagnostics.AddAttributeError(path.Root("name"), "Project name already in use",
				fmt.Sprintf("A Blast project named %q already exists. Pick another name, or import the existing project with `terraform import`.", plan.Name.ValueString()))
		}
	}
}

func (r *projectResource) apply(m *projectModel, p *client.Project) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(p.ID)
	m.Name = types.StringValue(p.Name)
	if p.Description != "" || !m.Description.IsNull() {
		m.Description = optString(p.Description)
	}
	if p.Status != "" {
		m.Status = types.StringValue(canonicalEnum(p.Status, projectStatuses))
	} else if m.Status.IsUnknown() {
		m.Status = types.StringValue("Active")
	}
	var typesOut []string
	for _, t := range p.Types {
		c := canonicalEnum(t, append(projectTypes, "Default"))
		if c != "Default" { // the default project's "Default" type is not user-settable
			typesOut = append(typesOut, c)
		}
	}
	m.Types = toSet(typesOut)

	ids := make([]string, 0, len(p.Units))
	units := make([]attr.Value, 0, len(p.Units))
	for _, u := range p.Units {
		ids = append(ids, u.ID)
		units = append(units, types.ObjectValueMust(projectUnitAttrTypes, map[string]attr.Value{
			"id":              types.StringValue(u.ID),
			"name":            types.StringValue(u.Name),
			"platform_type":   types.StringValue(u.PlatformType),
			"platform_org_id": types.StringValue(u.PlatformOrgID),
			"grouping_unit":   types.BoolValue(u.GroupingUnit),
			"unit_types":      toList(u.UnitTypes),
			"regions":         toList(u.Regions),
		}))
	}
	sort.Strings(ids)
	m.UnitIDs = toSet(ids)
	m.Units = types.ListValueMust(types.ObjectType{AttrTypes: projectUnitAttrTypes}, units)
	m.PlatformTypes = toList(p.PlatformTypes)
	switch {
	case p.NumberOfUnits != nil:
		m.NumberOfUnits = types.Int64Value(int64(*p.NumberOfUnits))
	default:
		m.NumberOfUnits = types.Int64Value(int64(len(p.Units)))
	}
	m.Default = types.BoolValue(p.Default)
	return diags
}

func joinTypes(ctx context.Context, s types.Set) (string, diag.Diagnostics) {
	vals, d := strSet(ctx, s)
	sort.Strings(vals)
	return strings.Join(vals, ","), d
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	units, d := strSet(ctx, plan.UnitIDs)
	resp.Diagnostics.Append(d...)
	typesStr, d := joinTypes(ctx, plan.Types)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.CreateProject(ctx, client.CreateProject{
		Name: plan.Name.ValueString(), Description: plan.Description.ValueString(),
		UnitIDs: units, Types: typesStr,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating project", err.Error())
		return
	}
	// Status is not part of the create call.
	if plan.Status.ValueString() == "Inactive" {
		if _, err := r.client.UpdateProject(ctx, p.ID, client.UpdateProject{Status: "Inactive"}); err != nil {
			resp.Diagnostics.AddError("Project created, but setting it Inactive failed", err.Error())
		}
	}
	if full, err := r.client.GetProject(ctx, p.ID); err == nil {
		p = full
	}
	resp.Diagnostics.Append(r.apply(&plan, p)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.GetProject(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading project", err.Error())
		return
	}
	resp.Diagnostics.Append(r.apply(&state, p)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.UpdateProject{Name: plan.Name.ValueString()}
	desc := plan.Description.ValueString()
	in.Description = &desc
	if !plan.Status.IsUnknown() {
		in.Status = plan.Status.ValueString()
	}
	if !plan.UnitIDs.IsUnknown() && !plan.UnitIDs.Equal(state.UnitIDs) {
		units, d := strSet(ctx, plan.UnitIDs)
		resp.Diagnostics.Append(d...)
		if units == nil {
			units = []string{}
		}
		in.UnitIDs = &units
	}
	if !plan.Types.IsUnknown() && !plan.Types.Equal(state.Types) {
		t, d := joinTypes(ctx, plan.Types)
		resp.Diagnostics.Append(d...)
		in.Types = &t
	}
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if _, err := r.client.UpdateProject(ctx, id, in); err != nil {
		resp.Diagnostics.AddError("Error updating project", err.Error())
		return
	}
	p, err := r.client.GetProject(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading project after update", err.Error())
		return
	}
	resp.Diagnostics.Append(r.apply(&plan, p)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if state.Default.ValueBool() {
		resp.Diagnostics.AddError("The default project cannot be deleted",
			"Remove it from Terraform with `terraform state rm` instead.")
		return
	}
	if err := r.client.DeleteProject(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting project", err.Error())
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
