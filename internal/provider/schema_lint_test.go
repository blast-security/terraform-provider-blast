package provider

// Schema lint: catches, at `go test` time, the bug classes found in review of the spec-sync PRs
// (#2 automation rules, #5 gaps). Each rule maps to a "Generated code" checklist item in AGENTS.md.
//
// knownViolations lists issues that exist on main today and are tracked for a fix. The list is
// strict both ways: a new violation fails, and an entry that no longer fires also fails, so the
// list only shrinks.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// Keep this short and tracked. Format: "<rule>: <resource>.<attribute path>".
var knownViolations = map[string]string{
	"empty-collection: blast_automation_rule.slack.allow_request_for": "fix/automation-rule-review",
	"empty-object: blast_automation_rule.slack":                       "fix/automation-rule-review",
	"empty-object: blast_automation_rule.conditions":                  "fix/automation-rule-review",
}

// lintExempt is for attributes the lint cannot judge, with the reason. Only write-once inputs that
// Read never overwrites from the API qualify (task-backed resources). Product/maintainer review
// required (CODEOWNERS); the spec-sync agent must never add entries here.
var lintExempt = map[string]string{
	"empty-object: blast_guardrail_deployment.pull_request":                "task input kept from config; `{}` means 'open a PR with defaults'",
	"empty-collection: blast_guardrail_deployment.pull_request.targets":    "task input kept from config; never read back",
	"empty-object: blast_security_control_change.pull_request":             "task input kept from config; `{}` means 'open a PR with defaults'",
	"empty-collection: blast_security_control_change.pull_request.targets": "task input kept from config; never read back",
	"empty-collection: blast_security_control_change.attach_unit_ids":      "task input kept from config; never read back",
	"empty-collection: blast_security_control_change.detach_unit_ids":      "task input kept from config; never read back",
}

func resourceSchema(t *testing.T, r resource.Resource) schema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}
	return resp.Schema
}

func describes(ctx context.Context, vs []string, want string) bool {
	for _, d := range vs {
		if strings.Contains(strings.ToLower(d), want) {
			return true
		}
	}
	return false
}

type lintCtx struct {
	ctx   context.Context
	found map[string]string // violation key -> message
}

func (l *lintCtx) add(key, msg string) { l.found[key] = msg }

// walk visits every attribute, including nested ones, with its dotted path.
func (l *lintCtx) walk(typeName, prefix string, attrs map[string]schema.Attribute) {
	for name, a := range attrs {
		p := prefix + name
		optionalInput := a.IsOptional() && !a.IsComputed()

		switch at := a.(type) {
		case schema.ListAttribute:
			if optionalInput && !describes(l.ctx, listDescs(l.ctx, at.Validators), "at least") {
				l.add("empty-collection: "+typeName+"."+p, "optional list without listvalidator.SizeAtLeast(1): `= []` is sent, the API returns null, and apply fails with \"inconsistent result\"")
			}
		case schema.SetAttribute:
			if optionalInput && !describes(l.ctx, listDescs(l.ctx, at.Validators), "at least") {
				l.add("empty-collection: "+typeName+"."+p, "optional set without setvalidator.SizeAtLeast(1)")
			}
		case schema.ListNestedAttribute:
			if optionalInput && !describes(l.ctx, listDescs(l.ctx, at.Validators), "at least") {
				l.add("empty-collection: "+typeName+"."+p, "optional nested list without listvalidator.SizeAtLeast(1)")
			}
			l.walk(typeName, p+".", at.NestedObject.Attributes)
		case schema.SetNestedAttribute:
			if optionalInput && !describes(l.ctx, listDescs(l.ctx, at.Validators), "at least") {
				l.add("empty-collection: "+typeName+"."+p, "optional nested set without setvalidator.SizeAtLeast(1)")
			}
			l.walk(typeName, p+".", at.NestedObject.Attributes)
		case schema.MapAttribute:
			if optionalInput && !describes(l.ctx, listDescs(l.ctx, at.Validators), "at least") {
				l.add("empty-collection: "+typeName+"."+p, "optional map without mapvalidator.SizeAtLeast(1)")
			}
		case schema.SingleNestedAttribute:
			if optionalInput && allChildrenOptional(at.Attributes) && len(at.Validators) == 0 {
				l.add("empty-object: "+typeName+"."+p, "optional object whose children are all optional, with no object validator: `= {}` is sent, the API returns null, and apply fails. Add objectvalidator.AtLeastOneOf / a custom 'not empty' validator")
			}
			l.walk(typeName, p+".", at.Attributes)
		}
	}
}

func allChildrenOptional(attrs map[string]schema.Attribute) bool {
	for _, a := range attrs {
		if a.IsRequired() {
			return false
		}
	}
	return true
}

func listDescs[T interface{ Description(context.Context) string }](ctx context.Context, vs []T) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.Description(ctx))
	}
	return out
}

// bareImport: a resource whose project_id is an input must use projectIDAttribute() and call
// planProjectReplace from ModifyPlan. With plain RequiresReplace, the next plan after a bare "<id>"
// import sees null -> "<project>" and destroys the imported object.
func (l *lintCtx) bareImport(typeName string, r resource.Resource, s schema.Schema) {
	a, ok := s.Attributes["project_id"]
	if !ok || a.IsComputed() {
		return
	}
	if sa, isStr := a.(schema.StringAttribute); isStr && sa.Description == projectIDDesc && len(sa.PlanModifiers) == 0 {
		if _, mp := r.(resource.ResourceWithModifyPlan); mp {
			return // shared projectIDAttribute(); ModifyPlan calls planProjectReplace
		}
	}
	l.add("bare-import: "+typeName+".project_id", "project_id is not projectIDAttribute() with a ModifyPlan that calls planProjectReplace. With plain RequiresReplace, importing by bare ID and then setting project_id destroys and recreates the object")
}

func TestSchemaLint(t *testing.T) {
	l := &lintCtx{ctx: context.Background(), found: map[string]string{}}
	for name, r := range registeredResources(t) {
		s := resourceSchema(t, r)
		l.walk(name, "", s.Attributes)
		l.bareImport(name, r, s)
		if _, ok := r.(resource.ResourceWithImportState); !ok {
			l.add("no-import: "+name, "resource does not implement ImportState")
		}
	}

	keys := make([]string, 0, len(l.found))
	for k := range l.found {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, known := knownViolations[k]; known {
			continue
		}
		if _, ok := lintExempt[k]; ok {
			continue
		}
		t.Errorf("%s\n    %s\n    See AGENTS.md \"Generated code and the spec-sync agent\".", k, l.found[k])
	}
	for k, fix := range knownViolations {
		if _, still := l.found[k]; !still {
			t.Errorf("knownViolations entry %q (tracked in %s) no longer fires. Remove it from the list.", k, fix)
		}
	}
	for k := range lintExempt {
		if _, still := l.found[k]; !still {
			t.Errorf("lintExempt entry %q no longer fires. Remove it from the list.", k)
		}
	}
	if testing.Verbose() {
		fmt.Printf("schema lint: %d findings, %d known\n", len(l.found), len(knownViolations))
	}
}
