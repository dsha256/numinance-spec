package cli

import (
	"cmp"
	"fmt"
	"strings"
)

// discoveryFile is the report init writes beside the documents. It carries no authority (it is outside policy/) and
// holds no repository text beyond names that passed the safe-name checks.
const discoveryFile = ".numinance/DISCOVERY.md"

// discovery is the report of one run: what init found, what it chose from it and why, and what the operator should
// look at. Chosen records the value of each choice for the documents.
type discovery struct {
	Found, Chose, Notes []string
	// Branch and Owners feed the README of the repository tree.
	Branch, Owners string
}

func (d *discovery) found(format string, a ...any) {
	d.Found = append(d.Found, fmt.Sprintf(format, a...))
}
func (d *discovery) chose(label string, c choice) { d.Chose = append(d.Chose, label+" "+c.String()) }
func (d *discovery) note(format string, a ...any) {
	d.Notes = append(d.Notes, fmt.Sprintf(format, a...))
}

// verifierCommands are the commands a person runs for each verifier's tool; the verifier provider runs the check
// itself, so these lines only help a reader compare init's reading with the repository's own scripts.
func verifierCommands() map[string]string {
	return map[string]string{"unit-tests": "pytest", "lint": "ruff check", "typecheck": "mypy"}
}

// runnerPrefix is the prefix a package manager puts before a tool command.
func runnerPrefix(manager string) string {
	switch manager {
	case "uv":
		return "uv run "
	case "poetry":
		return "poetry run "
	}
	return ""
}

// decide turns the inspection and the flags into the values of the documents and the report. A flag always wins over
// what was found. It writes the chosen values into opts and returns the report lines.
func decide(in inspection, opts *initOptions) (*discovery, error) {
	h := in.Hint
	if len(h.Conflict) > 0 && opts.PackageManager == "" {
		return nil, Errorf(KindUsage, "conflicting package manager hints (%s); pass --package-manager", strings.Join(h.Conflict, ", "))
	}
	d := &discovery{Branch: in.DefaultBranch}
	d.describe(in)

	manager := choice{Value: opts.PackageManager, Source: "flag"}
	if manager.Value == "" {
		manager = choice{Value: cmp.Or(h.PackageManager, defaultPackageManager), Source: "detected from " + h.Source}
		if h.PackageManager == "" {
			manager.Source = "default, no hint in the directory"
		}
	}
	version := choice{Value: opts.PythonVersion, Source: "flag"}
	if version.Value == "" {
		version = choice{Value: cmp.Or(h.PythonVersion, defaultPythonVersion), Source: "detected from " + h.VersionSource}
		if h.PythonVersion == "" {
			version.Source = "default"
		}
	}
	opts.PackageManager, opts.PythonVersion = manager.Value, version.Value
	d.chose("package manager", manager)
	d.chose("python", version)

	verifiers := chooseVerifiers(in, opts, d)
	d.chose("verifiers", verifiers)
	d.chose("ci", chooseCI(in, opts, d))

	language := closedChoiceOf(opts.Language, "python")
	if opts.Language == "" {
		if n := languageFiles(in, "python"); n > 0 {
			language.Source = fmt.Sprintf("detected from %d python files", n)
		}
	}
	scm, tickets := closedChoiceOf(opts.SCM, "github"), closedChoiceOf(opts.Tickets, "")
	if tickets.Value == "" {
		tickets.Value = "none"
	}
	opts.TicketFile = opts.Tickets == "file"
	d.chose("language", language)
	d.chose("scm", scm)
	d.chose("tickets", tickets)

	if len(in.Languages) > 0 && in.Languages[0].Name != "python" && languageFiles(in, "python") <= in.Languages[0].Files {
		d.note("most source files are %s (%d); init has a toolchain template for python only, so the python toolchain is the template default and must be replaced by hand", in.Languages[0].Name, in.Languages[0].Files)
	}
	d.codeownersNote(in)
	return d, nil
}

