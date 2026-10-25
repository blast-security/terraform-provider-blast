package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCanonicalEnum(t *testing.T) {
	cases := []struct {
		api    string
		labels []string
		want   string
	}{
		{"ACCOUNT_ID", exclusionIdentifiers, "Account ID"},
		{"Account ID", exclusionIdentifiers, "Account ID"},
		{"SOURCE_IP", exclusionIdentifiers, "Source IP"},
		{"EQUALS", exclusionOperators, "Equals"},
		{"NOT_EQUALS", exclusionOperators, "Not Equals"}, // must not collapse to "Equals"
		{"OPERATOR_NOT_EQUALS", exclusionOperators, "Not Equals"},
		{"AWS", platformTypes, "Amazon Web Services"},
		{"PLATFORM_TYPE_AWS", platformTypes, "Amazon Web Services"},
		{"AZURE", platformTypes, "Azure Cloud"},
		{"Something New", platformTypes, "Something New"},
	}
	for _, c := range cases {
		if got := canonicalEnum(c.api, c.labels); got != c.want {
			t.Errorf("canonicalEnum(%q) = %q, want %q", c.api, got, c.want)
		}
	}
}

func TestKeepEnum(t *testing.T) {
	if got := keepEnum(types.StringValue("equals"), "EQUALS", exclusionOperators); got.ValueString() != "equals" {
		t.Errorf("user spelling not kept: %s", got)
	}
	if got := keepEnum(types.StringValue("Equals"), "NOT_EQUALS", exclusionOperators); got.ValueString() != "Not Equals" {
		t.Errorf("real drift hidden: %s", got)
	}
	if got := keepEnum(types.StringNull(), "ACCOUNT_ID", exclusionIdentifiers); got.ValueString() != "Account ID" {
		t.Errorf("import not canonicalised: %s", got)
	}
}

func TestJSONEqual(t *testing.T) {
	if !jsonEqual(`{"a":1,"b":[1,2]}`, "{\n  \"b\": [1,2],\n  \"a\": 1\n}") {
		t.Error("formatting-only difference should be equal")
	}
	if jsonEqual(`{"a":1}`, `{"a":2}`) {
		t.Error("different documents reported equal")
	}
}

func TestSplitImportID(t *testing.T) {
	if p, id := splitImportID("prj-1/exc-9"); p != "prj-1" || id != "exc-9" {
		t.Errorf("got %q %q", p, id)
	}
	if p, id := splitImportID("exc-9"); p != "" || id != "exc-9" {
		t.Errorf("got %q %q", p, id)
	}
}
