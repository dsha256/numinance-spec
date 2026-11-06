package evidenceverify

// stepExpect applies --expect-change and --expect-head after the chain
// verified (verification.md section 8).
func (v *verifier) stepExpect() {
	if v.ch == nil {
		return
	}
	if want := v.opts.ExpectChange; want != "" && want != v.sum.changeID {
		v.fail(CodeExpectationMismatch, "the change is not the one expected")
	}
	if want := v.opts.ExpectHead; want != "" && want != v.ch.last.hash && want != v.ch.anchoredHash() {
		v.fail(CodeExpectationMismatch, "the head is not the one expected")
	}
}
