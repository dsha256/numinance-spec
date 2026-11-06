// Package catalog lists the checks the verifier-python provider can run and the rule ids each can report. A deployment
// reads it as data (the toolchain pack's check catalog): an ArchitecturePack rule is valid only when its verifier and
// rule_id appear here, so a rule that no check can ever report fails validation instead of passing silently. The package
// has no dependency on the provider, so a host composition can import it without the plugin SDK.
package catalog

import "strings"

// Check is one check of the provider: the name a config's "check" field and a LanguageToolchain verifier use, the layer of
// the verification order it belongs to and the rule ids its findings carry. An entry is an exact rule id
// or a path.Match pattern anchored to the whole id; a pattern always requires a non-empty token after the check's prefix, so
// "mypy/" and a bare prefix are never valid.
type Check struct {
	Name  string
	Layer string
	Rules []string
}

// DirectiveRule is the rule id family of the findings that report a suppression directive the tool cannot ignore. A pack
// rule may not name it: a directive finding is the provider's, and its severity is not the pack's to change.
const DirectiveRule = "numinance.directive/?*"

// mypyCodes are the error codes of mypy (docs: error code list), the exact ids of its findings, and "error" for a message
// that carries none. A code a later mypy adds is reported by the provider but cannot be bound by a pack until listed here.
//
//nolint:gochecknoglobals // a constant list
var mypyCodes = []string{
	"abstract", "annotation-unchecked", "arg-type", "assert-type", "assignment", "attr-defined", "await-not-async", "call-arg",
	"call-overload", "comparison-overlap", "dict-item", "empty-body", "error", "exit-return", "explicit-override", "func-returns-value",
	"has-type", "ignore-without-code", "import", "import-not-found", "import-untyped", "index", "list-item", "literal-required", "method-assign",
	"misc", "mutable-override", "name-defined", "name-match", "no-any-return", "no-any-unimported", "no-overload-impl", "no-redef",
	"no-untyped-call", "no-untyped-def", "operator", "override", "overload-cannot-match", "overload-overlap", "possibly-undefined",
	"prop-decorator", "redundant-cast", "redundant-expr", "redundant-self", "return", "return-value", "safe-super", "slice-index",
	"str-bytes-safe", "str-format", "syntax", "top-level-await", "truthy-bool", "truthy-function", "truthy-iterable", "type-abstract",
	"type-arg", "type-var", "typeddict-item", "typeddict-unknown-key", "unimported-reveal", "union-attr", "unreachable", "unused-awaitable",
	"unused-coroutine", "unused-ignore", "used-before-def", "valid-newtype", "valid-type", "var-annotated",
}

// Checks returns the checks in the order of the verification pipeline.
func Checks() []Check {
	mypy := make([]string, 0, len(mypyCodes)+1)
	for _, c := range mypyCodes {
		mypy = append(mypy, "mypy/"+c)
	}
	ruff := []string{"ruff/syntax-error"}
	// A ruff code is one to four capital letters and three or four digits (E501, PLR0913, RUF100).
	for letters := 1; letters <= 4; letters++ {
		for digits := 3; digits <= 4; digits++ {
			ruff = append(ruff, "ruff/"+strings.Repeat("[A-Z]", letters)+strings.Repeat("[0-9]", digits))
		}
	}
	return []Check{
		{Name: "mypy", Layer: "typecheck", Rules: append(mypy, DirectiveRule)},
		{Name: "ruff", Layer: "lint", Rules: append(ruff, DirectiveRule)},
		{Name: "pytest", Layer: "test", Rules: []string{"pytest/test-failed", "pytest/test-error", "pytest/no-tests", DirectiveRule}},
		// Semgrep rule ids and import-linter contracts are the customer's own and cannot be enumerated: an anchored pattern with a
		// non-empty token is the strongest statement available.
		{Name: "semgrep", Layer: "security", Rules: []string{"semgrep/?*", DirectiveRule}},
		{Name: "import-linter", Layer: "architecture", Rules: []string{"import-linter/?*", DirectiveRule}},
	}
}