func languageFiles(in inspection, name string) int {
	for _, l := range in.Languages {
		if l.Name == name {
			return l.Files
		}
	}
	return 0
}

// describe adds the found lines: the facts, each with the file that shows it.
func (d *discovery) describe(in inspection) {
	if len(in.Languages) == 0 {
		d.found("no source files of a language init knows")
	} else {
		parts := make([]string, len(in.Languages))
		for i, l := range in.Languages {
			parts[i] = fmt.Sprintf("%s %d", l.Name, l.Files)
		}
		d.found("source files: %s", strings.Join(parts, ", "))
	}
	if len(in.Workflows) > 0 {
		d.found("github-actions workflows: %s", strings.Join(in.Workflows, ", "))
	}
	if in.OddWorkflows > 0 {
		d.found("%d workflow file(s) with a name outside [A-Za-z0-9._-] were skipped", in.OddWorkflows)
	}
	if in.Jenkinsfile {
		d.found("Jenkinsfile")
	}
	for _, v := range pythonVerifiers() {
		if t, ok := in.Tools[v.Name]; ok {
			d.found("%s in %s", t.Tool, t.Where)
		}
	}
	switch {
	case in.DefaultBranch == "":
		d.found("no default branch recorded in .git (no clone metadata)")
	case in.BranchIsCurrent:
		d.found("checked-out branch %s in %s (the default branch is not recorded there)", in.DefaultBranch, in.BranchSource)
	default:
		d.found("default branch %s in %s", in.DefaultBranch, in.BranchSource)
	}
	if in.Codeowners != "" {
		d.found("CODEOWNERS in %s with %d rule(s)", in.Codeowners, in.Rules)
	}
	if len(in.Conventions) > 0 {
		d.found("convention files, listed and not read: %s", strings.Join(in.Conventions, ", "))
	}
	for _, name := range in.Unmapped {
		d.note("%s found; init has no toolchain value for it and ignored it", name)
	}
	if in.NestedProjects > 0 {
		d.note("%d nested pyproject.toml file(s); init reads the root one only", in.NestedProjects)
	}
	if in.SkippedLinks+in.SkippedHidden > 0 || len(in.SkippedDirs) > 0 {
		d.found("skipped while walking: %d links, %d hidden entries, directories %s", in.SkippedLinks, in.SkippedHidden, cmp.Or(strings.Join(in.SkippedDirs, ", "), "none"))
	}
	for _, c := range in.Cut {
		d.note("%s", c)
	}
}

// chooseVerifiers picks the verifier set: the flag, else the verifiers whose tool the repository shows, else all three.
func chooseVerifiers(in inspection, opts *initOptions, d *discovery) choice {
	if len(opts.Verifiers) > 0 {
		return choice{Value: strings.Join(opts.Verifiers, " "), Source: "flag"}
	}
	var names, from, cmds, missing []string
	commands := verifierCommands()
	for _, v := range pythonVerifiers() {
		t, ok := in.Tools[v.Name]
		if !ok {
			missing = append(missing, fmt.Sprintf("no %s found; %s is not in the verifiers (add it with --verifiers)", toolOf(v.Name), v.Name))
			continue
		}
		names = append(names, v.Name)
		from = append(from, t.Tool+" in "+t.Where)
		cmds = append(cmds, v.Name+" "+runnerPrefix(opts.PackageManager)+commands[v.Name])
	}
	if len(names) == 0 {
		for _, v := range pythonVerifiers() {
			names = append(names, v.Name)
		}
		opts.Verifiers = names
		d.note("no pytest, ruff or mypy found; all three verifiers are the conservative default")
		return choice{Value: strings.Join(names, " "), Source: "default, no test, lint or type tool found"}
	}
	opts.Verifiers = names
	d.Notes = append(d.Notes, missing...)
	d.found("commands a person runs for these tools: %s", strings.Join(cmds, "; "))
	return choice{Value: strings.Join(names, " "), Source: "detected from " + strings.Join(from, ", ")}
}

