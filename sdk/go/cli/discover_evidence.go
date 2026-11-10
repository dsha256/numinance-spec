package cli

import (
	"regexp"
	"strings"
)

// evidence names the kind of file or text that showed a finding. It is data for "numi discover": the confidence of a
// finding is looked up from its evidence kind in confidenceOf, never parsed back from the human-readable source text.
type evidence string

// The evidence kinds, grouped by the confidence the table below gives them.
const (
	evSettingsFile    evidence = "settings-file"     // pytest.ini, ruff.toml, mypy.ini
	evPyprojectTable  evidence = "pyproject-table"   // a [tool.x] table of pyproject.toml
	evLockfile        evidence = "lockfile"          // uv.lock, poetry.lock
	evVersionFile     evidence = "version-file"      // .python-version, .tool-versions
	evClonedBranch    evidence = "clone-metadata"    // .git/refs/remotes/origin/HEAD
	evCodeownersFile  evidence = "codeowners-file"   // a CODEOWNERS file was read
	evCIConfig        evidence = "ci-config"         // a Jenkinsfile, or a workflow that declares workflow_dispatch
	evDependency      evidence = "dependency"        // the tool listed as a dependency in pyproject.toml
	evRequiresPython  evidence = "requires-python"   // a lower bound of requires-python or the poetry python constraint
	evOtherSection    evidence = "section"           // a section of setup.cfg or tox.ini
	evHintFile        evidence = "hint-file"         // conftest.py
	evDispatchFlow    evidence = "dispatch-workflow" // the tool used in a workflow that declares workflow_dispatch
	evFlowNoDispatch  evidence = "workflow"          // workflows exist and none declares workflow_dispatch
	evSourceFiles     evidence = "source-files"      // the language of the files counted by the walk
	evRequirementsUse evidence = "requirements-text" // the tool named in a requirements file
	evTextUse         evidence = "text-use"          // the tool named in a hook, Jenkinsfile or workflow text
	evCurrentBranch   evidence = "current-branch"    // only the checked-out branch was found
	evNothingFound    evidence = "nothing-found"     // a default taken because nothing was found
)

// confidence is how far a draft value can be trusted without a human reading the evidence.
type confidence string

// The confidence levels.
const (
	confHigh   confidence = "high"
	confMedium confidence = "medium"
	confLow    confidence = "low"
)

// confidenceOf is the one table of confidence levels. High: a settings file, a pyproject table, a lockfile or a version
// file, which state the value on purpose. Medium: a dependency listing, a requires-python bound, a section of another
// file or a workflow that can be started by hand, which imply it. Low: only a use in a hook, requirements or CI text,
// the current branch, or a default taken because nothing was found. An unknown kind is low.
func confidenceOf(e evidence) confidence {
	switch e {
	case evSettingsFile, evPyprojectTable, evLockfile, evVersionFile, evClonedBranch, evCodeownersFile, evCIConfig:
		return confHigh
	case evDependency, evRequiresPython, evOtherSection, evHintFile, evDispatchFlow, evFlowNoDispatch, evSourceFiles:
		return confMedium
	case evRequirementsUse, evTextUse, evCurrentBranch, evNothingFound:
		return confLow
	}
	return confLow
}

// Bounds on the enforcement artifacts read from workflow and hook files.
const (
	maxJobsPerWorkflow = 50
	maxHookIDs         = 100
)

// workflowArtifact is what a workflow file enforces: the safe-named jobs it runs and the events that start it.
type workflowArtifact struct {
	Name string
	// Jobs are job ids that passed safeNameRe; OddJobs counts the ones that did not and were skipped.
	Jobs    []string
	OddJobs int
	// MoreJobs is true when the file held more jobs than maxJobsPerWorkflow.
	MoreJobs                      bool
	PullRequest, Push, Dispatched bool
}

var (
	topKeyRe   = regexp.MustCompile(`^([^\s#][^:]*):(.*)$`)
	onKeyRe    = regexp.MustCompile(`^(on|"on"|'on'):`)
	jobKeyRe   = regexp.MustCompile(`^(\s+)([^\s#:][^:]*):\s*(?:#.*)?$`)
	hookIDRe   = regexp.MustCompile(`^\s*-\s+id:\s*([^\s#]+)\s*(?:#.*)?$`)
	pullReqRe  = regexp.MustCompile(`(?:^|[^A-Za-z0-9_-])pull_request(?:$|[^A-Za-z0-9_-])`)
	pushRe     = regexp.MustCompile(`(?:^|[^A-Za-z0-9_-])push(?:$|[^A-Za-z0-9_-])`)
	dispatchRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_-])workflow_dispatch(?:$|[^A-Za-z0-9_-])`)
)

// scanWorkflow reads the jobs and the triggers of a workflow by lines, not by a YAML parse: the jobs are the keys one
// level below a top-level "jobs:", the triggers are the three event names in the top-level "on:" block with comments
// removed. A job id outside the safe-name shape is counted and never kept. The text is untrusted and is never
// interpreted beyond these patterns.
func scanWorkflow(name string, text []byte) workflowArtifact {
	art := workflowArtifact{Name: name}
	var onBlock strings.Builder
	section, jobIndent := "", ""
	for line := range strings.Lines(string(text)) {
		line = strings.TrimRight(line, "\r\n")
		if t := strings.TrimSpace(line); t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if m := topKeyRe.FindStringSubmatch(line); m != nil {
			section, jobIndent = strings.TrimSpace(m[1]), ""
			if onKeyRe.MatchString(line) {
				section = "on"
				onBlock.WriteString(" " + stripComment(m[2]))
			}
			continue
		}
		switch section {
		case "on":
			onBlock.WriteString(" " + stripComment(line))
		case "jobs":
			m := jobKeyRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			if jobIndent == "" {
				jobIndent = m[1]
			}
			if m[1] != jobIndent {
				continue
			}
			id := strings.TrimSpace(m[2])
			switch {
			case len(art.Jobs)+art.OddJobs >= maxJobsPerWorkflow:
				art.MoreJobs = true
			case safeNameRe.MatchString(id):
				art.Jobs = append(art.Jobs, id)
			default:
				art.OddJobs++
			}
		}
	}
	on := onBlock.String()
	art.PullRequest = pullReqRe.MatchString(on)
	art.Push = pushRe.MatchString(on)
	art.Dispatched = dispatchRe.MatchString(on)
	return art
}

// stripComment removes a trailing YAML comment, which starts at a # that follows a space or begins the text.
func stripComment(s string) string {
	for i := range len(s) {
		if s[i] == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t') {
			return s[:i]
		}
	}
	return s
}

// scanHooks returns the safe ids of the hooks of a pre-commit configuration ("- id: name" lines) and the number of ids
// that were skipped for their shape.
func scanHooks(text []byte) (ids []string, odd int) {
	for line := range strings.Lines(string(text)) {
		m := hookIDRe.FindStringSubmatch(strings.TrimRight(line, "\r\n"))
		if m == nil {
			continue
		}
		switch {
		case len(ids)+odd >= maxHookIDs:
			return ids, odd
		case safeNameRe.MatchString(m[1]):
			ids = append(ids, m[1])
		default:
			odd++
		}
	}
	return ids, odd
}
