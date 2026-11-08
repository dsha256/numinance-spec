package ticketingkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"    //nolint:depguard // standard library; the scan reads the kit's own source with go/parser
	"go/parser" //nolint:depguard // standard library
	"go/token"  //nolint:depguard // standard library
	"html"
	"io"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// ReservedHostSuffix is the only DNS name space a fixture may name: RFC 2606 reserves .test, and a fixture host is
// example.test or a name under it. numinance-spec is public and mirrored on every tag, so a fixture never carries a
// customer, employer or vendor tenant (ADR 0060 decision 10); gitleaks does not catch tenant names, this scan does.
const ReservedHostSuffix = "example.test"

// ScanExemptMarker is the comment that exempts a test file from the Go literal scan: a file whose name ends in _test.go and
// that is named in Allow.Exempt and carries this comment before its package clause holds the hostile inputs of the scan's own mutation tests. The marker
// in any other file is itself a problem.
const ScanExemptMarker = "ticketingkit:scan-exempt"

// TenantAllowed reports whether name is on the positive allowlist of tenant, account and site names a fixture may carry.
func TenantAllowed(name string) bool {
	switch name {
	case "acme", "globex":
		return true
	}
	return false
}

// Allow is the closed set of names a scan accepts. A tenant name or any other value outside it is a problem; there is no
// pattern that accepts an arbitrary alphanumeric. A provider that ships its own replay fixtures builds its own Allow with
// the tokens its fixtures need and runs ScanDirWith.
type Allow struct {
	// Tokens are the structural string values: projects, issue keys, state names.
	Tokens []string
	// Keys are the JSON object keys.
	Keys []string
	// Dotted are the dotted names that prose and Go source may hold besides hosts under example.test: file names, Go and
	// proto identifiers, versions. A real host is never listed.
	Dotted []string
	// Exempt are the base names of the test files that may carry ScanExemptMarker. The marker in any other file is a problem.
	Exempt []string
}

// DefaultAllow is the closed set of this kit's own fixture (testdata/tracker.json), README.md and Go source.
func DefaultAllow() Allow {
	a := Allow{
		Exempt: []string{"kit_test.go", "scan_go_test.go"},
		Tokens: []string{"PAY", "OPS", "SEC", "OPEN", "IN_PROGRESS", "DONE", "CANCELED"},
		Keys: []string{"tenant", "host", "base_url", "note", "forbidden_projects", "issues", "project", "key", "title", "body", "body_repeat",
			"text", "times", "state", "type", "labels", "operation_ids", "hidden", "reachable"},
		Dotted: []string{
			"1.2.0", "1.1.0", "0.2.0", "1.0.0",
			"kit.go", "kit_safety.go", "fixtures.go", "fake_test.go", "kit_test.go", "README.md", "tracker.json", "ticketing.proto",
			"test.go", "testdata/tracker.json", "1.x", "Subject.TrackerTexts", "Allow.Keys", "Allow.Tokens", "Allow.Dotted",
			"TicketingService.TransitionIssue", "ticketingkit.Run", "numinance.ports.ticketing.v1.TicketingService", "numinance.ports.ticketing.v1",
			"ticketing.query_language", "ticketing.get_issue", "ticketing.comment_on_issue", "ticketing.transition_issue", "ticketing.link_issue",
			"ticketing.v1", "numinance.annotations.v1", "numinance.plugin.v1", "numinance.ports", "e.g", "i.e", "v1.2", "scan_go_test.go", "ticketingkit.Observer", "Allow.Exempt", "ActionContext.dry_run",
			"ActionContext.idempotency_key", "HostService.EmitEvidence",
		},
	}
	for _, n := range []struct {
		prefix string
		last   int
	}{{"PAY", 14}, {"OPS", 2}, {"SEC", 1}} {
		for i := 1; i <= n.last; i++ {
			a.Tokens = append(a.Tokens, fmt.Sprintf("%s-%d", n.prefix, i))
		}
	}
	return a
}

type scanner struct {
	tokens, keys, dotted, exempt map[string]bool
}

func newScanner(a Allow) scanner {
	set := func(xs []string, lower bool) map[string]bool {
		m := map[string]bool{}
		for _, x := range xs {
			if lower {
				x = strings.ToLower(x)
			}
			m[x] = true
		}
		return m
	}
	return scanner{tokens: set(a.Tokens, false), keys: set(a.Keys, false), dotted: set(a.Dotted, true), exempt: set(a.Exempt, false)}
}

