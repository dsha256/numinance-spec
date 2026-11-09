package evidenceverify

import (
	"bytes"
	"encoding/json"
	"strings"
)

func artifactsAny(as []artifact) []any {
	out := make([]any, 0, len(as))
	for _, a := range as {
		out = append(out, map[string]any{"ref": a.ref, "sha256": a.sha256})
	}
	return out
}

// verdictOf returns the verdict entry of a record, or false when the record is
// not a verdict record (derivations.md section 5).
func verdictOf(r *record) (map[string]any, string, bool) {
	if r.action != actionVerdict || r.kind != "runner" || r.outcome != "succeeded" ||
		len(r.inputs) < 1 || len(r.inputs) > 2 || len(r.outputs) != 2 {
		return nil, "", false
	}
	var subject, overrules, document, status string
	subjects := 0
	for _, in := range r.inputs {
		switch in.ref {
		case "verdict/subject":
			subject = in.sha256
			subjects++
		case "verdict/overrules":
			overrules = in.sha256
		default:
			return nil, "", false
		}
	}
	if subjects != 1 || len(r.inputs) == 2 && overrules == "" {
		return nil, "", false
	}
	for _, out := range r.outputs {
		switch {
		case out.ref == "verdict/document":
			if out.sha256 == zeroDigest || document != "" {
				return nil, "", false
			}
			document = out.sha256
		case len(out.ref) > len("verdict/status/") && out.ref[:len("verdict/status/")] == "verdict/status/":
			st := out.ref[len("verdict/status/"):]
			if (st != "pass" && st != "fail" && st != "inconclusive") || status != "" || out.sha256 != sha256Digest([]byte(st)) {
				return nil, "", false
			}
			status = st
		default:
			return nil, "", false
		}
	}
	if document == "" || status == "" {
		return nil, "", false
	}
	e := map[string]any{"sequence": r.sequence, "ev_id": r.evID, "record_hash": r.hash, "candidate": subject, "status": status, "document": document}
	if overrules != "" {
		e["overrules"] = overrules
	}
	return e, subject, true
}

func (v *verifier) deriveVerdicts() []byte {
	var verdicts, reports []any
	for _, r := range v.ch.records {
		if e, _, ok := verdictOf(r); ok {
			verdicts = append(verdicts, e)
		}
		if r.action == "verifier.run" {
			e := map[string]any{"sequence": r.sequence, "ev_id": r.evID, "record_hash": r.hash, "action_outcome": r.outcome}
			if len(r.outputs) > 0 {
				e["outputs"] = artifactsAny(r.outputs)
			}
			reports = append(reports, e)
		}
	}
	if len(verdicts) == 0 && len(reports) == 0 {
		return nil
	}
	doc := map[string]any{"export_version": uint64(1), "change_id": v.sum.changeID}
	if len(verdicts) > 0 {
		doc["verdicts"] = verdicts
	}
	if len(reports) > 0 {
		doc["reports"] = reports
	}
	return canonical(doc)
}

func (v *verifier) deriveReview() []byte {
	var decisions, reviewers []any
	for _, r := range v.ch.records {
		e := map[string]any{"sequence": r.sequence, "ev_id": r.evID, "record_hash": r.hash, "action": r.action,
			"actor": r.principal, "actor_kind": r.kind, "action_outcome": r.outcome}
		if r.runID != "" {
			e["actor_run_id"] = r.runID
		}
		if r.action == actionGate {
			decisions = append(decisions, e)
		}
		if r.kind == "reviewer-agent" {
			reviewers = append(reviewers, e)
		}
	}
	if len(decisions) == 0 && len(reviewers) == 0 {
		return nil
	}
	doc := map[string]any{"export_version": uint64(1), "change_id": v.sum.changeID}
	if len(decisions) > 0 {
		doc["decisions"] = decisions
	}
	if len(reviewers) > 0 {
		doc["reviewer_records"] = reviewers
	}
	return canonical(doc)
}

