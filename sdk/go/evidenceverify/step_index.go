package evidenceverify

import "errors"

type entryDoc struct {
	path, sha256 string
	size         uint64
}

type indexDoc struct {
	changeID string
	entries  []entryDoc
	absent   []Part
}

func entriesOf(items []*node) []entryDoc {
	out := make([]entryDoc, 0, len(items))
	for _, it := range items {
		out = append(out, entryDoc{it.s("path"), it.s("sha256"), it.u("size")})
	}
	return out
}

func decodeIndex(data []byte) (*indexDoc, error) {
	n, err := decodeDoc(data, maxDoc, indexFields())
	if err != nil {
		return nil, err
	}
	d := &indexDoc{changeID: n.s("change_id"), entries: entriesOf(n.list("entries"))}
	if len(d.entries) == 0 {
		return nil, errors.New("an index lists at least one file")
	}
	seen := map[string]bool{}
	for _, e := range d.entries {
		if e.path == pathIndex || e.path == pathManifest || seen[e.path] {
			return nil, errors.New("an index lists itself, the manifest or a path twice")
		}
		seen[e.path] = true
	}
	for _, a := range n.list("absent") {
		p := Part{Part: a.s("part"), Reason: a.s("reason"), Owner: a.s("owner"), NotApplicable: a.flag("not_applicable")}
		d.absent = append(d.absent, p)
	}
	return d, nil
}

// stepIndex decodes the index and compares every file with its entry.
func (v *verifier) stepIndex() {
	data, err := v.b.read(pathIndex)
	if err == nil {
		v.hashes[pathIndex] = fileHash{sha256Digest(data), uint64(len(data))}
	}
	if err != nil {
		v.fail(CodeIndexInvalid, "export-index.json is missing or cannot be read")
		return
	}
	d, err := decodeIndex(data)
	if err != nil {
		v.fail(CodeIndexInvalid, "export-index.json does not decode: "+err.Error())
		return
	}
	v.idx = d
	for _, e := range d.entries {
		v.listed[e.path] = true
		if !v.b.has(e.path) {
			v.fail(CodeFileMissing, "a listed file is not in the directory")
			continue
		}
		body, err := v.b.read(e.path)
		if err != nil {
			v.fail(CodeDigestMismatch, "a listed file cannot be read to be hashed")
			continue
		}
		h := fileHash{sha256Digest(body), uint64(len(body))}
		v.hashes[e.path] = h
		if h.size != e.size || h.digest != e.sha256 {
			v.fail(CodeDigestMismatch, "a file differs from its index entry")
		}
	}
	for _, name := range knownPaths() {
		if name != pathIndex && name != pathManifest && v.b.has(name) && !v.listed[name] {
			v.fail(CodeFileUnlisted, "a known file is not listed in the index")
		}
	}
}

type summaryDoc struct {
	changeID      string
	records       uint64
	head          string
	denied        uint64
	ended, sealed bool
	terminalState string
}

func (v *verifier) stepSummary() {
	data, err := v.b.read(pathSummary)
	if err != nil {
		v.fail(CodeSummaryInvalid, "summary.json is missing or cannot be read")
		return
	}
	n, err := decodeDoc(data, maxDoc, summaryFields())
	if err == nil {
		switch {
		case n.u("last_sequence") != n.u("records"):
			err = errors.New("last_sequence is not records")
		case n.flag("ended") != n.has("terminal_state"):
			err = errors.New("ended and terminal_state disagree")
		case n.flag("sealed") && !n.flag("ended"):
			err = errors.New("a sealed chain has ended")
		}
	}
	if err != nil {
		v.fail(CodeSummaryInvalid, "summary.json does not decode: "+err.Error())
		return
	}
	v.sum = &summaryDoc{changeID: n.s("change_id"), records: n.u("records"), head: n.s("head_record_hash"), denied: n.u("denied"),
		ended: n.flag("ended"), sealed: n.flag("sealed"), terminalState: n.s("terminal_state")}
}

// stepSummaryCompare compares the summary with what the records give.
func (v *verifier) stepSummaryCompare() {
	if v.ch == nil || v.sum == nil {
		return
	}
	c := v.ch
	var denied uint64
	for _, r := range c.records {
		if r.outcome == "denied" {
			denied++
		}
	}
	state := ""
	if c.ended {
		state = terminalState(c.terminal.action)
	}
	if v.sum.records != uint64(len(c.records)) || v.sum.head != c.last.hash || v.sum.denied != denied ||
		v.sum.ended != c.ended || v.sum.sealed != c.sealed || v.sum.terminalState != state {
		v.fail(CodeSummaryMismatch, "the summary disagrees with what the records give")
	}
}

type signersDoc struct {
	keys map[string][]byte
}

// stepSigners decodes provenance/signers.json and, in --trust-bundle-keys mode,
// takes its keys as the trusted set.
func (v *verifier) stepSigners() {
	if !v.b.has(pathSigners) {
		if v.opts.TrustBundleKeys {
			v.fail(CodeSignersInvalid, "provenance/signers.json is missing and it is the trust source")
		}
		return
	}
	data, err := v.b.read(pathSigners)
	var n *node
	if err == nil {
		n, err = decodeDoc(data, maxDoc, signersFields())
	}
	keys := map[string][]byte{}
	if err == nil {
		if len(n.list("signers")) == 0 {
			err = errors.New("no signers")
		}
		for _, s := range n.list("signers") {
			raw := decodeBase64(s.s("public_key"))
			if keyID(raw) != s.s("key_id") {
				err = errors.New("a key_id is not the digest of its key")
				break
			}
			keys[s.s("key_id")] = raw
		}
	}
	if err != nil {
		v.fail(CodeSignersInvalid, "provenance/signers.json does not decode: "+err.Error())
		return
	}
	v.signers = &signersDoc{keys: keys}
	if v.opts.TrustBundleKeys {
		for id, raw := range keys {
			v.trusted[id] = raw
		}
	}
}

// stepDiff compares diff.digest with the execution.diff record.
func (v *verifier) stepDiff() {
	if v.ch == nil {
		return
	}
	want := ""
	for i := len(v.ch.records) - 1; i >= 0 && want == ""; i-- {
		r := v.ch.records[i]
		if r.action != "execution.diff" {
			continue
		}
		for _, o := range r.outputs {
			if o.ref == "execution/diff" {
				want = o.sha256
				break
			}
		}
	}
	has := v.b.has(pathDiff)
	switch {
	case want == "" && has:
		v.fail(CodeDiffMismatch, "diff.digest is present and the chain has no diff")
	case want != "" && !has:
		v.fail(CodeDiffMismatch, "the chain has a diff and diff.digest is missing")
	case want != "":
		data, err := v.b.read(pathDiff)
		if err != nil || string(data) != want+"\n" {
			v.fail(CodeDiffMismatch, "diff.digest is not the digest of the execution.diff record")
		}
	}
}
