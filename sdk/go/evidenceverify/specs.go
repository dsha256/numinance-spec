package evidenceverify

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// The field rules of each document, from the pages of docs/evidence and the
// schemas under schemas/evidence/v1.

const (
	maxDoc        = 1 << 20   // every JSON document of the export
	maxBigFile    = 256 << 20 // records.ndjson and tool-events.ndjson
	maxRecordLine = 3 << 20
	maxProof      = 8192
	maxCheckpoint = 4096
	maxChain      = 1 << 15
)

var (
	// A capability is port.verb, or tool.<plugin>.<name> for a tool of the tool gateway (the plugin is one lowercase DNS label).
	capabilityRe = regexp.MustCompile(`^(?:[a-z][a-z0-9]*\.[a-z][a-z0-9_]*|tool\.[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?\.[a-z][a-z0-9_]*)$`)
	lifecycleRe  = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*\.v([1-9]\d*)$`)
	semverRe     = regexp.MustCompile(`^(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)$`)
	viewTokenRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
	stateRe      = regexp.MustCompile(`^[a-z_]{1,32}$`)
)

func actorKinds() []string {
	return []string{"author-agent", "reviewer-agent", "release-pipeline", "human-approver", "runner", "break-glass"}
}

// recordActorKinds are the kinds a record or the attestation's actors may carry: the six actor kinds and the two trusted
// components that author their own receipts (docs/evidence/receipts.md). The author of a change is never a component, so
// author_kind keeps actorKinds.
func recordActorKinds() []string {
	return append(actorKinds(), "broker", "egress-proxy")
}

func isAction(s string) bool {
	if len(s) > 128 {
		return false
	}
	if capabilityRe.MatchString(s) {
		return true
	}
	m := lifecycleRe.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	major, err := strconv.ParseUint(m[1], 10, 64)
	return err == nil && major <= math.MaxInt32
}

func isCapability(s string) bool { return capabilityRe.MatchString(s) }

func printable256(s string) bool { return isPrintableText(s, 256) }

func nonEmptyText(limit int) func(string) bool {
	return func(s string) bool { return s != "" && isPrintableText(s, limit) }
}

func nonEmptyBytes(limit int) func(string) bool {
	return func(s string) bool { return s != "" && len(s) <= limit }
}

func artifactFields() []field {
	return []field{str("ref", true, isRef), {name: "sha256", t: tStr, req: true, ok: isDigest}}
}

func lifecycleFields() []field {
	return []field{
		str("from", true, func(s string) bool { return stateRe.MatchString(s) }),
		str("to", true, func(s string) bool { return stateRe.MatchString(s) }),
		uintf("row_version", true, 1, maxSafe),
		str("author_run_id", false, isRunID),
		uintf("max_repair_attempts", false, 0, 65536),
	}
}

func recordFields() []field {
	return []field{
		str("ev_id", true, isEvID),
		str("change_id", true, isChangeID),
		uintf("sequence", true, 1, maxSafe),
		str("action", true, isAction),
		str("actor_principal", true, isASCIIText),
		str("actor_kind", true, oneOf(recordActorKinds()...)),
		str("actor_run_id", false, isRunID),
		str("policy_decision_id", false, isDecisionID),
		str("grant_id", false, isGrantID),
		str("risk_class", true, oneOf("READ", "LOW", "MEDIUM", "HIGH", "PRIVILEGED")),
		digest("profile_digest", true),
		digest("policy_bundle_digest", true),
		digest("plan_digest", false),
		str("executor_id", false, printable256),
		str("model_id", false, printable256),
		digest("sandbox_image_digest", false),
		str("sandbox_profile", false, oneOf("process", "container", "gvisor", "microvm")),
		arrf("inputs", 1024, objf("", true, artifactFields())),
		arrf("outputs", 1024, objf("", true, artifactFields())),
		str("action_outcome", true, oneOf("succeeded", "partial", "failed", "denied")),
		digest("corrects_record_hash", false),
		digest("previous_record_hash", false),
		digest("record_hash", false),
		str("timestamp", true, isTimestamp),
		uintf("canon_version", true, 2, 2),
		sigf(false),
		objf("lifecycle", false, lifecycleFields()),
		digest("signing_key_id", false),
	}
}

func proofFields() []field {
	return []field{
		uintf("leaf_index", true, 0, maxSafe-1),
		uintf("tree_size", true, 1, maxSafe),
		arrf("audit_path", 64, str("", true, isDigest)),
	}
}

func checkpointFields() []field {
	return []field{
		str("origin", true, func(s string) bool { return s != "" && isPrintableText(s, 256) }),
		uintf("tree_size", true, 1, maxSafe),
		{name: "root_hash", t: tStr, req: true, ok: isDigest},
		str("timestamp", true, isTimestamp),
		uintf("canon_version", true, 1, 1),
		sigf(true),
		{name: "signing_key_id", t: tStr, req: true, ok: isDigest},
	}
}

func summaryFields() []field {
	return []field{
		uintf("export_version", true, 1, 1),
		str("change_id", true, isChangeID),
		uintf("records", true, 1, math.MaxUint64),
		uintf("first_sequence", true, 1, 1),
		uintf("last_sequence", true, 1, maxSafe),
		{name: "head_record_hash", t: tStr, req: true, ok: isDigest},
		uintf("denied", true, 0, math.MaxUint64),
		boolf("ended", true),
		str("terminal_state", false, oneOf("Closed", "Abandoned", "RolledBack")),
		boolf("sealed", true),
	}
}

func gapFields() []field {
	return []field{
		uintf("export_version", true, 1, 1),
		str("change_id", true, isChangeID),
		str("reason", true, nonEmptyText(512)),
	}
}

func signersFields() []field {
	return []field{
		uintf("export_version", true, 1, 1),
		arrf("signers", 64, objf("", true, []field{
			{name: "key_id", t: tStr, req: true, ok: isDigest},
			str("public_key", true, func(s string) bool { return isBase64Len(s, 32) }),
		})),
		objf("log", false, []field{
			str("origin", true, func(s string) bool { return s != "" && isPrintableText(s, 256) }),
			str("public_key", true, func(s string) bool { return isBase64Len(s, 32) }),
		}),
	}
}

func absentFields() []field {
	return []field{
		str("part", true, oneOf(designParts()...)),
		str("reason", true, nonEmptyBytes(512)),
		str("owner", true, nonEmptyBytes(512)),
		boolf("not_applicable", false),
	}
}

func entryFields() []field {
	return []field{
		str("path", true, isKnownPath),
		{name: "sha256", t: tStr, req: true, ok: isDigest},
		uintf("size", true, 0, math.MaxUint64),
	}
}

func indexFields() []field {
	return []field{
		arrf("absent", 12, objf("", true, absentFields())),
		str("change_id", true, isChangeID),
		arrf("entries", 64, objf("", true, entryFields())),
		uintf("export_version", true, 1, 1),
	}
}

func attestationFields() []field {
	return []field{
		str("change_id", true, isChangeID),
		str("ticket", true, isTicket),
		str("repository", false, isASCIIText),
		str("base_commit", false, isASCIIText),
		str("result_commit", false, isASCIIText),
		str("executor_id", false, printable256),
		str("model_id", false, printable256),
		digest("sandbox_image_digest", false),
		digest("policy_bundle_digest", false),
		digest("profile_digest", false),
		digest("toolchain_digest", false),
		arrf("verifier_results", 1024, objf("", true, artifactFields())),
		arrf("ci_run_ids", 1024, str("", true, isASCIIText)),
		arrf("deployment_ids", 1024, str("", true, isASCIIText)),
		arrf("actors", 1024, objf("", true, []field{
			str("principal", true, isASCIIText),
			str("kind", true, oneOf(recordActorKinds()...)),
		})),
		arrf("credential_scopes", 1024, str("", true, isASCIIText)),
		objf("network_policy", false, []field{
			str("ref", true, nil),
			digest("sha256", true),
		}),
		arrf("approval_ev_ids", 1024, str("", true, isEvID)),
		uintf("first_sequence", true, 1, maxSafe),
		uintf("last_sequence", true, 1, maxSafe),
		{name: "head_record_hash", t: tStr, req: true, ok: isDigest},
		sigf(true),
		{name: "signing_key_id", t: tStr, req: true, ok: isDigest},
		uintf("canon_version", true, 1, 1),
		objf("checkpoint_ref", true, artifactFields()),
		objf("inclusion_proof", true, proofFields()),
		str("postgres_transport", false, oneOf("verify-full", "loopback-plaintext")),
	}
}

func manifestFields() []field {
	return []field{
		str("change_id", true, isChangeID),
		str("view", false, func(s string) bool { return viewTokenRe.MatchString(s) }),
		str("exported_at", true, isTimestamp),
		str("spec_version", true, func(s string) bool { return semverRe.MatchString(s) }),
		uintf("first_sequence", true, 1, maxSafe),
		uintf("last_sequence", true, 1, maxSafe),
		{name: "head_record_hash", t: tStr, req: true, ok: isDigest},
		arrf("entries", 64, objf("", true, entryFields())),
		sigf(true),
		{name: "signing_key_id", t: tStr, req: true, ok: isDigest},
		uintf("canon_version", true, 1, 1),
		objf("checkpoint_ref", true, artifactFields()),
		objf("inclusion_proof", true, proofFields()),
		objf("attestation_inclusion_proof", true, proofFields()),
	}
}

func changeFields() []field {
	return []field{
		uintf("export_version", true, 1, 1),
		str("change_id", true, isChangeID),
		str("state", false, func(s string) bool { return stateRe.MatchString(s) }),
		digest("plan_digest", false),
		str("author", false, isASCIIText),
		str("author_kind", false, oneOf(actorKinds()...)),
		arrf("author_runs", 1024, str("", true, isRunID)),
		uintf("repair_attempts", false, 1, maxSafe),
		uintf("max_repair_attempts", false, 1, math.MaxUint32),
		str("ticket", false, isTicket),
	}
}

// credentialScopesOK checks the grammar of the credential_scopes statement.
func credentialScopesOK(scopes []string) bool {
	if len(scopes) == 0 {
		return true
	}
	const head = "broker.scopes.v1 "
	if !strings.HasPrefix(scopes[0], head) || !isDigest(scopes[0][len(head):]) {
		return false
	}
	prev := ""
	h := "numinance.credential_scopes.v1"
	for i, line := range scopes[1:] {
		if !handleLineOK(line) || (i > 0 && line <= prev) {
			return false
		}
		prev = line
		h += "\n" + line
	}
	return sha256Digest([]byte(h)) == scopes[0][len(head):]
}

func handleLineOK(line string) bool {
	const p = "nhref_"
	if len(line) < len(p)+12+2 || !strings.HasPrefix(line, p) || !isLowerHex(line[len(p):len(p)+12]) || line[len(p)+12] != ' ' {
		return false
	}
	kind, value, ok := strings.Cut(line[len(p)+13:], ":")
	if !ok || kind == "" || len(kind) > 32 || value == "" {
		return false
	}
	for i := 0; i < len(kind); i++ {
		c := kind[i]
		lower := c >= 'a' && c <= 'z'
		if !lower && (i == 0 || ((c < '0' || c > '9') && c != '_')) {
			return false
		}
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 0x21 || value[i] > 0x7e {
			return false
		}
	}
	return true
}
