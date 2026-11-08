package ticketingkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// ReservedHostSuffix is the only DNS name space a fixture may name: RFC 2606 reserves .test, and a fixture host is
// example.test or a name under it. numinance-spec is public and mirrored on every tag, so a fixture never carries a
// customer, employer or vendor tenant (ADR 0060 decision 10); gitleaks does not catch tenant names, this scan does.
const ReservedHostSuffix = "example.test"

// TenantAllowed reports whether name is on the positive allowlist of tenant, account and site names a fixture may carry.
func TenantAllowed(name string) bool {
	switch name {
	case "acme", "globex":
		return true
	}
	return false
}

const maxFixtureBytes = 1 << 20

var (
	// A host in the reserved name space, optionally with a port: lower-case labels under example.test.
	hostRe = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)*example\.test(?::[0-9]{1,5})?$`)
	// An https URL whose host is in the reserved name space, with an optional plain path.
	urlOKRe = regexp.MustCompile(`^https://(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)*example\.test(?::[0-9]{1,5})?(?:/[A-Za-z0-9._~/-]*)?$`)
	// A structural token: a project, a key, a state, an id. No dot, colon, at sign or slash, so it cannot name a host.
	tokenRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	// A key that names a tenant, an account, a host or a URL, whatever its spelling (tenant_id, cloudId, base_url).
	identityKeyRe = regexp.MustCompile(`(?i)(tenant|account|cloud|instance|org|site|workspace|host|domain|url|email)`)
	// Dotted label sequences, IPv4 and IPv6 forms inside free text.
	dottedRe = regexp.MustCompile(`[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+`)
	ipv6Re   = regexp.MustCompile(`(?i)(?:[0-9a-f]{0,4}:){2,}[0-9a-f]{0,4}`)
	schemeRe = regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://[^\s"'<>]*`)
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

// ScanFixtures checks the files of the kit against the positive allowlist of ADR 0060 decision 10 and returns one problem
// per violation, in file order; no problem means the files are safe for the public mirror. Files are given by
// slash-separated name relative to the kit directory.
//
// What is checked:
//   - a fixture is a JSON file under testdata/; a file of any other kind there, a Go file under a testdata directory, an
//     unparsable file, a duplicate key or trailing data is a problem. README.md at the root is prose and is checked for
//     URLs only. Go files outside testdata are the kit's own code and out of scope.
//   - every key and every string value is decoded (JSON escapes, percent escapes) and then checked. A key matching
//     tenant, account, cloud, instance, org, site, workspace, host, domain, url or email must hold a string that is exactly
//     an allowed tenant name, an allowed host or an allowed URL. Any other string must be a structural token or exactly an
//     allowed host or URL; a free-text field (title, body, type, labels, operation ids, notes) is checked after the control,
//     format and bidirectional characters are removed and the ideographic and fullwidth dots are read as dots: it may hold
//     no at sign, no IP literal, no URL and no dotted name outside the reserved host space.
//   - a string with leading or trailing white space is a problem.
func ScanFixtures(files map[string][]byte) []string {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var problems []string
	for _, n := range names {
		for _, p := range scanFile(n, files[n]) {
			problems = append(problems, n+": "+p)
		}
	}
	return problems
}

func scanFile(name string, data []byte) []string {
	segments := strings.Split(name, "/")
	inTestdata := false
	for _, seg := range segments[:len(segments)-1] {
		inTestdata = inTestdata || seg == "testdata"
	}
	switch ext := path.Ext(name); {
	case ext == ".go" && inTestdata:
		return []string{"a Go file under testdata: fixtures must not live in code"}
	case ext == ".go":
		return nil // the kit's own code, out of scope
	case name == "README.md":
		var ps []string
		for _, u := range schemeRe.FindAllString(string(data), -1) {
			if !urlAllowed(u) {
				ps = append(ps, fmt.Sprintf("URL %q is not under %s", u, ReservedHostSuffix))
			}
		}
		return ps
	case ext != ".json" || !inTestdata:
		return []string{"not a known fixture: only testdata/**/*.json, README.md and Go source are allowed in the kit directory"}
	}
	return scanJSON(data)
}

func scanJSON(data []byte) []string {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var problems []string
	if err := walk(dec, "", &problems); err != nil {
		return append(problems, "not a valid JSON document: "+err.Error())
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		problems = append(problems, "data after the JSON document")
	}
	return problems
}

// walk reads one JSON value; key is the key it sits under (an array element inherits its array's key).
func walk(dec *json.Decoder, key string, problems *[]string) error {
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
				checkString(k, "", true, problems)
				if err := walk(dec, k, problems); err != nil {
					return err
				}
			}
		case '[':
			for dec.More() {
				if err := walk(dec, key, problems); err != nil {
					return err
				}
			}
		}
		_, err := dec.Token()
		return err
	case string:
		checkString(v, key, false, problems)
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
func checkString(s, key string, isKey bool, problems *[]string) {
	bad := func(format string, args ...any) { *problems = append(*problems, fmt.Sprintf(format, args...)) }
	if s != strings.TrimSpace(s) {
		bad("%q has leading or trailing white space", s)
	}
	if !isKey && key != "" && identityKeyRe.MatchString(key) {
		if !TenantAllowed(s) && !hostAllowed(s) && !urlAllowed(s) {
			bad("key %q holds %q, which is not an allowed tenant, host or URL", key, s)
		}
		return
	}
	if freeText(key) && !isKey {
		checkFreeText(s, key, bad)
		return
	}
	if tokenRe.MatchString(s) || TenantAllowed(s) || hostAllowed(s) || urlAllowed(s) {
		return
	}
	bad("%q (under %q) is neither a structural token nor an allowed host, URL or tenant", s, key)
}

// checkFreeText normalises a free-text string the way an attacker would hide a host in it, then looks for hosts.
func checkFreeText(s, key string, bad func(string, ...any)) {
	for _, form := range []string{normalise(s), normalise(percentDecode(s))} {
		if strings.Contains(form, "@") {
			bad("%q (under %q) holds an at sign: an e-mail address or userinfo", s, key)
			return
		}
		for _, u := range schemeRe.FindAllString(form, -1) {
			if !urlAllowed(strings.ReplaceAll(u, `\`, "/")) {
				bad("%q (under %q) holds the URL %q, which is not under %s", s, key, u, ReservedHostSuffix)
				return
			}
		}
		for _, d := range dottedRe.FindAllString(form, -1) {
			if !hostAllowed(strings.ToLower(d)) {
				bad("%q (under %q) holds %q, a dotted name or number that is not under %s", s, key, d, ReservedHostSuffix)
				return
			}
		}
		if ipv6Re.MatchString(form) {
			bad("%q (under %q) holds an IPv6 literal", s, key)
			return
		}
	}
}

func percentDecode(s string) string {
	if d, err := url.PathUnescape(s); err == nil {
		return d
	}
	return s
}

// normalise removes control, format and bidirectional characters and reads the dot look-alikes as dots.
func normalise(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\u3002' || r == '\uff0e' || r == '\uff61' || r == '\u2024' || r == '\u2e2e':
			return '.'
		case r == '\uff1a':
			return ':'
		case r == '\uff20':
			return '@'
		case r == '\uff3c' || r == '\\':
			return '/'
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r):
			return -1
		}
		return r
	}, s)
}

// ScanDir applies ScanFixtures to every file of fsys, the kit directory. A file over 1 MiB is a problem of its own,
// because a fixture is small and hand-written.
func ScanDir(fsys fs.FS) ([]string, error) {
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
	return append(problems, ScanFixtures(files)...), err
}
