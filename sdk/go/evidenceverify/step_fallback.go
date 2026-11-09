package evidenceverify

// The refs and actions of a fallback route record (fallback-and-shadow.md section 2).
const (
	actionRoute        = "execution.route"
	actionStream       = "execution.stream"
	actionDiff         = "execution.diff"
	refConsidered      = "execution/considered"
	refFallbackOf      = "execution/fallback-of"
	refFallbackCause   = "execution/fallback-cause"
	refCandidate       = "execution/candidate"
	causeExecutorFail  = "executor_failed"
	causeProviderUnavl = "provider_unavailable"
)

// fallback is a valid fallback record of the span: the run it opens and the run whose route it leaves.
type fallback struct {
	sequence uint64
	opens    string
	leaves   string
}

// fallbacks returns the valid fallback records of the span, in chain order (fallback-and-shadow.md section 2): an execution.route
// record of the runner with outcome succeeded, a policy_decision_id and a grant_id and no executor_id, whose actor_run_id is a new run
// (no record of lower sequence has it), that carries execution/considered,
// exactly one execution/fallback-of input that is the record_hash of an earlier route record of the span (an execution.route or
// execution.stream record that carries execution/considered) whose actor_run_id is another run, and exactly one
// execution/fallback-cause input that is the sha256 of one of the two causes of the closed set. A record that does not have this shape
// is no fallback record and is not an error.
func fallbacks(span []*record) []fallback {
	routeRun := map[string]string{}
	seen := map[string]bool{}
	var out []fallback
	for _, r := range span {
		opened := seen[r.runID]
		if r.runID != "" {
			seen[r.runID] = true
		}
		if r.action != actionRoute && r.action != actionStream {
			continue
		}
		considered := false
		for _, in := range r.inputs {
			if in.ref == refConsidered {
				considered = true
			}
		}
		if !considered {
			continue
		}
		if r.action == actionRoute {
			if leaves, ok := validFallback(r, routeRun, opened); ok {
				out = append(out, fallback{sequence: r.sequence, opens: r.runID, leaves: leaves})
			}
		}
		routeRun[r.hash] = r.runID
	}
	return out
}

func validFallback(r *record, routeRun map[string]string, runKnown bool) (leaves string, ok bool) {
	if r.outcome != "succeeded" || r.kind != "runner" || r.executor != "" || r.runID == "" || runKnown || r.decisionID == "" || r.grantID == "" {
		return "", false
	}
	var of, cause int
	for _, in := range r.inputs {
		switch in.ref {
		case refFallbackOf:
			run, known := routeRun[in.sha256]
			if !known || run == "" || run == r.runID {
				return "", false
			}
			leaves = run
			of++
		case refFallbackCause:
			if in.sha256 != sha256Digest([]byte(causeExecutorFail)) && in.sha256 != sha256Digest([]byte(causeProviderUnavl)) {
				return "", false
			}
			cause++
		}
	}
	return leaves, of == 1 && cause == 1
}

// finalCandidate returns the run whose succeeded execution.diff record finalized the candidate the latest verdict passes, by the rule
// of ADR 0049 decision 5: after the verdict chain rule is applied and the forked candidates are left out, the newest verdict by
// sequence must be a pass; and no succeeded execution.diff record for another candidate may follow the receipt of that candidate.
func finalCandidate(span []*record) (run string, ok bool) {
	type state struct {
		prev  string
		forks bool
	}
	type verdict struct{ candidate, status string }
	by := map[string]*state{}
	var all []verdict
	for _, r := range span {
		e, subject, isVerdict := verdictOf(r)
		if !isVerdict {
			continue
		}
		overrules, _ := e["overrules"].(string)
		if overrules == "" {
			overrules = zeroDigest
		}
		s := by[subject]
		if s == nil {
			s = &state{prev: zeroDigest}
			by[subject] = s
		}
		if overrules != s.prev {
			s.forks = true
		}
		s.prev = r.hash
		all = append(all, verdict{subject, e["status"].(string)})
	}
	var latest *verdict
	for i := range all {
		if !by[all[i].candidate].forks {
			latest = &all[i]
		}
	}
	if latest == nil || latest.status != "pass" {
		return "", false
	}
	var receipt uint64
	for _, r := range span {
		if r.action != actionDiff || r.outcome != "succeeded" {
			continue
		}
		for _, o := range r.outputs {
			if o.ref == refCandidate && o.sha256 == latest.candidate {
				run, receipt = r.runID, r.sequence
			}
		}
	}
	if run == "" {
		return "", false
	}
	for _, r := range span {
		if r.action != actionDiff || r.outcome != "succeeded" || r.sequence <= receipt {
			continue
		}
		for _, o := range r.outputs {
			if o.ref == refCandidate && o.sha256 != latest.candidate {
				return "", false
			}
		}
	}
	return run, true
}

// fallbackBinding resolves the executor, model and sandbox image an attestation binds when the author records disagree and the span
// holds a fallback (ADR 0063 decision 5). The winning run is the run the last fallback record opens, and it must be the run whose
// succeeded execution.diff finalized the candidate the latest verdict passes (finalCandidate). Every other run that has an
// author-agent record stating an executor_id must have a record with outcome failed, no succeeded execution.diff and be left by a valid
// fallback record after that failure whose execution/fallback-of leads to that run's route record. ok is false when any of it does not
// hold, and the disagreement stands.
func fallbackBinding(span []*record) (executor, model, image string, ok bool) {
	fbs := fallbacks(span)
	if len(fbs) == 0 {
		return "", "", "", false
	}
	winner, ok := finalCandidate(span)
	if !ok || winner != fbs[len(fbs)-1].opens {
		return "", "", "", false
	}
	type run struct {
		executor, model, image string
		failedAt               uint64
		diffed                 bool
	}
	runs := map[string]*run{}
	for _, r := range span {
		if r.kind != "author-agent" {
			continue
		}
		if _, _, named := executorIDParts(r.executor); !named {
			continue
		}
		if r.runID == "" {
			return "", "", "", false
		}
		u := runs[r.runID]
		if u == nil {
			u = &run{}
			runs[r.runID] = u
		}
		u.executor, u.model, u.image = pick(u.executor, r.executor), pick(u.model, r.model), pick(u.image, r.sandboxDigest)
		if r.outcome == "failed" && u.failedAt == 0 {
			u.failedAt = r.sequence
		}
		if r.action == actionDiff && r.outcome == "succeeded" {
			u.diffed = true
		}
	}
	w := runs[winner]
	if w == nil || w.executor == "\x00" || w.model == "\x00" || w.image == "\x00" {
		return "", "", "", false
	}
	for id, u := range runs {
		if id == winner {
			continue
		}
		if u.failedAt == 0 || u.diffed || !leftAfter(fbs, id, u.failedAt) {
			return "", "", "", false
		}
	}
	return w.executor, w.model, w.image, true
}

// leftAfter reports whether a valid fallback record after the sequence leaves the route of run.
func leftAfter(fbs []fallback, run string, after uint64) bool {
	for _, f := range fbs {
		if f.leaves == run && f.sequence > after {
			return true
		}
	}
	return false
}
