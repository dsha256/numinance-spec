package evidenceverify

import (
	"fmt"
	"sort"
	"strings"
)

// The finding codes of verification.md section 3, and expectation_mismatch.
const (
	CodeDirectoryRefused    = "directory_refused"
	CodeUnknownFile         = "unknown_file"
	CodeAttestationMissing  = "attestation_missing"
	CodeAttestationInvalid  = "attestation_invalid"
	CodeAttestationMismatch = "attestation_mismatch"
	CodeAttestationProof    = "attestation_proof"
	CodeManifestMissing     = "manifest_missing"
	CodeManifestInvalid     = "manifest_invalid"
	CodeManifestMismatch    = "manifest_mismatch"
	CodeViewMismatch        = "view_mismatch"
	CodeChangeInvalid       = "change_invalid"
	CodeToolEventsMismatch  = "tool_events_mismatch"
	CodeIndexInvalid        = "index_invalid"
	CodeFileMissing         = "file_missing"
	CodeFileUnlisted        = "file_unlisted"
	CodeDigestMismatch      = "digest_mismatch"
	CodeSummaryInvalid      = "summary_invalid"
	CodeChangeMismatch      = "change_mismatch"
	CodeRecordInvalid       = "record_invalid"
	CodeChainBroken         = "chain_broken"
	CodeSignatureInvalid    = "signature_invalid"
	CodeSignerUntrusted     = "signer_untrusted"
	CodeSummaryMismatch     = "summary_mismatch"
	CodeDiffMismatch        = "diff_mismatch"
	CodePartMissing         = "part_missing"
	CodePartContradiction   = "part_contradiction"
	CodePartsAbsent         = "parts_absent"
	CodeSignersInvalid      = "signers_invalid"
	CodeAnchorInvalid       = "anchor_invalid"
	CodeAnchorMissing       = "anchor_missing"
	CodeTooManyRecords      = "too_many_records"
	CodeExpectationMismatch = "expectation_mismatch"
)

// Levels: the display string of a report is Level; LevelEnum is the part
// without suffixes.
const (
	LevelVerified        = "verified"
	LevelFailed          = "failed"
	LevelUnauthenticated = "unauthenticated"
	LevelSelfConsistent  = "self-consistent"
	LevelIncomplete      = "incomplete"
	LevelUnsigned        = "unsigned"
	LevelUnanchored      = "unanchored"
)

// Suffixes of a pass, in their fixed order.
const (
	SuffixUnanchored = "unanchored"
	SuffixIncomplete = "incomplete"
	SuffixUnsigned   = "unsigned export"
)

// Finding is one failure. The code is normative; the message is for people.
type Finding struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Part is a part of the design list that the index declares absent or not
// applicable.
type Part struct {
	Part          string `json:"part"`
	Reason        string `json:"reason"`
	Owner         string `json:"owner"`
	NotApplicable bool   `json:"not_applicable,omitempty"`
}

// Report is the result of a verification. Its JSON form is the report of
// verification.md section 6, including the NEW keys.
type Report struct {
	// OK is true exactly when the exit code is 0: a pass, possibly with an
	// allowance. Automation that needs a full pass tests LevelEnum and
	// WeakenedByFlags, never a prefix of Level.
	OK                     bool     `json:"ok"`
	ChangeID               string   `json:"change_id"`
	Records                int      `json:"records"`
	HeadRecordHash         string   `json:"head_record_hash"`
	AnchoredHeadRecordHash string   `json:"anchored_head_record_hash,omitempty"`
	Ended                  bool     `json:"ended"`
	TerminalState          string   `json:"terminal_state,omitempty"`
	Sealed                 bool     `json:"sealed"`
	Level                  string   `json:"level"`
	LevelEnum              string   `json:"level_enum"`
	WeakenedByFlags        []string `json:"weakened_by_flags"`
	Weaknesses             []string `json:"weaknesses,omitempty"`
	Chain                  string   `json:"chain"`
	Signatures             string   `json:"signatures"`
	Anchor                 string   `json:"anchor"`
	Attestation            string   `json:"attestation"`
	Absent                 []Part   `json:"absent,omitempty"`
	NotApplicable          []Part   `json:"not_applicable,omitempty"`
	CredentialScopes       string   `json:"credential_scopes"`
	NetworkPolicy          string   `json:"network_policy"`
	// Trajectory is the result of the runner's trajectory statement when the chain holds one (docs/evidence/trajectory.md): a finding to
	// read, never a failure of the export. Absent for a chain without the statement, which is valid.
	Trajectory string    `json:"trajectory,omitempty"`
	Findings   []Finding `json:"findings,omitempty"`
}

// ExitCode is the exit code of numi evidence verify for this report: 0 for a
// pass (with or without an allowance), 3 for a report with a finding and 5 for
// a consistent export that is weaker than verified.
func (r *Report) ExitCode() int {
	switch {
	case len(r.Findings) > 0:
		return 3
	case r.LevelEnum == LevelVerified:
		return 0
	}
	return 5
}

// FindingCodes returns the sorted set of finding codes.
func (r *Report) FindingCodes() []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range r.Findings {
		if !seen[f.Code] {
			seen[f.Code] = true
			out = append(out, f.Code)
		}
	}
	sort.Strings(out)
	return out
}

// Text renders the report for people. The wording is not normative.
func (r *Report) Text() string {
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("change: %s", r.ChangeID)
	line("records: %d, head %s", r.Records, r.HeadRecordHash)
	state := "open (no terminal record)"
	if r.Ended {
		state = "ended " + r.TerminalState
		if r.Sealed {
			state += ", sealed"
		}
	}
	line("state: %s", state)
	line("level: %s", r.Level)
	line("chain: %s", r.Chain)
	line("signatures: %s", r.Signatures)
	line("anchor: %s", r.Anchor)
	line("attestation: %s", r.Attestation)
	line("credential_scopes: %s", r.CredentialScopes)
	line("network_policy: %s", r.NetworkPolicy)
	if r.Trajectory != "" {
		line("trajectory: %s", r.Trajectory)
	}
	for _, p := range r.NotApplicable {
		line("not applicable: %s (%s)", p.Part, p.Reason)
	}
	for _, p := range r.Absent {
		line("absent: %s (%s; owner: %s)", p.Part, p.Reason, p.Owner)
	}
	for _, f := range r.Findings {
		line("FAIL %s: %s", f.Code, f.Message)
	}
	for _, w := range r.Weaknesses {
		line("WEAK %s", w)
	}
	switch {
	case len(r.Findings) > 0:
		line("result: %s", r.Level)
	case len(r.WeakenedByFlags) > 0:
		line("result: %s (a pass with an allowance: %s)", r.Level, strings.Join(r.WeakenedByFlags, ", "))
	case len(r.Absent) > 0 && r.LevelEnum == LevelVerified:
		line("result: %s PARTIAL (parts are absent)", r.Level)
	default:
		line("result: %s", r.Level)
	}
	return b.String()
}
