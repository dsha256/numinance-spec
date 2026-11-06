package provider

import (
	"fmt"
	"strings"

	"github.com/numinance/numinance-spec/plugins/official/verifier-repo/catalog"
)

// under reports whether p is the allowed entry a, or a path below it when a is written as a directory ("docs/").
func under(p, a string) bool {
	if dir, isDir := strings.CutSuffix(a, "/"); isDir {
		return strings.HasPrefix(p, dir+"/")
	}
	return p == a
}

// underFold is under without regard to case: a protected path is not escaped by writing it in another case. Both arguments are
// already lower case (checkScope folds each once).
func underFold(p, a string) bool {
	dir, isDir := strings.CutSuffix(a, "/")
	if isDir {
		return p == dir || strings.HasPrefix(p, dir+"/")
	}
	return p == a || strings.HasPrefix(p, a+"/")
}

// checkScope reports what the candidate changes outside the scope: paths outside the allowed set or under a protected one, a
// symbolic link, a modification where only new files are allowed, and more files, added lines or deletions than the bounds.
func checkScope(cand Candidate, s Scope) []finding {
	var out []finding
	var added, deleted int
	var addedBytes int64
	protected := lowered(s.ProtectedPaths)
	for _, c := range cand.Changes {
		if !anyUnder(c.Path, s.AllowPaths, under) {
			out = append(out, finding{rule: catalog.RuleScopePath, severity: sevHigh, path: c.Path, message: "the path is outside every path this change may touch"})
		}
		if anyUnder(strings.ToLower(c.Path), protected, underFold) {
			out = append(out, finding{rule: catalog.RuleScopeProtected, severity: sevCritical, path: c.Path, message: "the path is protected and no change may touch it"})
		}
		switch {
		case c.Status == statusDeleted:
			deleted++
		case c.Symlink:
			out = append(out, finding{rule: catalog.RuleScopeSymlink, severity: sevHigh, path: c.Path, message: "the change adds or changes a symbolic link"})
		default:
			added += c.Lines
			addedBytes += c.Bytes
		}
		if (c.Status == statusModified || c.Status == statusTypeChg) && !s.AllowModify {
			out = append(out, finding{rule: catalog.RuleScopeModified, severity: sevHigh, path: c.Path, message: "the change modifies a file that exists, and this scope allows new files only"})
		}
	}
	if n := len(cand.Changes); n > *s.MaxFiles {
		out = append(out, finding{rule: catalog.RuleScopeFiles, severity: sevHigh, message: fmt.Sprintf("the change touches %d files, at most %d are allowed", n, *s.MaxFiles)})
	}
	if added > *s.MaxAddedLines {
		out = append(out, finding{rule: catalog.RuleScopeAddedLines, severity: sevHigh, message: fmt.Sprintf("the change adds %d lines, at most %d are allowed", added, *s.MaxAddedLines)})
	}
	if addedBytes > int64(*s.MaxAddedBytes) {
		out = append(out, finding{rule: catalog.RuleScopeAddedBytes, severity: sevHigh, message: fmt.Sprintf("the change adds %d bytes, at most %d are allowed", addedBytes, *s.MaxAddedBytes)})
	}
	if deleted > *s.MaxDeletedFiles {
		out = append(out, finding{rule: catalog.RuleScopeDeleted, severity: sevHigh, message: fmt.Sprintf("the change deletes %d files, at most %d are allowed", deleted, *s.MaxDeletedFiles)})
	}
	return out
}

func anyUnder(p string, entries []string, match func(p, a string) bool) bool {
	for _, a := range entries {
		if match(p, a) {
			return true
		}
	}
	return false
}

// lowered returns the entries in lower case, for underFold.
func lowered(entries []string) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = strings.ToLower(e)
	}
	return out
}
