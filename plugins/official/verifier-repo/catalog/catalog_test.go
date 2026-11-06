package catalog_test

import (
	"path"
	"regexp"
	"testing"

	"github.com/numinance/numinance-spec/plugins/official/verifier-repo/catalog"
)

var ruleRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}$`)

func TestChecks_RuleIDsAreStableAndWellFormed(t *testing.T) {
	checks := catalog.Checks()
	if len(checks) != 1 || checks[0].Name != catalog.Name || checks[0].Layer != catalog.Layer {
		t.Fatalf("Checks() = %+v, want the one check %q in layer %q", checks, catalog.Name, catalog.Layer)
	}
	seen := map[string]bool{}
	for _, r := range checks[0].Rules {
		if !ruleRe.MatchString(r) || seen[r] {
			t.Errorf("rule %q is malformed or repeated", r)
		}
		if ok, _ := path.Match(catalog.Name+"/?*", r); !ok {
			t.Errorf("rule %q is not namespaced by the check %q", r, catalog.Name)
		}
		seen[r] = true
	}
	// The ids are a contract: renaming one is a breaking change of every ArchitecturePack that names it.
	for _, want := range []string{
		"repo/scope-path", "repo/scope-protected", "repo/scope-files", "repo/scope-added-lines", "repo/scope-added-bytes", "repo/scope-deleted",
		"repo/scope-modified", "repo/scope-symlink", "repo/markdown-fence", "repo/markdown-link", "repo/link-dangling", "repo/just-recipe",
	} {
		if !seen[want] {
			t.Errorf("rule %q is missing from the catalog", want)
		}
	}
}
