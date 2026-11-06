package evidenceverify

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// Receipt records (docs/evidence/receipts.md): a record of one of four actions authored by the broker or the egress proxy. The
// rules here are the shape of one record; they re-derive nothing from other records.

const (
	maxRequestLines   = 256
	maxScopeLines     = 256
	receiptRefPrefix  = "receipt/"
	actionHandleIssue = "HandleIssued.v1"
	actionHandleRevok = "HandleRevoked.v1"
	actionRequests    = "NetworkRequests.v1"
	actionOverflow    = "ReceiptsOverflow.v1"
)

var (
	receiptTokenRe  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	receiptHandleRe = regexp.MustCompile(`^nhref_[0-9a-f]{12}$`)
	receiptRuleRe   = regexp.MustCompile(`^[a-z_]{1,32}$`)
	receiptDestRe   = regexp.MustCompile(`^[A-Za-z0-9.-]{1,128}$`)
	receiptStepRe   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	receiptInvRe    = regexp.MustCompile(`^inv_[0-9a-f]{32}$`)
)

func isReceiptAction(a string) bool {
	return a == actionHandleIssue || a == actionHandleRevok || a == actionRequests || a == actionOverflow
}

func isReceiptKind(k string) bool { return k == "broker" || k == "egress-proxy" }

func receiptAuthorOK(action, kind string) bool {
	switch action {
	case actionHandleIssue, actionHandleRevok:
		return kind == "broker"
	case actionRequests:
		return kind == "egress-proxy"
	case actionOverflow:
		return isReceiptKind(kind)
	}
	return false
}

