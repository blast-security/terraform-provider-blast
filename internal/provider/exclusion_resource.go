package provider

import (
	"context"
	"regexp"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &exclusionResource{}
	_ resource.ResourceWithImportState    = &exclusionResource{}
	_ resource.ResourceWithValidateConfig = &exclusionResource{}
)

func NewExclusionResource() resource.Resource { return &exclusionResource{} }

type exclusionResource struct{ client *client.Client }

type ruleModel struct {
	Identifier types.String `tfsdk:"identifier"`
	Operator   types.String `tfsdk:"operator"`
	Values     types.List   `tfsdk:"values"`
}

var ruleAttrTypes = map[string]attr.Type{
	"identifier": types.StringType,
	"operator":   types.StringType,
	"values":     types.ListType{ElemType: types.StringType},
}

type exclusionModel struct {
	ID                  types.String `tfsdk:"id"`
	ProjectID           types.String `tfsdk:"project_id"`
	Name                types.String `tfsdk:"name"`
	Note                types.String `tfsdk:"note"`
	GuardrailCatalogIDs types.Set    `tfsdk:"guardrail_catalog_ids"`
	PlatformType        types.String `tfsdk:"platform_type"`
	Sources             types.List   `tfsdk:"sources"`
	Targets             types.List   `tfsdk:"targets"`
	Units               types.Set    `tfsdk:"units"`
	ExpirationDate      types.String `tfsdk:"expiration_date"`
	IsExpired           types.Bool   `tfsdk:"is_expired"`
	CreatedAt           types.String `tfsdk:"created_at"`
	CreatedBy           types.String `tfsdk:"created_by"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
}

func ruleListSchema(desc string) schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Optional:    true,
		Description: desc,
		Validators:  []validator.List{listvalidator.SizeAtLeast(1)},
		PlanModifiers: []planmodifier.List{
			// PATCH preserves omitted fields, so switching sources<->targets needs a new exclusion.
			listplanmodifier.RequiresReplaceIf(func(_ context.Context, req planmodifier.ListRequest, resp *listplanmodifier.RequiresReplaceIfFuncResponse) {
				resp.RequiresReplace = !req.StateValue.IsNull() && len(req.StateValue.Elements()) > 0 && req.PlanValue.IsNull()
			}, "Removing all rules of this kind requires replacing the exclusion.", "Removing all rules of this kind requires replacing the exclusion."),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"identifier": schema.StringAttribute{
					Required:    true,
					Description: "What to match: Account ID, ARN, Source IP or Tag.",
					Validators:  []validator.String{stringvalidator.OneOfCaseInsensitive(exclusionIdentifiers...)},
				},
				"operator": schema.StringAttribute{
					Required:    true,
					Description: "Equals, Not Equals or Like.",
					Validators:  []validator.String{stringvalidator.OneOfCaseInsensitive(exclusionOperators...)},
				},
				"values": schema.ListAttribute{
					Required:    true,
					ElementType: types.StringType,
					Validators:  []validator.List{listvalidator.SizeAtLeast(1)},
				},
			},
		},
	}
}

func (r *exclusionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_exclusion"
}

func (r *exclusionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An exclusion rule that exempts sources or targets from one or more guardrails.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"project_id": schema.StringAttribute{
				Optional:      true,
				Description:   "Blast project ID. Overrides the provider default.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Human-readable name of the exclusion rule.",
			},
			"note": schema.StringAttribute{
				Optional:    true,
				Description: "Reason / note for the exclusion.",
			},
			"guardrail_catalog_ids": schema.SetAttribute{
				Required:      true,
				ElementType:   types.StringType,
				Description:   "Guardrail catalog IDs this exclusion applies to (e.g. g-aws-vpc-f7264580). Changing this forces a new exclusion.",
				Validators:    []validator.Set{setvalidator.SizeAtLeast(1)},
				PlanModifiers: []planmodifier.Set{setplanmodifier.RequiresReplace()},
			},
			"platform_type": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Amazon Web Services, Azure Cloud or Google Cloud Platform. Resolved from the guardrail when omitted.",
				Validators:    []validator.String{stringvalidator.OneOfCaseInsensitive(platformTypes...)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"sources": ruleListSchema("Source match rules. Set either sources or targets."),
			"targets": ruleListSchema("Target match rules. Set either sources or targets."),
			"units": schema.SetAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "Cloud org unit IDs (e.g. r-abcd, ou-abcd-12345678). When omitted, the API applies the exclusion to all root org units of the guardrail.",
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
			"expiration_date": schema.StringAttribute{
				Optional:      true,
				Description:   "Expiration date (YYYY-MM-DD). The API cannot update it in place, so changing it replaces the exclusion.",
				Validators:    []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`), "must be YYYY-MM-DD")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"is_expired": schema.BoolAttribute{Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
			"created_at": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"created_by": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *exclusionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	c, d := providerClient(req.ProviderData)
	resp.Diagnostics.Append(d...)
	r.client = c
}

