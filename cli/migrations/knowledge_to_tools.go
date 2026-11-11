package migrations

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"google.golang.org/protobuf/proto"

	kindsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/kinds/v1"
	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
)

// ErrUnmapped reports a knowledge connection the codemod has no plugin for. The rewrite stops rather than guess one.
var ErrUnmapped = errors.New("migrations: knowledge connection has no plugin")

// PluginsByConnection maps each KnowledgeConnection name to the provider it names, the plugin of the AdapterTool that
// replaces the connection in an AdapterSelector.
func PluginsByConnection(conns ...*kindsv1.KnowledgeConnection) map[string]string {
	m := make(map[string]string, len(conns))
	for _, c := range conns {
		m[c.GetMetadata().GetName()] = c.GetSpec().GetProvider()
	}
	return m
}

// SelectorKnowledgeToTools returns a copy of sel in which every name of spec.ports.knowledge is a tools entry
// {plugin: plugins[name], instance: name} appended after the existing tools (an entry already present is not added
// twice) and the knowledge list is empty. A name without a plugin is ErrUnmapped. A selector without knowledge is
// returned as an equal copy.
func SelectorKnowledgeToTools(sel *kindsv1.AdapterSelector, plugins map[string]string) (*kindsv1.AdapterSelector, error) {
	out := proto.Clone(sel).(*kindsv1.AdapterSelector)
	spec := out.GetSpec()
	ports := spec.GetPorts()
	if len(ports.GetKnowledge()) == 0 {
		return out, nil
	}
	for _, name := range ports.GetKnowledge() {
		plugin, ok := plugins[name]
		if !ok || plugin == "" {
			return nil, fmt.Errorf("%w: %q", ErrUnmapped, name)
		}
		t := &kindsv1.AdapterTool{Plugin: plugin, Instance: name}
		if !slices.ContainsFunc(spec.Tools, func(e *kindsv1.AdapterTool) bool { return proto.Equal(e, t) }) {
			spec.Tools = append(spec.Tools, t)
		}
	}
	ports.Knowledge = nil
	return out, nil
}

// LockfileKnowledgeToPinnedTools returns a copy of lock in which every knowledge_tools entry is a pinned_tools entry
// {plugin: plugins[connection], connection, name, hash} and knowledge_tools is empty. plugins maps a connection name to
// the plugin_id of its extension as the lockfile pins it (a reverse-DNS id, not the provider path of a selector); a
// connection without one is ErrUnmapped. pinned_tools stay sorted by plugin, connection and name, as the install flow
// writes them. The hash is carried over: it is the approved hash and a codemod never re-approves.
func LockfileKnowledgeToPinnedTools(lock *kindsv1.Lockfile, plugins map[string]string) (*kindsv1.Lockfile, error) {
	out := proto.Clone(lock).(*kindsv1.Lockfile)
	spec := out.GetSpec()
	if len(spec.GetKnowledgeTools()) == 0 {
		return out, nil
	}
	for _, k := range spec.GetKnowledgeTools() {
		plugin, ok := plugins[k.GetConnection()]
		if !ok || plugin == "" {
			return nil, fmt.Errorf("%w: %q", ErrUnmapped, k.GetConnection())
		}
		t := &kindsv1.LockedTool{Plugin: plugin, Connection: k.GetConnection(), Name: k.GetName(), Hash: k.GetHash()}
		if !slices.ContainsFunc(spec.PinnedTools, func(e *kindsv1.LockedTool) bool { return proto.Equal(e, t) }) {
			spec.PinnedTools = append(spec.PinnedTools, t)
		}
	}
	spec.KnowledgeTools = nil
	slices.SortStableFunc(spec.PinnedTools, func(a, b *kindsv1.LockedTool) int {
		for _, c := range []int{cmp.Compare(a.GetPlugin(), b.GetPlugin()), cmp.Compare(a.GetConnection(), b.GetConnection()), cmp.Compare(a.GetName(), b.GetName())} {
			if c != 0 {
				return c
			}
		}
		return 0
	})
	return out, nil
}

// Plugins are the inputs of Document: SelectorPlugin maps a connection name to the provider of an AdapterTool, and
// LockPlugin maps it to the plugin_id of the Lockfile.
type Plugins struct {
	SelectorPlugin map[string]string
	LockPlugin     map[string]string
}

// Document applies the codemod that fits the kind of the YAML document data: an AdapterSelector or a Lockfile. It
// returns the rewritten document and true, or data and false for a document the codemod does not change (another
// kind, or no deprecated field set). The rewritten document passes the loader's checks, since Encode runs them.
func Document(l *kindyaml.Loader, data []byte, p Plugins) ([]byte, bool, error) {
	m, err := l.Decode(data)
	if err != nil {
		return nil, false, err
	}
	var out proto.Message
	switch k := m.(type) {
	case *kindsv1.AdapterSelector:
		if len(k.GetSpec().GetPorts().GetKnowledge()) == 0 {
			return data, false, nil
		}
		out, err = SelectorKnowledgeToTools(k, p.SelectorPlugin)
	case *kindsv1.Lockfile:
		if len(k.GetSpec().GetKnowledgeTools()) == 0 {
			return data, false, nil
		}
		out, err = LockfileKnowledgeToPinnedTools(k, p.LockPlugin)
	default:
		return data, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	b, err := l.Encode(out)
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}