func markerDigest(ref string) string {
	sum := sha256.Sum256([]byte(ref))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func positive(s string) bool {
	n, err := strconv.ParseUint(s, 10, 64)
	return err == nil && n >= 1 && n <= maxSafe && strconv.FormatUint(n, 10) == s
}

// positiveEpoch reports whether s is an instance epoch: the instant a component began recording, in Unix nanoseconds, as a
// canonical positive integer. It is a token and not a JSON number, so it is not bound by 2^53-1.
func positiveEpoch(s string) bool {
	n, err := strconv.ParseInt(s, 10, 64)
	return err == nil && n >= 1 && strconv.FormatInt(n, 10) == s
}

// checkReceiptRules applies the receipt rules to a record that has one of the four actions or a component kind.
func checkReceiptRules(r *record) error {
	isAction, isKind := isReceiptAction(r.action), isReceiptKind(r.kind)
	if !isAction && !isKind {
		return nil
	}
	switch {
	case isAction != isKind || !receiptAuthorOK(r.action, r.kind):
		return errors.New("a receipt action and its component author go together")
	case r.outcome != "succeeded" || r.lc != nil || len(r.inputs) != 0 || r.corrects != "":
		return errors.New("a receipt record is a succeeded record without inputs, lifecycle or correction")
	}
	kv := map[string]string{}
	var requests []string
	scopes := 0
	for _, a := range r.outputs {
		rest, ok := strings.CutPrefix(a.ref, receiptRefPrefix)
		key, tok, found := strings.Cut(rest, "/")
		if !ok || !found || tok == "" {
			return errors.New("a receipt output is not receipt/<key>/<value>")
		}
		if key == "scope" {
			if tok != strconv.Itoa(scopes) {
				return errors.New("receipt scope outputs are not numbered from 0 in order")
			}
			scopes++
			continue
		}
		if a.sha256 != markerDigest(a.ref) {
			return errors.New("a receipt output digest is not the digest of its ref")
		}
		if key == "request" {
			requests = append(requests, tok)
			continue
		}
		if _, dup := kv[key]; dup {
			return errors.New("a receipt output key repeats")
		}
		kv[key] = tok
	}
	switch r.action {
	case actionHandleIssue, actionHandleRevok:
		return checkHandleReceipt(r, kv, scopes, len(requests))
	case actionRequests:
		return checkRequestReceipt(r, kv, scopes, requests)
	}
	return checkOverflowReceipt(r, kv, scopes, len(requests))
}

func checkHandleReceipt(r *record, kv map[string]string, scopes, requests int) error {
	issued := r.action == actionHandleIssue
	keys := []string{"epoch", "number", "handle", "invocation", "reason"}
	if issued {
		keys = []string{"epoch", "number", "handle", "invocation", "intent", "delivery", "expires"}
	}
	if len(kv) != len(keys) || requests != 0 || scopes > maxScopeLines || (!issued && scopes != 0) || r.grantID == "" {
		return errors.New("a handle receipt does not have the keys of its action")
	}
	for _, k := range keys {
		if _, ok := kv[k]; !ok {
			return errors.New("a handle receipt does not have the keys of its action")
		}
	}
	if !positiveEpoch(kv["epoch"]) || !positive(kv["number"]) || !receiptHandleRe.MatchString(kv["handle"]) || !receiptInvRe.MatchString(kv["invocation"]) {
		return errors.New("a handle receipt needs a number, a handle ref and an invocation")
	}
	if !issued {
		if !receiptRuleRe.MatchString(kv["reason"]) {
			return errors.New("a handle receipt reason is not a fixed name")
		}
		return nil
	}
	if !positive(kv["expires"]) || !receiptTokenRe.MatchString(kv["intent"]) || !receiptTokenRe.MatchString(kv["delivery"]) {
		return errors.New("a handle receipt needs an intent, a delivery and an expiry")
	}
	return nil
}

func checkRequestReceipt(r *record, kv map[string]string, scopes int, requests []string) error {
	if len(kv) != 1 || !positiveEpoch(kv["epoch"]) || scopes != 0 || len(requests) == 0 || len(requests) > maxRequestLines || r.grantID != "" || r.decisionID != "" {
		return errors.New("a request receipt holds 1 to 256 request lines and nothing else")
	}
	var prev uint64
	for _, raw := range requests {
		f := strings.Split(raw, "/")
		if len(f) != 10 || !positive(f[0]) {
			return errors.New("a request line is outside the grammar")
		}
		n, _ := strconv.ParseUint(f[0], 10, 64)
		port, errP := strconv.ParseUint(f[4], 10, 16)
		status, errS := strconv.Atoi(f[7])
		size, errB := strconv.ParseInt(f[8], 10, 64)
		handleOK := f[9] == "none" || receiptHandleRe.MatchString(f[9])
		if n <= prev || errP != nil || port > 65535 || errS != nil || status < 0 || status > 999 || errB != nil || size < 0 ||
			(f[1] != "allow" && f[1] != "deny") || !receiptRuleRe.MatchString(f[2]) || !receiptRuleRe.MatchString(f[3]) ||
			!receiptStepRe.MatchString(f[5]) || !receiptDestRe.MatchString(f[6]) || !handleOK {
			return errors.New("a request line is outside the grammar")
		}
		prev = n
	}
	return nil
}

func checkOverflowReceipt(r *record, kv map[string]string, scopes, requests int) error {
	if len(kv) != 5 || scopes != 0 || requests != 0 || r.grantID != "" || r.decisionID != "" || !positiveEpoch(kv["epoch"]) {
		return errors.New("an overflow receipt states a reason, an epoch, first, last and dropped")
	}
	reason := kv["overflow"]
	switch reason {
	case "ledger_full", "chain_cap", "component_restarted", "component_started", "denied_overflow":
	default:
		return errors.New("an overflow receipt states a known reason")
	}
	first, last, dropped := kv["first"], kv["last"], kv["dropped"]
	if !positive(first) || !positive(last) {
		return errors.New("an overflow receipt states a number range")
	}
	f, _ := strconv.ParseUint(first, 10, 64)
	l, _ := strconv.ParseUint(last, 10, 64)
	switch reason {
	case "component_restarted", "component_started":
		if dropped != "0" || f != l || (reason == "component_started" && f != 1) {
			return errors.New("a restart or baseline statement names one number and no count")
		}
		return nil
	case "denied_overflow":
		if r.kind != "egress-proxy" {
			return errors.New("only the egress proxy states denials folded past its budget")
		}
		if !positive(dropped) || f != l {
			return errors.New("a denied_overflow statement names the one number the proxy had reached and a count")
		}
		return nil
	}
	d, _ := strconv.ParseUint(dropped, 10, 64)
	if !positive(dropped) || l < f || d > l-f+1 {
		return errors.New("an overflow receipt count is outside its range")
	}
	return nil
}
