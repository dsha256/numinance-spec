// Package evidenceverify is the open verifier of an evidence/v1 export
// directory.
//
// It is written from the public specification (docs/evidence), the JSON
// Schemas (schemas/evidence/v1) and the conformance vectors
// (conformance/evidence/v1) only. It uses the standard library only, opens no
// network connection, writes nothing and bounds every read.
//
// The caller chooses the trust: there is no built-in trust root. A result
// reaches "verified" only through runner keys and a log key and origin that
// the caller pins; a key stated by the export never does.
package evidenceverify
