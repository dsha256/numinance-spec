package evidenceverify

import (
	"bytes"
	"errors"
	"strings"
)

type manifestDoc struct {
	n *node
}

func decodeManifest(data []byte) (*manifestDoc, error) {
	n, err := decodeDoc(data, maxDoc, manifestFields())
	if err != nil {
		return nil, err
	}
	if n.u("first_sequence") > n.u("last_sequence") {
		return nil, errors.New("first_sequence is after last_sequence")
	}
	for _, key := range []string{"inclusion_proof", "attestation_inclusion_proof"} {
		if _, err := proofFromNode(n.obj(key)); err != nil {
			return nil, errors.New(key + " is not a well-formed proof: " + err.Error())
		}
	}
	entries := n.list("entries")
	if len(entries) == 0 {
		return nil, errors.New("a manifest lists at least one file")
	}
	seen := map[string]bool{}
	for _, e := range entries {
		p := e.s("path")
		if p == pathManifest || seen[strings.ToLower(p)] {
			return nil, errors.New("a manifest lists itself or a path twice")
		}
		seen[strings.ToLower(p)] = true
	}
	return &manifestDoc{n: n}, nil
}

// stepManifest checks the signed bundle manifest (attestation-and-manifest.md
// section 4). The signed manifest is checked against a verified chain.
func (v *verifier) stepManifest() {
	has := v.b.has(pathManifest)
	if !has {
		// A change that has an attestation file needs a manifest whether or not its chain verified; so does a Closed
		// change that states no valid attestation gap.
		if v.b.has(pathAttest) || (v.ch != nil && v.endedClosed() && !v.validGap) {
			v.fail(CodeManifestMissing, "the change has an attestation and bundle-manifest.json is not there")
		}
		return
	}
	if v.ch == nil {
		return
	}
	data, err := v.b.read(pathManifest)
	if err != nil {
		v.fail(CodeManifestInvalid, "bundle-manifest.json cannot be read")
		return
	}
	m, err := decodeManifest(data)
	if err != nil {
		v.fail(CodeManifestInvalid, "bundle-manifest.json does not decode: "+err.Error())
		return
	}
	if !v.opts.SkipSignatures {
		pub, ok := v.trusted[m.n.s("signing_key_id")]
		if !ok || !verifySigned(pub, ctxManifest, digestBytes(m.n.canonicalWithout("signature")), decodeBase64(m.n.s("signature"))) {
			v.fail(CodeManifestInvalid, "the manifest is signed by no trusted key or its signature fails")
			return
		}
	}
	if reason := v.manifestMismatch(m); reason != "" {
		v.fail(CodeManifestMismatch, reason)
	}
}

func (v *verifier) manifestMismatch(m *manifestDoc) string {
	n := m.n
	reason := ""
	note := func(s string) {
		if reason == "" {
			reason = s
		}
	}
	// 1. The entries are exactly the files of the directory but the manifest.
	covered := map[string]bool{}
	for _, e := range n.list("entries") {
		p := e.s("path")
		covered[p] = true
		h, hashed := v.hashes[p]
		switch {
		case !v.b.has(p) || !hashed:
			note("a manifest entry names a file that was not found and hashed")
		case h.digest != e.s("sha256") || h.size != e.u("size"):
			note("a manifest entry differs from the file")
		case p != pathIndex && !v.listed[p]:
			note("a manifest entry names a file the index does not list")
		}
	}
	for name := range v.b.files {
		if name != pathManifest && !covered[name] {
			note("a file of the directory is not covered by the manifest")
		}
	}
	// 2. Binding.
	c := v.ch
	if n.s("change_id") != v.sum.changeID || n.u("first_sequence") != 1 || !c.ended ||
		n.u("last_sequence") != c.terminal.sequence || n.s("head_record_hash") != c.anchoredHash() {
		note("the manifest binding differs from the chain")
	}
	if v.att == nil && !v.b.has(pathAttest) {
		note("the manifest has no attestation to bind")
	}
	if v.attVerified {
		a := v.att.n
		if n.s("change_id") != a.s("change_id") || n.u("first_sequence") != a.u("first_sequence") ||
			n.u("last_sequence") != a.u("last_sequence") || n.s("head_record_hash") != a.s("head_record_hash") {
			note("the manifest binding differs from the attestation")
		}
	}
	// 3. Anchor files.
	cpData, cpErr := v.b.read(pathHeadCP)
	pData, pErr := v.b.read(pathHeadProof)
	switch {
	case !v.b.has(pathHeadCP) || !v.b.has(pathHeadProof) || cpErr != nil || pErr != nil:
		note("the head checkpoint or proof is missing")
	default:
		if sha256Digest(cpData) != n.obj("checkpoint_ref").s("sha256") {
			note("the head checkpoint is not the one checkpoint_ref names")
		}
		if !bytes.Equal(pData, canonical(n.obj("inclusion_proof").toAny())) {
			note("the head proof differs from the manifest's inclusion_proof")
		}
	}
	// 4. The attestation leaf under the head checkpoint.
	if v.log != nil && v.attVerified {
		ok := false
		if cpErr == nil {
			if cp, err := decodeCheckpoint(cpData); err == nil {
				if p, err := proofFromNode(n.obj("attestation_inclusion_proof")); err == nil {
					ok = v.log.verifyLeaf(cp, p, headLeaf(leafAttestation, v.att.digest[:])) == nil
				}
			}
		}
		if !ok {
			note("the attestation leaf does not verify under the head checkpoint")
		}
		v.manAttProof = ok
	}
	return reason
}
