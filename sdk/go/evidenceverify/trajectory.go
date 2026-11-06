package evidenceverify

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strconv"
	"strings"
)

// Trajectory statements (docs/evidence/trajectory.md): the runner's record of how the executor's reported tool use compares with
// the records of the host and the proxy. The rules here are the shape of one record, the derivation of its ev_id from the change,
// its place in the chain (one, before the terminal record) and the report of its result. The verifier does not recompute the
// comparison and does not recompute the file set: it checks the shape and the cited digests, nothing the artifacts hold.

const (
	actionTrajectory = "TrajectoryCompared.v1"
	trajectoryPrefix = "trajectory/"
	maxTrajectoryRun = 64
	maxTrajectoryNum = 1 << 20
)

// trajectoryCounts are the count markers of a statement in the order they appear.
func trajectoryCounts() []string {
	return []string{
		"runs", "files_reported", "files_recorded", "files_unrecorded", "files_unreported",
		"network_reported", "network_recorded", "network_unrecorded", "network_unreported", "commands_reported",
	}
}

// trajectoryReasons are the reasons of an unverifiable result. The first two are caused by the trajectory stream; the rest are
// about what the trusted side holds, and the producer records an attestation gap for them.
func trajectoryReasons() []string {
	return []string{"stream_unreadable", "stream_malformed", "candidate_unreadable", "candidate_malformed", "receipts_incomplete", "too_many_runs"}
}

// trajectoryEvID is the ev_id of the statement of a change: the statement is unique per change, so its id derives from the change id
// alone.
func trajectoryEvID(changeID string) string {
	sum := sha256.Sum256([]byte("numinance.trajectory.v1\n" + changeID))
	return "ev_" + hex.EncodeToString(sum[:16])
}

// trajectoryStatement is a parsed statement.
type trajectoryStatement struct {
	result, reason string
	discrepancies  int
	counts         map[string]int
	// streams are the digests of the streams the statement cites, in order.
	streams []string
}

// parseTrajectory reads the shape of a TrajectoryCompared.v1 record.
func parseTrajectory(r *record) (trajectoryStatement, error) {
	var st trajectoryStatement
	if r.kind != "runner" || r.outcome != "succeeded" || r.lc != nil || r.corrects != "" || len(r.outputs) == 0 {
		return st, errors.New("a trajectory statement is the runner's succeeded record without lifecycle or correction")
	}
	streams, candidates := 0, 0
	for _, in := range r.inputs {
		rest, ok := strings.CutPrefix(in.ref, trajectoryPrefix)
		kind, num, found := strings.Cut(rest, "/")
		if !ok || !found || in.sha256 == "" {
			return st, errors.New("a trajectory input is not trajectory/<stream|candidate>/<i>")
		}
		switch {
		case kind == "stream" && candidates == 0 && num == strconv.Itoa(streams):
			streams++
			st.streams = append(st.streams, in.sha256)
		case kind == "candidate" && num == strconv.Itoa(candidates):
			candidates++
		default:
			return st, errors.New("trajectory inputs are not numbered from 0 in order, streams first")
		}
	}
	if streams > maxTrajectoryRun || candidates > maxTrajectoryRun {
		return st, errors.New("a trajectory statement cites at most 64 streams and candidates")
	}
	rest := r.outputs
	next := func() (key, val string, ok bool) {
		if len(rest) == 0 {
			return "", "", false
		}
		a := rest[0]
		ref, cut := strings.CutPrefix(a.ref, trajectoryPrefix)
		k, v, found := strings.Cut(ref, "/")
		if !cut || !found || v == "" || strings.Contains(v, "/") || a.sha256 != markerDigest(a.ref) {
			return "", "", false
		}
		rest = rest[1:]
		return k, v, true
	}
	key, result, ok := next()
	if !ok || key != "result" {
		return st, errors.New("a trajectory statement starts with its result")
	}
	st.result, st.counts = result, map[string]int{}
	if len(rest) > 0 && strings.HasPrefix(rest[0].ref, trajectoryPrefix+"reason/") {
		_, st.reason, _ = next()
	}
	for _, want := range trajectoryCounts() {
		k, v, ok := next()
		n, err := strconv.Atoi(v)
		if !ok || k != want || err != nil || n < 0 || n > maxTrajectoryNum || strconv.Itoa(n) != v {
			return st, errors.New("trajectory counts are not the fixed list of canonical numbers")
		}
		st.counts[want] = n
		if strings.HasSuffix(want, "unrecorded") || strings.HasSuffix(want, "unreported") {
			st.discrepancies += n
		}
	}
	if len(rest) != 0 {
		return st, errors.New("a trajectory statement has outputs outside its grammar")
	}
	switch {
	case st.result == "corroborated" && st.reason == "" && st.discrepancies == 0:
	case st.result == "mismatch" && st.reason == "" && st.discrepancies > 0:
	case st.result == "unverifiable" && containsString(trajectoryReasons(), st.reason):
	default:
		return st, errors.New("a trajectory result, reason and counts disagree")
	}
	return st, nil
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// checkTrajectoryRules applies the rules of a TrajectoryCompared.v1 record, and nothing to any other.
func checkTrajectoryRules(r *record) error {
	if r.action != actionTrajectory {
		return nil
	}
	_, err := parseTrajectory(r)
	return err
}

// coversStreams reports whether the streams the statement cites are the streams of the chain's execution.stream records, in order. A
// statement is written before the terminal record, and one that a retry replayed over a chain that grew cites fewer runs than the
// chain holds, so its corroboration is of a part. A comparison that stopped at a stream or a candidate cites a prefix, by design.
func (s trajectoryStatement) coversStreams(records []*record) bool {
	var chain []string
	for _, r := range records {
		if r.action != "execution.stream" {
			continue
		}
		for _, o := range r.outputs {
			if o.ref == "execution/stream" {
				chain = append(chain, o.sha256)
				break
			}
		}
	}
	if len(s.streams) > len(chain) {
		return false
	}
	switch s.reason {
	case "stream_unreadable", "stream_malformed", "candidate_unreadable", "candidate_malformed", "too_many_runs":
		return slices.Equal(chain[:len(s.streams)], s.streams)
	}
	return slices.Equal(chain, s.streams)
}

// summary is the line of the report for a statement: the result with its reason, and the number of discrepancies of a mismatch. It
// names no path and no host: the statement holds none.
func (s trajectoryStatement) summary() string {
	switch s.result {
	case "mismatch":
		return "mismatch (" + strconv.Itoa(s.discrepancies) + " discrepancies between the executor's reported tool use and the host and proxy records; a finding to read, signed over by the attestation)"
	case "unverifiable":
		return "unverifiable (" + s.reason + ")"
	}
	return s.result
}
