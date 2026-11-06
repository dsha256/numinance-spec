package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	verifierv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/verifier/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
)

// Bounds of the verifier port (verifier.proto, Bounds).
const (
	maxItems   = 1024
	maxMessage = 1024
	maxPath    = 1024
)

// The severities the checks use. A scope breach of a protected path is critical; every other rule is high, so any finding blocks
// under the default block level, and a pack can only raise them.
const (
	sevCritical = typesv1.FindingSeverity_FINDING_SEVERITY_CRITICAL
	sevHigh     = typesv1.FindingSeverity_FINDING_SEVERITY_HIGH
)

// finding is one violation a check found, before normalisation.
type finding struct {
	rule     string
	severity typesv1.FindingSeverity
	path     string // relative to the candidate root, or empty for a finding about the whole candidate
	line     uint32
	message  string
}

// printable is the set of the port: Unicode categories L, M, N, P, S and U+0020.
func printable(r rune) bool {
	return r == ' ' || unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S)
}

// validPath reports whether p is a candidate-relative path of the port's shape: '/'-separated, printable, no leading '/', no
// '\', no empty, '.' or '..' segment, at most 1024 bytes. A name that fails it is refused where it is read, so no finding
// ever carries a path the host would reject.
func validPath(p string) bool {
	if p == "" || len(p) > maxPath || strings.HasPrefix(p, "/") || strings.ContainsRune(p, '\\') || !utf8.ValidString(p) {
		return false
	}
	for seg := range strings.SplitSeq(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
		for _, r := range seg {
			if !printable(r) {
				return false
			}
		}
	}
	return true
}

// normalizeMessage replaces every character the port does not allow with U+FFFD and shortens the text at a character boundary
// to the bound. The text carries names and link targets read from the candidate, which are untrusted: they are data in a message,
// never instructions. An empty result becomes a fixed text, because a message is required.
func normalizeMessage(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t' || r == '\n' || printable(r):
			b.WriteRune(r)
		default:
			b.WriteRune(utf8.RuneError)
		}
	}
	out := b.String()
	if len(out) > maxMessage {
		cut := maxMessage
		for cut > 0 && !utf8.RuneStart(out[cut]) {
			cut--
		}
		out = out[:cut]
	}
	if strings.TrimSpace(out) == "" {
		return "the check reported no message"
	}
	return out
}

// fingerprint is a stable identity of a finding across runs: the check, the rule, the file and the message, never the line (a
// line moves between related candidates). It is 64 printable ASCII characters.
func fingerprint(check string, f finding) string {
	h := sha256.New()
	for _, part := range []string{check, f.rule, f.path, f.message} {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// buildResponse normalises, merges by fingerprint, orders most severe first and counts. The response has unique fingerprints,
// items ordered by severity, and a summary that counts every merged finding; at most 1024 items are listed.
func buildResponse(check string, in []finding) *verifierv1.RunResponse {
	seen := map[string]bool{}
	var items []*verifierv1.Finding
	for _, f := range in {
		f.message = normalizeMessage(f.message)
		fp := fingerprint(check, f)
		if seen[fp] {
			continue
		}
		seen[fp] = true
		item := &verifierv1.Finding{RuleId: f.rule, Severity: f.severity, Message: f.message, Fingerprint: fp}
		if f.path != "" {
			loc := &verifierv1.Location{Path: f.path, StartLine: f.line, EndLine: f.line}
			item.Location = loc
		}
		items = append(items, item)
	}
	slices.SortStableFunc(items, func(a, b *verifierv1.Finding) int {
		if c := int(b.GetSeverity()) - int(a.GetSeverity()); c != 0 {
			return c
		}
		if c := strings.Compare(a.GetLocation().GetPath(), b.GetLocation().GetPath()); c != 0 {
			return c
		}
		if c := int(a.GetLocation().GetStartLine()) - int(b.GetLocation().GetStartLine()); c != 0 {
			return c
		}
		return strings.Compare(a.GetRuleId(), b.GetRuleId())
	})
	counts := map[typesv1.FindingSeverity]uint64{}
	for _, it := range items {
		counts[it.GetSeverity()]++
	}
	var summary verifierv1.FindingsSummary
	for sev := typesv1.FindingSeverity_FINDING_SEVERITY_CRITICAL; sev >= typesv1.FindingSeverity_FINDING_SEVERITY_INFO; sev-- {
		if n := counts[sev]; n > 0 {
			summary.Counts = append(summary.Counts, &verifierv1.SeverityCount{Severity: sev, Count: n})
		}
	}
	if len(items) > maxItems {
		items = items[:maxItems]
	}
	return &verifierv1.RunResponse{Items: items, Summary: &summary}
}
