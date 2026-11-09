package evidenceverify

import (
	"math"
)

// refRoutePerformance is the ref of the optional input that names a performance snapshot by the sha256 of its canonical document
// (route-performance.md).
const refRoutePerformance = "execution/performance"

const (
	maxSnapshots       = 4096
	maxSnapshotMembers = 256
)

func routePerformanceFields() []field {
	return []field{
		uintf("export_version", true, 1, 1),
		reqArr("snapshots", maxSnapshots, objf("", true, snapshotFields())),
	}
}

// reqArr is an array that must be written, even when it is empty.
func reqArr(name string, maxItems int, elem field) field {
	a := arrf(name, maxItems, elem)
	a.req = true
	return a
}

func snapshotFields() []field {
	return []field{
		uintf("as_of", true, 0, maxSafe),
		reqArr("candidates", maxSnapshotMembers, objf("", true, snapshotCandidateFields())),
		str("language", true, printable256),
		uintf("n", true, 1, math.MaxUint32),
		str("task_class", true, printable256),
	}
}

func snapshotCandidateFields() []field {
	return []field{
		str("executor", true, nonEmptyText(256)),
		objf("figure", false, []field{
			uintf("accepted", true, 0, math.MaxUint32),
			uintf("n", true, 1, math.MaxUint32),
		}),
		str("model", true, nonEmptyText(256)),
		uintf("rank", true, 1, math.MaxUint32),
	}
}

// routePerformanceDigests returns the distinct digests the records name as execution/performance inputs, in first-seen order.
func (v *verifier) routePerformanceDigests() []string {
	var want []string
	seen := map[string]bool{}
	for _, r := range v.ch.records {
		for _, in := range r.inputs {
			if in.ref == refRoutePerformance && !seen[in.sha256] {
				seen[in.sha256] = true
				want = append(want, in.sha256)
			}
		}
	}
	return want
}

// stepRoutePerformance checks provenance/route-performance.json against the execution/performance inputs of the chain: the file is
// present exactly when a record names a snapshot, holds one canonical document for every digest named and no other
// (route-performance.md section 3).
func (v *verifier) stepRoutePerformance() {
	if v.ch == nil {
		return
	}
	want := v.routePerformanceDigests()
	have := v.b.has(pathRoutePerformance)
	switch {
	case len(want) == 0 && have:
		v.fail(CodeDigestMismatch, "provenance/route-performance.json is present and no record names a performance snapshot")
	case len(want) > 0 && !have:
		v.fail(CodePartMissing, "provenance/route-performance.json is required: a record names a performance snapshot by digest")
	case have:
		v.checkRoutePerformanceFile(want)
	}
}

// checkRoutePerformanceFile decodes the file (shape, canonical form), takes the sha256 of the canonical form of each snapshot and
// compares the set with the digests the records name. It does not check that the candidates are in any order: the ranking is the
// router's, and the file only shows what it read.
func (v *verifier) checkRoutePerformanceFile(want []string) {
	data, err := v.b.read(pathRoutePerformance)
	if err != nil {
		v.fail(CodeDigestMismatch, "provenance/route-performance.json cannot be read")
		return
	}
	n, err := decodeDoc(data, maxDoc, routePerformanceFields())
	if err != nil {
		v.fail(CodeDigestMismatch, "provenance/route-performance.json is not a canonical performance document")
		return
	}
	haveSet := map[string]bool{}
	var have []string
	for _, snap := range n.list("snapshots") {
		if !snapshotFiguresOK(snap) {
			v.fail(CodeDigestMismatch, "provenance/route-performance.json holds a figure with more accepted outcomes than outcomes")
			return
		}
		d := sha256Digest(canonical(snap.toAny()))
		if haveSet[d] {
			v.fail(CodeDigestMismatch, "provenance/route-performance.json holds one snapshot twice")
			return
		}
		haveSet[d] = true
		have = append(have, d)
	}
	wantSet := make(map[string]bool, len(want))
	for _, d := range want {
		wantSet[d] = true
	}
	for _, d := range have {
		if !wantSet[d] {
			v.fail(CodeDigestMismatch, "provenance/route-performance.json holds a snapshot that no record names")
			return
		}
	}
	for _, d := range want {
		if !haveSet[d] {
			v.fail(CodePartMissing, "provenance/route-performance.json lacks a snapshot that a record names")
			return
		}
	}
}

// snapshotFiguresOK reports whether every figure of the snapshot has no more accepted outcomes than outcomes.
func snapshotFiguresOK(snap *node) bool {
	for _, c := range snap.list("candidates") {
		if f := c.obj("figure"); f != nil && f.u("accepted") > f.u("n") {
			return false
		}
	}
	return true
}
