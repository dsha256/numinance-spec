package provider

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func goodConfig() map[string]any {
	return map[string]any{
		"check": "repo",
		"scope": map[string]any{
			"allow_paths": []string{"docs/runbooks/"}, "protected_paths": []string{".claude/"},
			"max_files": 1, "max_added_lines": 50, "max_added_bytes": 1 << 20, "max_deleted_files": 0, "allow_modify": false,
		},
		"context": map[string]any{"base": testBase, "paths": []string{"README.md"}, "justfile": "check:\n"},
	}
}

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseConfig_AcceptsAScope(t *testing.T) {
	c, err := ParseConfig(marshal(t, goodConfig()))
	if err != nil || c.Check != "repo" || *c.Scope.MaxAddedLines != 50 || c.Context.Base != testBase || c.Scope.AllowModify {
		t.Fatalf("ParseConfig() = %+v, %v, want the scope and the context", c, err)
	}
}

func TestParseConfig_RefusesWhatIsNotAScope(t *testing.T) {
	edit := func(f func(m map[string]any)) []byte {
		m := goodConfig()
		f(m)
		return marshal(t, m)
	}
	scopeOf := func(m map[string]any) map[string]any { return m["scope"].(map[string]any) }
	ctxOf := func(m map[string]any) map[string]any { return m["context"].(map[string]any) }
	tests := []struct {
		name string
		b    []byte
	}{
		{"not json", []byte("{")},
		{"trailing data", append(marshal(t, goodConfig()), []byte(" {}")...)},
		{"an unknown field", edit(func(m map[string]any) { m["extra"] = 1 })},
		{"an unknown scope field", edit(func(m map[string]any) { scopeOf(m)["settings"] = "x" })},
		{"no check", edit(func(m map[string]any) { m["check"] = "" })},
		{"no allowed path", edit(func(m map[string]any) { scopeOf(m)["allow_paths"] = []string{} })},
		{"an allowed path that climbs", edit(func(m map[string]any) { scopeOf(m)["allow_paths"] = []string{"../x/"} })},
		{"an absolute protected path", edit(func(m map[string]any) { scopeOf(m)["protected_paths"] = []string{"/etc"} })},
		{"too many scope paths", edit(func(m map[string]any) { scopeOf(m)["allow_paths"] = make([]string, maxScopePaths+1) })},
		{"no max_files", edit(func(m map[string]any) { delete(scopeOf(m), "max_files") })},
		{"no max_added_lines", edit(func(m map[string]any) { delete(scopeOf(m), "max_added_lines") })},
		{"no max_added_bytes", edit(func(m map[string]any) { delete(scopeOf(m), "max_added_bytes") })},
		{"a context link of a path that climbs", edit(func(m map[string]any) { ctxOf(m)["links"] = map[string][]string{"../x": {"a.md"}} })},
		{"a linking document of a path that climbs", edit(func(m map[string]any) { ctxOf(m)["links"] = map[string][]string{"a.md": {"../x.md"}} })},
		{"no max_deleted_files", edit(func(m map[string]any) { delete(scopeOf(m), "max_deleted_files") })},
		{"a negative bound", edit(func(m map[string]any) { scopeOf(m)["max_files"] = -1 })},
		{"a bound out of range", edit(func(m map[string]any) { scopeOf(m)["max_added_lines"] = maxBoundValue + 1 })},
		{"a short base", edit(func(m map[string]any) { ctxOf(m)["base"] = "main" })},
		{"a context path that climbs", edit(func(m map[string]any) { ctxOf(m)["paths"] = []string{"a/../../b"} })},
		{"an over-long justfile", edit(func(m map[string]any) { ctxOf(m)["justfile"] = strings.Repeat("x", maxJustfile+1) })},
		{"a config over the limit", []byte(strings.Repeat(" ", MaxConfigBytes+1))},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if c, err := ParseConfig(tc.b); !errors.Is(err, ErrConfig) {
				t.Errorf("ParseConfig() = %+v, %v, want ErrConfig", c, err)
			}
		})
	}
}

func TestParseConfig_ABoundOfZeroIsAllowedAndMeansNone(t *testing.T) {
	m := goodConfig()
	m["scope"].(map[string]any)["max_added_lines"] = 0
	c, err := ParseConfig(marshal(t, m))
	if err != nil || *c.Scope.MaxAddedLines != 0 {
		t.Fatalf("ParseConfig(max_added_lines 0) = %+v, %v, want a scope that allows no added line", c, err)
	}
	if got := rules(checkScope(parse(t, add("docs/runbooks/a.md", "x\n")), c.Scope)); !has(checkScope(parse(t, add("docs/runbooks/a.md", "x\n")), c.Scope), "repo/scope-added-lines") {
		t.Errorf("checkScope(one line, bound 0) rules = %v, want repo/scope-added-lines", got)
	}
}
