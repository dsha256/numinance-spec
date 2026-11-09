// Package providertemplate renders the SDK template of a capability provider (design section 17: the Capability Gap
// Resolver generates "a provider from the SDK template: source, manifest.yaml with permissions, conformance tests and
// security tests"). The template is plain embedded files, not a kind: an author can render it by hand to start a provider,
// and the Engine renders it to start a build.
//
// Render returns a skeleton. It ships no behaviour: the provider serves nothing, and the conformance and security tests
// fail until they are written. The manifest declares risk, side effect and idempotency at their worst case, because the
// template cannot know the verb, and carries no SBOM or provenance reference, which the build adds. Output is
// deterministic: the same Params give the same bytes.
//
// Every value interpolated into a file is checked against a strict character set first, so a parameter cannot add a
// field, a path or code to a file. The package imports only the standard library and sdk/go/extpkg.
package providertemplate

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/numinance/numinance-spec/sdk/go/extpkg"
)

// ErrParams is wrapped by every refusal of the parameters.
var ErrParams = errors.New("providertemplate: invalid parameters")

//go:embed templates/*.tmpl
var templateFS embed.FS

// Bounds on the parameters; a provider asks for few intents, and a long list is a mistake or an attack.
const (
	maxIntents = 32
	maxSources = 16
)

