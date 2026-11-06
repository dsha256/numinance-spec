package evidenceverify

// stepChange checks change.json against the chain and, when an attestation
// verified, its ticket (derivations.md section 6).
func (v *verifier) stepChange() {
	if v.ch == nil || !v.b.has(pathChange) {
		return
	}
	data, err := v.b.read(pathChange)
	if err != nil {
		v.fail(CodeChangeInvalid, "change.json cannot be read")
		return
	}
	n, err := decodeDoc(data, maxDoc, changeFields())
	if err != nil {
		v.fail(CodeChangeInvalid, "change.json does not decode: "+err.Error())
		return
	}
	if reason := v.compareChange(n); reason != "" {
		v.fail(CodeChangeInvalid, reason)
	}
}

func (v *verifier) compareChange(n *node) string {
	if n.s("change_id") != v.sum.changeID {
		return "change_id is not the chain's"
	}
	var state, plan string
	anyLifecycle := false
	var maxRepair uint64
	var repairs uint64
	authors := map[actorPair]bool{}
	authorRuns := map[string]bool{}
	for _, r := range v.ch.records {
		if r.plan != "" {
			plan = r.plan
		}
		if r.lc != nil {
			anyLifecycle = true
			state = r.lc.to
			if r.lc.to == "repairing" {
				repairs++
			}
			if r.lc.maxRepair > 0 {
				maxRepair = r.lc.maxRepair
			}
			if r.lc.authorRun != "" {
				authorRuns[r.lc.authorRun] = true
			}
		}
		if r.kind == "author-agent" {
			authors[actorPair{r.principal, r.kind}] = true
			authorRuns[r.runID] = true
		}
	}
	if v.ch.ended {
		switch v.ch.terminal.action {
		case actionClosed:
			state = "closed"
		case actionAbandoned:
			state = "abandoned"
		case actionRolledBack:
			state = "rolled_back"
		}
	}
	if n.s("state") != state {
		return "state differs from the chain"
	}
	if n.s("plan_digest") != plan {
		return "plan_digest differs from the chain"
	}
	if n.has("author_kind") && !n.has("author") {
		return "author_kind is set without author"
	}
	if n.has("author") {
		if len(authors) != 1 || !authors[actorPair{n.s("author"), n.s("author_kind")}] {
			return "author differs from the chain"
		}
	}
	for _, run := range n.strs("author_runs") {
		if !authorRuns[run] {
			return "an author run is not named by the chain"
		}
	}
	if !anyLifecycle {
		if n.u("repair_attempts") != 0 || n.u("max_repair_attempts") != 0 {
			return "repair counters are stated for a chain with no lifecycle record"
		}
	} else if n.u("repair_attempts") != repairs || n.u("max_repair_attempts") != maxRepair {
		return "repair counters differ from the chain"
	}
	if v.attVerified && n.s("ticket") != v.att.n.s("ticket") {
		return "ticket differs from the attestation's"
	}
	return ""
}
