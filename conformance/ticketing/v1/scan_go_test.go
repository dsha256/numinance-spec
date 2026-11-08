// ticketingkit:scan-exempt: this file holds the hostile inputs of the fixture scan's own mutation tests.

package ticketingkit_test

import (
	"testing"

	ticketingkit "github.com/numinance/numinance-spec/conformance/ticketing/v1"
)

// TestFixtures_ScanReadsGoSource: the string literals and comments of the kit's own Go files are scanned (go/parser), so a
// host in code cannot reach the public mirror. Only a _test.go file with the marker comment is exempt.
func TestFixtures_ScanReadsGoSource(t *testing.T) {
	t.Parallel()
	goFile := func(body string) map[string][]byte {
		return map[string][]byte{"helper.go": []byte("package x\n\n" + body + "\n")}
	}
	for _, tc := range []struct {
		name  string
		files map[string][]byte
	}{
		{"a host in a string literal", goFile(`var h = "jira.realco.de"`)},
		{"a host in a raw string literal", goFile("var h = `jira.realco.de`")},
		{"a URL in a string literal", goFile(`var u = "https://realco.atlassian.net/rest"`)},
		{"an e-mail address in a string literal", goFile(`var e = "bob@realco.de"`)},
		{"an IPv4 address in a string literal", goFile(`var ip = "10.0.0.5"`)},
		{"an IPv6 address in a string literal", goFile(`var ip = "fe80::1"`)},
		{"a hex IP in a string literal", goFile(`var ip = "0x7f000001"`)},
		{"a decimal IP in a string literal", goFile(`var ip = "2130706433"`)},
		{"a fullwidth dot in a string literal", goFile(`var h = "acme\uff0ecorp"`)},
		{"an escaped dot in a string literal", goFile(`var h = "acme\u002ecorp"`)},
		{"a hex-escaped dot in a string literal", goFile(`var h = "acme\x2ecorp"`)},
		{"a percent-encoded dot in a string literal", goFile(`var h = "acme%2ecorp"`)},
		{"a double percent-encoded dot in a string literal", goFile(`var h = "acme%252ecorp"`)},
		{"a defanged host in a string literal", goFile(`var h = "acme[.]corp"`)},
		{"a filler in a string literal", goFile(`var h = "acme.\u3164corp"`)},
		{"a host in a const", goFile(`const h = "jira.realco.de"`)},
		{"a host in a string of a composite literal", goFile(`var m = map[string]string{"k": "jira.realco.de"}`)},
		{"an unlisted dotted name in a string literal", goFile(`var f = "secrets.yaml"`)},
		{"a host in a comment", goFile("// see jira.realco.de\nvar x = 1")},
		{"a host with an unlisted ending in a comment", goFile("// see jira.realco.company\nvar x = 1")},
		{"a host with an unlisted ending in another comment", goFile("// see jira.realco.systems\nvar x = 1")},
		{"a defanged host in a comment", goFile("// see realco[.]de\nvar x = 1")},
		{"an e-mail address in a comment", goFile("// write to bob@realco.de\nvar x = 1")},
		{"a URL in a comment", goFile("/* see https://realco.atlassian.net */\nvar x = 1")},
		{"an IP in a comment", goFile("// the server is 10.0.0.5\nvar x = 1")},
		{"Go source that does not parse", goFile("var = ")},
		{"the marker in a fresh test file", map[string][]byte{"fresh_test.go": []byte("// ticketingkit:scan-exempt: no\n\npackage x\n\nvar h = \"jira.realco.de\"\n")}},
		{"the marker in a file that is not a test", map[string][]byte{"helper.go": []byte("// ticketingkit:scan-exempt: no\n\npackage x\n\nvar h = \"jira.realco.de\"\n")}},
		{"a test file without the marker", map[string][]byte{"helper_test.go": []byte("package x\n\nvar h = \"jira.realco.de\"\n")}},
		{"the marker after the package clause", map[string][]byte{"helper_test.go": []byte("package x\n\n// ticketingkit:scan-exempt: late\nvar h = \"jira.realco.de\"\n")}},
		{"a Go file under testdata even with the marker", map[string][]byte{"testdata/helper_test.go": []byte("// ticketingkit:scan-exempt: no\n\npackage x\n")}},
	} {
		if got := ticketingkit.ScanFixtures(tc.files); len(got) == 0 {
			t.Errorf("ScanFixtures(%s) = none, want a problem", tc.name)
		}
	}
	exempt := map[string][]byte{"kit_test.go": []byte("// ticketingkit:scan-exempt: mutation inputs\n\npackage x\n\nvar h = \"jira.realco.de\"\n")}
	if got := ticketingkit.ScanFixtures(exempt); len(got) != 0 {
		t.Errorf("ScanFixtures(a test file with the marker) = %v, want none", got)
	}
}

// TestFixtures_ScanWithTheCallersAllowlist: a provider's fixtures are checked against its own closed set, and a name outside
// it is a problem even when it would pass a pattern.
func TestFixtures_ScanWithTheCallersAllowlist(t *testing.T) {
	t.Parallel()
	a := ticketingkit.Allow{Tokens: []string{"ENG", "ENG-1", "OPEN"}, Keys: []string{"project", "key", "state"}}
	ok := map[string][]byte{"testdata/p.json": []byte(`{"project": "ENG", "key": "ENG-1", "state": "OPEN"}`)}
	if got := ticketingkit.ScanFixturesWith(ok, a); len(got) != 0 {
		t.Errorf("ScanFixturesWith(its own names) = %v, want none", got)
	}
	for name, doc := range map[string]string{
		"a project of the default set":  `{"project": "PAY", "key": "ENG-1", "state": "OPEN"}`,
		"a key that is not listed":      `{"project": "ENG", "key": "ENG-2", "state": "OPEN"}`,
		"a JSON key that is not listed": `{"project": "ENG", "owner": "ENG", "state": "OPEN"}`,
	} {
		if got := ticketingkit.ScanFixturesWith(map[string][]byte{"testdata/p.json": []byte(doc)}, a); len(got) == 0 {
			t.Errorf("ScanFixturesWith(%s) = none, want a problem", name)
		}
	}
}
