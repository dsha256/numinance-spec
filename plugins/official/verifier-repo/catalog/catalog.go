// Package catalog lists the check the verifier-repo provider runs and the rule ids it can report. A deployment reads it as data
// (the toolchain's check catalog): an ArchitecturePack rule is valid only when its verifier and rule_id appear here, so a rule
// that no check can ever report fails validation instead of passing silently. The package has no dependency on the provider, so
// a host composition can import it without the plugin SDK.
package catalog

// Name is the verifier name a LanguageToolchain declares for the provider and the value of the config's "check" field.
const Name = "repo"

// Layer is the layer of the verification order the checks belong to: they are static lint over the changed
// files, and no candidate code runs.
const Layer = "lint"

// The rule ids the check reports. They are stable: a rule id never changes meaning and is never reused.
const (
	// RuleScopePath is a changed path outside every allowed path.
	RuleScopePath = "repo/scope-path"
	// RuleScopeProtected is a changed path under a protected path.
	RuleScopeProtected = "repo/scope-protected"
	// RuleScopeFiles is more changed files than the bound.
	RuleScopeFiles = "repo/scope-files"
	// RuleScopeAddedLines is more added lines than the bound.
	RuleScopeAddedLines = "repo/scope-added-lines"
	// RuleScopeAddedBytes is more added bytes than the bound.
	RuleScopeAddedBytes = "repo/scope-added-bytes"
	// RuleScopeDeleted is more deleted files than the bound.
	RuleScopeDeleted = "repo/scope-deleted"
	// RuleScopeModified is a modified file where the scope allows only new files.
	RuleScopeModified = "repo/scope-modified"
	// RuleScopeSymlink is a symbolic link among the changes.
	RuleScopeSymlink = "repo/scope-symlink"
	// RuleMarkdownFence is a code fence that is never closed.
	RuleMarkdownFence = "repo/markdown-fence"
	// RuleMarkdownLink is a relative link whose target does not exist in the candidate tree.
	RuleMarkdownLink = "repo/markdown-link"
	// RuleLinkDangling is a deleted file that a Markdown document the change does not touch links to.
	RuleLinkDangling = "repo/link-dangling"
	// RuleJustRecipe is a `just` recipe a changed document names that the Justfile does not define.
	RuleJustRecipe = "repo/just-recipe"
)

// Check is the one check of the provider: the name a config's "check" field and a LanguageToolchain verifier use, the layer
// of the verification order its findings belong to and the rule ids they carry.
type Check struct {
	Name  string
	Layer string
	Rules []string
}

// Checks returns the checks of the provider.
func Checks() []Check {
	return []Check{{
		Name: Name, Layer: Layer,
		Rules: []string{
			RuleScopePath, RuleScopeProtected, RuleScopeFiles, RuleScopeAddedLines, RuleScopeAddedBytes, RuleScopeDeleted, RuleScopeModified, RuleScopeSymlink,
			RuleMarkdownFence, RuleMarkdownLink, RuleLinkDangling, RuleJustRecipe,
		},
	}}
}
