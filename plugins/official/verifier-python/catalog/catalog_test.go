package catalog_test

import (
	"path"
	"testing"

	"github.com/numinance/numinance-spec/plugins/official/verifier-python/catalog"
)

func reports(name, id string) bool {
	for _, c := range catalog.Checks() {
		if c.Name != name {
			continue
		}
		for _, p := range c.Rules {
			if ok, err := path.Match(p, id); err == nil && ok {
				return true
			}
		}
	}
	return false
}

func TestChecks_PublishExactIDsOrAnchoredPatterns(t *testing.T) {
	good := map[string][]string{
		"mypy": {"mypy/return-value", "mypy/error"}, "ruff": {"ruff/F401", "ruff/PLR0913", "ruff/syntax-error"},
		"pytest": {"pytest/test-failed", "pytest/no-tests"}, "semgrep": {"semgrep/no-eval"}, "import-linter": {"import-linter/domain-is-pure"},
	}
	for check, ids := range good {
		for _, id := range ids {
			if !reports(check, id) {
				t.Errorf("%s does not report %q, want it valid", check, id)
			}
		}
	}
	bad := map[string][]string{
		"mypy": {"mypy/", "mypy/*", "mypy/typo", "mypy"}, "ruff": {"ruff/", "ruff/f401", "ruff/F4", "ruff/*"},
		"pytest": {"pytest/", "pytest/other"}, "semgrep": {"semgrep/", "semgrep"}, "import-linter": {"import-linter/"},
	}
	for check, ids := range bad {
		for _, id := range ids {
			if reports(check, id) {
				t.Errorf("%s reports %q, want it refused", check, id)
			}
		}
	}
}

func TestChecks_LayersFollowThePipeline(t *testing.T) {
	layers := make([]string, 0, len(catalog.Checks()))
	for _, c := range catalog.Checks() {
		layers = append(layers, c.Layer)
	}
	want := []string{"typecheck", "lint", "test", "security", "architecture"}
	for i := range want {
		if layers[i] != want[i] {
			t.Errorf("layers = %v, want %v", layers, want)
			break
		}
	}
}