func (v *verifier) deriveProvenance() []byte {
	var plans, candidates, executors, runs []any
	seenPlan, seenCand, seenExec, seenRun := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	addCand := func(d string) {
		if d != "" && !seenCand[d] {
			seenCand[d] = true
			candidates = append(candidates, d)
		}
	}
	last := v.ch.last
	for _, r := range v.ch.records {
		if r.plan != "" && !seenPlan[r.plan] {
			seenPlan[r.plan] = true
			plans = append(plans, r.plan)
		}
		if r.action == "execution.diff" {
			for _, o := range r.outputs {
				if o.ref == "execution/candidate" {
					addCand(o.sha256)
				}
			}
		}
		if _, subject, ok := verdictOf(r); ok {
			addCand(subject)
		}
		if r.executor != "" {
			key := r.executor + "\x00" + r.model + "\x00" + r.sandboxProfile + "\x00" + r.sandboxDigest
			if !seenExec[key] {
				seenExec[key] = true
				e := map[string]any{"executor_id": r.executor}
				if r.model != "" {
					e["model_id"] = r.model
				}
				if r.sandboxProfile != "" {
					e["sandbox_profile"] = r.sandboxProfile
				}
				if r.sandboxDigest != "" {
					e["sandbox_image_digest"] = r.sandboxDigest
				}
				executors = append(executors, e)
			}
		}
		if r.runID != "" && !seenRun[r.runID] {
			seenRun[r.runID] = true
			runs = append(runs, map[string]any{"run_id": r.runID, "principal": r.principal, "kind": r.kind})
		}
	}
	doc := map[string]any{"export_version": uint64(1), "change_id": v.sum.changeID, "profile_digest": last.profile, "policy_bundle_digest": last.policy}
	for k, l := range map[string][]any{"plans": plans, "candidates": candidates, "executors": executors, "runs": runs} {
		if len(l) > 0 {
			doc[k] = l
		}
	}
	return canonical(doc)
}

// stepViews compares the three derived views with their derivations.
func (v *verifier) stepViews() {
	if v.ch == nil {
		return
	}
	views := []struct {
		path   string
		derive func() []byte
	}{{pathVerdicts, v.deriveVerdicts}, {pathReview, v.deriveReview}, {pathProvenance, v.deriveProvenance}}
	for _, view := range views {
		want := view.derive()
		if !v.b.has(view.path) {
			continue
		}
		got, err := v.b.read(view.path)
		if err != nil || want == nil || !bytes.Equal(got, want) {
			v.fail(CodeViewMismatch, "a derived view differs from its derivation or has nothing to derive")
		}
	}
}

// stepToolEvents checks tool-events.ndjson against the streams the chain cites
// (derivations.md section 7).
func (v *verifier) stepToolEvents() {
	if v.ch == nil || !v.b.has(pathToolEvents) {
		return
	}
	var cited []string
	seen := map[string]bool{}
	for _, r := range v.ch.records {
		if r.action != "execution.stream" {
			continue
		}
		for _, o := range r.outputs {
			if o.ref == "execution/stream" {
				if !seen[o.sha256] {
					seen[o.sha256] = true
					cited = append(cited, o.sha256)
				}
				break
			}
		}
	}
	data, err := v.b.read(pathToolEvents)
	if err != nil {
		v.fail(CodeToolEventsMismatch, "tool-events.ndjson cannot be read")
		return
	}
	got, ok := streamDigests(data)
	if !ok || len(got) != len(cited) {
		v.fail(CodeToolEventsMismatch, "tool-events.ndjson is not the concatenation of the cited streams")
		return
	}
	for i := range got {
		if got[i] != cited[i] {
			v.fail(CodeToolEventsMismatch, "tool-events.ndjson is not the concatenation of the cited streams")
			return
		}
	}
}

