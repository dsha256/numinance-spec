package cli

import (
	"fmt"
	"slices"
	"strings"
)

// annotationKey is the reverse-DNS key under extensions that holds the confidence annotations of a drafted document.
// The Engine never reads extensions, so an annotation cannot change what a document means.
const annotationKey = "dev.numinance.discover"

// discoverRow is one decision of a draft: what was chosen, the evidence that showed it and the confidence in it.
type discoverRow struct {
	Key, Label, Value string
	// Source is the file (and table) that showed the value, or the reason a default was taken.
	Source string
	Level  confidence
	// NoEvidence is true for a verifier that the draft names with no tool found for it.
	NoEvidence bool
}

// discoverFacts is what one read of the repository decided: the inspection, the init options the decisions filled in
// and the rows. Two reads of an unchanged repository give equal fingerprints.
type discoverFacts struct {
	in   inspection
	opts initOptions
	disc *discovery
	rows []discoverRow
}

func newDiscoverFacts(in inspection, opts initOptions, d *discovery) *discoverFacts {
	f := &discoverFacts{in: in, opts: opts, disc: d}
	h := in.Hint
	add := func(key, label, value, source string, ev evidence) {
		f.rows = append(f.rows, discoverRow{Key: key, Label: label, Value: value, Source: source, Level: confidenceOf(ev)})
	}

	if h.PackageManager != "" {
		add("package_manager", "package manager", opts.PackageManager, h.Source, h.ManagerEvidence)
	} else {
		add("package_manager", "package manager", opts.PackageManager, "default, no hint in the repository", evNothingFound)
	}
	if h.PythonVersion != "" {
		add("python", "python", opts.PythonVersion, h.VersionSource, h.VersionEvidence)
	} else {
		add("python", "python", opts.PythonVersion, "default, no version found", evNothingFound)
	}
	if n := languageFiles(in, "python"); n > 0 {
		add("language", "language", "python", fmt.Sprintf("%d python files in the bounded walk", n), evSourceFiles)
	} else {
		add("language", "language", "python", "default, no python files found", evNothingFound)
	}
	for _, name := range opts.Verifiers {
		if t, ok := in.Tools[name]; ok {
			add("verifier:"+name, "verifier "+name, t.Tool, t.Where, t.Evidence)
			continue
		}
		add("verifier:"+name, "verifier "+name, toolOf(name), "no evidence, conservative default", evNothingFound)
		f.rows[len(f.rows)-1].NoEvidence = true
	}
	f.rows = append(f.rows, f.ciRow())
	add("scm", "scm", "github", "default, github is the only provider that exists", evNothingFound)
	add("tickets", "tickets", "none", "default, no ticketing provider exists yet", evNothingFound)
	switch {
	case in.DefaultBranch == "":
		add("branch", "default branch", "none", "no clone metadata in .git", evNothingFound)
	case in.BranchIsCurrent:
		add("branch", "default branch", in.DefaultBranch, in.BranchSource+" (the checked-out branch, not the default)", evCurrentBranch)
	default:
		add("branch", "default branch", in.DefaultBranch, in.BranchSource, evClonedBranch)
	}
	switch {
	case in.Codeowners == "":
		add("codeowners", "CODEOWNERS", "none found", "no CODEOWNERS file", evNothingFound)
	case in.Covers == "":
		add("codeowners", "CODEOWNERS", "no rule for .numinance/", in.Codeowners, evCodeownersFile)
	default:
		add("codeowners", "CODEOWNERS", "covers .numinance/ by "+in.Covers, in.Codeowners, evCodeownersFile)
	}
	// The autonomy level is a decision of this command, not a finding, so it carries no evidence kind.
	f.rows = append(f.rows, discoverRow{Key: "autonomy", Label: "autonomy", Value: "PR only", Source: "fixed by numi discover, not discovered", Level: confHigh})
	return f
}

// ciRow is the row of the CI systems: a Jenkinsfile, or a workflow that declares workflow_dispatch, is firm evidence;
// workflows with no dispatch trigger are weaker because the github-actions provider starts a workflow_dispatch workflow.
func (f *discoverFacts) ciRow() discoverRow {
	in, opts := f.in, f.opts
	var names, sources []string
	levels := []confidence{}
	if opts.CIGitHub {
		names = append(names, ciGitHubActions)
		src, ev := "default, no .github/workflows or Jenkinsfile found", evNothingFound
		if len(in.Artifacts) > 0 {
			src, ev = ".github/workflows/"+in.Artifacts[0].Name, evFlowNoDispatch
			for _, a := range in.Artifacts {
				if a.Dispatched {
					src, ev = ".github/workflows/"+a.Name, evCIConfig
					break
				}
			}
		}
		sources = append(sources, src)
		levels = append(levels, confidenceOf(ev))
	}
	if opts.CIJenkins {
		names = append(names, ciJenkins)
		sources = append(sources, "Jenkinsfile")
		levels = append(levels, confidenceOf(evCIConfig))
	}
	return discoverRow{Key: "ci", Label: "ci", Value: strings.Join(names, " and "), Source: strings.Join(sources, " and "), Level: lowest(levels...)}
}

