package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Enum values the API documents (human-readable labels; raw enum keys are also accepted by the API).
var (
	platformTypes        = []string{"Amazon Web Services", "Azure Cloud", "Google Cloud Platform", "Entra Id"}
	securityControlTypes = []string{
		"Service Control Policy", "Resource Control Policy", "Multi-Factor Authentication",
		"S3 Bucket Policy", "IAM Policy", "IAM Trust Relationship Policy", "Bedrock Guardrail",
		"Azure Policy Assignment", "Conditional Access Policy",
	}
	// Known exclusion identifiers, used only to normalise label/raw-key spellings.
	// They are NOT enforced: the API is the source of truth (see GET /enums), so new
	// identifiers work without a provider release.
	exclusionIdentifiers = []string{
		"Account ID", "ARN", "Source IP", "Tag", "Region",
		"VPC ID", "VPC Endpoint ID", "Principal Organization Path",
		"Scope Identifier", "Users", "Groups", "Roles", "Applications",
	}
	exclusionOperators = []string{"Equals", "Not Equals", "Like"}
	deliveryModes      = []string{deliveryTask, deliveryOpenPR}
)

const (
	deliveryTask   = "task"
	deliveryOpenPR = "open_pr"
)

func providerClient(data any) (*client.Client, diag.Diagnostics) {
	var diags diag.Diagnostics
	if data == nil {
		return nil, diags // provider not configured yet (e.g. during validate)
	}
	c, ok := data.(*client.Client)
	if !ok {
		diags.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", data))
	}
	return c, diags
}

func strList(ctx context.Context, l types.List) ([]string, diag.Diagnostics) {
	var out []string
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	d := l.ElementsAs(ctx, &out, false)
	return out, d
}

func strSet(ctx context.Context, s types.Set) ([]string, diag.Diagnostics) {
	var out []string
	if s.IsNull() || s.IsUnknown() {
		return nil, nil
	}
	d := s.ElementsAs(ctx, &out, false)
	return out, d
}

func toList(vals []string) types.List {
	if vals == nil {
		vals = []string{}
	}
	elems := make([]attr.Value, len(vals))
	for i, v := range vals {
		elems[i] = types.StringValue(v)
	}
	return types.ListValueMust(types.StringType, elems)
}

func toSet(vals []string) types.Set {
	if vals == nil {
		vals = []string{}
	}
	elems := make([]attr.Value, len(vals))
	for i, v := range vals {
		elems[i] = types.StringValue(v)
	}
	return types.SetValueMust(types.StringType, elems)
}

