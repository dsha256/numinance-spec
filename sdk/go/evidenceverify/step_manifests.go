package evidenceverify

import "bytes"

// maxManifests is the most manifest documents provenance/executor-manifests.json holds (bundle-layout.md section 6).
const maxManifests = 16

// checkManifestsFile checks provenance/executor-manifests.json against the digests the executor_ids name: one canonical object
// {"export_version": 1, "manifests": [...]} of at most maxManifests documents, each a JSON object whose canonical form hashes to a
// digest named, every digest named present once, and the "name" of each the one before "@" in every executor_id that names it. The
// order of the elements is not checked.
func (v *verifier) checkManifestsFile(want []string) {
	const bad = "provenance/executor-manifests.json is not a canonical manifest list"
	data, err := v.b.read(pathExecutorManifests)
	if err != nil {
		v.fail(CodeDigestMismatch, "provenance/executor-manifests.json cannot be read")
		return
	}
	n, err := parseJSON(data)
	if err != nil || n.kind != nObject || len(n.members) != 2 || !bytes.Equal(canonical(n.toAny()), data) {
		v.fail(CodeDigestMismatch, bad)
		return
	}
	ver, list := n.members["export_version"], n.members["manifests"]
	if ver == nil || ver.kind != nNumber || ver.str != "1" || list == nil || list.kind != nArray || len(list.items) == 0 || len(list.items) > maxManifests {
		v.fail(CodeDigestMismatch, bad)
		return
	}
	names := make(map[string]string, len(list.items))
	for _, m := range list.items {
		if m.kind != nObject {
			v.fail(CodeDigestMismatch, bad)
			return
		}
		d := sha256Digest(canonical(m.toAny()))
		if _, dup := names[d]; dup {
			v.fail(CodeDigestMismatch, "provenance/executor-manifests.json holds one manifest twice")
			return
		}
		names[d] = ""
		if nm := m.members["name"]; nm != nil && nm.kind == nString {
			names[d] = nm.str
		}
	}
	wanted := make(map[string]bool, len(want))
	for _, d := range want {
		wanted[d] = true
	}
	for d := range names {
		if !wanted[d] {
			v.fail(CodeDigestMismatch, "provenance/executor-manifests.json holds a manifest that no executor_id names")
			return
		}
	}
	for _, d := range want {
		if _, ok := names[d]; !ok {
			v.fail(CodePartMissing, "provenance/executor-manifests.json lacks a manifest an executor_id names")
			return
		}
	}
	for _, r := range v.ch.records {
		if nm, d, ok := executorIDParts(r.executor); ok && names[d] != nm {
			v.fail(CodeDigestMismatch, "the executor_id names the manifest "+nm+" and provenance/executor-manifests.json names it "+names[d])
			return
		}
	}
}