const maxFixtureBytes = 1 << 20

var (
	// A host in the reserved name space, optionally with a port: lower-case labels under example.test.
	hostRe = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)*example\.test(?::[0-9]{1,5})?$`)
	// An https URL whose host is in the reserved name space, with an optional plain path.
	urlOKRe = regexp.MustCompile(`^https://(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)*example\.test(?::[0-9]{1,5})?(?:/[A-Za-z0-9._~/-]*)?$`)
	// A key that names a tenant, an account, a host, an address or a URL, whatever its spelling (tenant_id, cloudId, base_url).
	identityKeyRe = regexp.MustCompile(`(?i)(tenant|account|cloud|instance|org|site|workspace|host|domain|url|email|server|endpoint|address|addr|origin|remote|href|uri|proxy|fqdn|netloc|base|self|(^|[^a-z])ipv?[46]?([^a-z]|$))`)
	// Dotted label sequences (any letter or digit, so internationalised names count), IPv4 and IPv6 forms, numeric IP
	// spellings (hex and decimal) inside text.
	dottedRe  = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}_-]*(?:\.[\p{L}\p{N}][\p{L}\p{N}_-]*)+`)
	ipv6Re    = regexp.MustCompile(`(?i)(?:[0-9a-f]{1,4}:){2,}[0-9a-f]{0,4}|(?:[0-9a-f]{1,4})?::[0-9a-f]{1,4}`)
	numIPRe   = regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}_])(?:0x[0-9a-f]{4,}|[0-9]{8,})(?:$|[^\p{L}\p{N}_])`)
	schemeRe  = regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://[^\s"'<>]*`)
	emailRe   = regexp.MustCompile(`[\p{L}\p{N}._%+-]+@[\p{L}\p{N}_-]+`)
	escapeRe  = regexp.MustCompile(`\\(?:u([0-9a-fA-F]{4})|U([0-9a-fA-F]{8})|x([0-9a-fA-F]{2}))`)
	percentRe = regexp.MustCompile(`%([0-9a-fA-F]{2})`)
	// Defanged separators: a dot or the word dot in brackets, and a dot with a space on both sides. In fixture prose the bare
	// word dot between spaces and a dot after a space are separators too.
	defangDotRe   = regexp.MustCompile(`\s*[\[({<]\s*(?:\.|dot|d0t|period)\s*[\])}>]\s*|\s+\.\s+`)
	defangProseRe = regexp.MustCompile(`\s+dot\s+|\s+\.([\p{L}\p{N}])`)
	defangAtRe    = regexp.MustCompile(`\s*[\[({<]\s*(?:@|at)\s*[\])}>]\s*`)
)

// freeText reports whether key names a field of prose or tracker text, which a fixture may fill with characters the host
// must strip. The host-like checks still apply to it, after the strip.
func freeText(key string) bool {
	switch key {
	case "title", "body", "text", "note", "type", "labels", "operation_ids":
		return true
	}
	return false
}

// ScanFixtures checks the files of the kit against the positive allowlist of ADR 0060 decision 10 with DefaultAllow and
// returns one problem per violation, in file order; no problem means the files are safe for the public mirror. Files are
// given by slash-separated name relative to the kit directory.
func ScanFixtures(files map[string][]byte) []string { return ScanFixturesWith(files, DefaultAllow()) }