// streamDigests splits the file into streams at every line whose seq is 1 and
// returns the digest of each. The seq is read as Go's JSON decoding reads it:
// the key matches without regard to case and the last value wins.
func streamDigests(data []byte) ([]string, bool) {
	var out []string
	start := 0
	for pos := 0; pos < len(data); {
		end := bytes.IndexByte(data[pos:], '\n')
		if end < 0 {
			return nil, false // a line without its line feed
		}
		var line struct {
			Seq *uint64 `json:"seq"`
		}
		if err := json.Unmarshal(data[pos:pos+end], &line); err != nil || line.Seq == nil {
			return nil, false
		}
		if *line.Seq == 1 {
			if pos > start {
				out = append(out, sha256Digest(data[start:pos]))
			}
			start = pos
		} else if pos == 0 {
			return nil, false
		}
		pos += end + 1
	}
	if len(data) > start {
		out = append(out, sha256Digest(data[start:]))
	}
	return out, true
}

// executorIDParts splits an executor_id of the form "<name>@sha256:<64 lowercase hex>" into the name and the digest; any other
// form, a bare name included, makes no manifest claim (bundle-layout.md section 6).
func executorIDParts(executorID string) (name, digest string, ok bool) {
	i := strings.LastIndexByte(executorID, '@')
	if i < 1 || !isDigest(executorID[i+1:]) {
		return "", "", false
	}
	return executorID[:i], executorID[i+1:], true
}

// executorManifestDigests returns the distinct manifest digests the executor_ids of the chain name, in first-seen order.
func (v *verifier) executorManifestDigests() []string {
	var want []string
	seen := map[string]bool{}
	for _, r := range v.ch.records {
		if _, d, ok := executorIDParts(r.executor); ok && !seen[d] {
			seen[d] = true
			want = append(want, d)
		}
	}
	return want
}

// stepExecutorManifest checks the manifest files against the executor_ids of the chain (bundle-layout.md section 6): one digest is
// carried by provenance/executor-manifest.json, two or more by provenance/executor-manifests.json, never both, and a file that no
// executor_id names is a mismatch.
func (v *verifier) stepExecutorManifest() {
	if v.ch == nil {
		return
	}
	want := v.executorManifestDigests()
	one, many := v.b.has(pathExecutorManifest), v.b.has(pathExecutorManifests)
	switch {
	case one && many:
		v.fail(CodeDigestMismatch, "provenance/executor-manifest.json and provenance/executor-manifests.json are both present")
	case len(want) == 0 && (one || many):
		v.fail(CodeDigestMismatch, "an executor manifest file is present and no executor_id names a manifest")
	case len(want) == 1 && many:
		v.fail(CodeDigestMismatch, "provenance/executor-manifests.json is present and the executor_ids name one manifest")
	case len(want) > 1 && one:
		v.fail(CodeDigestMismatch, "provenance/executor-manifest.json is present and the executor_ids name more than one manifest")
	case len(want) == 1 && !one:
		v.fail(CodePartMissing, "provenance/executor-manifest.json is required: an executor_id names a manifest by digest")
	case len(want) > 1 && !many:
		v.fail(CodePartMissing, "provenance/executor-manifests.json is required: the executor_ids name more than one manifest")
	case one:
		v.checkManifestFile(want[0])
	case many:
		v.checkManifestsFile(want)
	}
}

// checkManifestFile checks the manifest file: its sha256 is the digest the executor_ids name, it is a JSON object already in
// canonical form (one digest, one document) and its "name" is the name before "@" in every executor_id that names it.
func (v *verifier) checkManifestFile(want string) {
	data, err := v.b.read(pathExecutorManifest)
	if err != nil || sha256Digest(data) != want {
		v.fail(CodeDigestMismatch, "provenance/executor-manifest.json is not the manifest the executor_ids name")
		return
	}
	n, err := parseJSON(data)
	if err != nil || n.kind != nObject || !bytes.Equal(canonical(n.toAny()), data) {
		v.fail(CodeDigestMismatch, "provenance/executor-manifest.json is not a JSON object in canonical form")
		return
	}
	var name string
	if m := n.members["name"]; m != nil && m.kind == nString {
		name = m.str
	}
	for _, r := range v.ch.records {
		if nm, d, ok := executorIDParts(r.executor); ok && d == want && nm != name {
			v.fail(CodeDigestMismatch, "the executor_id names the manifest "+nm+" and provenance/executor-manifest.json is named "+name)
			return
		}
	}
}
