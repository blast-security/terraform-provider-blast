package provider

import (
	"context"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	_ resource.Resource                   = &guardrailDeploymentResource{}
	_ resource.ResourceWithImportState    = &guardrailDeploymentResource{}
	_ resource.ResourceWithValidateConfig = &guardrailDeploymentResource{}
)

func NewGuardrailDeploymentResource() resource.Resource { return &guardrailDeploymentResource{} }

type guardrailDeploymentResource struct{ client *client.Client }

type guardrailDeploymentModel struct {
	taskCommon
	Name                      types.String `tfsdk:"name"`
	GuardrailCatalogIDs       types.Set    `tfsdk:"guardrail_catalog_ids"`
	SecurityControlType       types.String `tfsdk:"security_control_type"`
	SecurityControlName       types.String `tfsdk:"security_control_name"`
	ExistingSecurityControlID types.String `tfsdk:"existing_security_control_id"`
	PlatformUnitIDs           types.Set    `tfsdk:"platform_unit_ids"`
	PlatformType              types.String `tfsdk:"platform_type"`
	SecurityControlID         types.String `tfsdk:"security_control_id"`
}

func (r *guardrailDeploymentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_guardrail_deployment"
}

func (r *guardrailDeploymentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	attrs := taskCommonAttrs()
	attrs["name"] = schema.StringAttribute{Required: true, Description: "Task name. Can be changed in place."}
	attrs["guardrail_catalog_ids"] = schema.SetAttribute{
		Required: true, ElementType: types.StringType,
		Description:   "Guardrail catalog IDs to merge into one security control.",
		Validators:    []validator.Set{setvalidator.SizeAtLeast(1)},
		PlanModifiers: []planmodifier.Set{setplanmodifier.RequiresReplace()},
	}
	attrs["security_control_type"] = schema.StringAttribute{
		Required: true, Description: "Control type to create, e.g. Service Control Policy or Resource Control Policy.",
		Validators:    []validator.String{stringvalidator.OneOfCaseInsensitive(securityControlTypes...)},
		PlanModifiers: replace,
	}
	attrs["security_control_name"] = schema.StringAttribute{
		Required: true, Description: "Display name of the new control, or the name of the existing control when existing_security_control_id is set.",
		// The task API does not return this value, so after an import it is adopted from config instead of forcing a replace.
		PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplaceIf(func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
			resp.RequiresReplace = !req.StateValue.IsNull()
		}, "Changing the control name creates a new task.", "")},
	}
	attrs["existing_security_control_id"] = schema.StringAttribute{
		Optional: true, Description: "Merge the guardrails into this existing control instead of creating a new one.",
		PlanModifiers: replace,
	}
	attrs["platform_unit_ids"] = schema.SetAttribute{
		Required: true, ElementType: types.StringType,
		Description:   "Org unit / account IDs to enforce on, e.g. ou-abcd-12345678.",
		Validators:    []validator.Set{setvalidator.SizeAtLeast(1)},
		PlanModifiers: []planmodifier.Set{setplanmodifier.RequiresReplace()},
	}
	attrs["platform_type"] = schema.StringAttribute{
		Optional: true, Computed: true, Description: "Derived from the guardrails when omitted.",
		Validators:    []validator.String{stringvalidator.OneOfCaseInsensitive(platformTypes...)},
		PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
	}
	attrs["security_control_id"] = schema.StringAttribute{
		Computed: true, Description: "Security control the task creates or changes (may be empty until deployed).",
	}
	resp.Schema = schema.Schema{
		Description: "Deploys one or more catalog guardrails as a security control (SCP, RCP, ...) by creating a Blast task, " +
			"optionally opening a Terraform PR. The cloud is changed only when the task is deployed (in Blast or by merging the PR).",
		Attributes: attrs,
	}
}

func (r *guardrailDeploymentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	c, d := providerClient(req.ProviderData)
	resp.Diagnostics.Append(d...)
	r.client = c
}

func (r *guardrailDeploymentResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m guardrailDeploymentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validatePRConfig(ctx, m.DeliveryMode, m.PullRequest)...)
}

func (r *guardrailDeploymentResource) apply(m *guardrailDeploymentModel, t *client.Task) {
	m.applyTask(t)
	m.PlatformType = keepEnum(m.PlatformType, t.PlatformType, platformTypes)
	m.SecurityControlID = types.StringValue(t.SecurityControlID)
	if m.SecurityControlID.ValueString() == "" && !m.ExistingSecurityControlID.IsNull() {
		m.SecurityControlID = m.ExistingSecurityControlID
	}
}