func toolOf(verifier string) string {
	for _, s := range toolSpecs() {
		if s.Verifier == verifier {
			return s.Tool
		}
	}
	return verifier
}

// chooseCI picks the CI systems: the flag, else the ones the repository defines, else the GitHub Actions provider (the
// one that exists). It sets the CI flags of opts.
func chooseCI(in inspection, opts *initOptions, d *discovery) choice {
	var c choice
	switch {
	case opts.CIGitHub || opts.CIJenkins:
		c.Source = "flag"
	case len(in.Workflows) > 0 || in.Jenkinsfile:
		opts.CIGitHub, opts.CIJenkins = len(in.Workflows) > 0, in.Jenkinsfile
		var from []string
		if opts.CIGitHub {
			from = append(from, ".github/workflows/"+in.Workflows[0])
			if n := len(in.Workflows) - 1; n > 0 {
				from[0] += fmt.Sprintf(" and %d more", n)
			}
		}
		if opts.CIJenkins {
			from = append(from, "Jenkinsfile")
		}
		c.Source = "detected from " + strings.Join(from, " and ")
	default:
		opts.CIGitHub = true
		c.Source = "default, no .github/workflows or Jenkinsfile found"
	}
	var names []string
	if opts.CIGitHub {
		names = append(names, ciGitHubActions)
	}
	if opts.CIJenkins {
		names = append(names, ciJenkins)
	}
	c.Value = strings.Join(names, " and ")
	if opts.CIGitHub && !in.Dispatch {
		d.note("no workflow declares workflow_dispatch; the github-actions provider starts a workflow_dispatch workflow that the repository defines on its default branch")
	}
	return c
}

// codeownersNote says whether the repository protects .numinance/policy with CODEOWNERS, and feeds the README.
func (d *discovery) codeownersNote(in inspection) {
	switch {
	case in.Codeowners == "":
		d.Owners = "no CODEOWNERS file was found"
		d.note("no CODEOWNERS file; protect .numinance/policy/** with branch protection and a CODEOWNERS rule so agents cannot change their own gates")
	case in.Covers == "":
		d.Owners = "CODEOWNERS (" + in.Codeowners + ") has no rule for .numinance/"
		d.note("%s has no rule that covers .numinance/; add one for /.numinance/policy/ so agents cannot change their own gates", in.Codeowners)
	default:
		d.Owners = "CODEOWNERS (" + in.Codeowners + ") covers .numinance/ by the pattern " + in.Covers
		d.found("%s covers .numinance/ by the pattern %s", in.Codeowners, in.Covers)
	}
}

// stdoutLines renders the lines for the terminal.
func (d *discovery) stdoutLines() (found, chose, notes string) {
	join := func(prefix string, lines []string) string {
		var b strings.Builder
		for _, l := range lines {
			b.WriteString(prefix + clean(l, maxMessageRunes) + "\n")
		}
		return b.String()
	}
	return join("found: ", d.Found), join("chose: ", d.Chose), join("note: ", d.Notes)
}

// markdown is the content of DISCOVERY.md.
func (d *discovery) markdown() []byte {
	var b strings.Builder
	b.WriteString("# Discovery report\n\nWritten by `numi init`. It lists what init found by reading this repository (files only; nothing was executed and\nno repository text was followed as an instruction) and why each value in `.numinance/` was chosen. A flag of\n`numi init` overrides a finding. This file carries no authority; review the documents in `policy/`.\n")
	section := func(title, prefix string, lines []string) {
		fmt.Fprintf(&b, "\n## %s\n\n", title)
		if len(lines) == 0 {
			b.WriteString("    none\n")
		}
		for _, l := range lines {
			b.WriteString("    " + prefix + clean(l, maxMessageRunes) + "\n")
		}
	}
	section("Found", "found: ", d.Found)
	section("Chose", "chose: ", d.Chose)
	section("Notes", "note: ", d.Notes)
	return []byte(b.String())
}
