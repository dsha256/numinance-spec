package cli

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
)

// maxPackedRefsBytes bounds the read of .git/packed-refs, which is larger than a document in a repository with many
// branches. A file over the bound is not searched and HEAD is reported as not resolved.
const maxPackedRefsBytes = 1 << 20

var commitIDRe = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

// headInfo is the HEAD commit found by reading .git, or the reason it was not found. Not resolving HEAD is a finding,
// never an error: a directory that is not a git repository, a gitfile, a worktree or a detached HEAD is fine.
type headInfo struct {
	// Commit and Ref are set when resolved; Ref is the branch ref (a safe name) HEAD names.
	Commit, Ref string
	// Reason is a fixed text saying why HEAD was not resolved.
	Reason string
}

func (h headInfo) String() string {
	if h.Commit == "" {
		return "HEAD not resolved (" + h.Reason + ")"
	}
	return "HEAD " + h.Commit + " (" + h.Ref + ")"
}

// resolveHead reads .git/HEAD, then the loose ref file or packed-refs, through r: regular files only, no link
// followed, each read bounded. It runs no git command.
func resolveHead(r *repoReader) headInfo {
	unresolved := func(reason string) headInfo { return headInfo{Reason: reason} }
	b := r.text(".git/HEAD", maxVersionBytes)
	if r.err != nil {
		return unresolved("unreadable")
	}
	if b == nil {
		return unresolved("no readable .git/HEAD: not a git repository, a gitfile or a worktree")
	}
	line, _, _ := strings.Cut(string(b), "\n")
	line = strings.TrimSpace(line)
	ref, isRef := strings.CutPrefix(line, "ref: ")
	if !isRef {
		if commitIDRe.MatchString(line) {
			return unresolved("detached HEAD, not followed")
		}
		return unresolved("HEAD is not understood")
	}
	name, ok := strings.CutPrefix(ref, "refs/heads/")
	if !ok || !plainBranchRef(name) {
		return unresolved("HEAD names a ref that is not a plain branch")
	}
	full := "refs/heads/" + name
	if loose := r.text(".git/"+full, maxVersionBytes); loose != nil {
		first, _, _ := strings.Cut(string(loose), "\n")
		if first = strings.TrimSpace(first); commitIDRe.MatchString(first) {
			return headInfo{Commit: first, Ref: full}
		}
		return unresolved("the ref file does not hold a commit id")
	}
	if r.err != nil {
		return unresolved("unreadable")
	}
	packed := r.text(".git/packed-refs", maxPackedRefsBytes)
	if r.err != nil {
		return unresolved("unreadable")
	}
	if slices.Contains(r.cut, ".git/packed-refs") {
		return unresolved("packed-refs is larger than the read bound")
	}
	for line := range strings.Lines(string(packed)) {
		id, refName, ok := strings.Cut(strings.TrimSpace(line), " ")
		if ok && refName == full && commitIDRe.MatchString(id) {
			return headInfo{Commit: id, Ref: full}
		}
	}
	return unresolved("the branch has no commit id in .git (unborn, or the ref is not stored as a file)")
}

// plainBranchRef is a branch name that is a path of plain components inside refs/heads: the safe shape, and none of
// the shapes git itself refuses or that could point elsewhere in .git.
func plainBranchRef(name string) bool {
	if !branchRe.MatchString(name) || strings.HasSuffix(name, ".lock") {
		return false
	}
	for part := range strings.SplitSeq(name, "/") {
		if part == "" || strings.HasPrefix(part, ".") {
			return false
		}
	}
	return !strings.Contains(name, "..")
}

// discoverRead is one reading of the repository: the inspection and the HEAD commit.
type discoverRead struct {
	in   inspection
	head headInfo
}

func readForDiscover(ctx context.Context, root *os.Root) (discoverRead, error) {
	in, err := inspectRepo(ctx, root)
	if err != nil {
		return discoverRead{}, err
	}
	return discoverRead{in: in, head: resolveHead(&repoReader{root: root})}, nil
}

type selfTestInput struct {
	Path  string
	Files []outFile
	First discoverRead
	Facts *discoverFacts
	// BeforeReread is a test seam; nil in a real run.
	BeforeReread func()
}

// selfTestResult is what the self-test established; it is written into the report and the pull request text.
type selfTestResult struct {
	Head      headInfo
	Documents int
	// NoEvidence names the verifiers the draft has with no tool found, as "verifier (tool)".
	NoEvidence []string
}

// selfTest checks the draft against the repository HEAD as it is now, in process:
//
//	(a) HEAD is recorded (found in .git, or reported as not resolved);
//	(b) every drafted document decodes with the kind loader (schema and loader checks): exit 3 when one does not;
//	(c) a second read of the repository gives the same decisions and the same HEAD: exit 5 when not, because the
//	    repository changed while discover ran;
//	(d) a verifier with no evidence is reported, not failed.
//
// It writes nothing.
func selfTest(ctx context.Context, in selfTestInput) (selfTestResult, error) {
	res := selfTestResult{Head: in.First.head}
	loader, err := kindyaml.NewLoader()
	if err != nil {
		return res, Errorf(KindInternal, "cannot build the kind loader")
	}
	for _, f := range in.Files {
		if !strings.HasSuffix(f.Path, ".yaml") {
			continue
		}
		if _, err := loader.Decode(f.Data); err != nil {
			return res, safeErrorf(KindInvalid, "self-test failed: the drafted document %s does not load: %s", quote(f.Path), quote(err.Error()))
		}
		res.Documents++
	}

	if in.BeforeReread != nil {
		in.BeforeReread()
	}
	root, err := os.OpenRoot(in.Path)
	if err != nil {
		return res, safeErrorf(KindConflict, "self-test failed: %s cannot be opened again; the repository changed while discover ran", quote(in.Path))
	}
	defer func() { _ = root.Close() }()
	second, err := readForDiscover(ctx, root)
	if err != nil {
		return res, err
	}
	var opts initOptions
	disc, err := decide(second.in, &opts)
	if err != nil {
		return res, Errorf(KindConflict, "self-test failed: the repository changed while discover ran")
	}
	again := newDiscoverFacts(second.in, opts, disc)
	if diff := differing(in.Facts, again, in.First.head, second.head); len(diff) > 0 {
		return res, Errorf(KindConflict, "self-test failed: the repository changed while discover ran (%s differ)", strings.Join(diff, ", "))
	}

	for _, r := range in.Facts.rows {
		if r.NoEvidence {
			res.NoEvidence = append(res.NoEvidence, fmt.Sprintf("%s (%s)", strings.TrimPrefix(r.Label, "verifier "), r.Value))
		}
	}
	return res, ctx.Err()
}

// differing names, by fixed label, the decisions that are not equal in the two reads, in the order of the first.
func differing(a, b *discoverFacts, headA, headB headInfo) []string {
	pairs := func(f *discoverFacts) map[string]string {
		m := make(map[string]string, len(f.rows))
		for i, fp := range f.fingerprint() {
			m[f.rows[i].Label] = fp
		}
		return m
	}
	ma, mb := pairs(a), pairs(b)
	var out []string
	for _, r := range a.rows {
		if ma[r.Label] != mb[r.Label] {
			out = append(out, r.Label)
		}
	}
	for _, r := range b.rows {
		if _, ok := ma[r.Label]; !ok {
			out = append(out, r.Label)
		}
	}
	if headA != headB {
		out = append(out, "HEAD")
	}
	return out
}
