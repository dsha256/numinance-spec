package evidenceverify

import (
	"errors"
	"sort"
)

type actorPair struct{ principal, kind string }

type attDoc struct {
	n      *node
	digest [32]byte // signing digest, also the attestation leaf hash
}

func decodeGap(data []byte, changeID string) error {
	n, err := decodeDoc(data, maxDoc, gapFields())
	if err != nil {
		return err
	}
	if n.s("change_id") != changeID {
		return errors.New("the gap names another change")
	}
	return nil
}

func decodeAttestation(data []byte) (*attDoc, error) {
	n, err := decodeDoc(data, maxDoc, attestationFields())
	if err != nil {
		return nil, err
	}
	if n.u("first_sequence") > n.u("last_sequence") {
		return nil, errors.New("first_sequence is after last_sequence")
	}
	if _, err := proofFromNode(n.obj("inclusion_proof")); err != nil {
		return nil, errors.New("inclusion_proof is not a well-formed proof: " + err.Error())
	}
	if !credentialScopesOK(n.strs("credential_scopes")) {
		return nil, errors.New("credential_scopes does not follow its grammar")
	}
	if np := n.obj("network_policy"); np != nil && np.s("ref") != "egress-policy.v1/"+n.s("change_id") {
		return nil, errors.New("network_policy does not name this change")
	}
	return &attDoc{n: n, digest: digestBytes(n.canonicalWithout("signature"))}, nil
}

// endedClosed reports whether the chain ended with ChangeClosed.v1.
func (v *verifier) endedClosed() bool {
	return v.ch.ended && v.ch.terminal.action == actionClosed
}

// stepAttestation checks the attestation or its gap (verification.md section 2,
// step 8). The presence of the attestation and of the gap is checked whether or
// not the chain verified; the attestation itself is checked against a verified
// chain.
func (v *verifier) stepAttestation() {
	hasAtt, hasGap := v.b.has(pathAttest), v.b.has(pathGap)
	if hasAtt && hasGap {
		v.fail(CodeAttestationMismatch, "an attestation and an attestation gap are both present")
		return
	}
	if !hasAtt {
		v.stepGap(hasGap)
		return
	}
	if v.ch == nil {
		return
	}
	data, err := v.b.read(pathAttest)
	if err != nil {
		v.fail(CodeAttestationInvalid, "attestation.json cannot be read")
		return
	}
	a, err := decodeAttestation(data)
	if err != nil {
		v.fail(CodeAttestationInvalid, "attestation.json does not decode: "+err.Error())
		return
	}
	if !v.opts.SkipSignatures {
		pub, ok := v.trusted[a.n.s("signing_key_id")]
		if !ok || !verifySigned(pub, ctxAttestation, a.digest, decodeBase64(a.n.s("signature"))) {
			v.fail(CodeAttestationInvalid, "the attestation is signed by no trusted key or its signature fails")
			return
		}
	}
	v.att = a
	mismatch := v.rederiveAttestation(a)
	if mismatch != "" {
		v.fail(CodeAttestationMismatch, mismatch)
	}
	if mismatch != "" {
		return
	}
	v.attVerified = true
	proofOK := v.attestationProof(a)
	v.attProofOK = proofOK && v.log != nil
}

// attestationProof checks the per-change checkpoint and, with a pinned log,
// the head leaf under it. It reports whether everything that could be checked
// held.
func (v *verifier) attestationProof(a *attDoc) bool {
	cpData, err := v.b.read(pathAttestCP)
	if err != nil || !v.b.has(pathAttestCP) {
		v.fail(CodeAttestationProof, "the per-change checkpoint is missing")
		return false
	}
	ref := a.n.obj("checkpoint_ref")
	if sha256Digest(cpData) != ref.s("sha256") {
		v.fail(CodeAttestationProof, "the per-change checkpoint is not the one checkpoint_ref names")
		return false
	}
	cp, err := decodeCheckpoint(cpData)
	if err != nil {
		v.fail(CodeAttestationProof, "the per-change checkpoint does not decode")
		return false
	}
	if v.log == nil {
		return true
	}
	p, err := proofFromNode(a.n.obj("inclusion_proof"))
	if err == nil {
		raw, _ := rawDigest(a.n.s("head_record_hash"))
		err = v.log.verifyLeaf(cp, p, headLeaf(leafHead, raw))
	}
	if err != nil {
		v.fail(CodeAttestationProof, "the attestation head leaf does not verify under the pinned log: "+err.Error())
		return false
	}
	return true
}

