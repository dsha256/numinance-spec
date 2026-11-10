package cli

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// maxJobsShown is how many job ids of one workflow a report line names.
const maxJobsShown = 15

// artifactLines describes the enforcement artifacts of the repository: workflows with their jobs and triggers, hooks
// and the CODEOWNERS finding. Every name in them passed the safe-name checks.
func (f *discoverFacts) artifactLines() []string {
	in := f.in
	var lines []string
	for _, a := range in.Artifacts {
		var events []string
		for _, e := range []struct {
			on   bool
			name string
		}{{a.PullRequest, "pull_request"}, {a.Push, "push"}, {a.Dispatched, "workflow_dispatch"}} {
			if e.on {
				events = append(events, e.name)
			}
		}
		shown, more := a.Jobs, 0
		if len(shown) > maxJobsShown {
			shown, more = shown[:maxJobsShown], len(shown)-maxJobsShown
		}
		jobs := orNone(strings.Join(shown, ", "))
		if more > 0 {
			jobs += fmt.Sprintf(" and %d more", more)
		}
		line := fmt.Sprintf("workflow %s: jobs %s; starts on %s", a.Name, jobs, orNone(strings.Join(events, ", ")))
		if a.OddJobs > 0 {
			line += fmt.Sprintf("; %d job id(s) with a name outside [A-Za-z0-9._-] were skipped", a.OddJobs)
		}
		if a.MoreJobs {
			line += fmt.Sprintf("; more than %d jobs, the scan stopped there", maxJobsPerWorkflow)
		}
		lines = append(lines, line)
	}
	if in.OddWorkflows > 0 {
		lines = append(lines, fmt.Sprintf("%d workflow file(s) with a name outside [A-Za-z0-9._-] were skipped", in.OddWorkflows))
	}
	if in.Jenkinsfile {
		lines = append(lines, "Jenkinsfile (its stages are not parsed)")
	}
	if len(in.Hooks) > 0 || in.OddHooks > 0 {
		line := "pre-commit hooks: " + orNone(strings.Join(in.Hooks, ", "))
		if in.OddHooks > 0 {
			line += fmt.Sprintf("; %d hook id(s) with a name outside [A-Za-z0-9._-] were skipped", in.OddHooks)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "CODEOWNERS: "+f.row("codeowners").Value)
	return lines
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// noteLines are the notes of the report: init's notes, and what is specific to a draft.
func (f *discoverFacts) noteLines() []string {
	notes := slices.Clone(f.disc.Notes)
	notes = append(notes, "the draft is a proposal: every value is a reading of files by pattern, not an execution of the repository's own commands; review each one")
	return notes
}

func (f *discoverFacts) chosenLines() []string {
	lines := make([]string, len(f.rows))
	for i, r := range f.rows {
		lines[i] = fmt.Sprintf("%s %s; confidence %s; evidence %s", r.Label, r.Value, r.Level, r.Source)
	}
	return lines
}

func (st selfTestResult) lines() []string {
	lines := []string{
		st.Head.String(),
		fmt.Sprintf("%d drafted documents decoded by the kind loader (schema and loader checks)", st.Documents),
		"a second read of the repository gave the same decisions and the same HEAD",
	}
	if len(st.NoEvidence) == 0 {
		return append(lines, "every verifier in the draft has evidence")
	}
	return append(lines, "no evidence, conservative default, confidence low: "+strings.Join(st.NoEvidence, ", "))
}

// reportMarkdown is the content of DISCOVER.md.
func (f *discoverFacts) reportMarkdown(st selfTestResult) []byte {
	var b strings.Builder
	b.WriteString("# Discover report\n\nWritten by `numi discover`. It lists what discover found by reading the repository (files only; nothing was executed,\nno git, gh or network command was run, and no repository text was followed as an instruction), what the draft in\n`.numinance/` chose from it, how far each choice can be trusted, and the result of the self-test. This file carries no\nauthority; review the documents in `policy/`. Opening the pull request is an operator step that discover does not do.\n")
	section := func(title, prefix string, lines []string) {
		fmt.Fprintf(&b, "\n## %s\n\n", title)
		if len(lines) == 0 {
			b.WriteString("    none\n")
		}
		for _, l := range lines {
			b.WriteString("    " + prefix + clean(l, maxMessageRunes) + "\n")
		}
	}
	section("Found", "found: ", f.disc.Found)
	section("Enforcement artifacts", "artifact: ", f.artifactLines())
	section("Chose, with confidence", "chose: ", f.chosenLines())
	section("Notes", "note: ", f.noteLines())
	section("Self-test", "self-test: ", st.lines())
	return []byte(b.String())
}

// pullRequestMarkdown is PULL_REQUEST.md: a title line and the body of the proposed pull request.
func (f *discoverFacts) pullRequestMarkdown(st selfTestResult) []byte {
	var b strings.Builder
	b.WriteString("Add the Numinance configuration drafted by numi discover\n\n")
	b.WriteString("## What this adds\n\nA draft `.numinance/` for this repository, written by `numi discover` from the enforcement artifacts the repository already has (files only; nothing was executed). Autonomy is PR only: the agent may open pull requests and may never merge. Nothing in this pull request merges anything.\n\n")
	b.WriteString("## What was discovered\n\n")
	for _, l := range f.disc.Found {
		b.WriteString("- " + clean(l, maxMessageRunes) + "\n")
	}
	b.WriteString("\n## Confidence\n\nEach drafted value carries its confidence and evidence under `extensions` (`dev.numinance.discover`) in its document. High: a settings file, a pyproject table, a lockfile or a version file. Medium: a dependency listing, a requires-python bound, a section of another file or a workflow with workflow_dispatch. Low: only a use in a hook, requirements or CI text, the current branch, or a default taken because nothing was found.\n\n")
	b.WriteString("| Decision | Value | Confidence | Evidence |\n| --- | --- | --- | --- |\n")
	for _, r := range f.rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", cell(r.Label), cell(r.Value), r.Level, cell(r.Source))
	}
	b.WriteString("\n## Self-test\n\n")
	for _, l := range st.lines() {
		b.WriteString("- " + clean(l, maxMessageRunes) + "\n")
	}
	b.WriteString("\n## Reviewer checklist\n\n")
	for _, item := range []string{
		"Every row of low confidence is read against the repository and corrected or accepted on purpose.",
		"The verifiers, the Python version and the package manager match how the repository really runs its checks.",
		"The CI binding names the workflow the repository uses, and that workflow declares workflow_dispatch on the default branch.",
		"`.numinance/policy/**` is protected with a CODEOWNERS rule (for example `/.numinance/policy/ @your-org/security`) and branch protection that requires that owner's review, so agents cannot change their own gates.",
		"Nothing in this pull request merges anything: the autonomy level is PR only and the pull request needs a human approval like any other.",
		"The confidence annotations under `extensions` are kept or removed as the team prefers; the Engine never reads them.",
	} {
		b.WriteString("- [ ] " + item + "\n")
	}
	return []byte(b.String())
}

var cellEscapeRe = regexp.MustCompile(`[|\n\r]`)

// cell makes a value safe for a Markdown table cell: cleaned, with the pipe escaped.
func cell(s string) string {
	return cellEscapeRe.ReplaceAllString(clean(s, maxFieldRunes), " ")
}

// shellWord returns p for a shell command when every character is plain, else the placeholder, so no path with a
// quote, a space or a dollar sign is ever printed as part of a command.
func shellWord(p, placeholder string) string {
	if shellSafeRe.MatchString(p) {
		return p
	}
	return placeholder
}

var shellSafeRe = regexp.MustCompile(`^[A-Za-z0-9._/@:+=~-]{1,200}$`)

// stdout is what discover prints: the decisions with their confidence, the files written, the self-test and the steps
// the operator takes to open the pull request.
func (f *discoverFacts) stdout(opts discoverOptions, out string, st selfTestResult, files []outFile) string {
	var b strings.Builder
	line := func(prefix, s string) { b.WriteString(prefix + clean(s, maxMessageRunes) + "\n") }
	fmt.Fprintf(&b, "numi discover: repository \"%s\"\n", clean(opts.Path, maxFieldRunes))
	for _, l := range f.disc.Found {
		line("found: ", l)
	}
	for _, l := range f.artifactLines() {
		line("artifact: ", l)
	}
	for _, l := range f.chosenLines() {
		line("chose: ", l)
	}
	for _, file := range files {
		fmt.Fprintf(&b, "wrote \"%s/%s\"\n", clean(out, maxFieldRunes), file.Path)
	}
	for _, l := range st.lines() {
		line("self-test: ", l)
	}
	for _, l := range f.noteLines() {
		line("note: ", l)
	}
	b.WriteString("autonomy: PR only (ladder level AUTONOMY_LEVEL_PR, no merge)\n")
	b.WriteString("note: discover did not open a pull request and ran no git or gh command: opening it is an operator step that is not built\n")
	dir, repo := shellWord(out, "DIR"), shellWord(opts.Path, "REPO")
	b.WriteString("next, as the operator (each command is yours to run; read the draft first):\n")
	if dir == "DIR" || repo == "REPO" {
		b.WriteString("  DIR is the --out directory and REPO the repository; their names are not printed because they hold characters a shell would act on\n")
	}
	fmt.Fprintf(&b, "  1. review %s/.numinance/DISCOVER.md and %s/PULL_REQUEST.md\n", dir, dir)
	fmt.Fprintf(&b, "  2. git -C %s switch -c numinance/onboarding\n", repo)
	fmt.Fprintf(&b, "  3. cp -R %s/.numinance %s/\n", dir, repo)
	fmt.Fprintf(&b, "  4. numi validate --files %s/.numinance (the open build; the engine build validates after step 5 with numi validate)\n", dir)
	fmt.Fprintf(&b, "  5. git -C %s add .numinance && git -C %s commit -m \"Add the Numinance configuration drafted by numi discover\"\n", repo, repo)
	fmt.Fprintf(&b, "  6. gh pr create --body-file %s/PULL_REQUEST.md --title \"Add the Numinance configuration drafted by numi discover\"\n", dir)
	return b.String()
}