var (
	pluginIDRe   = regexp.MustCompile(`^[a-z]([a-z0-9-]*[a-z0-9])?(\.[a-z]([a-z0-9-]*[a-z0-9])?)+$`)
	capabilityRe = regexp.MustCompile(`^([a-z][a-z0-9]*)\.([a-z][a-z0-9_]*)$`)
	versionRe    = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)
	// signerRe is "<issuer>#<subject>": URL and e-mail characters, never a quote, a backslash or a space.
	signerRe     = regexp.MustCompile(`^[A-Za-z0-9:/._#@+=%-]{1,512}$`)
	credentialRe = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,127}$`)
	tokenRe      = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
	hostRe       = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
	pathRe       = regexp.MustCompile(`^/[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$`)
	sourceRe     = regexp.MustCompile(`^[A-Za-z0-9._/:-]{1,256}$`)
)

// Network is one egress destination.
type Network struct {
	Host string
	Port uint32
}

// Path is one sandbox path; ReadWrite false means read-only.
type Path struct {
	Path      string
	ReadWrite bool
}

// Params are the values the template is rendered from.
type Params struct {
	// PluginID is the reverse-DNS id of the provider, at least two labels.
	PluginID string
	// Version is x.y.z.
	Version string
	// Signer is the signer identity the manifest declares, "<issuer>#<subject>".
	Signer string
	// Capability is the capability the provider offers, port.verb; its port is the one port the provider implements.
	Capability string
	// PortMajor is the major of the port; zero means 1.
	PortMajor int
	// Credentials, Network, Filesystem and Environment are the intents the manifest declares: the authority asked for.
	// Environment must name at least one token, because the manifest rejects an empty scope and never reads it as all.
	Credentials []string
	Network     []Network
	Filesystem  []Path
	Environment []string
	// Sources are opaque references to the material discovery found; they are listed in SOURCES.md, never interpreted.
	Sources []string
}

// File is one rendered file: a clean relative slash-separated path and its content.
type File struct {
	Path string
	Data []byte
}

func refuse(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrParams, fmt.Sprintf(format, args...))
}

// validate checks every parameter and returns the port and the verb.
func (p Params) validate() (port, verb string, err error) {
	switch {
	case len(p.PluginID) > 253 || !pluginIDRe.MatchString(p.PluginID):
		return "", "", refuse("plugin id is not lowercase reverse-DNS labels")
	case !versionRe.MatchString(p.Version):
		return "", "", refuse("version is not x.y.z")
	case !signerRe.MatchString(p.Signer):
		return "", "", refuse("signer identity has characters outside the allowed set")
	case p.PortMajor < 0 || p.PortMajor > 99:
		return "", "", refuse("port major is out of range")
	case len(p.Environment) == 0:
		return "", "", refuse("environment scope is empty")
	case len(p.Credentials) > maxIntents || len(p.Network) > maxIntents || len(p.Filesystem) > maxIntents || len(p.Environment) > maxIntents:
		return "", "", refuse("more than %d of an intent", maxIntents)
	case len(p.Sources) > maxSources:
		return "", "", refuse("more than %d sources", maxSources)
	}
	m := capabilityRe.FindStringSubmatch(p.Capability)
	if m == nil || len(p.Capability) > 128 {
		return "", "", refuse("capability is not port.verb")
	}
	for _, c := range p.Credentials {
		if !credentialRe.MatchString(c) {
			return "", "", refuse("a credential intent is not a credential name")
		}
	}
	for _, e := range p.Environment {
		if !tokenRe.MatchString(e) {
			return "", "", refuse("an environment token is not a token")
		}
	}
	for _, n := range p.Network {
		if len(n.Host) > 253 || !hostRe.MatchString(n.Host) || n.Port < 1 || n.Port > 65535 {
			return "", "", refuse("a network intent is not a lowercase DNS name and a port")
		}
	}
	for _, f := range p.Filesystem {
		if len(f.Path) > 1024 || !pathRe.MatchString(f.Path) || hasDotSegment(f.Path) {
			return "", "", refuse("a filesystem intent is not a clean absolute path")
		}
	}
	for _, s := range p.Sources {
		if !sourceRe.MatchString(s) {
			return "", "", refuse("a source is not a short opaque reference")
		}
	}
	return m[1], m[2], nil
}

func hasDotSegment(p string) bool {
	for s := range strings.SplitSeq(p, "/") {
		if s == "." || s == ".." {
			return true
		}
	}
	return false
}

// data is what the templates see.
type data struct {
	Params
	Port, Verb, Name, Digest string
	PortMajor                int
}

// Render returns the files of the provider skeleton for p, in a fixed order. The manifest declares the content digest of
// every other file, as extpkg computes it, so the package measures as it declares.
func Render(p Params) ([]File, error) {
	port, verb, err := p.validate()
	if err != nil {
		return nil, err
	}
	major := p.PortMajor
	if major == 0 {
		major = 1
	}
	labels := strings.Split(p.PluginID, ".")
	d := data{Params: p, Port: port, Verb: verb, Name: labels[len(labels)-1], PortMajor: major}

	plan := []struct{ path, tmpl string }{
		{"README.md", "README.md.tmpl"},
		{"main.go", "main.go.tmpl"},
		{"provider/provider.go", "provider.go.tmpl"},
		{"provider/conformance_test.go", "conformance_test.go.tmpl"},
		{"provider/security_test.go", "security_test.go.tmpl"},
	}
	if len(p.Sources) > 0 {
		plan = append(plan, struct{ path, tmpl string }{"SOURCES.md", "sources.md.tmpl"})
	}
	funcs := template.FuncMap{"join": func(s []string) string { return strings.Join(s, ", ") }}
	var files []File
	for _, e := range plan {
		b, err := execute(funcs, e.tmpl, d)
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: e.path, Data: b})
	}
	tree := make([]extpkg.File, len(files))
	for i, f := range files {
		tree[i] = extpkg.File{Path: f.Path, Data: f.Data}
	}
	d.Digest = extpkg.ContentDigest(tree)
	manifest, err := execute(funcs, "manifest.yaml.tmpl", d)
	if err != nil {
		return nil, err
	}
	return append(files, File{Path: extpkg.ManifestFile, Data: manifest}), nil
}

func execute(funcs template.FuncMap, name string, d data) ([]byte, error) {
	t, err := template.New(name).Funcs(funcs).Option("missingkey=error").ParseFS(templateFS, "templates/"+name)
	if err != nil {
		return nil, fmt.Errorf("providertemplate: parse %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return nil, fmt.Errorf("providertemplate: render %s: %w", name, err)
	}
	return buf.Bytes(), nil
}
