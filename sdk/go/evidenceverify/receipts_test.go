package evidenceverify

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func marker(ref string) artifact { return artifact{ref: ref, sha256: markerDigest(ref)} }

func goodIssued() *record {
	return &record{
		action: actionHandleIssue, kind: "broker", outcome: "succeeded", grantID: "grant_05050505050505050505050505050505",
		outputs: []artifact{
			marker("receipt/epoch/1793958000000000000"), marker("receipt/number/1"), marker("receipt/handle/nhref_0123456789ab"), marker("receipt/invocation/inv_07070707070707070707070707070707"),
			marker("receipt/intent/scm.token"), marker("receipt/delivery/header"), marker("receipt/expires/1793958600"),
			{ref: "receipt/scope/0", sha256: markerDigest("a line")},
		},
	}
}

func goodRequests() *record {
	return &record{action: actionRequests, kind: "egress-proxy", outcome: "succeeded", outputs: []artifact{
		marker("receipt/epoch/1793958000000000000"),
		marker("receipt/request/1/allow/allow_list/write/443/inv_07070707070707070707070707070707/api.github.com/201/12/nhref_0123456789ab"),
		marker("receipt/request/2/deny/not_allowed/read/80/inv_07070707070707070707070707070707/h-0123456789abcdef/403/0/none"),
	}}
}

func TestCheckReceiptRules(t *testing.T) {
	tests := []struct {
		name    string
		rec     func() *record
		wantErr bool
	}{
		{"a handle issue", goodIssued, false},
		{"a request batch", goodRequests, false},
		{"not a receipt at all", func() *record { return &record{action: "ChangeClosed.v1", kind: "runner", outcome: "succeeded"} }, false},
		{"the runner authors a receipt action", func() *record { r := goodIssued(); r.kind = "runner"; return r }, true},
		{"the proxy authors a handle receipt", func() *record { r := goodIssued(); r.kind = "egress-proxy"; return r }, true},
		{"a broker authors another action", func() *record { r := goodIssued(); r.action = "ChangeClosed.v1"; return r }, true},
		{"a failed receipt", func() *record { r := goodIssued(); r.outcome = "failed"; return r }, true},
		{"a receipt with an input", func() *record { r := goodIssued(); r.inputs = []artifact{{ref: "a", sha256: "b"}}; return r }, true},
		{"a marker digest that is not the ref's", func() *record { r := goodIssued(); r.outputs[1].sha256 = markerDigest("x"); return r }, true},
		{"a handle instead of a ref", func() *record { r := goodIssued(); r.outputs[1] = marker("receipt/handle/nh_" + "ab"); return r }, true},
		{"a repeated key", func() *record { r := goodIssued(); r.outputs = append(r.outputs, r.outputs[0]); return r }, true},
		{"no grant on a handle receipt", func() *record { r := goodIssued(); r.grantID = ""; return r }, true},
		{"a scope output out of order", func() *record { r := goodIssued(); r.outputs[6].ref = "receipt/scope/1"; return r }, true},
		{"a URL in a destination", func() *record {
			r := goodRequests()
			r.outputs[0] = marker("receipt/request/1/allow/allow_list/write/443/inv_x/a.example/p/201/12/none")
			return r
		}, true},
		{"request numbers that do not ascend", func() *record { r := goodRequests(); r.outputs[2], r.outputs[1] = r.outputs[1], r.outputs[2]; return r }, true},
		{"an overflow", func() *record {
			return &record{action: actionOverflow, kind: "egress-proxy", outcome: "succeeded", outputs: []artifact{
				marker("receipt/overflow/chain_cap"), marker("receipt/epoch/1793958000000000000"), marker("receipt/first/9"), marker("receipt/last/12"), marker("receipt/dropped/4"),
			}}
		}, false},
		{"a restart statement", func() *record {
			return &record{action: actionOverflow, kind: "broker", outcome: "succeeded", outputs: []artifact{
				marker("receipt/overflow/component_restarted"), marker("receipt/epoch/1793958900000000000"), marker("receipt/first/4"), marker("receipt/last/4"), marker("receipt/dropped/0"),
			}}
		}, false},
		{"a restart statement that states a count", func() *record {
			return &record{action: actionOverflow, kind: "broker", outcome: "succeeded", outputs: []artifact{
				marker("receipt/overflow/component_restarted"), marker("receipt/epoch/1793958900000000000"), marker("receipt/first/4"), marker("receipt/last/4"), marker("receipt/dropped/1"),
			}}
		}, true},
		{"a baseline statement", func() *record {
			return &record{action: actionOverflow, kind: "egress-proxy", outcome: "succeeded", outputs: []artifact{
				marker("receipt/overflow/component_started"), marker("receipt/epoch/1793958000000000000"), marker("receipt/first/1"), marker("receipt/last/1"), marker("receipt/dropped/0"),
			}}
		}, false},
		{"a baseline that names another number", func() *record {
			return &record{action: actionOverflow, kind: "egress-proxy", outcome: "succeeded", outputs: []artifact{
				marker("receipt/overflow/component_started"), marker("receipt/epoch/1793958000000000000"), marker("receipt/first/2"), marker("receipt/last/2"), marker("receipt/dropped/0"),
			}}
		}, true},
		{"denials folded past the budget", func() *record {
			return &record{action: actionOverflow, kind: "egress-proxy", outcome: "succeeded", outputs: []artifact{
				marker("receipt/overflow/denied_overflow"), marker("receipt/epoch/1793958000000000000"), marker("receipt/first/40"), marker("receipt/last/40"), marker("receipt/dropped/9000"),
			}}
		}, false},
		{"denials folded, stated by the broker", func() *record {
			return &record{action: actionOverflow, kind: "broker", outcome: "succeeded", outputs: []artifact{
				marker("receipt/overflow/denied_overflow"), marker("receipt/epoch/1793958000000000000"), marker("receipt/first/40"), marker("receipt/last/40"), marker("receipt/dropped/9"),
			}}
		}, true},
		{"a receipt with no epoch", func() *record { r := goodIssued(); r.outputs = r.outputs[1:]; return r }, true},
		{"an overflow that drops more than its range", func() *record {
			return &record{action: actionOverflow, kind: "broker", outcome: "succeeded", outputs: []artifact{
				marker("receipt/overflow/ledger_full"), marker("receipt/epoch/1793958000000000000"), marker("receipt/first/9"), marker("receipt/last/10"), marker("receipt/dropped/3"),
			}}
		}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkReceiptRules(tc.rec()); (err != nil) != tc.wantErr {
				t.Errorf("checkReceiptRules() = %v, want an error: %v", err, tc.wantErr)
			}
		})
	}
}

