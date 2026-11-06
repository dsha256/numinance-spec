package evidenceverify

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func goodTrajectory(result string, counts ...int) *record {
	r := &record{action: actionTrajectory, kind: "runner", outcome: "succeeded", inputs: []artifact{{ref: "trajectory/stream/0", sha256: "sha256:aa"}}}
	r.outputs = []artifact{marker("trajectory/result/" + result)}
	if result == "unverifiable" {
		r.outputs = append(r.outputs, marker("trajectory/reason/stream_unreadable"))
	}
	for i, k := range trajectoryCounts() {
		n := 0
		if i < len(counts) {
			n = counts[i]
		}
		r.outputs = append(r.outputs, marker("trajectory/"+k+"/"+strconv.Itoa(n)))
	}
	return r
}

func TestCheckTrajectoryRules(t *testing.T) {
	tests := []struct {
		name    string
		rec     func() *record
		wantErr bool
	}{
		{"corroborated", func() *record { return goodTrajectory("corroborated", 1, 1, 1) }, false},
		{"a mismatch with a discrepancy", func() *record { return goodTrajectory("mismatch", 1, 2, 1, 1) }, false},
		{"unverifiable with a reason", func() *record { return goodTrajectory("unverifiable") }, false},
		{"not a trajectory at all", func() *record { return &record{action: "ChangeClosed.v1", kind: "runner", outcome: "succeeded"} }, false},
		{"corroborated with a discrepancy", func() *record { return goodTrajectory("corroborated", 1, 1, 1, 1) }, true},
		{"a mismatch with none", func() *record { return goodTrajectory("mismatch", 1) }, true},
		{"another author", func() *record { r := goodTrajectory("corroborated"); r.kind = "engine"; return r }, true},
		{"a failed outcome", func() *record { r := goodTrajectory("corroborated"); r.outcome = "failed"; return r }, true},
		{"a digest that is not the ref's", func() *record { r := goodTrajectory("corroborated"); r.outputs[0].sha256 = "sha256:00"; return r }, true},
		{"an extra output", func() *record {
			r := goodTrajectory("corroborated")
			r.outputs = append(r.outputs, marker("trajectory/path/docs"))
			return r
		}, true},
		{"a count out of order", func() *record {
			r := goodTrajectory("corroborated")
			r.outputs[1], r.outputs[2] = r.outputs[2], r.outputs[1]
			return r
		}, true},
		{"a count that is not canonical", func() *record {
			r := goodTrajectory("corroborated")
			r.outputs[1] = marker("trajectory/runs/01")
			return r
		}, true},
		{"streams numbered from one", func() *record {
			r := goodTrajectory("corroborated")
			r.inputs[0].ref = "trajectory/stream/1"
			return r
		}, true},
		{"an input of another kind", func() *record {
			r := goodTrajectory("corroborated")
			r.inputs = append(r.inputs, artifact{ref: "trajectory/path/0", sha256: "sha256:aa"})
			return r
		}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkTrajectoryRules(tc.rec()); (err != nil) != tc.wantErr {
				t.Errorf("checkTrajectoryRules() = %v, want an error: %v", err, tc.wantErr)
			}
		})
	}
}

func TestTrajectoryReasons(t *testing.T) {
	for _, reason := range trajectoryReasons() {
		r := goodTrajectory("unverifiable")
		r.outputs[1] = marker("trajectory/reason/" + reason)
		if err := checkTrajectoryRules(r); err != nil {
			t.Errorf("checkTrajectoryRules(unverifiable/%s) = %v, want none", reason, err)
		}
	}
	r := goodTrajectory("unverifiable")
	r.outputs[1] = marker("trajectory/reason/unreadable")
	if err := checkTrajectoryRules(r); err == nil {
		t.Error("checkTrajectoryRules(unverifiable/unreadable) = nil, want an error: the reasons are the closed set")
	}
}

func TestChainRules_Trajectory(t *testing.T) {
	const change = "chg_0123456789abcdef0123456789abcdef"
	v := &verifier{sum: &summaryDoc{changeID: change}}
	statement := func(mod func(*record)) *record {
		r := goodTrajectory("corroborated")
		r.changeID, r.evID, r.sequence, r.hash = change, trajectoryEvID(change), 1, "sha256:aa"
		if mod != nil {
			mod(r)
		}
		return r
	}
	tests := []struct {
		name          string
		rec           *record
		afterTerminal bool
		sealed        bool
		seen          bool
		wantErr       bool
	}{
		{"the derived ev_id before the terminal record", statement(nil), false, false, false, false},
		{"an ev_id that is not the derived one", statement(func(r *record) { r.evID = "ev_00000000000000000000000000000000" }), false, false, false, true},
		{"the ev_id of another change", statement(func(r *record) { r.evID = trajectoryEvID("chg_ffffffffffffffffffffffffffffffff") }), false, false, false, true},
		{"after the terminal record", statement(nil), true, false, false, true},
		{"a second statement", statement(nil), false, false, true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// A statement that passes its own rules goes on to the hash check, which a fixture record does not satisfy: that error
			// is the proof the trajectory rules let it through.
			err := v.chainRules(tc.rec, 1, "", map[string]bool{}, map[string]bool{}, tc.afterTerminal, tc.sealed, tc.seen)
			passed := errors.Is(err, errHashMismatch)
			if passed == tc.wantErr {
				t.Errorf("chainRules() = %v, want the trajectory rules to refuse it: %v", err, tc.wantErr)
			}
		})
	}
}

func TestTrajectoryStatement_Summary(t *testing.T) {
	for _, tc := range []struct {
		rec  *record
		want string
	}{
		{goodTrajectory("corroborated"), "corroborated"},
		{goodTrajectory("mismatch", 1, 2, 1, 1), "mismatch (1 discrepancies"},
		{goodTrajectory("unverifiable"), "unverifiable (stream_unreadable)"},
	} {
		st, err := parseTrajectory(tc.rec)
		if err != nil {
			t.Fatal(err)
		}
		if got := st.summary(); !strings.HasPrefix(got, tc.want) {
			t.Errorf("summary() = %q, want it to start with %q", got, tc.want)
		}
	}
}

func TestTrajectoryStatement_CoversStreams(t *testing.T) {
	run := func(d string) *record {
		return &record{action: "execution.stream", outputs: []artifact{{ref: "execution/stream", sha256: d}}}
	}
	statement := func(reason string, streams ...string) trajectoryStatement {
		return trajectoryStatement{reason: reason, streams: streams}
	}
	tests := []struct {
		name  string
		st    trajectoryStatement
		chain []*record
		want  bool
	}{
		{"every run is cited", statement("", "a", "b"), []*record{run("a"), run("b")}, true},
		{"a run landed after the statement", statement("", "a"), []*record{run("a"), run("b")}, false},
		{"a cited stream is not the chain's", statement("", "a", "c"), []*record{run("a"), run("b")}, false},
		{"a comparison that stopped at a stream cites a prefix", statement("stream_unreadable", "a"), []*record{run("a"), run("b")}, true},
		{"a stopped comparison citing what the chain lacks", statement("stream_malformed", "x"), []*record{run("a")}, false},
		{"no runs and none cited", statement(""), nil, true},
	}
	for _, tc := range tests {
		if got := tc.st.coversStreams(tc.chain); got != tc.want {
			t.Errorf("%s: coversStreams() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