func (r *exclusionResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m exclusionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if m.Sources.IsUnknown() || m.Targets.IsUnknown() {
		return
	}
	hasS, hasT := !m.Sources.IsNull(), !m.Targets.IsNull()
	if hasS == hasT {
		resp.Diagnostics.AddAttributeError(path.Root("sources"), "Invalid exclusion rules", "Exactly one of sources or targets must be set.")
	}
}

func rulesFromModel(ctx context.Context, l types.List) ([]client.ExclusionRule, diag.Diagnostics) {
	var diags diag.Diagnostics
	if l.IsNull() || l.IsUnknown() {
		return nil, diags
	}
	var rules []ruleModel
	diags.Append(l.ElementsAs(ctx, &rules, false)...)
	out := make([]client.ExclusionRule, 0, len(rules))
	for _, r := range rules {
		vals, d := strList(ctx, r.Values)
		diags.Append(d...)
		out = append(out, client.ExclusionRule{Identifier: r.Identifier.ValueString(), Operator: r.Operator.ValueString(), Values: vals})
	}
	return out, diags
}

// rulesToModel converts API rules, keeping the user's label spelling when the API returns the raw key.
func rulesToModel(ctx context.Context, prior types.List, api []client.ExclusionRule) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	objType := types.ObjectType{AttrTypes: ruleAttrTypes}
	if len(api) == 0 {
		return types.ListNull(objType), diags
	}
	var priorRules []ruleModel
	if !prior.IsNull() && !prior.IsUnknown() {
		diags.Append(prior.ElementsAs(ctx, &priorRules, false)...)
	}
	elems := make([]attr.Value, len(api))
	for i, r := range api {
		pi, po := types.StringNull(), types.StringNull()
		if i < len(priorRules) {
			pi, po = priorRules[i].Identifier, priorRules[i].Operator
		}
		obj, d := types.ObjectValue(ruleAttrTypes, map[string]attr.Value{
			"identifier": keepEnum(pi, r.Identifier, exclusionIdentifiers),
			"operator":   keepEnum(po, r.Operator, exclusionOperators),
			"values":     toList(r.Values),
		})
		diags.Append(d...)
		elems[i] = obj
	}
	l, d := types.ListValue(objType, elems)
	diags.Append(d...)
	return l, diags
}

