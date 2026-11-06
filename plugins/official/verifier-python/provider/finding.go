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
	maxItems       = 1024
	maxMessage     = 1024
	maxFingerprint = 256
	maxRuleID      = 256
)

// finding is one violation a parser read from a tool's output, before normalisation.
type finding struct {
	rule     string
	severity typesv1.FindingSeverity
	path     string // relative to the candidate root, or empty for a finding about the whole candidate
	start    uint32
	end      uint32
	message  string
}

// printable is the set of the port: Unicode categories L, M, N, P, S and U+0020, plus U+0009 and U+000A in a message.
func printable(r rune) bool {
	return r == ' ' || unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S)
}

// normalizeMessage replaces every character the port does not allow with U+FFFD and shortens the text at a character
// boundary to the bound. The full tool output belongs in evidence, not here. An empty result becomes a fixed text, because a
// message is required.
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

// cleanPath turns the path a tool printed into a candidate-relative path of the port's shape: '/'-separated, printable, no
// leading '/', no '\', no empty, '.' or '..' segment. The extraction directory is stripped. ok is false for a path outside
// the candidate or of another shape; the finding then has no location rather than a wrong one.
func cleanPath(dir, p string) (string, bool) {
	p = strings.TrimPrefix(p, dir)
	p = strings.TrimPrefix(p, "/candidate")
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimPrefix(p, "./")
	if p == "" || strings.ContainsRune(p, '\\') || strings.HasPrefix(p, "/") || len(p) > maxMessage {
		return "", false
	}
	for seg := range strings.SplitSeq(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", false
		}
		for _, r := range seg {
			if !printable(r) {
				return "", false
			}
		}
	}
	return p, true
}

// fingerprint is a stable identity of a finding across runs of one check on related candidates: the check, the rule, the file
// and the message, never the line (a line moves between related candidates). It is 64 printable ASCII characters.
func fingerprint(check string, f finding) string {
	h := sha256.New()
	for _, part := range []string{check, f.rule, f.path, f.message} {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// buildResponse normalises, merges by fingerprint, orders most severe first and counts. It returns the response the port
// describes: unique fingerprints, items ordered by severity, the summary counting every merged finding and the items the
// 1024 most severe when there are more.
func buildResponse(check string, in []finding) *verifierv1.RunResponse {
	seen := map[string]bool{}
	var items []*verifierv1.Finding
	for _, f := range in {
		rule := f.rule
		if len(rule) > maxRuleID {
			rule = rule[:maxRuleID]
		}
		f.rule = rule
		f.message = normalizeMessage(f.message)
		fp := fingerprint(check, f)
		if seen[fp] {
			continue
		}
		seen[fp] = true
		item := &verifierv1.Finding{RuleId: f.rule, Severity: f.severity, Message: f.message, Fingerprint: fp}
		if f.path != "" {
			loc := &verifierv1.Location{Path: f.path, StartLine: f.start, EndLine: f.end}
			if loc.StartLine == 0 {
				loc.EndLine = 0
			} else if loc.EndLine < loc.StartLine {
				loc.EndLine = loc.StartLine
			}
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
