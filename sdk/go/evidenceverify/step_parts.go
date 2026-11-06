package evidenceverify

import "strings"

// stepParts checks that every part of the design list is held or declared
// absent, and that no part is both (bundle-layout.md section 3). It needs the
// index.
func (v *verifier) stepParts() {
	if v.idx == nil {
		return
	}
	declared := map[string]Part{}
	for _, a := range v.idx.absent {
		declared[a.Part] = a
	}
	rolledBack := false
	switch {
	case v.ch != nil:
		rolledBack = v.ch.ended && v.ch.terminal.action == actionRolledBack
	case v.sum != nil:
		rolledBack = v.sum.terminalState == "RolledBack"
	}
	for _, part := range designParts() {
		if part == "rollback.json" && !rolledBack {
			continue
		}
		held := v.b.has(part)
		if strings.HasSuffix(part, "/") {
			held = v.b.holdsDir(part)
		}
		d, isDeclared := declared[part]
		switch {
		case held && isDeclared:
			v.fail(CodePartContradiction, "a part is both present and declared absent")
		case !held && !isDeclared:
			v.fail(CodePartMissing, "a required part is neither present nor declared absent")
		case isDeclared && d.NotApplicable:
			// Only deployment.json and observation.json can be not applicable, and only when the verified chain shows
			// the change closed at pull-request level (bundle-layout.md section 3). Any other declaration, and any
			// declaration beside a chain that did not verify, is a contradiction and the part is counted as absent.
			if (part == "deployment.json" || part == "observation.json") && v.ch != nil && v.prClosedAtPR() {
				v.rep.NotApplicable = append(v.rep.NotApplicable, d)
				continue
			}
			v.fail(CodePartContradiction, "a part is declared not applicable and the chain does not bear it out")
			v.absent(d)
		case isDeclared:
			v.absent(d)
		}
	}
}

// absent lists a part as absent; under --complete it is also a finding.
func (v *verifier) absent(d Part) {
	v.rep.Absent = append(v.rep.Absent, d)
	if v.opts.Complete {
		v.fail(CodePartsAbsent, "a part is absent and --complete was given")
	}
}

// prClosedAtPR reports whether the last ChangeClosed.v1 record carries a
// lifecycle whose from is merging (derivations.md section 4).
func (v *verifier) prClosedAtPR() bool {
	for i := len(v.ch.records) - 1; i >= 0; i-- {
		r := v.ch.records[i]
		if r.action == actionClosed {
			return r.lc != nil && r.lc.from == "merging"
		}
	}
	return false
}
