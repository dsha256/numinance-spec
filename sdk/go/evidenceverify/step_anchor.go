package evidenceverify

// stepAnchor checks the head checkpoint and proof (verification.md section 2,
// step 6). Without a pinned log they are neither decoded nor checked: they
// must only come as a pair.
func (v *verifier) stepAnchor() {
	hasCP, hasProof := v.b.has(pathHeadCP), v.b.has(pathHeadProof)
	if hasCP != hasProof {
		v.fail(CodeAnchorInvalid, "the head checkpoint and the head proof must come together")
		return
	}
	if v.log == nil {
		return
	}
	if !hasCP {
		v.fail(CodeAnchorMissing, "a log is pinned and the export holds no head proof")
		return
	}
	if v.ch == nil {
		return
	}
	cp, p, err := v.readAnchor(pathHeadCP, pathHeadProof)
	if err == nil {
		raw, _ := rawDigest(v.ch.anchoredHash())
		err = v.log.verifyLeaf(cp, p, headLeaf(leafHead, raw))
	}
	if err != nil {
		v.fail(CodeAnchorInvalid, "the head proof does not verify under the pinned log: "+err.Error())
		return
	}
	v.headProofOK = true
}

func (v *verifier) readAnchor(cpPath, proofPath string) (*checkpoint, *proof, error) {
	cpData, err := v.b.read(cpPath)
	if err != nil {
		return nil, nil, err
	}
	cp, err := decodeCheckpoint(cpData)
	if err != nil {
		return nil, nil, err
	}
	pData, err := v.b.read(proofPath)
	if err != nil {
		return nil, nil, err
	}
	p, err := decodeProof(pData)
	if err != nil {
		return nil, nil, err
	}
	return cp, p, nil
}
