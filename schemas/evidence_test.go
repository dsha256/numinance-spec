package schemas_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/numinance/numinance-spec/sdk/go/schema"
)

const (
	evidenceDir     = "evidence/v1"
	evidenceExports = "../conformance/evidence/v1/exports"
	evidenceVectors = "../conformance/evidence/v1/vectors"
)

// evidencePositive are the export directories that verify; every other
// directory under exports/ is deliberately tampered and is not validated.
//
//nolint:gochecknoglobals // immutable test fixture
var evidencePositive = []string{
	"closed-complete", "closed-attested-fields", "closed-sealed", "abandoned", "rolled-back",
	"open-prefix", "closed-gap", "closed-canon-unicode", "closed-canon-timestamps", "closed-basic",
	"closed-ticket-none", "closed-author", "closed-streams", "closed-canon-control-escapes",
	"closed-route-performance", "closed-route-performance-two",
}

// evidenceFiles maps a file of an export to its schema; records.ndjson is
// handled line by line.
//
//nolint:gochecknoglobals // immutable test fixture
var evidenceFiles = map[string]string{
	"attestation.json":            "attestation",
	"bundle-manifest.json":        "bundle-manifest",
	"head-checkpoint.json":        "checkpoint",
	"attestation-checkpoint.json": "checkpoint",
	"head-inclusion-proof.json":   "inclusion-proof",
	"summary.json":                "summary",
	"change.json":                 "change",
	"export-index.json":           "export-index",
	"provenance/signers.json":     "signers",
	"attestation-gap.json":        "attestation-gap",
	"verification/verdicts.json":  "verification",
	"review/review.json":          "review",
	"provenance/provenance.json":  "provenance",

	"provenance/route-performance.json": "route-performance",
}