// A record whose actor_kind the verifier does not know fails: an old verifier meeting a kind added later refuses the export and
// never ignores the record (ADR 0050; after the first tag a new actor kind is contract-breaking for verifiers).
func TestRecordFields_AnUnknownActorKindFailsAndTheKnownReceiptKindsPass(t *testing.T) {
	zeros64 := strings.Repeat("a", 64) // a digest that is not the zero digest
	line := func(kind string) []byte {
		return []byte(`{"action":"ChangeOpened.v1","action_outcome":"succeeded","actor_kind":"` + kind + `","actor_principal":"p","canon_version":2,` +
			`"change_id":"chg_01010101010101010101010101010101","ev_id":"ev_01010101010101010101010101010101","policy_bundle_digest":"sha256:` + zeros64 + `",` +
			`"profile_digest":"sha256:` + zeros64 + `","risk_class":"LOW","sequence":1,"timestamp":"2026-11-06T10:00:00Z"}`)
	}
	for _, kind := range []string{"witness", "engine", "BROKER", "broker "} {
		if _, err := decodeRecord(line(kind)); err == nil {
			t.Errorf("decodeRecord(actor_kind %q) succeeded, want a refusal: the verifier does not know the kind", kind)
		}
	}
	// "broker" and "egress-proxy" are known kinds, but not of a ChangeOpened.v1 record: the receipt rule refuses that.
	for _, kind := range []string{"broker", "egress-proxy"} {
		if _, err := decodeRecord(line(kind)); err == nil {
			t.Errorf("decodeRecord(%s authoring ChangeOpened.v1) succeeded, want a refusal: a component authors receipts only", kind)
		}
	}
	if _, err := decodeRecord(line("runner")); err != nil {
		t.Errorf("decodeRecord(runner) = %v, want it accepted", err)
	}
}

// requestBatch is a request receipt with one request line.
func requestBatch(line string) *record {
	r := goodRequests()
	r.outputs = []artifact{marker("receipt/epoch/1793958000000000000"), marker("receipt/request/" + line)}
	return r
}

// TestCheckReceiptRules_RequestLineInjectedMarker pins the injected marker (ADR 0059): ten fields, or eleven when the last is
// the literal injected on a line whose handle is a ref. It fails if the verifier takes exactly ten fields.
func TestCheckReceiptRules_RequestLineInjectedMarker(t *testing.T) {
	const head = "1/allow/allow_list/write/443/inv_07070707070707070707070707070707/api.github.com/201/12/"
	tests := []struct {
		name    string
		line    string
		wantErr bool
	}{
		{"ten fields with a handle ref", head + "nhref_0123456789ab", false},
		{"ten fields with none", head + "none", false},
		{"allow with injected", head + "nhref_0123456789ab/injected", false},
		{"deny with injected", "1/deny/quota/write/443/inv_x/api.github.com/403/0/nhref_0123456789ab/injected", false},
		{"allow with injected and none", head + "none/injected", true},
		{"deny with injected and none", "1/deny/quota/write/443/inv_x/api.github.com/403/0/none/injected", true},
		{"empty eleventh", head + "nhref_0123456789ab/", true},
		{"capitalised eleventh", head + "nhref_0123456789ab/Injected", true},
		{"other eleventh", head + "nhref_0123456789ab/plain", true},
		{"twelfth field", head + "nhref_0123456789ab/injected/injected", true},
		{"nine fields", "1/allow/allow_list/write/443/inv_x/api.github.com/201/12", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkReceiptRules(requestBatch(tc.line)); (err != nil) != tc.wantErr {
				t.Errorf("checkReceiptRules(%q) = %v, want an error: %v", tc.line, err, tc.wantErr)
			}
		})
	}
}

// TestCheckReceiptRules_RequestLineVectors runs the hand-written line vectors of the conformance kit through the receipt rules,
// so the kit and the verifier cannot drift.
func TestCheckReceiptRules_RequestLineVectors(t *testing.T) {
	b, err := os.ReadFile("../../../conformance/evidence/v1/receipt-request-lines.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Cases []struct {
			Name  string `json:"name"`
			Line  string `json:"line"`
			Valid bool   `json:"valid"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) < 12 {
		t.Fatalf("the kit holds %d cases, want at least 12", len(f.Cases))
	}
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			if err := checkReceiptRules(requestBatch(c.Line)); (err == nil) != c.Valid {
				t.Errorf("checkReceiptRules(%q) = %v, want valid: %v", c.Line, err, c.Valid)
			}
		})
	}
}
