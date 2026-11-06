package schemas

import "embed"

// FS holds the generated kind schemas, gen/numinance.kinds.v1.<Message>.jsonschema.strict.bundle.json,
// so a loader validates against the schemas of the spec version it was built with.
//
//go:embed gen/numinance.kinds.v1.*.jsonschema.strict.bundle.json
var FS embed.FS