// ScanFixturesWith is ScanFixtures with the closed set of names of the caller.
//
// What is checked:
//   - a fixture is a JSON file under testdata/; a file of any other kind there, a Go file under a testdata directory, an
//     unparsable file, a duplicate key or trailing data is a problem.
//   - every JSON key must be in Allow.Keys. A value under a key that matches tenant, account, cloud, instance, org, site,
//     workspace, host, domain, url, email, server, endpoint, address, origin, remote, href, uri, proxy, base, self or ip must
//     be exactly an allowed tenant, an allowed host or an allowed URL. Any other string must be exactly an Allow.Tokens value
//     or an allowed host or URL: an arbitrary alphanumeric, a tenant name or a vendor name is not a structural token. A
//     free-text field (title, body, type, labels, operation ids, notes) may hold the characters a provider must strip;
//     nothing else invisible. It is read in every decoded form (JSON, percent, HTML entity and backslash escapes, repeated
//     until nothing changes) after the invisible characters are removed, the dot and at look-alikes and the fullwidth forms
//     are read as ASCII and defanged separators ([.], (dot), a spaced dot) are closed up: it may hold no at sign, no IP
//     literal in any spelling, no URL and no dotted name outside the reserved host space.
//   - any .md file and every comment and string literal of a Go file (parsed with go/parser, import paths and struct tags
//     excepted) are read the same way, except that a dotted name listed in Allow.Dotted is accepted and an at sign is
//     accepted unless it follows a name (an e-mail address). A file named in Allow.Exempt (a _test.go file) with the ScanExemptMarker
//     comment before its package clause is skipped: it holds the mutation inputs.
//   - an invisible filler (a Hangul or Khmer filler, a braille blank, a variation selector, a tag, a soft hyphen, any other
//     format character the provider is not required to strip) is a problem anywhere in a string.
//   - a string with leading or trailing white space is a problem.
func ScanFixturesWith(files map[string][]byte, a Allow) []string {
	sc := newScanner(a)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var problems []string
	for _, n := range names {
		for _, p := range sc.scanFile(n, files[n]) {
			problems = append(problems, n+": "+p)
		}
	}
	return problems
}

func (sc scanner) scanFile(name string, data []byte) []string {
	segments := strings.Split(name, "/")
	inTestdata := false
	for _, seg := range segments[:len(segments)-1] {
		inTestdata = inTestdata || seg == "testdata"
	}
	switch ext := path.Ext(name); {
	case ext == ".go" && inTestdata:
		return []string{"a Go file under testdata: fixtures must not live in code"}
	case ext == ".go":
		return sc.scanGo(name, data)
	case ext == ".md":
		var ps []string
		sc.checkProse(string(data), markdown, func(f string, args ...any) { ps = append(ps, fmt.Sprintf(f, args...)) })
		return ps
	case ext != ".json" || !inTestdata:
		return []string{"not a known fixture: only testdata/**/*.json, .md files and Go source are allowed in the kit directory"}
	}
	return sc.scanJSON(data)
}

// scanGo reads the comments and string literals of a Go file of the kit: a host in a literal reaches the public mirror like
// one in a fixture.
func (sc scanner) scanGo(name string, data []byte) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, data, parser.ParseComments)
	if err != nil {
		return []string{"does not parse as Go, so it cannot be scanned: " + err.Error()}
	}
	marked := false
	for _, cg := range f.Comments {
		if cg.Pos() < f.Package && strings.Contains(cg.Text(), ScanExemptMarker) {
			marked = true
		}
	}
	if marked {
		if !strings.HasSuffix(name, "_test.go") || !sc.exempt[path.Base(name)] || strings.Contains(name, "/") {
			return []string{ScanExemptMarker + " is honoured only in the named test files of Allow.Exempt, in the kit directory"}
		}
		return nil
	}
	skip := map[*ast.BasicLit]bool{}
	for _, imp := range f.Imports {
		skip[imp.Path] = true
	}
	var problems []string
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.Field:
			if v.Tag != nil {
				skip[v.Tag] = true
			}
		case *ast.BasicLit:
			if v.Kind != token.STRING || skip[v] {
				return true
			}
			s, err := strconv.Unquote(v.Value)
			if err != nil {
				problems = append(problems, fmt.Sprintf("line %d: a string literal that does not unquote", fset.Position(v.Pos()).Line))
				return true
			}
			sc.checkProse(s, goString, func(format string, args ...any) {
				problems = append(problems, fmt.Sprintf("line %d: %s", fset.Position(v.Pos()).Line, fmt.Sprintf(format, args...)))
			})
		}
		return true
	})
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			sc.checkProse(c.Text, goComment, func(format string, args ...any) {
				problems = append(problems, fmt.Sprintf("line %d: %s", fset.Position(c.Pos()).Line, fmt.Sprintf(format, args...)))
			})
		}
	}
	return problems
}

func (sc scanner) scanJSON(data []byte) []string {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var problems []string
	if err := sc.walk(dec, "", &problems); err != nil {
		return append(problems, "not a valid JSON document: "+err.Error())
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		problems = append(problems, "data after the JSON document")
	}
	return problems
}

