package kindyaml

import "encoding/json"

// YAMLToJSON reads b as one YAML document whose root is a mapping and returns its JSON form, under the limits and
// refusals of a kind document (MaxBytes, MaxDepth, MaxNodes; no anchors, aliases, merge keys, tags, duplicate keys or
// second document). It is for the documents that are not kinds but share the dialect, such as the manifest.yaml of an
// extension package; the caller validates the result against its own schema. Errors carry line numbers, never text.
func YAMLToJSON(b []byte) ([]byte, error) {
	v, err := parse(b)
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
