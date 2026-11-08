package cli

import (
	"bytes"
	"embed"
	"fmt"
	"slices"
	"strings"
	"text/template"
)

//go:embed initdata/*.tmpl
var initTemplates embed.FS

// maxInitFileBytes bounds one generated file. The templates are small; the bound is a tripwire for a template edit
// that grows without a reason.
const maxInitFileBytes = 16 << 10

// The CI systems a repository can be wired to.
const (
	ciGitHubActions = "github-actions"
	ciJenkins       = "jenkins"
	ciBoth          = "both"
)

// ciSystem is one CI system in the generated documents. Vendor names appear here and in the templates only: they are
// template data of the first onboarding, not logic of the CLI.
type ciSystem struct {
	Stage    string
	Provider string
	Instance string
	Comment  string
}

func githubActions() ciSystem {
	return ciSystem{
		Stage:    "ci-github-actions",
		Provider: "dev.numinance.ci-github-actions",
		Instance: "ci",
		Comment:  "GitHub Actions: runs the repository's own workflow through the ciexecutor port.",
	}
}

func jenkins() ciSystem {
	return ciSystem{
		Stage:    "ci-jenkins",
		Provider: "dev.numinance.official/jenkins",
		Instance: "jenkins-onprem",
		Comment:  "Jenkins: the ciexecutor port exists, no Jenkins provider exists yet (placeholder id).",
	}
}

// verifierSpec is one verifier of the Python toolchain: the name the playbook uses and the check the verifier
// provider runs.
type verifierSpec struct {
	Name    string
	Check   string
	Timeout int
}

// pythonVerifiers is the closed set of verifiers, in the order the toolchain and the playbook list them.
func pythonVerifiers() []verifierSpec {
	return []verifierSpec{
		{Name: "unit-tests", Check: "pytest", Timeout: 300},
		{Name: "lint", Check: "ruff", Timeout: 60},
		{Name: "typecheck", Check: "mypy", Timeout: 120},
	}
}

// initData is what the templates read. Every string in it comes from a closed set or a validated pattern, so it can be
// written into YAML as it is.
type initData struct {
	// OrgMode is true when the shared documents go to an organization policy repository at the org scope.
	OrgMode bool
	// Scope is the metadata.scope of the shared documents.
	Scope                 string
	PythonVersion         string
	PackageManager        string
	Verifiers             []verifierSpec
	CI                    []ciSystem
	GitHub, Jenkins, Both bool
	PrimaryCI             ciSystem
	// TicketFile is true for --tickets file: only the README says where tickets come from; no binding is written.
	TicketFile bool
}

func newInitData(o initOptions) initData {
	d := initData{
		OrgMode:        o.OrgDir != "",
		Scope:          "SCOPE_REPO",
		PythonVersion:  o.PythonVersion,
		PackageManager: o.PackageManager,
		TicketFile:     o.TicketFile,
	}
	if d.OrgMode {
		d.Scope = "SCOPE_ORG"
	}
	for _, v := range pythonVerifiers() {
		for _, want := range o.Verifiers {
			if v.Name == want {
				d.Verifiers = append(d.Verifiers, v)
			}
		}
	}
	if o.CIGitHub {
		d.CI = append(d.CI, githubActions())
		d.GitHub = true
	}
	if o.CIJenkins {
		d.CI = append(d.CI, jenkins())
		d.Jenkins = true
	}
	d.Both = d.GitHub && d.Jenkins
	// The one effective ciexecutor binding is the GitHub Actions one when it is selected: its provider exists.
	d.PrimaryCI = d.CI[0]
	return d
}

// CIText names the CI systems for a comment or a README.
func (d initData) CIText() string {
	names := make([]string, len(d.CI))
	for i, c := range d.CI {
		names[i] = strings.TrimPrefix(c.Stage, "ci-")
	}
	return strings.Join(names, " and ")
}

// CIStageList is the requires list of the pr-gate.
func (d initData) CIStageList() string {
	stages := make([]string, len(d.CI))
	for i, c := range d.CI {
		stages[i] = c.Stage
	}
	return strings.Join(stages, ", ")
}

