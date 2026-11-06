package evidenceverify

import "strings"

// finish fills the report from what the steps found and computes the level
// (verification.md section 4: "Precedence and the algorithm").
func (v *verifier) finish() *Report {
	r := v.rep
	if v.sum != nil {
		r.ChangeID = v.sum.changeID
	}
	if c := v.ch; c != nil {
		r.Records = len(c.records)
		r.HeadRecordHash = c.last.hash
		r.AnchoredHeadRecordHash = c.anchoredHash()
		r.Ended, r.Sealed = c.ended, c.sealed
		if c.ended {
			r.TerminalState = terminalState(c.terminal.action)
		}
	}
	if v.ch != nil && v.ch.trajectory != nil {
		r.Trajectory = v.ch.trajectory.summary()
	}
	r.CredentialScopes = "unsourced: no credential broker statement is in the attestation"
	r.NetworkPolicy = "unsourced: no egress proxy statement is in the attestation"
	if v.att != nil {
		if len(v.att.n.list("credential_scopes")) > 0 {
			r.CredentialScopes = "digest line and handle scope lines are present; the origin rests on the runner's signature"
		}
		if v.att.n.has("network_policy") {
			r.NetworkPolicy = "reference to this change and a digest are present; the digest cannot be re-derived from the bundle"
		}
	}
	v.describe()
	r.WeakenedByFlags = []string{}
	defer func() { r.OK = r.ExitCode() == 0 }()
	if len(r.Findings) > 0 {
		r.Level, r.LevelEnum = LevelFailed, LevelFailed
		return r
	}
	var weak []string // the first one names the level
	var weakEnum []string
	var allowed []string
	add := func(enum, sentence string) {
		weakEnum = append(weakEnum, enum)
		weak = append(weak, sentence)
	}
	switch {
	case v.opts.SkipSignatures:
		add(LevelUnauthenticated, "no signature was checked")
	case v.opts.TrustBundleKeys:
		add(LevelSelfConsistent, "signatures verified only under keys the export states")
	}
	if t := v.ch.trajectory; t != nil && !t.coversStreams(v.ch.records) {
		add(LevelIncomplete, "the trajectory statement does not cite every run of the chain: its comparison is of part of it")
	}
	if v.endedClosed() && v.att == nil {
		add(LevelIncomplete, "the change ended Closed and its attestation was not made")
	}
	if !v.ch.ended {
		if v.opts.AllowIncomplete {
			allowed = append(allowed, SuffixIncomplete)
		} else {
			add(LevelIncomplete, "the chain has no terminal record and may be a prefix of a longer chain")
		}
	}
	if !v.fullyAnchored() {
		if v.opts.AllowUnanchored {
			allowed = append([]string{SuffixUnanchored}, allowed...)
		} else {
			add(LevelUnanchored, "the head was not proved under a pinned log")
		}
	}
	if v.ch.ended && !v.endedClosed() {
		if v.opts.AllowUnsigned {
			allowed = append(allowed, SuffixUnsigned)
		} else {
			add(LevelUnsigned, "the change did not end Closed: the files are vouched for only by the unsigned index")
		}
	}
	r.Weaknesses = weak
	if len(weak) == 0 {
		r.LevelEnum = LevelVerified
		r.Level = LevelVerified
		if len(allowed) > 0 {
			r.Level += ", " + strings.Join(allowed, ", ")
			r.WeakenedByFlags = allowed
		}
		return r
	}
	r.Level, r.LevelEnum = weakEnum[0], weakEnum[0]
	return r
}

// fullyAnchored: the head proof verified under the pinned log and, for a
// change with an attestation, the attestation's own proof and the manifest's
// attestation proof too. A Closed change with a gap is never fully anchored.
func (v *verifier) fullyAnchored() bool {
	if v.log == nil || !v.headProofOK {
		return false
	}
	if v.att != nil {
		return v.attProofOK && v.manAttProof
	}
	return !v.endedClosed()
}

func (v *verifier) describe() {
	r := v.rep
	switch {
	case v.ch != nil:
		r.Chain = "records link, hash and end as stated"
	case v.sum == nil:
		r.Chain = "not checked (no readable summary)"
	default:
		r.Chain = "failed"
	}
	switch {
	case v.opts.SkipSignatures:
		r.Signatures = "not checked (--skip-signatures)"
	case v.opts.TrustBundleKeys:
		r.Signatures = "verified under keys the export states only"
	case v.ch != nil:
		r.Signatures = "verified under the pinned runner keys"
	default:
		r.Signatures = "not verified"
	}
	switch {
	case v.log == nil:
		r.Anchor = "not checked (no log pinned)"
	case v.headProofOK:
		r.Anchor = "head proved under the pinned log"
	default:
		r.Anchor = "not proved"
	}
	switch {
	case v.attVerified:
		r.Attestation = "signed, re-derived from the chain"
	case v.validGap:
		r.Attestation = "not made (a gap is stated)"
	case v.att != nil:
		r.Attestation = "present, not verified"
	default:
		r.Attestation = "none"
	}
}