func (r *exclusionResource) apply(ctx context.Context, m *exclusionModel, e *client.Exclusion) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(e.ID)
	if e.Name != "" {
		m.Name = types.StringValue(e.Name)
	}
	if e.Note != "" || !m.Note.IsNull() {
		m.Note = optString(e.Note)
	}
	if len(e.GuardrailCatalogIDs) > 0 {
		m.GuardrailCatalogIDs = toSet(e.GuardrailCatalogIDs)
	} else if e.GuardrailID != "" && m.GuardrailCatalogIDs.IsNull() {
		m.GuardrailCatalogIDs = toSet([]string{e.GuardrailID})
	}
	m.PlatformType = keepEnum(m.PlatformType, e.PlatformType, platformTypes)
	var d diag.Diagnostics
	m.Sources, d = rulesToModel(ctx, m.Sources, e.Sources)
	diags.Append(d...)
	m.Targets, d = rulesToModel(ctx, m.Targets, e.Targets)
	diags.Append(d...)
	// The response DTO does not document units; trust it when present, otherwise keep what we sent.
	if e.Units != nil {
		m.Units = toSet(e.Units)
	} else if m.Units.IsUnknown() || m.Units.IsNull() {
		// Unknown to the API response: record "API-resolved" as an empty set so plans stay stable.
		m.Units = toSet(nil)
	}
	if e.ExpirationDate != "" {
		// API may return a full timestamp; keep the user's date if it is the same day.
		if !m.ExpirationDate.IsNull() && len(e.ExpirationDate) >= 10 && e.ExpirationDate[:10] == m.ExpirationDate.ValueString() {
			// unchanged
		} else if len(e.ExpirationDate) >= 10 {
			m.ExpirationDate = types.StringValue(e.ExpirationDate[:10])
		}
	} else {
		m.ExpirationDate = types.StringNull()
	}
	m.IsExpired = types.BoolValue(e.IsExpired)
	m.CreatedAt = types.StringValue(e.CreatedAt)
	m.CreatedBy = types.StringValue(e.CreatedBy)
	m.UpdatedAt = types.StringValue(e.UpdatedAt)
	return diags
}

func (r *exclusionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan exclusionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	guardrails, d := strSet(ctx, plan.GuardrailCatalogIDs)
	resp.Diagnostics.Append(d...)
	sources, d := rulesFromModel(ctx, plan.Sources)
	resp.Diagnostics.Append(d...)
	targets, d := rulesFromModel(ctx, plan.Targets)
	resp.Diagnostics.Append(d...)
	units, d := strSet(ctx, plan.Units)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := client.CreateExclusion{
		GuardrailCatalogIDs: guardrails,
		Name:                plan.Name.ValueString(),
		Note:                plan.Note.ValueString(),
		Sources:             sources,
		Targets:             targets,
		Units:               units,
		ExpirationDate:      plan.ExpirationDate.ValueString(),
	}
	if !plan.PlatformType.IsUnknown() {
		in.PlatformType = plan.PlatformType.ValueString()
	}
	e, err := r.client.CreateExclusion(ctx, plan.ProjectID.ValueString(), in)
	if err != nil {
		resp.Diagnostics.AddError("Error creating exclusion", err.Error())
		return
	}
	// Re-read so state reflects the stored object (create response may be partial).
	if full, err := r.client.GetExclusion(ctx, plan.ProjectID.ValueString(), e.ID); err == nil {
		e = full
	}
	resp.Diagnostics.Append(r.apply(ctx, &plan, e)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *exclusionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state exclusionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	e, err := r.client.GetExclusion(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading exclusion", err.Error())
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, &state, e)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *exclusionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state exclusionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sources, d := rulesFromModel(ctx, plan.Sources)
	resp.Diagnostics.Append(d...)
	targets, d := rulesFromModel(ctx, plan.Targets)
	resp.Diagnostics.Append(d...)
	units, d := strSet(ctx, plan.Units)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.UpdateExclusion{
		Name:    plan.Name.ValueString(),
		Note:    plan.Note.ValueString(),
		Sources: sources,
		Targets: targets,
		Units:   units,
	}
	if !plan.PlatformType.IsUnknown() {
		in.PlatformType = plan.PlatformType.ValueString()
	}
	id := state.ID.ValueString()
	if _, err := r.client.UpdateExclusion(ctx, plan.ProjectID.ValueString(), id, in); err != nil {
		resp.Diagnostics.AddError("Error updating exclusion", err.Error())
		return
	}
	e, err := r.client.GetExclusion(ctx, plan.ProjectID.ValueString(), id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading exclusion after update", err.Error())
		return
	}
	plan.ID = state.ID
	resp.Diagnostics.Append(r.apply(ctx, &plan, e)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *exclusionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state exclusionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteExclusion(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting exclusion", err.Error())
	}
}

// ImportState accepts "<id>" or "<project_id>/<id>".
func (r *exclusionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	project, id := splitImportID(req.ID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	if project != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), project)...)
	}
}