// VerifierList is the verifiers list of a playbook step.
func (d initData) VerifierList() string { return strings.Join(d.verifierNames(), ", ") }

// VerifierText is the verifier set for a README.
func (d initData) VerifierText() string { return strings.Join(d.verifierNames(), ", ") }

// HasTests reports whether the unit-tests verifier is in the set.
func (d initData) HasTests() bool {
	for _, v := range d.Verifiers {
		if v.Name == "unit-tests" {
			return true
		}
	}
	return false
}

func (d initData) verifierNames() []string {
	names := make([]string, len(d.Verifiers))
	for i, v := range d.Verifiers {
		names[i] = v.Name
	}
	return names
}

// fileSpec names a generated file and the template it is rendered from.
type fileSpec struct{ path, tmpl string }

// outFile is one file to write, by slash-separated path relative to the repository root.
type outFile struct {
	Path string
	Data []byte
}

// protectedDir is where the documents that carry authority live (design section 12).
const protectedDir = ".numinance/policy/"

// planRepoTree returns the files of the repository tree. With an organization directory the shared documents go there
// instead (planOrgTree) and the repository holds its own profile only.
func planRepoTree(d initData) ([]outFile, error) {
	specs := []fileSpec{
		{path: protectedDir + "sdlc-profile.yaml", tmpl: "sdlc-profile-repo.yaml.tmpl"},
	}
	if !d.OrgMode {
		specs = append(specs,
			fileSpec{path: protectedDir + "autonomy.yaml", tmpl: "autonomy.yaml.tmpl"},
			fileSpec{path: protectedDir + "stage-registry.yaml", tmpl: "stage-registry.yaml.tmpl"},
			fileSpec{path: protectedDir + "playbook-bugfix-bounded.yaml", tmpl: "playbook.yaml.tmpl"},
			fileSpec{path: protectedDir + "toolchain-python.yaml", tmpl: "toolchain.yaml.tmpl"},
		)
	}
	if d.Both && !d.OrgMode {
		specs = append(specs, fileSpec{path: ".numinance/ci-jenkins.yaml", tmpl: "ci-jenkins.yaml.tmpl"})
	}
	specs = append(specs, fileSpec{path: ".numinance/README.md", tmpl: "readme-repo.md.tmpl"})
	return render(specs, d)
}

// planOrgTree returns the files of the organization policy tree.
func planOrgTree(d initData) ([]outFile, error) {
	specs := []fileSpec{
		{path: protectedDir + "sdlc-profile.yaml", tmpl: "sdlc-profile-org.yaml.tmpl"},
		{path: protectedDir + "autonomy.yaml", tmpl: "autonomy.yaml.tmpl"},
		{path: protectedDir + "stage-registry.yaml", tmpl: "stage-registry.yaml.tmpl"},
		{path: protectedDir + "playbook-bugfix-bounded.yaml", tmpl: "playbook.yaml.tmpl"},
		{path: protectedDir + "toolchain-python.yaml", tmpl: "toolchain.yaml.tmpl"},
		{path: ".numinance/README.md", tmpl: "readme-org.md.tmpl"},
	}
	if d.Both {
		specs = slices.Insert(specs, len(specs)-1, fileSpec{path: protectedDir + "ci-jenkins.yaml", tmpl: "ci-jenkins-org.yaml.tmpl"})
	}
	return render(specs, d)
}

func render(specs []fileSpec, d initData) ([]outFile, error) {
	out := make([]outFile, 0, len(specs))
	for _, s := range specs {
		t, err := template.ParseFS(initTemplates, "initdata/"+s.tmpl)
		if err != nil {
			return nil, fmt.Errorf("numi init: template %s: %w", s.tmpl, err)
		}
		var b bytes.Buffer
		if err := t.Execute(&b, d); err != nil {
			return nil, fmt.Errorf("numi init: template %s: %w", s.tmpl, err)
		}
		if b.Len() > maxInitFileBytes {
			return nil, fmt.Errorf("numi init: %s is larger than %d bytes", s.path, maxInitFileBytes)
		}
		out = append(out, outFile{Path: s.path, Data: b.Bytes()})
	}
	return out, nil
}
