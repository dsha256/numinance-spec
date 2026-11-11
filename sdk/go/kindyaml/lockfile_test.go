package kindyaml_test

import (
	"errors"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	kindsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/kinds/v1"
	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
)

const lockfileExample = "Lockfile/numinance-lock.yaml"

func TestLockfile_Decode_Example(t *testing.T) {
	t.Parallel()
	l, err := kindyaml.NewLoader()
	if err != nil {
		t.Fatalf("NewLoader() = %v", err)
	}
	m, err := l.Decode([]byte(example(t, lockfileExample)))
	if err != nil {
		t.Fatalf("Decode(%s) = %v", lockfileExample, err)
	}
	got, ok := m.(*kindsv1.Lockfile)
	if !ok {
		t.Fatalf("Decode(%s) = %T, want *kindsv1.Lockfile", lockfileExample, m)
	}
	e := got.GetSpec().GetExtensions()
	if len(e) != 1 || e[0].GetId() != "dev.numinance.official.scm-github" || e[0].GetVersion() != "1.2.0" {
		t.Errorf("Decode(%s) extensions = %v, want one dev.numinance.official.scm-github 1.2.0", lockfileExample, e)
	}
	out, err := l.Encode(m)
	if err != nil {
		t.Fatalf("Encode(%s) = %v", lockfileExample, err)
	}
	back, err := l.Decode(out)
	if err != nil || !proto.Equal(back, m) {
		t.Errorf("Decode(Encode(m)) = %v, %v; want %v", back, err, m)
	}
}

func TestLockfile_Decode_Rejects(t *testing.T) {
	t.Parallel()
	doc := example(t, lockfileExample)
	const signerLine = "      signer: \"https://token.actions.githubusercontent.com#https://github.com/numinance/plugins/.github/workflows/release.yml@refs/tags/v1.2.0\"\n"
	tests := []struct {
		name string
		old  string
		new  string
		want error
	}{
		{name: "unknown field", old: "      version: 1.2.0\n", new: "      version: 1.2.0\n      note: free\n", want: kindyaml.ErrSchema},
		{name: "extension id with one label", old: "id: dev.numinance.official.scm-github", new: "id: scmgithub", want: kindyaml.ErrInvalid},
		{name: "extension id with an upper-case letter", old: "id: dev.numinance.official.scm-github", new: "id: dev.numinance.Official", want: kindyaml.ErrInvalid},
		{name: "extension id that is a path", old: "id: dev.numinance.official.scm-github", new: "id: ../dev.numinance", want: kindyaml.ErrInvalid},
		{name: "version with a pre-release part", old: "version: 1.2.0", new: "version: 1.2.0-rc.1", want: kindyaml.ErrInvalid},
		{name: "h1 of the wrong prefix", old: "h1:qlU4", new: "h2:qlU4", want: kindyaml.ErrInvalid},
		{name: "h1 that is short", old: "h1:qlU4YlPoeX3FQnGgA4XKl0ZUVXGBn0C5lEcRB6Zo9yk=", new: "h1:qlU4YlPoeX3FQnGgA4XKl0ZUVXGBn0C5lEcRB6Zo9y=", want: kindyaml.ErrInvalid},
		{name: "zh in upper case", old: "zh:9b2d4f6a8c0e1b3d5f7a9c2e4b6d8f0a1c3e5b7d9f2a4c6e8b0d2f4a6c8e0b1d", new: "zh:9B2D4F6A8C0E1B3D5F7A9C2E4B6D8F0A1C3E5B7D9F2A4C6E8B0D2F4A6C8E0B1D", want: kindyaml.ErrInvalid},
		{name: "zh that is a digest", old: "zh:9b2d4f6a8c0e1b3d5f7a9c2e4b6d8f0a1c3e5b7d9f2a4c6e8b0d2f4a6c8e0b1d", new: "sha256:9b2d4f6a8c0e1b3d5f7a9c2e4b6d8f0a1c3e5b7d9f2a4c6e8b0d2f4a6c8e0b1d", want: kindyaml.ErrInvalid},
		{name: "digest that is short", old: "sha256:3f1a9c0e5b7d2a4f6e8c1b3d5a7f9e2c4b6d8a0f1e3c5b7d9a2f4e6c8b0d1a3f", new: "sha256:3f1a", want: kindyaml.ErrInvalid},
		{name: "permissions digest omitted", old: "      permissions_digest: {value: \"sha256:7c4e2a1b9d8f6e5c3a2b1d0f9e8c7b6a5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0a\"}\n", new: "", want: kindyaml.ErrInvalid},
		{name: "signer omitted", old: signerLine, new: "", want: kindyaml.ErrSchema},
		{name: "signer with a space", old: "signer: \"https://token", new: "signer: \"x y https://token", want: kindyaml.ErrInvalid},
		{name: "duplicate extension id", old: "  pinned_tools:\n", new: "    - {id: dev.numinance.official.scm-github, version: 1.3.0, digest: {value: \"sha256:3f1a9c0e5b7d2a4f6e8c1b3d5a7f9e2c4b6d8a0f1e3c5b7d9a2f4e6c8b0d1a3f\"}, h1: \"h1:qlU4YlPoeX3FQnGgA4XKl0ZUVXGBn0C5lEcRB6Zo9yk=\", zh: \"zh:9b2d4f6a8c0e1b3d5f7a9c2e4b6d8f0a1c3e5b7d9f2a4c6e8b0d2f4a6c8e0b1d\", signer: \"https://i.example#s\", permissions_digest: {value: \"sha256:7c4e2a1b9d8f6e5c3a2b1d0f9e8c7b6a5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0a\"}}\n  pinned_tools:\n", want: kindyaml.ErrInvalid},
		{name: "tool name with a colon", old: "name: getConfluencePage", new: "name: get:Page", want: kindyaml.ErrInvalid},
		{name: "pinned tool without a plugin", old: "    - plugin: dev.numinance.official.atlassian\n      connection:", new: "    - connection:", want: kindyaml.ErrSchema},
		{name: "pinned tool plugin with one label", old: "plugin: dev.numinance.official.atlassian", new: "plugin: atlassian", want: kindyaml.ErrInvalid},
		{name: "pinned tool plugin that is a path", old: "plugin: dev.numinance.official.atlassian", new: "plugin: dev.numinance.official/atlassian", want: kindyaml.ErrInvalid},
		{name: "pinned tool without a connection", old: "      connection: confluence-eng\n", new: "", want: kindyaml.ErrSchema},
		{name: "pinned tool with an empty connection", old: "connection: confluence-eng", new: `connection: ""`, want: kindyaml.ErrInvalid},
		{name: "pinned tool without a hash", old: "      hash: {value: \"sha256:9b2d4f6a8c0e1b3d5f7a9c2e4b6d8f0a1c3e5b7d9f2a4c6e8b0d2f4a6c8e0b1d\"}\n", new: "", want: kindyaml.ErrInvalid},
		{name: "pinned tool with a short hash", old: "sha256:9b2d4f6a8c0e1b3d5f7a9c2e4b6d8f0a1c3e5b7d9f2a4c6e8b0d2f4a6c8e0b1d", new: "sha256:9b2d", want: kindyaml.ErrInvalid},
		{name: "pinned tool with an unknown field", old: "      name: getConfluencePage\n", new: "      name: getConfluencePage\n      note: x\n", want: kindyaml.ErrSchema},
		{name: "pinned tool twice", old: "  pinned_tools:\n", new: "  pinned_tools:\n    - {plugin: dev.numinance.official.atlassian, connection: confluence-eng, name: getConfluencePage, hash: {value: \"sha256:9b2d4f6a8c0e1b3d5f7a9c2e4b6d8f0a1c3e5b7d9f2a4c6e8b0d2f4a6c8e0b1d\"}}\n", want: kindyaml.ErrInvalid},
		{name: "scope unspecified", old: "scope: SCOPE_REPO", new: "scope: SCOPE_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "extension key that is not reverse-DNS", old: "spec:\n", new: "extensions: {vendor: 1}\nspec:\n", want: kindyaml.ErrInvalid},
	}
	l, err := kindyaml.NewLoader()
	if err != nil {
		t.Fatalf("NewLoader() = %v", err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(doc, tt.old) {
				t.Fatalf("the example has no %q", tt.old)
			}
			m, err := l.Decode([]byte(strings.Replace(doc, tt.old, tt.new, 1)))
			if !errors.Is(err, tt.want) {
				t.Errorf("Decode(%s) = %v, %v; want error %v", tt.name, m, err, tt.want)
			}
		})
	}
}

