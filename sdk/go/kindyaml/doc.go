// Package kindyaml loads and writes the configuration kinds of numinance.kinds.v1
// as YAML. Decode is the only way a kind document enters a Go program: it caps
// the input, parses strict YAML, validates the value against the kind's
// generated JSON Schema, decodes it with protojson and then enforces the
// checks the schema cannot express (see the README).
package kindyaml