// lowest is the lowest of the levels.
func lowest(levels ...confidence) confidence {
	rank := map[confidence]int{confLow: 0, confMedium: 1, confHigh: 2}
	out := confHigh
	for _, l := range levels {
		if rank[l] < rank[out] {
			out = l
		}
	}
	return out
}

func (f *discoverFacts) row(key string) discoverRow {
	for _, r := range f.rows {
		if r.Key == key {
			return r
		}
	}
	return discoverRow{}
}

// fingerprint is every decision as one list, for the comparison of two reads.
func (f *discoverFacts) fingerprint() []string {
	out := make([]string, len(f.rows))
	for i, r := range f.rows {
		out[i] = r.Label + "|" + r.Value + "|" + r.Source + "|" + string(r.Level)
	}
	return out
}

// annotation is one field of a document with the confidence and evidence of its value.
type annotation struct {
	Field, Evidence string
	Level           confidence
}

// annotationsFor lists the annotations of the document at path, from the one table of where each decision lands. The
// rows behind a field are listed by key; the level is the lowest of them.
func (f *discoverFacts) annotationsFor(path string) []annotation {
	type target struct {
		field string
		keys  []string
	}
	targets := make([]target, 0, 3+len(f.opts.Verifiers))
	verifierKeys := make([]string, 0, len(f.opts.Verifiers))
	for _, name := range f.opts.Verifiers {
		verifierKeys = append(verifierKeys, "verifier:"+name)
	}
	switch path {
	case protectedDir + "sdlc-profile.yaml":
		targets = append(targets,
			target{"spec.toolchains", []string{"python"}},
			target{"spec.adapters.ciexecutor", []string{"ci"}},
			target{"spec.adapters.scm", []string{"scm"}},
		)
	case protectedDir + "toolchain-python.yaml":
		targets = append(targets,
			target{"spec.language", []string{"language"}},
			target{"spec.version", []string{"python"}},
			target{"spec.package_manager", []string{"package_manager"}},
		)
		for _, name := range f.opts.Verifiers {
			targets = append(targets, target{"spec.verifiers." + name, []string{"verifier:" + name}})
		}
	case protectedDir + "playbook-bugfix-bounded.yaml":
		targets = append(targets, target{"spec.steps.fix.verifiers", verifierKeys})
	case protectedDir + "stage-registry.yaml":
		targets = append(targets, target{"spec.stages.ci", []string{"ci"}})
	case protectedDir + "autonomy.yaml":
		targets = append(targets, target{"spec.defaults.level", []string{"autonomy"}})
	case ".numinance/ci-jenkins.yaml":
		targets = append(targets, target{"spec.ports.ciexecutor", []string{"ci"}})
	}
	anns := make([]annotation, 0, len(targets))
	for _, t := range targets {
		levels := make([]confidence, 0, len(t.keys))
		var sources []string
		for _, k := range t.keys {
			r := f.row(k)
			levels = append(levels, r.Level)
			if !slices.Contains(sources, r.Source) {
				sources = append(sources, r.Source)
			}
		}
		anns = append(anns, annotation{Field: t.field, Level: lowest(levels...), Evidence: strings.Join(sources, "; ")})
	}
	return anns
}

// discoverDocuments returns the drafted files with the confidence annotations added to each YAML document. The
// README of init's tree is left out: it says init wrote it, and DISCOVER.md is this draft's report.
func discoverDocuments(files []outFile, f *discoverFacts) []outFile {
	out := make([]outFile, 0, len(files))
	for _, file := range files {
		if file.Path == ".numinance/README.md" {
			continue
		}
		if strings.HasSuffix(file.Path, ".yaml") {
			file.Data = annotate(file.Data, f.annotationsFor(file.Path))
		}
		out = append(out, file)
	}
	return out
}

// annotate adds the top-level extensions block after the metadata line. The templates have exactly one metadata line
// and no extensions; a document without that line is returned unchanged, and the self-test then reports the missing
// annotations through the loader or the review of the draft.
func annotate(doc []byte, anns []annotation) []byte {
	lines := strings.SplitAfter(string(doc), "\n")
	at := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "metadata:") })
	if at < 0 || len(anns) == 0 {
		return doc
	}
	var b strings.Builder
	b.WriteString("# The extensions block holds the confidence of each drafted value (high, medium or low) and the evidence\n")
	b.WriteString("# that showed it. The Engine never reads it. Written by \"numi discover\"; review each value before merging.\n")
	b.WriteString("extensions:\n  " + annotationKey + ":\n")
	for _, a := range anns {
		fmt.Fprintf(&b, "    %s: {confidence: %s, evidence: \"%s\"}\n", a.Field, a.Level, yamlText(a.Evidence))
	}
	out := slices.Concat(lines[:at+1], []string{b.String()}, lines[at+1:])
	return []byte(strings.Join(out, ""))
}

// yamlText keeps the characters an evidence text can have (file names and fixed words) and replaces any other with
// '?', so no byte of repository text can break out of a double-quoted YAML scalar.
func yamlText(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case strings.ContainsRune(" ._-/:;,()[]+", r):
			return r
		}
		return '?'
	}, s)
}