func optString(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

var tokenSplit = regexp.MustCompile(`[^a-z0-9]+`)

var enumAliases = map[string]string{
	"amazonwebservices":   "aws",
	"azurecloud":          "azure",
	"googlecloudplatform": "gcp",
}

// enumTokens lowercases and splits on non-alphanumerics; well-known long labels collapse to their short key.
func enumTokens(s string) []string {
	var toks []string
	for _, t := range tokenSplit.Split(strings.ToLower(s), -1) {
		if t != "" {
			toks = append(toks, t)
		}
	}
	if a, ok := enumAliases[strings.Join(toks, "")]; ok {
		return []string{a}
	}
	return toks
}

func hasTokenSuffix(raw, label []string) bool {
	if len(label) == 0 || len(label) > len(raw) {
		return false
	}
	off := len(raw) - len(label)
	for i := range label {
		if raw[off+i] != label[i] {
			return false
		}
	}
	return true
}

// canonicalEnum maps an API value (label or raw key such as ACCOUNT_ID / PLATFORM_TYPE_AWS)
// to the documented label it denotes. The longest matching label wins, so NOT_EQUALS maps to
// "Not Equals" rather than "Equals". Unknown values are returned unchanged.
func canonicalEnum(api string, labels []string) string {
	raw := enumTokens(api)
	best, bestLen := api, 0
	for _, l := range labels {
		lt := enumTokens(l)
		if hasTokenSuffix(raw, lt) && len(lt) > bestLen {
			best, bestLen = l, len(lt)
		}
	}
	return best
}

// keepEnum keeps the user's spelling when the API returns an equivalent value, and otherwise
// stores the documented label, so label/raw-key round trips never show up as drift.
func keepEnum(prior types.String, api string, labels []string) types.String {
	if api == "" {
		if prior.IsUnknown() {
			return types.StringNull()
		}
		return prior
	}
	canon := canonicalEnum(api, labels)
	if !prior.IsNull() && !prior.IsUnknown() && strings.EqualFold(canonicalEnum(prior.ValueString(), labels), canon) {
		return prior
	}
	return types.StringValue(canon)
}

// splitImportID parses "<project_id>/<id>" or "<id>".
func splitImportID(raw string) (project, id string) {
	if i := strings.LastIndex(raw, "/"); i > 0 {
		return raw[:i], raw[i+1:]
	}
	return "", raw
}

func pathRoot(name string) path.Path { return path.Root(name) }

// setAll writes a map of computed attribute values into state.
func setAll(ctx context.Context, st *tfsdk.State, vals map[string]attr.Value) diag.Diagnostics {
	var diags diag.Diagnostics
	for k, v := range vals {
		diags.Append(st.SetAttribute(ctx, path.Root(k), v)...)
	}
	return diags
}

var objectAsOpts = basetypes.ObjectAsOptions{UnhandledNullAsEmpty: true, UnhandledUnknownAsEmpty: true}

// projectIDAttribute is the project_id input every project-scoped resource uses. It has no
// RequiresReplace modifier on purpose: plain RequiresReplace destroys an object imported with a bare
// "<id>" as soon as the config sets project_id, and fires when a user spells out the provider
// default. Replacement is decided in ModifyPlan by planProjectReplace, which compares the
// EFFECTIVE project (configured value, else the provider default) with the project the object was
// created or imported in (kept in private state).
func projectIDAttribute() schema.StringAttribute {
	return schema.StringAttribute{Optional: true, Description: projectIDDesc}
}

const projectIDDesc = "Blast project ID. Overrides the provider default. Changing the effective project forces a new resource."

const privateProjectKey = "effective_project"

type privateStore interface {
	GetKey(context.Context, string) ([]byte, diag.Diagnostics)
	SetKey(context.Context, string, []byte) diag.Diagnostics
}

func effectiveProject(v types.String, c *client.Client) string {
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return c.DefaultProjectID()
	}
	return v.ValueString()
}

// rememberProject records the project the object lives in. Call it after Create (overwrite=true)
// and after Read (overwrite=false, so the value from Create/import is kept).
func rememberProject(ctx context.Context, priv privateStore, v types.String, c *client.Client, overwrite bool) diag.Diagnostics {
	if priv == nil {
		return nil
	}
	if !overwrite {
		if b, d := priv.GetKey(ctx, privateProjectKey); d.HasError() || len(b) > 0 {
			return d
		}
	}
	b, _ := json.Marshal(effectiveProject(v, c))
	return priv.SetKey(ctx, privateProjectKey, b)
}

// planProjectReplace forces replacement only when the effective project changes.
func planProjectReplace(ctx context.Context, c *client.Client, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var prior, planned types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("project_id"), &prior)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("project_id"), &planned)...)
	if planned.IsUnknown() {
		resp.RequiresReplace = append(resp.RequiresReplace, path.Root("project_id"))
		return
	}
	was := effectiveProject(prior, c)
	if req.Private != nil {
		if b, _ := req.Private.GetKey(ctx, privateProjectKey); len(b) > 0 {
			_ = json.Unmarshal(b, &was)
		}
	}
	if was != effectiveProject(planned, c) {
		resp.RequiresReplace = append(resp.RequiresReplace, path.Root("project_id"))
	}
}

// notEmptyObject rejects an optional object written as `{}`. The API drops an empty object, so
// Terraform would read back null and fail with "inconsistent result after apply".
func notEmptyObject() validator.Object { return notEmptyObjectValidator{} }

type notEmptyObjectValidator struct{}

func (notEmptyObjectValidator) Description(_ context.Context) string {
	return "must set at least one attribute (omit the block instead of writing {})"
}

func (v notEmptyObjectValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v notEmptyObjectValidator) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	for _, a := range req.ConfigValue.Attributes() {
		if !a.IsNull() {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(req.Path, "Empty block", req.Path.String()+" "+v.Description(ctx)+".")
}