// walk reads one JSON value; key is the key it sits under (an array element inherits its array's key).
func (sc scanner) walk(dec *json.Decoder, key string, problems *[]string) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	switch v := tok.(type) {
	case json.Delim:
		switch v {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return err
				}
				k, _ := kt.(string)
				if seen[k] {
					*problems = append(*problems, fmt.Sprintf("duplicate key %q", k))
				}
				seen[k] = true
				sc.checkString(k, "", true, problems)
				if err := sc.walk(dec, k, problems); err != nil {
					return err
				}
			}
		case '[':
			for dec.More() {
				if err := sc.walk(dec, key, problems); err != nil {
					return err
				}
			}
		}
		_, err := dec.Token()
		return err
	case string:
		sc.checkString(v, key, false, problems)
	default:
		if key != "" && identityKeyRe.MatchString(key) {
			*problems = append(*problems, fmt.Sprintf("key %q must hold an allowed name, not %v", key, v))
		}
	}
	return nil
}

func hostAllowed(h string) bool { return hostRe.MatchString(h) }

func urlAllowed(u string) bool { return urlOKRe.MatchString(u) }

// checkString applies the allowlist to one decoded string: a key itself (isKey) or the value under key.
func (sc scanner) checkString(s, key string, isKey bool, problems *[]string) {
	bad := func(format string, args ...any) { *problems = append(*problems, fmt.Sprintf(format, args...)) }
	if s != strings.TrimSpace(s) {
		bad("%q has leading or trailing white space", s)
	}
	switch {
	case isKey:
		if !sc.keys[s] {
			bad("key %q is not one of the closed set of key names", s)
		}
	case key != "" && identityKeyRe.MatchString(key):
		if !TenantAllowed(s) && !hostAllowed(s) && !urlAllowed(s) {
			bad("key %q holds %q, which is not an allowed tenant, host or URL", key, s)
		}
	case freeText(key):
		sc.checkProse(s, jsonText, func(f string, args ...any) { bad("(under %q) "+f, append([]any{key}, args...)...) })
	case sc.tokens[s] || hostAllowed(s) || urlAllowed(s):
	default:
		bad("%q (under %q) is not one of the closed set of structural tokens, nor an allowed host or URL", s, key)
	}
}

// filler reports whether r is an invisible character that hides text and that a provider is not required to strip.
func filler(r rune) bool {
	switch {
	case r == 0x034F, r == 0x115F, r == 0x1160, r == 0x17B4, r == 0x17B5, r == 0x2800, r == 0x3164, r == 0xFFA0, r == 0x00AD,
		r >= 0x180B && r <= 0x180F, r >= 0xFE00 && r <= 0xFE0F, r >= 0xE0000 && r <= 0xE01EF, r >= 0xFFF9 && r <= 0xFFFB,
		r >= 0x2061 && r <= 0x2064, r >= 0x206A && r <= 0x206F, r >= 0x1D173 && r <= 0x1D17A:
		return true
	case unicode.Is(unicode.Cf, r):
		_, strip := forbidden(string(r), true)
		return !strip
	}
	return false
}

// textMode says what a string is, which decides how strictly it is read.
type textMode int

const (
	jsonText  textMode = iota // free text of a fixture: no at sign, no dotted name outside the reserved space
	markdown                  // a .md file: dotted names of Allow.Dotted are accepted
	goString                  // a string literal of kit source: dotted names of Allow.Dotted are accepted
	goComment                 // a comment of kit source: read like a literal, dotted names of Allow.Dotted are accepted
)

