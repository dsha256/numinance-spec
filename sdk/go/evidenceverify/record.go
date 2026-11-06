package evidenceverify

import (
	"errors"
	"slices"
)

type artifact struct{ ref, sha256 string }

type lifecycle struct {
	from, to  string
	rowVer    uint64
	authorRun string
	maxRepair uint64
}

// record is a decoded and rule-checked record, reduced to what the derivations
// read. The raw node is not kept.
type record struct {
	evID, changeID  string
	sequence        uint64
	action          string
	principal, kind string
	runID           string
	outcome         string
	plan            string
	executor, model string
	sandboxProfile  string
	sandboxDigest   string
	inputs, outputs []artifact
	corrects        string
	previous        string
	hash            string
	profile, policy string
	signature       []byte
	signingKeyID    string
	lc              *lifecycle
	hashedBytes     [32]byte // sha256 of the canonical form without record_hash and signature
	signingDigest   [32]byte // sha256 of the canonical form without signature
	decisionID      string
	grantID         string
}

func lifecycleActions() []string {
	return []string{
		"ChangeTransitioned.v1", "ChangeUnderstood.v1", "ChangeClassified.v1", "PlanProduced.v1",
		"PlanGateDecided.v1", "CandidateAssembled.v1", "ChangeEscalated.v1", "ChangeClosed.v1",
		"ChangeAbandoned.v1", "ChangeRolledBack.v1",
	}
}

const (
	actionClosed     = "ChangeClosed.v1"
	actionAbandoned  = "ChangeAbandoned.v1"
	actionRolledBack = "ChangeRolledBack.v1"
	actionSealed     = "EvidenceSealed.v1"
	actionGate       = "PlanGateDecided.v1"
	actionVerdict    = "VerdictIssued.v1"
)

func isTerminalAction(a string) bool {
	return a == actionClosed || a == actionAbandoned || a == actionRolledBack
}

func terminalState(a string) string {
	switch a {
	case actionClosed:
		return "Closed"
	case actionAbandoned:
		return "Abandoned"
	case actionRolledBack:
		return "RolledBack"
	}
	return ""
}

func artifacts(items []*node) []artifact {
	out := make([]artifact, 0, len(items))
	for _, it := range items {
		out = append(out, artifact{it.s("ref"), it.s("sha256")})
	}
	return out
}

// decodeRecord decodes one line of records.ndjson (without its line feed) and
// applies the record validity rules of records-and-chain.md section 1.
func decodeRecord(line []byte) (*record, error) {
	n, err := decodeDoc(line, maxRecordLine, recordFields())
	if err != nil {
		return nil, err
	}
	r := &record{
		evID: n.s("ev_id"), changeID: n.s("change_id"), sequence: n.u("sequence"), action: n.s("action"),
		principal: n.s("actor_principal"), kind: n.s("actor_kind"), runID: n.s("actor_run_id"),
		outcome: n.s("action_outcome"), plan: n.s("plan_digest"), executor: n.s("executor_id"),
		model: n.s("model_id"), sandboxProfile: n.s("sandbox_profile"), sandboxDigest: n.s("sandbox_image_digest"),
		inputs: artifacts(n.list("inputs")), outputs: artifacts(n.list("outputs")),
		corrects: n.s("corrects_record_hash"), previous: n.s("previous_record_hash"), hash: n.s("record_hash"),
		profile: n.s("profile_digest"), policy: n.s("policy_bundle_digest"), signingKeyID: n.s("signing_key_id"),
		decisionID: n.s("policy_decision_id"), grantID: n.s("grant_id"),
	}
	if sig := n.s("signature"); sig != "" {
		r.signature = decodeBase64(sig)
	}
	if lc := n.obj("lifecycle"); lc != nil {
		r.lc = &lifecycle{from: lc.s("from"), to: lc.s("to"), rowVer: lc.u("row_version"), authorRun: lc.s("author_run_id"), maxRepair: lc.u("max_repair_attempts")}
	}
	if err := checkRecordRules(r); err != nil {
		return nil, err
	}
	r.hashedBytes = digestBytes(n.canonicalWithout("record_hash", "signature"))
	r.signingDigest = digestBytes(n.canonicalWithout("signature"))
	return r, nil
}

func checkRecordRules(r *record) error {
	switch {
	case (r.kind == "author-agent" || r.kind == "reviewer-agent") && r.runID == "":
		return errors.New("agent actor without a run id")
	case (r.sandboxProfile == "") != (r.sandboxDigest == ""):
		return errors.New("sandbox profile and image digest are not both set")
	case r.executor != "" && r.sandboxProfile == "":
		return errors.New("executor without a sandbox")
	case (r.sequence == 1) != (r.previous == ""):
		return errors.New("previous record hash does not fit the sequence")
	case r.sequence == 1 && r.corrects != "":
		return errors.New("a correction at sequence 1")
	case r.corrects != "" && r.corrects == r.hash:
		return errors.New("a record corrects itself")
	case len(r.signature) > 0 && (r.signingKeyID == "" || r.hash == ""):
		return errors.New("signature without key id or record hash")
	}
	if isCapability(r.action) && (r.decisionID == "" || (r.grantID == "" && r.outcome != "denied")) {
		return errors.New("capability record without a policy decision or grant")
	}
	if r.lc != nil {
		if !slices.Contains(lifecycleActions(), r.action) || r.outcome != "succeeded" {
			return errors.New("lifecycle on an action or outcome that does not take it")
		}
	}
	if err := checkReceiptRules(r); err != nil {
		return err
	}
	return checkTrajectoryRules(r)
}