func (r *guardrailDeploymentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan guardrailDeploymentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	guardrails, d := strSet(ctx, plan.GuardrailCatalogIDs)
	resp.Diagnostics.Append(d...)
	units, d := strSet(ctx, plan.PlatformUnitIDs)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.CreateTask{
		Name:                plan.Name.ValueString(),
		GuardrailCatalogIDs: guardrails,
		SecurityControlType: plan.SecurityControlType.ValueString(),
		PlatformUnitIDs:     units,
		SecurityControlName: plan.SecurityControlName.ValueString(),
	}
	if !plan.PlatformType.IsUnknown() {
		in.PlatformType = plan.PlatformType.ValueString()
	}
	if id := plan.ExistingSecurityControlID.ValueString(); id != "" {
		f := false
		in.NewControl = &f
		in.SecurityControlID = id
	}
	task, err := r.client.CreateTask(ctx, plan.ProjectID.ValueString(), in)
	if err != nil {
		resp.Diagnostics.AddError("Error creating guardrail deployment task", err.Error())
		return
	}
	r.apply(&plan, task)
	resp.Diagnostics.Append(plan.openPR(ctx, r.client, plan.Name.ValueString())...)
	if full, err := r.client.GetTask(ctx, plan.ProjectID.ValueString(), task.ID); err == nil {
		prURL := plan.TerraformPrURL
		r.apply(&plan, full)
		if plan.TerraformPrURL.IsNull() {
			plan.TerraformPrURL = prURL
		}
	}
	if plan.TerraformPrURL.IsUnknown() {
		plan.TerraformPrURL = types.StringNull()
	}
	// Saving state even on PR failure: Terraform then marks the resource tainted.
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *guardrailDeploymentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state guardrailDeploymentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	task, err := r.client.GetTask(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading task", err.Error())
		return
	}
	r.apply(&state, task)
	// Import: fill inputs from the task.
	if state.Name.IsNull() {
		state.Name = types.StringValue(task.Name)
		state.GuardrailCatalogIDs = toSet(task.GuardrailCatalogIDs)
		state.PlatformUnitIDs = toSet(task.PlatformUnitIDs)
		state.SecurityControlType = optString(task.SecurityControlType)
		if state.DeliveryMode.IsNull() {
			state.DeliveryMode = types.StringValue(deliveryTask)
			if task.TerraformPrURL != "" {
				state.DeliveryMode = types.StringValue(deliveryOpenPR)
			}
		}
		if state.OnDestroy.IsNull() {
			state.OnDestroy = types.StringValue(onDestroyDelete)
		}
		if state.PullRequest.IsUnknown() {
			state.PullRequest = types.ObjectNull(pullRequestAttrTypes)
		}
	} else {
		state.Name = types.StringValue(task.Name)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *guardrailDeploymentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state guardrailDeploymentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	if !plan.Name.Equal(state.Name) {
		if _, err := r.client.UpdateTask(ctx, plan.ProjectID.ValueString(), state.ID.ValueString(), client.UpdateTask{Name: plan.Name.ValueString()}); err != nil {
			resp.Diagnostics.AddError("Error renaming task", err.Error())
			return
		}
	}
	plan.TerraformPrURL = state.TerraformPrURL
	if state.DeliveryMode.ValueString() != deliveryOpenPR && plan.DeliveryMode.ValueString() == deliveryOpenPR {
		resp.Diagnostics.Append(plan.openPR(ctx, r.client, plan.Name.ValueString())...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	task, err := r.client.GetTask(ctx, plan.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading task", err.Error())
		return
	}
	prURL := plan.TerraformPrURL
	r.apply(&plan, task)
	if plan.TerraformPrURL.IsNull() || plan.TerraformPrURL.IsUnknown() {
		plan.TerraformPrURL = prURL
	}
	if plan.TerraformPrURL.IsUnknown() {
		plan.TerraformPrURL = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *guardrailDeploymentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state guardrailDeploymentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(state.destroy(ctx, r.client)...)
}

func (r *guardrailDeploymentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	project, id := splitImportID(req.ID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	if project != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), project)...)
	}
}
