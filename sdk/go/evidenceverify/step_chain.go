package evidenceverify

import (
	"bytes"
	"errors"
)

// chain is the result of walking records.ndjson.
type chain struct {
	records  []*record
	ok       bool
	sealed   bool
	ended    bool
	terminal *record // the terminal record: the last one, or the one before the seal
	last     *record
	// trajectory is the statement the chain holds, nil for none.
	trajectory *trajectoryStatement
}

// anchoredHash is the record_hash that a head leaf holds
// (log-and-checkpoints.md section 5).
func (c *chain) anchoredHash() string {
	if c.ended {
		return c.terminal.hash
	}
	return c.last.hash
}

// stepChain walks the chain. It needs summary.json for the change it is told.
func (v *verifier) stepChain() {
	if v.sum == nil {
		return
	}
	data, err := v.b.read(pathRecords)
	if err != nil {
		v.fail(CodeFileMissing, "records.ndjson is missing or cannot be read")
		return
	}
	if len(data) == 0 {
		v.fail(CodeChainBroken, "the chain has no records")
		return
	}
	c := &chain{}
	seen := map[string]bool{}
	hashes := map[string]bool{}
	afterTerminal, sealedDone := false, false
	var statement *trajectoryStatement
	prev := ""
	for pos, count := 0, uint64(0); pos < len(data); count++ {
		number := count + 1
		if err := v.cancelled(); err != nil {
			v.abort = err
			return
		}
		end := bytes.IndexByte(data[pos:], '\n')
		var line []byte
		if end < 0 {
			line, pos = data[pos:], len(data)
		} else {
			line, pos = data[pos:pos+end], pos+end+1
		}
		if number > v.maxRecords {
			v.fail(CodeTooManyRecords, "the chain holds more records than the bound")
			return
		}
		if len(line) > maxRecordLine {
			v.fail(CodeRecordInvalid, "a record line is over the size bound")
			return
		}
		r, err := decodeRecord(line)
		if err != nil {
			v.fail(CodeRecordInvalid, "a record is not a valid canonical record: "+err.Error())
			return
		}
		if err := v.chainRules(r, number, prev, seen, hashes, afterTerminal, sealedDone, statement != nil); err != nil {
			v.fail(CodeChainBroken, err.Error())
			return
		}
		if code, msg := v.checkSignature(r); code != "" {
			v.fail(code, msg)
			return
		}
		switch {
		case r.action == actionSealed:
			sealedDone = true
		case isTerminalAction(r.action):
			afterTerminal = true
		}
		if r.action == actionTrajectory {
			st, _ := parseTrajectory(r) // the shape was checked by decodeRecord
			statement = &st
		}
		seen[r.evID], hashes[r.hash] = true, true
		prev = r.hash
		c.records = append(c.records, r)
	}
	c.last = c.records[len(c.records)-1]
	c.trajectory = statement
	c.sealed = c.last.action == actionSealed
	switch {
	case c.sealed:
		c.ended, c.terminal = true, c.records[len(c.records)-2]
	case isTerminalAction(c.last.action):
		c.ended, c.terminal = true, c.last
	}
	c.ok = true
	v.ch = c
}

// errHashMismatch is the last rule of chainRules: a record that passed every rule before it and does not match its record_hash.
var errHashMismatch = errors.New("a record does not match its record_hash")

func (v *verifier) chainRules(r *record, n uint64, prev string, seen, hashes map[string]bool, afterTerminal, sealedDone, statementSeen bool) error {
	switch {
	case r.hash == "":
		return errors.New("a record has no record_hash")
	case sealedDone:
		return errors.New("a record follows the seal record")
	case r.action == actionTrajectory && (afterTerminal || sealedDone):
		return errors.New("a trajectory statement does not precede the terminal record")
	case r.action == actionTrajectory && statementSeen:
		return errors.New("a change has at most one trajectory statement")
	case r.action == actionTrajectory && r.evID != trajectoryEvID(r.changeID):
		return errors.New("the ev_id of a trajectory statement is not the one derived from its change")
	case afterTerminal && r.action != actionSealed:
		return errors.New("a record other than the seal record follows the terminal record")
	case r.changeID != v.sum.changeID:
		return errors.New("a record names another change")
	case seen[r.evID]:
		return errors.New("an ev_id repeats")
	case r.corrects != "" && !hashes[r.corrects]:
		return errors.New("a correction names no earlier record")
	case r.sequence != n || r.previous != prev:
		return errors.New("a sequence or link is wrong")
	case sha256Hex(r.hashedBytes) != r.hash:
		return errHashMismatch
	case r.action == actionSealed && !afterTerminal:
		return errors.New("a seal record does not follow a terminal record")
	case r.action == actionSealed && (len(r.outputs) != 1 || len(r.inputs) != 0 || r.corrects != ""):
		return errors.New("a seal record is not one output, no input and no correction")
	}
	return nil
}

func sha256Hex(d [32]byte) string { return "sha256:" + hexString(d[:]) }

// checkSignature applies the key mode to a record (verification.md section 1).
func (v *verifier) checkSignature(r *record) (code, msg string) {
	if v.opts.SkipSignatures {
		return "", ""
	}
	if len(r.signature) == 0 {
		return CodeSignatureInvalid, "a record is not signed"
	}
	pub, ok := v.trusted[r.signingKeyID]
	if !ok {
		return CodeSignerUntrusted, "a record names a key that is not trusted"
	}
	if !verifySigned(pub, ctxRecord, r.signingDigest, r.signature) {
		return CodeSignatureInvalid, "a record signature does not verify"
	}
	return "", ""
}