// rederiveAttestation recomputes the attestation from the chain
// (attestation-and-manifest.md section 3) and returns a reason on a difference.
func (v *verifier) rederiveAttestation(a *attDoc) string {
	n := a.n
	term := v.ch.terminal
	if term == nil || term.action != actionClosed {
		return "the chain does not end with a ChangeClosed.v1 record"
	}
	span := v.ch.records[:term.sequence]
	if n.s("change_id") != v.sum.changeID {
		return "change_id differs from the chain"
	}
	if n.u("first_sequence") != 1 || n.u("last_sequence") != term.sequence || n.s("head_record_hash") != term.hash {
		return "the range or head differs from the chain"
	}
	if n.s("policy_bundle_digest") != term.policy || n.s("profile_digest") != term.profile {
		return "the policy bundle or profile digest differs from the terminal record"
	}
	pairs := map[actorPair]bool{}
	var executor, model, sandbox, runSandbox string
	var verifierResults []artifact
	var approvals []string
	for _, r := range span {
		pairs[actorPair{r.principal, r.kind}] = true
		if r.kind == "author-agent" {
			if _, _, ok := executorIDParts(r.executor); r.executor != "" && !ok {
				return "an author record states an executor_id that is not <name>@sha256:<digest>; no attestation is made"
			}
			executor, model, sandbox = pick(executor, r.executor), pick(model, r.model), pick(sandbox, r.sandboxDigest)
			if r.executor != "" {
				runSandbox = pick(runSandbox, r.sandboxDigest)
			}
		}
		if r.action == "verifier.run" {
			verifierResults = append(verifierResults, r.outputs...)
		}
		if r.action == actionGate {
			approvals = append(approvals, r.evID)
		}
	}
	// A provider call made for the author states the image of the provider's sandbox, not the image the author ran in: when any author
	// record states an executor, the image is the one those records agree on.
	if runSandbox != "" {
		sandbox = runSandbox
	}
	if executor == "\x00" || model == "\x00" || sandbox == "\x00" {
		return "the author records disagree on the executor, model or sandbox image"
	}
	actors := make([]actorPair, 0, len(pairs))
	for p := range pairs {
		actors = append(actors, p)
	}
	sort.Slice(actors, func(i, j int) bool {
		if actors[i].principal != actors[j].principal {
			return actors[i].principal < actors[j].principal
		}
		return kindNumber(actors[i].kind) < kindNumber(actors[j].kind)
	})
	var gotActors []actorPair
	for _, it := range n.list("actors") {
		gotActors = append(gotActors, actorPair{it.s("principal"), it.s("kind")})
	}
	if !equalSlices(gotActors, actors) {
		return "actors differ from the chain"
	}
	if n.s("executor_id") != executor || n.s("model_id") != model || n.s("sandbox_image_digest") != sandbox {
		return "executor, model or sandbox image differs from the author records"
	}
	if !equalSlices(artifacts(n.list("verifier_results")), verifierResults) {
		return "verifier_results differ from the verifier.run outputs"
	}
	if !equalSlices(n.strs("approval_ev_ids"), approvals) {
		return "approval_ev_ids differ from the gate decisions"
	}
	if rc := n.s("result_commit"); rc != "" {
		want := sha256Digest([]byte(rc))
		found := false
		for _, r := range v.ch.records {
			for _, o := range r.outputs {
				found = found || (o.ref == "publish/commit" && o.sha256 == want)
			}
		}
		if !found {
			return "result_commit is supported by no publication"
		}
	}
	return ""
}

// pick merges a value into the one non-empty value; two different non-empty
// values make "\x00" (ambiguous).
func pick(cur, next string) string {
	switch {
	case cur == "\x00" || next == "":
		return cur
	case cur == "" || cur == next:
		return next
	}
	return "\x00"
}

func kindNumber(k string) int {
	for i, a := range actorKinds() {
		if a == k {
			return i + 1
		}
	}
	// The kind numbers of the types.v1 PrincipalKind enum: 7 is the producer-only engine, never an actor.
	switch k {
	case "broker":
		return 8
	case "egress-proxy":
		return 9
	}
	return 0
}

func equalSlices[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// stepGap handles an export with no attestation: a Closed change needs one, or
// a valid gap for that change. A gap beside a chain that did not verify or did
// not end Closed is a finding, because nothing shows that the change ended
// Closed.
func (v *verifier) stepGap(hasGap bool) {
	closed := v.ch != nil && v.endedClosed()
	if !closed {
		if hasGap {
			v.fail(CodeAttestationInvalid, "a gap is stated for a change that did not end Closed")
		}
		return
	}
	if !hasGap {
		v.fail(CodeAttestationMissing, "the change ended Closed and has no attestation")
		return
	}
	data, err := v.b.read(pathGap)
	if err != nil || decodeGap(data, v.sum.changeID) != nil {
		v.fail(CodeAttestationInvalid, "the attestation gap does not decode or names another change")
		return
	}
	v.validGap = true
}