// checkProse reads a string the way an attacker hides a host in it.
func (sc scanner) checkProse(s string, mode textMode, bad func(string, ...any)) {
	for _, r := range s {
		if filler(r) {
			bad("%q holds the invisible filler %U", clip(s), r)
			return
		}
	}
	for _, form := range decodeForms(s) {
		n := normalise(form, mode == jsonText)
		if (mode != jsonText && emailRe.MatchString(n)) || (mode == jsonText && strings.Contains(n, "@")) {
			bad("%q holds an at sign: an e-mail address or userinfo", clip(s))
			return
		}
		for _, u := range schemeRe.FindAllString(n, -1) {
			if mode == goString && strings.ContainsAny(u, "([^$") {
				continue // a regular expression that mentions a scheme; its host part is still read below
			}
			if !urlAllowed(strings.ReplaceAll(u, `\`, "/")) {
				bad("%q holds the URL %q, which is not under %s", clip(s), u, ReservedHostSuffix)
				return
			}
		}
		for _, d := range dottedRe.FindAllString(n, -1) {
			if hostAllowed(d) || (mode != jsonText && sc.dotted[d]) {
				continue
			}
			bad("%q holds %q, a dotted name or number that is not under %s", clip(s), d, ReservedHostSuffix)
			return
		}
		if ipv6Re.MatchString(n) {
			bad("%q holds an IPv6 literal", clip(s))
			return
		}
		if numIPRe.MatchString(n) {
			bad("%q holds a long number or a hex number, which can spell an IP address", clip(s))
			return
		}
	}
}

// clip shortens a string for a problem message.
func clip(s string) string {
	if r := []rune(s); len(r) > 80 {
		return string(r[:80]) + "..."
	}
	return s
}

// decodeForms returns s and every form that results from decoding backslash escapes, percent escapes and HTML entities,
// repeatedly, until nothing changes (at most eight rounds): %252e, &amp;#46; and a backslash escape of the dot all end as a dot.
func decodeForms(s string) []string {
	forms := []string{s}
	cur := s
	for range 8 {
		next := html.UnescapeString(lenientPercent(unescapeRunes(cur)))
		if next == cur {
			break
		}
		forms = append(forms, next)
		cur = next
	}
	return forms
}

func unescapeRunes(s string) string {
	return escapeRe.ReplaceAllStringFunc(s, func(m string) string {
		digits := m[2:]
		n, err := strconv.ParseUint(digits, 16, 32)
		if err != nil || n > unicode.MaxRune {
			return m
		}
		return string(rune(n))
	})
}

// lenientPercent decodes every well-formed %XX and leaves the rest, so one bad escape does not hide the others.
func lenientPercent(s string) string {
	return percentRe.ReplaceAllStringFunc(s, func(m string) string {
		n, _ := strconv.ParseUint(m[1:], 16, 8)
		return string([]byte{byte(n)})
	})
}

// normalise removes the invisible characters, reads the look-alikes of dot, at sign, colon and slash and the fullwidth
// forms as ASCII, lower-cases, and closes up defanged separators.
func normalise(s string, fixtureProse bool) string {
	s = strings.Map(func(r rune) rune {
		if r >= 0xFF01 && r <= 0xFF5E {
			r -= 0xFEE0 // fullwidth ASCII: the dot U+FF0E, the at sign U+FF20, the colon U+FF1A and the letters
		}
		switch {
		case r == '\u3002' || r == '\uff61' || r == '\u2024' || r == '\u2e2e' || r == '\ufe52' || r == '\u0701' || r == '\u0702' ||
			r == '\u0589' || r == '\u06d4' || r == '\u1803' || r == '\u1809' || r == '\u2e33' || r == '\ua60e' || r == '\ua6f3' || r == '\u22c5':
			return '.'
		case r == '\ufe55':
			return ':'
		case r == '\ufe6b':
			return '@'
		case r == '\uff3c' || r == '\\':
			return '/'
		case unicode.IsSpace(r):
			return ' '
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || filler(r):
			return -1
		}
		return r
	}, s)
	s = strings.ToLower(s)
	s = defangDotRe.ReplaceAllString(s, ".")
	if fixtureProse {
		s = defangProseRe.ReplaceAllString(s, ".$1")
	}
	return defangAtRe.ReplaceAllString(s, "@")
}

// ScanDir applies ScanFixtures to every file of fsys, the kit directory. A file over 1 MiB is a problem of its own,
// because a fixture is small and hand-written.
func ScanDir(fsys fs.FS) ([]string, error) { return ScanDirWith(fsys, DefaultAllow()) }

// ScanDirWith is ScanDir with the closed set of names of the caller.
func ScanDirWith(fsys fs.FS, a Allow) ([]string, error) {
	files := map[string][]byte{}
	var problems []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxFixtureBytes {
			problems = append(problems, fmt.Sprintf("%s: %d bytes is over the 1 MiB fixture limit", p, info.Size()))
			return nil
		}
		b, err := fs.ReadFile(fsys, p)
		files[p] = b
		return err
	})
	return append(problems, ScanFixturesWith(files, a)...), err
}