func TestLockfile_Decode_EmptyListsAreValid(t *testing.T) {
	t.Parallel()
	l, err := kindyaml.NewLoader()
	if err != nil {
		t.Fatalf("NewLoader() = %v", err)
	}
	doc := "apiVersion: numinance.dev/v1\nkind: Lockfile\nmetadata: {name: numinance-lock, scope: SCOPE_REPO}\nspec: {}\n"
	if _, err := l.Decode([]byte(doc)); err != nil {
		t.Errorf("Decode(empty lockfile) = %v, want nil", err)
	}
}

func TestLockfile_Decode_PinnedToolKeyIncludesThePlugin(t *testing.T) {
	t.Parallel()
	l, err := kindyaml.NewLoader()
	if err != nil {
		t.Fatalf("NewLoader() = %v", err)
	}
	doc := example(t, lockfileExample)
	other := "    - {plugin: dev.numinance.official.other, connection: confluence-eng, name: getConfluencePage, hash: {value: \"sha256:9b2d4f6a8c0e1b3d5f7a9c2e4b6d8f0a1c3e5b7d9f2a4c6e8b0d2f4a6c8e0b1d\"}}\n"
	m, err := l.Decode([]byte(doc + other))
	if err != nil {
		t.Fatalf("Decode(same connection and name under another plugin) = %v, want nil", err)
	}
	if n := len(m.(*kindsv1.Lockfile).GetSpec().GetPinnedTools()); n != 2 {
		t.Errorf("pinned_tools = %d entries, want 2", n)
	}
}

func TestLockfile_Decode_LegacyKnowledgeToolsStillLoad(t *testing.T) {
	t.Parallel()
	l, err := kindyaml.NewLoader()
	if err != nil {
		t.Fatalf("NewLoader() = %v", err)
	}
	m, err := l.Decode([]byte(example(t, "Lockfile/legacy-knowledge-tools.yaml")))
	if err != nil {
		t.Fatalf("Decode(legacy lockfile) = %v, want nil: a deprecated field loads for the deprecation window", err)
	}
	if n := len(m.(*kindsv1.Lockfile).GetSpec().GetKnowledgeTools()); n != 1 {
		t.Errorf("knowledge_tools = %d entries, want 1", n)
	}
}