// evidenceSchemas compiles every schema under evidence/v1, keyed by the file
// name without .schema.json.
func evidenceSchemas(t *testing.T) map[string]*schema.Schema {
	t.Helper()
	files, err := fs.Glob(os.DirFS(evidenceDir), "*.schema.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("Glob(%s/*.schema.json) = %v, %v; want at least one schema", evidenceDir, files, err)
	}
	out := map[string]*schema.Schema{}
	for _, f := range files {
		b, err := os.ReadFile(path.Join(evidenceDir, f)) //nolint:gosec // fixed directory of this repository
		if err != nil {
			t.Fatalf("ReadFile(%s/%s) = %v", evidenceDir, f, err)
		}
		s, err := schema.Compile(b)
		if err != nil {
			t.Errorf("schema.Compile(%s/%s) = %v", evidenceDir, f, err)
			continue
		}
		out[strings.TrimSuffix(f, ".schema.json")] = s
	}
	return out
}

// decodeJSON decodes one JSON value with numbers kept exact, and fails on
// trailing data.
func decodeJSON(b []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, bytes.ErrTooLarge
	}
	return v, nil
}

// TestEvidenceSchemas_SelfContained checks that every evidence schema is
// draft 2020-12, has a root $id under https://numinance.dev/schemas/evidence/v1/
// naming its own file, compiles with the repo validator, and that every $ref
// points into the same file and resolves there.
func TestEvidenceSchemas_SelfContained(t *testing.T) {
	evidenceSchemas(t)
	files, err := fs.Glob(os.DirFS(evidenceDir), "*.json")
	if err != nil {
		t.Fatalf("Glob(%s/*.json) = %v", evidenceDir, err)
	}
	for _, f := range files {
		if !strings.HasSuffix(f, ".schema.json") {
			t.Errorf("%s/%s: files of this directory are named <name>.schema.json", evidenceDir, f)
			continue
		}
		b, err := os.ReadFile(path.Join(evidenceDir, f)) //nolint:gosec // fixed directory of this repository
		if err != nil {
			t.Fatalf("ReadFile(%s/%s) = %v", evidenceDir, f, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Errorf("%s/%s is not JSON: %v", evidenceDir, f, err)
			continue
		}
		if got := doc["$schema"]; got != draft {
			t.Errorf("%s/%s $schema = %v, want %q", evidenceDir, f, got, draft)
		}
		if got, want := doc["$id"], "https://numinance.dev/schemas/evidence/v1/"+f; got != want {
			t.Errorf("%s/%s $id = %v, want %q", evidenceDir, f, got, want)
		}
		for _, k := range []string{"title", "description"} {
			if s, _ := doc[k].(string); s == "" {
				t.Errorf("%s/%s has no %s", evidenceDir, f, k)
			}
		}
		if s, _ := doc["description"].(string); !strings.Contains(s, "json") {
			t.Errorf("%s/%s description does not name the file it describes", evidenceDir, f)
		}
		defs, _ := doc["$defs"].(map[string]any)
		for _, ref := range refs(doc) {
			name, ok := strings.CutPrefix(ref, "#/$defs/")
			if !ok {
				t.Errorf("%s/%s: $ref %q is not local to the file (#/$defs/...)", evidenceDir, f, ref)
				continue
			}
			if _, ok := defs[name]; !ok {
				t.Errorf("%s/%s: $ref %q does not resolve", evidenceDir, f, ref)
			}
		}
	}
}

// objectsClosed returns the JSON Pointer of every schema object that declares
// properties without additionalProperties: false.
func objectsClosed(node any, at string) []string {
	v, ok := node.(map[string]any)
	if !ok {
		return nil
	}
	var open []string
	if props, ok := v["properties"].(map[string]any); ok {
		if ap, ok := v["additionalProperties"].(bool); !ok || ap {
			open = append(open, at)
		}
		for k, e := range props {
			open = append(open, objectsClosed(e, at+"/properties/"+k)...)
		}
	}
	if defs, ok := v["$defs"].(map[string]any); ok {
		for k, e := range defs {
			open = append(open, objectsClosed(e, at+"/$defs/"+k)...)
		}
	}
	if items, ok := v["items"]; ok {
		open = append(open, objectsClosed(items, at+"/items")...)
	}
	if anyOf, ok := v["anyOf"].([]any); ok {
		for _, e := range anyOf {
			open = append(open, objectsClosed(e, at+"/anyOf")...)
		}
	}
	return open
}

// TestEvidenceSchemas_ObjectsAreClosed checks that every object schema refuses
// unknown keys, as the shipped decoders do.
func TestEvidenceSchemas_ObjectsAreClosed(t *testing.T) {
	files, _ := fs.Glob(os.DirFS(evidenceDir), "*.schema.json")
	for _, f := range files {
		b, err := os.ReadFile(path.Join(evidenceDir, f)) //nolint:gosec // fixed directory of this repository
		if err != nil {
			t.Fatalf("ReadFile(%s/%s) = %v", evidenceDir, f, err)
		}
		var doc any
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatalf("Unmarshal(%s/%s) = %v", evidenceDir, f, err)
		}
		if open := objectsClosed(doc, ""); len(open) != 0 {
			t.Errorf("%s/%s: objects without additionalProperties false: %q", evidenceDir, f, open)
		}
	}
}

// TestEvidenceExports_ValidateAgainstSchemas validates every file of every
// positive export against its schema, and checks that every schema is used.
func TestEvidenceExports_ValidateAgainstSchemas(t *testing.T) {
	schemas := evidenceSchemas(t)
	used := map[string]int{}
	check := func(name, what string, b []byte) {
		t.Helper()
		v, err := decodeJSON(b)
		if err != nil {
			t.Errorf("%s: not one JSON value: %v", what, err)
			return
		}
		s, ok := schemas[name]
		if !ok {
			t.Errorf("%s: no schema %s", what, name)
			return
		}
		used[name]++
		if err := s.Validate(v); err != nil {
			t.Errorf("Validate(%s, %s) = %v", what, name, err)
		}
	}
	for _, export := range evidencePositive {
		dir := path.Join(evidenceExports, export)
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("positive export %s: %v", export, err)
			continue
		}
		recs, err := os.ReadFile(path.Join(dir, "records.ndjson")) //nolint:gosec // fixed directory of this repository
		if err != nil {
			t.Errorf("ReadFile(%s/records.ndjson) = %v", export, err)
			continue
		}
		sc := bufio.NewScanner(bytes.NewReader(recs))
		sc.Buffer(nil, 4<<20)
		lines := 0
		for sc.Scan() {
			lines++
			check("record", export+"/records.ndjson:"+strconv.Itoa(lines), sc.Bytes())
		}
		if err := sc.Err(); err != nil || lines == 0 {
			t.Errorf("%s/records.ndjson: %d lines, err %v; want at least one line", export, lines, err)
		}
		for file, name := range evidenceFiles {
			b, err := os.ReadFile(path.Join(dir, file)) //nolint:gosec // fixed directory of this repository
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				t.Errorf("ReadFile(%s/%s) = %v", export, file, err)
				continue
			}
			check(name, export+"/"+file, b)
		}
	}
	for name := range schemas {
		if strings.HasPrefix(name, "vector") {
			continue
		}
		if used[name] == 0 {
			t.Errorf("schema %s is used by no positive export", name)
		}
	}
}

// TestEvidenceVectors_ValidateAgainstSchemas validates every vector's
// expected.json and the vector index.
func TestEvidenceVectors_ValidateAgainstSchemas(t *testing.T) {
	schemas := evidenceSchemas(t)
	expected, index := schemas["vector-expected"], schemas["vectors-index"]
	if expected == nil || index == nil {
		t.Fatal("vector-expected or vectors-index schema is missing")
	}
	dirs, err := os.ReadDir(evidenceVectors)
	if err != nil {
		t.Fatalf("ReadDir(%s) = %v", evidenceVectors, err)
	}
	n := 0
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		b, err := os.ReadFile(path.Join(evidenceVectors, d.Name(), "expected.json"))
		if err != nil {
			t.Errorf("ReadFile(%s/expected.json) = %v", d.Name(), err)
			continue
		}
		v, err := decodeJSON(b)
		if err != nil {
			t.Errorf("%s/expected.json: %v", d.Name(), err)
			continue
		}
		n++
		if err := expected.Validate(v); err != nil {
			t.Errorf("Validate(%s/expected.json) = %v", d.Name(), err)
		}
		if m, _ := v.(map[string]any); m["vector"] != d.Name() {
			t.Errorf("%s/expected.json vector = %v, want the directory name", d.Name(), m["vector"])
		}
	}
	if n == 0 {
		t.Fatalf("no vector directories under %s", evidenceVectors)
	}
	b, err := os.ReadFile(path.Join(evidenceVectors, "index.json"))
	if err != nil {
		t.Fatalf("ReadFile(index.json) = %v", err)
	}
	v, err := decodeJSON(b)
	if err != nil {
		t.Fatalf("index.json: %v", err)
	}
	if err := index.Validate(v); err != nil {
		t.Errorf("Validate(index.json) = %v", err)
	}
	if vs, _ := v.(map[string]any)["vectors"].([]any); len(vs) != n {
		t.Errorf("index.json lists %d vectors, want %d (one per directory)", len(vs), n)
	}
}

// TestEvidenceSchemas_RejectBadDocuments checks that hand-made bad documents
// fail, so a schema that accepts everything cannot pass the tests above.
func TestEvidenceSchemas_RejectBadDocuments(t *testing.T) {
	schemas := evidenceSchemas(t)
	b, err := os.ReadFile(path.Join(evidenceExports, "closed-basic", "records.ndjson"))
	if err != nil {
		t.Fatalf("ReadFile(closed-basic/records.ndjson) = %v", err)
	}
	first, _, _ := bytes.Cut(b, []byte{'\n'})
	var base map[string]any
	dec := json.NewDecoder(bytes.NewReader(first))
	dec.UseNumber()
	if err := dec.Decode(&base); err != nil {
		t.Fatalf("decode first record: %v", err)
	}
	mutate := func(f func(m map[string]any)) any {
		m := map[string]any{}
		for k, v := range base {
			m[k] = v
		}
		f(m)
		return m
	}
	if err := schemas["record"].Validate(mutate(func(map[string]any) {})); err != nil {
		t.Fatalf("Validate(unmodified first record) = %v", err)
	}
	tests := []struct {
		name string
		doc  any
	}{
		{"record with an unknown key", mutate(func(m map[string]any) { m["extra"] = "x" })},
		{"record with an upper-case digest", mutate(func(m map[string]any) {
			m["profile_digest"] = "sha256:" + strings.Repeat("A", 64)
		})},
		{"record with a short digest", mutate(func(m map[string]any) { m["profile_digest"] = "sha256:abc" })},
		{"record with an unknown risk class", mutate(func(m map[string]any) { m["risk_class"] = "SEVERE" })},
		{"record with an unknown actor kind", mutate(func(m map[string]any) { m["actor_kind"] = "root" })},
		{"record with an unknown outcome", mutate(func(m map[string]any) { m["action_outcome"] = "ok" })},
		{"record without a timestamp", mutate(func(m map[string]any) { delete(m, "timestamp") })},
		{"record with a zero sequence", mutate(func(m map[string]any) { m["sequence"] = json.Number("0") })},
		{"record with a sequence above 2^53-1", mutate(func(m map[string]any) { m["sequence"] = json.Number("9007199254740992") })},
		{"record with a timestamp offset", mutate(func(m map[string]any) { m["timestamp"] = "2026-11-01T12:00:01+01:00" })},
		{"record with a trailing zero in the fraction", mutate(func(m map[string]any) { m["timestamp"] = "2026-11-01T12:00:01.50Z" })},
		{"record with a short signature", mutate(func(m map[string]any) { m["signature"] = "AAAA" })},
		{"record with a ref that escapes", mutate(func(m map[string]any) {
			m["outputs"] = []any{map[string]any{"ref": "/etc/passwd", "sha256": "sha256:" + strings.Repeat("0", 64)}}
		})},
		{"record with an artifact with an unknown key", mutate(func(m map[string]any) {
			m["outputs"] = []any{map[string]any{"ref": "a", "sha256": "sha256:" + strings.Repeat("0", 64), "size": json.Number("1")}}
		})},
		{"record with a change id of another prefix", mutate(func(m map[string]any) { m["change_id"] = "ev_" + strings.Repeat("0", 32) })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := schemas["record"].Validate(tt.doc); err == nil {
				t.Errorf("Validate(%s) = nil, want a violation", tt.name)
			}
		})
	}
	others := []struct {
		name   string
		schema string
		json   string
	}{
		{"summary with an unknown terminal state", "summary", `{"change_id":"chg_` + strings.Repeat("0", 32) + `","denied":0,"ended":true,"export_version":1,"first_sequence":1,"head_record_hash":"sha256:` + strings.Repeat("0", 64) + `","last_sequence":1,"records":1,"sealed":false,"terminal_state":"Done"}`},
		{"summary with export_version 2", "summary", `{"change_id":"chg_` + strings.Repeat("0", 32) + `","denied":0,"ended":false,"export_version":2,"first_sequence":1,"head_record_hash":"sha256:` + strings.Repeat("0", 64) + `","last_sequence":1,"records":1,"sealed":false}`},
		{"checkpoint with an unknown key", "checkpoint", `{"canon_version":1,"origin":"o","root_hash":"sha256:` + strings.Repeat("0", 64) + `","tree_size":1,"extra":1}`},
		{"inclusion proof with a negative leaf index", "inclusion-proof", `{"leaf_index":-1,"tree_size":1}`},
		{"signers with a short key", "signers", `{"export_version":1,"signers":[{"key_id":"sha256:` + strings.Repeat("0", 64) + `","public_key":"AAAA"}]}`},
		{"route performance with a candidate that has no rank", "route-performance", `{"export_version":1,"snapshots":[{"as_of":1,"candidates":[{"executor":"a","model":"m"}],"language":"go","n":100,"task_class":"bugfix"}]}`},
		{"route performance with a figure of no outcomes", "route-performance", `{"export_version":1,"snapshots":[{"as_of":1,"candidates":[{"executor":"a","figure":{"accepted":0,"n":0},"model":"m","rank":1}],"language":"go","n":100,"task_class":"bugfix"}]}`},
		{"route performance with an unknown key", "route-performance", `{"export_version":1,"snapshots":[{"as_of":1,"candidates":[],"extra":1,"language":"go","n":100,"task_class":"bugfix"}]}`},
		{"index with the manifest listed", "export-index", `{"change_id":"chg_` + strings.Repeat("0", 32) + `","entries":[{"path":"bundle-manifest.json","sha256":"sha256:` + strings.Repeat("0", 64) + `","size":1}],"export_version":1}`},
		{"vector with an unknown status", "vector-expected", `{"vector":"a","description":"d","status":"draft","flags":{},"keys":[],"expect":{"level":null,"exit_code":0,"finding_codes":[],"target":{"level_enum":null,"weakened_by_flags":[]}},"covers":[]}`},
	}
	for _, tt := range others {
		t.Run(tt.name, func(t *testing.T) {
			v, err := decodeJSON([]byte(tt.json))
			if err != nil {
				t.Fatalf("decodeJSON(%s) = %v", tt.name, err)
			}
			if err := schemas[tt.schema].Validate(v); err == nil {
				t.Errorf("Validate(%s) = nil, want a violation", tt.name)
			}
		})
	}
	if !slices.Contains(evidencePositive, "closed-basic") {
		t.Error("closed-basic is not a positive export")
	}
}
