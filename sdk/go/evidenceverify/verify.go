package evidenceverify

import (
	"context"
	"crypto/ed25519"
	"errors"
)

type fileHash struct {
	digest string
	size   uint64
}

type verifier struct {
	cancelled  func() error // the caller context's Err
	opts       Options
	b          *bundle
	log        *logPin
	maxRecords uint64
	rep        *Report
	abort      error

	trusted map[string]ed25519.PublicKey

	idx     *indexDoc
	hashes  map[string]fileHash // digests of the files, as the index step computed them
	listed  map[string]bool
	sum     *summaryDoc
	signers *signersDoc
	ch      *chain

	att         *attDoc // decoded, signed under a trusted key
	attVerified bool    // and re-derived from the chain
	attProofOK  bool
	manAttProof bool
	headProofOK bool
	validGap    bool
}

// Verify verifies the export directory at dir. It returns a Report, whose
// level is failed when it has a finding, or an *Error when no report exists:
// an invalid option or an empty path (KindUsage), a refused path (KindInvalid), a missing path
// (KindNotFound), an I/O error (KindUnavailable) or an expired deadline
// (KindTimeout).
func Verify(ctx context.Context, dir string, opts Options) (*Report, error) {
	return verify(ctx, dir, opts, maxChain)
}

func verify(ctx context.Context, dir string, opts Options, maxRecords uint64) (*Report, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	if dir == "" {
		// filepath.Clean("") is ".": an empty argument must not verify the working directory.
		return nil, errf(KindUsage, "the directory path is empty")
	}
	b, refusal, err := openBundle(dir)
	if err != nil {
		return nil, err
	}
	defer b.close()
	v := &verifier{cancelled: ctx.Err, opts: opts, b: b, maxRecords: maxRecords, rep: &Report{}, trusted: map[string]ed25519.PublicKey{},
		hashes: map[string]fileHash{}, listed: map[string]bool{}}
	if len(opts.LogKey) > 0 {
		v.log = &logPin{key: opts.LogKey, origin: opts.LogOrigin}
	}
	for _, k := range opts.RunnerKeys {
		v.trusted[keyID(k)] = k
	}
	if refusal != "" {
		v.fail(CodeDirectoryRefused, refusal)
		v.rep.Chain, v.rep.Signatures, v.rep.Anchor, v.rep.Attestation = "not checked", "not checked", "not checked", "not checked"
		return v.finish(), nil
	}
	v.run()
	if v.abort != nil {
		if errors.Is(v.abort, context.DeadlineExceeded) {
			return nil, &Error{Kind: KindTimeout, Msg: "the deadline expired", Err: v.abort}
		}
		return nil, &Error{Kind: KindInternal, Msg: "the verification was cancelled", Err: v.abort}
	}
	return v.finish(), nil
}

func (v *verifier) fail(code, msg string) {
	v.rep.Findings = append(v.rep.Findings, Finding{Code: code, Message: msg})
}

// run applies the checks of verification.md section 2, in order.
func (v *verifier) run() {
	for name := range v.b.files {
		if !isKnownPath(name) {
			v.fail(CodeUnknownFile, "the export holds a file the format does not define")
			break
		}
	}
	v.stepIndex()
	v.stepSummary()
	if v.idx != nil && v.sum != nil && v.idx.changeID != v.sum.changeID {
		v.fail(CodeChangeMismatch, "the index and the summary name different changes")
	}
	v.stepSigners()
	v.stepChain()
	if v.abort != nil {
		return
	}
	v.stepSummaryCompare()
	v.stepDiff()
	v.stepAnchor()
	v.stepViews()
	v.stepToolEvents()
	v.stepExecutorManifest()
	v.stepRoutePerformance()
	v.stepAttestation()
	v.stepChange()
	v.stepManifest()
	v.stepParts()
	v.stepExpect()
}
