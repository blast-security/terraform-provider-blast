package provider

// Guard tests for code produced by the spec-sync bot (and by people). They encode the product
// policy in terraform-sync-policy.json and the "done" checklist from AGENTS.md section 11, so a
// resource that should not exist, or an unfinished one, fails CI instead of relying on review.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

const repoRoot = "../.."

type syncPolicy struct {
	Objects map[string]struct {
		APIBase  string `json:"api_base"`
		Resource struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"resource"`
		DataSources []string `json:"data_sources"`
	} `json:"objects"`
}

func loadPolicy(t *testing.T) syncPolicy {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, "terraform-sync-policy.json"))
	if err != nil {
		t.Fatalf("terraform-sync-policy.json: %v", err)
	}
	var p syncPolicy
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("terraform-sync-policy.json is not valid JSON: %v", err)
	}
	return p
}

func testProvider() provider.Provider { return New("test")() }

// registeredResources returns every resource the provider serves, by type name.
func registeredResources(t *testing.T) map[string]resource.Resource {
	t.Helper()
	ctx := context.Background()
	out := map[string]resource.Resource{}
	for _, f := range testProvider().Resources(ctx) {
		r := f()
		var resp resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "blast"}, &resp)
		if _, dup := out[resp.TypeName]; dup {
			t.Errorf("resource %s is registered twice (a generated *_gen.go next to a hand-written file?)", resp.TypeName)
		}
		out[resp.TypeName] = r
	}
	return out
}

func registeredDataSources(t *testing.T) map[string]datasource.DataSource {
	t.Helper()
	ctx := context.Background()
	out := map[string]datasource.DataSource{}
	for _, f := range testProvider().DataSources(ctx) {
		d := f()
		var resp datasource.MetadataResponse
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "blast"}, &resp)
		if _, dup := out[resp.TypeName]; dup {
			t.Errorf("data source %s is registered twice", resp.TypeName)
		}
		out[resp.TypeName] = d
	}
	return out
}

// A resource exists only if product approved it in terraform-sync-policy.json.
func TestResourcesAreApprovedInPolicy(t *testing.T) {
	p := loadPolicy(t)
	approved, denied := map[string]bool{}, map[string]string{}
	for obj, o := range p.Objects {
		switch o.Resource.Status {
		case "approved":
			approved[o.Resource.Name] = true
		case "denied":
			denied[o.Resource.Name] = obj
		}
	}
	for name := range registeredResources(t) {
		if obj, ok := denied[name]; ok && name != "" {
			t.Errorf("resource %s is registered but terraform-sync-policy.json denies a resource for %q. Remove it; keep the data sources.", name, obj)
			continue
		}
		if !approved[name] {
			t.Errorf("resource %s is not approved in terraform-sync-policy.json. New resources need a product decision recorded there first (AGENTS.md section 11).", name)
		}
	}
}

func TestDataSourcesAreListedInPolicy(t *testing.T) {
	p := loadPolicy(t)
	listed := map[string]bool{}
	for _, o := range p.Objects {
		for _, d := range o.DataSources {
			listed[d] = true
		}
	}
	for name := range registeredDataSources(t) {
		if !listed[name] {
			t.Errorf("data source %s is not listed in terraform-sync-policy.json; add it under its API object", name)
		}
	}
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, rel))
	if err != nil {
		return ""
	}
	return string(b)
}

// e2eConfigs returns the e2e script plus every example it copies in, which is what e2e really runs.
func e2eConfigs(t *testing.T) string {
	t.Helper()
	script := readRepoFile(t, "scripts/e2e_mock.sh")
	all := script
	for _, m := range regexp.MustCompile(`examples/[A-Za-z0-9_-]+`).FindAllString(script, -1) {
		files, _ := filepath.Glob(filepath.Join(repoRoot, m, "*.tf"))
		for _, f := range files {
			b, _ := os.ReadFile(f)
			all += string(b)
		}
	}
	return all
}

// Every resource and data source ships with Registry docs, an example, an e2e run and a CHANGELOG entry.
func TestEveryTypeIsDocumentedTestedAndInChangelog(t *testing.T) {
	changelog := readRepoFile(t, "CHANGELOG.md")
	e2e := e2eConfigs(t)
	examples := ""
	files, _ := filepath.Glob(filepath.Join(repoRoot, "examples", "*", "*.tf"))
	for _, f := range files {
		b, _ := os.ReadFile(f)
		examples += string(b)
	}
	check := func(kind, name, docDir string) {
		short := strings.TrimPrefix(name, "blast_")
		doc := readRepoFile(t, filepath.Join("docs", docDir, short+".md"))
		switch {
		case doc == "":
			t.Errorf("%s %s: missing docs/%s/%s.md (Terraform Registry page)", kind, name, docDir, short)
		case strings.Contains(doc, "TODO"):
			t.Errorf("%s %s: docs/%s/%s.md still contains TODO", kind, name, docDir, short)
		}
		block := regexp.MustCompile(`(resource|data)\s+"` + regexp.QuoteMeta(name) + `"`)
		if !block.MatchString(examples) {
			t.Errorf("%s %s: no example under examples/ uses it", kind, name)
		}
		if !block.MatchString(e2e) {
			t.Errorf("%s %s: not exercised by scripts/e2e_mock.sh (directly or through an example it runs)", kind, name)
		}
		if !strings.Contains(changelog, "`"+name+"`") {
			t.Errorf("%s %s: not mentioned in CHANGELOG.md", kind, name)
		}
	}
	for name := range registeredResources(t) {
		check("resource", name, "resources")
	}
	for name := range registeredDataSources(t) {
		check("data source", name, "data-sources")
	}
}

// Generated skeletons are a starting point only; they must not be merged unfinished.
func TestNoUnfinishedGeneratedCode(t *testing.T) {
	var bad []string
	_ = filepath.Walk(filepath.Join(repoRoot, "internal"), func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, _ := os.ReadFile(p)
		s := string(b)
		if strings.HasSuffix(p, "_gen.go") || strings.Contains(s, "It is a starting point") || strings.Contains(s, "rawJSON(") {
			bad = append(bad, strings.TrimPrefix(p, repoRoot+"/"))
		}
		return nil
	})
	sort.Strings(bad)
	for _, f := range bad {
		t.Errorf("%s: unfinished generated code (a *_gen.go skeleton, its header, or rawJSON). Finish it into a hand-maintained file per AGENTS.md section 11, or remove it.", f)
	}
}
