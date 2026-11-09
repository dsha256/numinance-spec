package kindyaml

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/numinance/numinance-spec/schemas"
	kindsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/kinds/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
	"github.com/numinance/numinance-spec/sdk/go/schema"
)

// APIVersion is the apiVersion of every kind in numinance.kinds.v1 except the alpha kinds.
const APIVersion = "numinance.dev/v1"

// AlphaAPIVersion is the apiVersion of an alpha kind (pre-GA, off by default): a Loader reads one only when it was built
// with WithAlphaKinds.
const AlphaAPIVersion = "numinance.dev/v1alpha1"

// producedError is ErrProduced: a kind the engine produces. It unwraps to
// ErrKind, so callers that refuse unknown kinds keep working.
type producedError struct{}

func (producedError) Error() string { return "kind: produced kind" }
func (producedError) Unwrap() error { return ErrKind }

// alphaError is ErrAlpha: an alpha kind the loader was not told to read. It unwraps to ErrKind, so callers that refuse
// unknown kinds keep working.
type alphaError struct{}

func (alphaError) Error() string { return "kind: alpha kind is not enabled" }
func (alphaError) Unwrap() error { return ErrKind }

// MaxNameBytes bounds metadata.name.
const MaxNameBytes = 253

var (
	// ErrKind reports a document whose kind is missing or not a kind of numinance.kinds.v1.
	ErrKind = errors.New("kind: unknown kind")
	// ErrProduced wraps ErrKind; it is returned for a kind the engine produces and never loads.
	ErrProduced error = producedError{}
	// ErrAlpha wraps ErrKind; it is returned for an alpha kind (EvalSet) when the Loader was built without WithAlphaKinds.
	ErrAlpha error = alphaError{}
	// ErrSchema reports a document that fails its kind schema or protojson decoding.
	ErrSchema = errors.New("kind: does not match the kind schema")
	// ErrInvalid reports a document that passes the schema but fails a loader check.
	ErrInvalid = errors.New("kind: invalid")
)

// Loader decodes and encodes kind documents. Its zero value is not usable;
// call NewLoader. A Loader is immutable, so it is safe for concurrent use.
type Loader struct {
	kinds    map[string]entry
	produced map[string]entry
	required map[protoreflect.FullName]bool
	checks   map[protoreflect.FullName]func(protoreflect.Value) bool
	unique   map[protoreflect.FullName]func(protoreflect.Value) string
	alpha    bool
}

// Option configures NewLoader.
type Option func(*Loader)

// WithAlphaKinds makes the Loader read alpha kinds (EvalSet, apiVersion numinance.dev/v1alpha1). Without it they are
// refused with ErrAlpha. Only a composition root that runs an alpha feature sets it (RFC-0004).
func WithAlphaKinds() Option { return func(l *Loader) { l.alpha = true } }

type entry struct {
	schema *schema.Schema
	typ    protoreflect.MessageType
}

// requiredFields names the repeated fields a document must not leave empty,
// the implicit-presence fields it must not leave zero and the singular
// message fields it must not leave unset, which the generated schema cannot
// express. A kind change adds its rows here.
func requiredFields() []protoreflect.FullName {
	return []protoreflect.FullName{
		"numinance.kinds.v1.ContextBundleSpec.change_id",
		"numinance.kinds.v1.ContextBundleSpec.work_unit_id",
		"numinance.kinds.v1.ContextBundleSpec.stage",
		"numinance.kinds.v1.ContextBundleSpec.context_generation",
		"numinance.kinds.v1.ContextBundleSpec.authz_context_digest",
		"numinance.kinds.v1.ContextBundleSpec.resolver",
		"numinance.kinds.v1.ContextBundleSpec.budget_used",
		"numinance.kinds.v1.BundleResolver.model_id",
		"numinance.kinds.v1.BundleResolver.prompt_version",
		"numinance.kinds.v1.BundleQuery.tool",
		"numinance.kinds.v1.BundleQuery.args_digest",
		"numinance.kinds.v1.BundleSource.ref",
		"numinance.kinds.v1.BundleSource.system",
		"numinance.kinds.v1.BundleSource.fetched_at",
		"numinance.kinds.v1.BundleSource.digest",
		"numinance.kinds.v1.BundleSource.selection_reason",
		"numinance.kinds.v1.BundleConflict.topic",
		"numinance.kinds.v1.IndexerPluginSpec.indexers",
		"numinance.kinds.v1.Indexer.provider",
		"numinance.kinds.v1.StageRegistrySpec.stages",
		"numinance.kinds.v1.StageRegistryEntry.name",
		"numinance.kinds.v1.PlaybookSpec.change_type",
		"numinance.kinds.v1.PlaybookSpec.steps",
		"numinance.kinds.v1.PlaybookStep.name",
		"numinance.kinds.v1.PlaybookStep.capability_set",
		"numinance.kinds.v1.RolloutPlan.staging",
		"numinance.kinds.v1.RolloutPlan.production",
		"numinance.kinds.v1.RolloutPlan.stages",
		"numinance.kinds.v1.RolloutPlan.window",
		"numinance.kinds.v1.CanaryStage.name",
		"numinance.kinds.v1.CanaryStage.weight",
		"numinance.kinds.v1.ArchitecturePackSpec.rules",
		"numinance.kinds.v1.ArchitectureRule.id",
		"numinance.kinds.v1.ArchitectureRule.statement",
		"numinance.kinds.v1.ArchitectureRule.check",
		"numinance.kinds.v1.ArchitectureCheck.verifier",
		"numinance.kinds.v1.ArchitectureCheck.rule_id",
		"numinance.kinds.v1.LanguageToolchainSpec.language",
		"numinance.kinds.v1.LanguageToolchainSpec.version",
		"numinance.kinds.v1.LanguageToolchainSpec.verifiers",
		"numinance.kinds.v1.ToolchainVerifier.name",
		"numinance.kinds.v1.ToolchainVerifier.provider",
		"numinance.kinds.v1.EnvironmentProviderSpec.provider",
		"numinance.kinds.v1.EnvironmentProviderSpec.ttl",
		"numinance.kinds.v1.AdapterSelectorSpec.ports",
		"numinance.kinds.v1.AdapterBinding.provider",
		"numinance.kinds.v1.AdapterBinding.instance",
		"numinance.kinds.v1.AutonomyRule.match",
		"numinance.kinds.v1.AutonomyFloors.rules",
		"numinance.kinds.v1.AutonomyFloor.match",
		"numinance.kinds.v1.ResidencyPolicySpec.regions",
		"numinance.kinds.v1.ResidencyRule.match",
		"numinance.kinds.v1.KnowledgeConnectionSpec.provider",
		"numinance.kinds.v1.KnowledgeConnectionSpec.auth",
		"numinance.kinds.v1.KnowledgeConnectionSpec.scope",
		"numinance.kinds.v1.PinnedTool.name",
		"numinance.kinds.v1.PinnedTool.hash",
		"numinance.types.v1.Digest.value", // isDigest refuses "" too; the schema row states the rule
		"numinance.kinds.v1.ControlPackSpec.jurisdiction",
		"numinance.kinds.v1.ControlPackSpec.framework",
		"numinance.kinds.v1.ControlPackSpec.version",
		"numinance.kinds.v1.ControlPackSpec.effective_date",
		"numinance.kinds.v1.ControlPackSpec.controls",
		"numinance.kinds.v1.Control.id",
		"numinance.kinds.v1.ExportView.name",
		"numinance.kinds.v1.ExportView.controls",
		"numinance.kinds.v1.ExecutorManifestSpec.provider",
		"numinance.kinds.v1.ExecutorManifestSpec.capabilities",
		"numinance.kinds.v1.ExecutorManifestSpec.models",
		"numinance.kinds.v1.ExecutorModel.id",
		"numinance.kinds.v1.ExecutorModel.max_context",
		"numinance.kinds.v1.EvalSetSpec.version",
		"numinance.kinds.v1.EvalSetSpec.cases",
		"numinance.kinds.v1.EvalCase.name",
		"numinance.kinds.v1.EvalCase.outcome",
		"numinance.kinds.v1.EvalCase.expect",
		"numinance.kinds.v1.EvalLayerCount.layer",
		"numinance.kinds.v1.EvalLayerCount.count",
		"numinance.kinds.v1.EvalSeeded.defect",
		"numinance.kinds.v1.EvalSeeded.caught_by",
		"numinance.kinds.v1.LockedExtension.id",
		"numinance.kinds.v1.LockedExtension.version",
		"numinance.kinds.v1.LockedExtension.digest",
		"numinance.kinds.v1.LockedExtension.h1",
		"numinance.kinds.v1.LockedExtension.zh",
		"numinance.kinds.v1.LockedExtension.signer",
		"numinance.kinds.v1.LockedExtension.permissions_digest",
		"numinance.kinds.v1.LockedKnowledgeTool.connection",
		"numinance.kinds.v1.LockedKnowledgeTool.name",
		"numinance.kinds.v1.LockedKnowledgeTool.hash",
	}
}

// orgOnly reports the kinds whose metadata.scope must be SCOPE_ORG. The profile context also refuses one loaded
// below the org, since the scope comes from where a document was loaded.
func orgOnly(kind protoreflect.Name) bool {
	return kind == "KnowledgeConnection" || kind == "ControlPack" || kind == "ExecutorManifest"
}

// asciiName reports the kinds whose metadata.name is a name profiles select
// and so must be ASCII segments.
func asciiName(kind protoreflect.Name) bool {
	return kind == "ExecutorManifest"
}

// alphaKind reports the kinds that are pre-GA: they carry AlphaAPIVersion and load only through WithAlphaKinds.
func alphaKind(kind protoreflect.Name) bool {
	return kind == "EvalSet"
}

// apiVersionOf returns the one apiVersion a document of the kind carries.
func apiVersionOf(kind protoreflect.Name) string {
	if alphaKind(kind) {
		return AlphaAPIVersion
	}
	return APIVersion
}

// produced reports the kinds Numinance produces rather than loads. The loader does not load them, so a
// repository file spelled as one is refused with ErrKind and never becomes
// configuration; a kind missing from this list keeps the envelope check.
func produced(kind protoreflect.Name) bool {
	return kind == "ContextBundle"
}

// uniqueKeys maps the list fields whose elements must be unique to the key
// that identifies an element.
func uniqueKeys() map[protoreflect.FullName]func(protoreflect.Value) string {
	str := func(v protoreflect.Value) string { return v.String() }
	return map[protoreflect.FullName]func(protoreflect.Value) string{
		"numinance.kinds.v1.SDLCProfileSpec.playbooks":          str,
		"numinance.kinds.v1.SDLCProfileSpec.control_packs":      str,
		"numinance.kinds.v1.SDLCProfileSpec.architecture_packs": str,
		"numinance.kinds.v1.SDLCProfileSpec.toolchains":         str,
		"numinance.kinds.v1.ControlPackSpec.sources":            str,
		"numinance.kinds.v1.RolloutPlan.stages":                 field("name"),
		"numinance.kinds.v1.ControlPackSpec.controls":           field("id"),
		"numinance.kinds.v1.ControlPackSpec.export_views":       field("name"),
		"numinance.kinds.v1.Control.requires_evidence":          str,
		"numinance.kinds.v1.ExportView.controls":                str,
		"numinance.kinds.v1.ExecutorManifestSpec.capabilities":  func(v protoreflect.Value) string { return strconv.Itoa(int(v.Enum())) },
		"numinance.kinds.v1.ExecutorManifestSpec.models":        field("id"),
		// Folded, since a source may compare scope names without case.
		"numinance.kinds.v1.KnowledgeConnectionSpec.scope":        func(v protoreflect.Value) string { return strings.ToLower(v.String()) },
		"numinance.kinds.v1.KnowledgeConnectionSpec.pinned_tools": field("name"),
		"numinance.kinds.v1.LockfileSpec.extensions":              field("id"),
		"numinance.kinds.v1.LockfileSpec.knowledge_tools":         pairKey("connection", "name"),
	}
}

// field returns the key of a list element that is a message: its string field name.
func field(name protoreflect.Name) func(protoreflect.Value) string {
	return func(v protoreflect.Value) string {
		m := v.Message()
		return m.Get(m.Descriptor().Fields().ByName(name)).String()
	}
}

// valueChecks maps the fields whose present values pass a check the schema
// cannot express to that check. A kind change adds
// its rows here.
func valueChecks() map[protoreflect.FullName]func(protoreflect.Value) bool {
	m := map[protoreflect.FullName]func(protoreflect.Value) bool{
		"numinance.kinds.v1.EnvironmentProviderSpec.ttl":       func(v protoreflect.Value) bool { return durationSign(v) > 0 },
		"numinance.kinds.v1.ResidencyPolicySpec.retention_max": func(v protoreflect.Value) bool { return durationSign(v) >= 0 },
		"numinance.kinds.v1.RolloutPlan.staging":               func(v protoreflect.Value) bool { return isRolloutName(v.String()) },
		"numinance.kinds.v1.RolloutPlan.production":            func(v protoreflect.Value) bool { return isRolloutName(v.String()) },
		"numinance.kinds.v1.RolloutPlan.window":                isRolloutWindow,
		"numinance.kinds.v1.CanaryStage.name":                  func(v protoreflect.Value) bool { return isRolloutName(v.String()) },
		"numinance.kinds.v1.CanaryStage.weight":                isRolloutWeight,
		"numinance.kinds.v1.EnvironmentBudget.max_concurrent":  func(v protoreflect.Value) bool { return v.Uint() > 0 },
		"numinance.kinds.v1.StageGate.required_approvals":      func(v protoreflect.Value) bool { return v.Uint() > 0 },
		"numinance.kinds.v1.KnowledgeConnectionSpec.endpoint":  func(v protoreflect.Value) bool { return isEndpoint(v.String()) },
		"numinance.kinds.v1.KnowledgeOAuth.client_id":          func(v protoreflect.Value) bool { return isClientID(v.String()) },
		"numinance.kinds.v1.KnowledgeConnectionSpec.read_only": func(v protoreflect.Value) bool { return v.Bool() },
		"numinance.kinds.v1.KnowledgeConnectionSpec.cache_ttl": func(v protoreflect.Value) bool { return durationSign(v) >= 0 },
		"numinance.kinds.v1.KnowledgeBudgets.calls_per_stage":  func(v protoreflect.Value) bool { return v.Uint() > 0 },
		"numinance.kinds.v1.KnowledgeBudgets.bytes_per_call":   func(v protoreflect.Value) bool { return v.Uint() > 0 },
		// Retrieved text is untrusted; a bundle may not raise its label.
		"numinance.kinds.v1.BundleSource.trust": func(v protoreflect.Value) bool {
			return v.Enum() == protoreflect.EnumNumber(typesv1.TrustClass_TRUST_CLASS_UNTRUSTED_RETRIEVED)
		},
		"numinance.types.v1.Digest.value":                   func(v protoreflect.Value) bool { return isDigest(v.String()) },
		"numinance.kinds.v1.ControlPackSpec.effective_date": func(v protoreflect.Value) bool { return isDate(v.String()) },
		"numinance.kinds.v1.ControlPackSpec.sources":        func(v protoreflect.Value) bool { return isText(v.String(), MaxSourceBytes) },
		"numinance.kinds.v1.ControlPackSpec.applicability":  func(v protoreflect.Value) bool { return isText(v.String(), MaxApplicabilityBytes) },
		"numinance.kinds.v1.ExecutorModel.id":               func(v protoreflect.Value) bool { return isModelID(v.String()) },
		"numinance.kinds.v1.EvalSetSpec.version":            func(v protoreflect.Value) bool { return isSemver(v.String()) },
		"numinance.kinds.v1.LockedExtension.id":             func(v protoreflect.Value) bool { return isExtensionID(v.String()) },
		"numinance.kinds.v1.LockedExtension.version":        func(v protoreflect.Value) bool { return isSemver(v.String()) },
		"numinance.kinds.v1.LockedExtension.h1":             func(v protoreflect.Value) bool { return isH1(v.String()) },
		"numinance.kinds.v1.LockedExtension.zh":             func(v protoreflect.Value) bool { return isZH(v.String()) },
		"numinance.kinds.v1.LockedExtension.signer":         func(v protoreflect.Value) bool { return isSigner(v.String()) },
	}
	for _, f := range []protoreflect.FullName{
		"numinance.kinds.v1.EvalCase.name",
		"numinance.kinds.v1.EvalLayerCount.layer",
		"numinance.kinds.v1.EvalSeeded.defect",
		"numinance.kinds.v1.EvalSeeded.caught_by",
	} {
		m[f] = func(v protoreflect.Value) bool { return isToken(v.String()) }
	}
	for _, f := range []protoreflect.FullName{
		"numinance.kinds.v1.EnvironmentProviderSpec.provider",
		"numinance.kinds.v1.EnvironmentProviderSpec.source",
		"numinance.kinds.v1.EnvironmentProviderSpec.seed",
		"numinance.kinds.v1.AutonomyMatch.playbook",
		"numinance.kinds.v1.AutonomyMatch.environment",
		"numinance.kinds.v1.ResidencyPolicySpec.regions",
		"numinance.kinds.v1.ResidencyMatch.component",
		"numinance.kinds.v1.ResidencyMatch.environment",
		"numinance.kinds.v1.KnowledgeConnectionSpec.provider",
		"numinance.kinds.v1.PinnedTool.name",
		"numinance.kinds.v1.LockedKnowledgeTool.connection",
		"numinance.kinds.v1.LockedKnowledgeTool.name",
	} {
		m[f] = func(v protoreflect.Value) bool { return isName(v.String()) }
	}
	// ASCII only, so a confusable letter cannot make two names look alike.
	for _, f := range []protoreflect.FullName{
		"numinance.kinds.v1.KnowledgeConnectionSpec.scope",
		"numinance.kinds.v1.SDLCProfileSpec.stages",
		"numinance.kinds.v1.SDLCProfileSpec.playbooks",
		"numinance.kinds.v1.SDLCProfileSpec.control_packs",
		"numinance.kinds.v1.SDLCProfileSpec.architecture_packs",
		"numinance.kinds.v1.SDLCProfileSpec.toolchains",
		"numinance.kinds.v1.AdapterPorts.knowledge",
		"numinance.kinds.v1.AdapterBinding.provider",
		"numinance.kinds.v1.AdapterBinding.instance",
		"numinance.kinds.v1.ControlPackSpec.jurisdiction",
		"numinance.kinds.v1.ControlPackSpec.framework",
		"numinance.kinds.v1.ControlPackSpec.version",
		"numinance.kinds.v1.Control.id",
		"numinance.kinds.v1.Control.requires_evidence",
		"numinance.kinds.v1.ControlApproval.group",
		"numinance.kinds.v1.ExportView.name",
		"numinance.kinds.v1.ExportView.controls",
		"numinance.kinds.v1.ExecutorManifestSpec.provider",
	} {
		m[f] = func(v protoreflect.Value) bool { return isPath(v.String()) }
	}
	return m
}

// isName reports whether s names one thing: non-empty (an empty match key
// would match nothing, so its floor would never apply), at most MaxNameBytes,
// and free of ':', '@', '*', whitespace and control or bidi characters, so it
// carries no URL, credential, command or wildcard.
func isName(s string) bool {
	return s != "" && len(s) <= MaxNameBytes && !strings.ContainsFunc(s, func(r rune) bool {
		return r == ':' || r == '@' || r == '*' || unicode.IsSpace(r) || unsafeNameRune(r)
	})
}

// MaxSourceBytes bounds one ControlPack source citation.
const MaxSourceBytes = 1024

// MaxApplicabilityBytes bounds a ControlPack applicability expression.
const MaxApplicabilityBytes = 4096

// isText reports whether s is one non-empty line of at most limit bytes with
// no control or bidi character, so a citation or an expression cannot hide
// text from a reviewer or break a log line.
func isText(s string, limit int) bool {
	return s != "" && len(s) <= limit && !strings.ContainsFunc(s, unsafeNameRune)
}

// MaxClientIDBytes bounds a KnowledgeOAuth client_id.
const MaxClientIDBytes = 2048

// isClientID reports whether s is a client id: 1 to MaxClientIDBytes bytes of
// printable ASCII (no whitespace, control or bidi character), and, when it
// is URL-shaped, an https URL that passes the endpoint rule. An id is URL-shaped
// when it starts with a scheme ("name:"), starts with "//", or holds a backslash
// or "%", contains "://", or has a colon before its first "/", "?" or "#", so no parser can read a malformed form such as "https:/10.0.0.1/x" or
// "https:%2F%2F10.0.0.1" as a host. A plain id therefore has no colon after a
// leading letter run. A "." or ".." path segment is refused outright.
func isClientID(s string) bool {
	if s == "" || len(s) > MaxClientIDBytes || strings.ContainsFunc(s, func(r rune) bool { return r <= ' ' || r >= 0x7f }) {
		return false
	}
	if slices.Contains(strings.Split(s, "/"), ".") || slices.Contains(strings.Split(s, "/"), "..") {
		return false
	}
	return !urlShaped(s) || isEndpoint(s)
}

// urlShaped reports whether s could be read as a URL by some client.
func urlShaped(s string) bool {
	if strings.Contains(s, "://") || strings.HasPrefix(s, "//") || strings.ContainsAny(s, "\\%") {
		return true
	}
	// A colon before the first "/", "?" or "#" is a scheme separator to some parser, whatever precedes it.
	if c := strings.IndexByte(s, ':'); c >= 0 && (!strings.ContainsAny(s, "/?#") || c < strings.IndexAny(s, "/?#")) {
		return true
	}
	i := 0
	for i < len(s) && (s[i] >= 'a' && s[i] <= 'z' || s[i] >= 'A' && s[i] <= 'Z' || i > 0 && (s[i] >= '0' && s[i] <= '9' || s[i] == '+' || s[i] == '-' || s[i] == '.')) {
		i++
	}
	return i > 0 && i < len(s) && s[i] == ':'
}

// isDate reports whether s is a calendar day in the form YYYY-MM-DD.
func isDate(s string) bool {
	_, err := time.Parse(time.DateOnly, s)
	return err == nil
}

// MaxEndpointBytes bounds a KnowledgeConnection endpoint.
const MaxEndpointBytes = 2048

// isPath reports whether s is one name, space, project or relative path of
// at most MaxNameBytes: "/"-joined segments, each an ASCII letter or digit
// followed by letters, digits, ".", "_" or "-". An allow-list, so a scope
// never means "everything" (no "*", leading "/", "." or ".." segment) and a
// name has no escape, confusable or Unicode-normalisation form. It is not
// query-safe: a provider passes a scope as a typed parameter.
func isPath(s string) bool {
	return len(s) <= MaxNameBytes && !slices.ContainsFunc(strings.Split(s, "/"), func(seg string) bool {
		return seg == "" || !isASCIIAlnum(rune(seg[0])) || strings.ContainsFunc(seg, func(r rune) bool {
			return !isASCIIAlnum(r) && r != '.' && r != '_' && r != '-'
		})
	})
}

// isModelID reports whether s is a model id of at most MaxNameBytes: an ASCII
// letter or digit followed by letters, digits, ".", "_", "-", ":", "/" and
// "@", the characters of vendor model ids, name@version ids and inference
// profile ARNs. No "/" segment is empty, "." or "..". At most one "@", in
// the last segment and not its last character, and a last segment with an
// "@" has no ":", so a user:secret@host form fails.
func isModelID(s string) bool {
	if s == "" || len(s) > MaxNameBytes || !isASCIIAlnum(rune(s[0])) || strings.ContainsFunc(s, func(r rune) bool {
		return !isASCIIAlnum(r) && !strings.ContainsRune("._-:/@", r)
	}) || slices.ContainsFunc(strings.Split(s, "/"), func(seg string) bool {
		return seg == "" || seg == "." || seg == ".."
	}) {
		return false
	}
	at := strings.Index(s, "@")
	last := s[strings.LastIndex(s, "/")+1:]
	return at < 0 || at == strings.LastIndex(s, "@") && at > strings.LastIndex(s, "/") && at < len(s)-1 && !strings.Contains(last, ":")
}

func isASCIIAlnum(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}

// isEndpoint reports whether s is an https URL of at most MaxEndpointBytes
// printable ASCII bytes, with a DNS host (isHost), a port in 1 to 65535 if
// any, and no user information (which could carry a credential), query,
// fragment, ";", "%" or "\\" (so no escaped control byte or path parameter),
// and no "." or ".." path segment, so every client resolves the same path.
func isEndpoint(s string) bool {
	if len(s) > MaxEndpointBytes || !strings.HasPrefix(s, "https://") || strings.ContainsFunc(s, func(r rune) bool {
		return r <= ' ' || r >= 0x7f || strings.ContainsRune(`?#;%\`, r)
	}) {
		return false
	}
	u, err := url.Parse(s)
	if err != nil || u.User != nil || !isHost(u.Hostname()) || slices.ContainsFunc(strings.Split(u.Path, "/"), func(seg string) bool {
		return seg == "." || seg == ".."
	}) {
		return false
	}
	p := u.Port()
	if p == "" {
		return !strings.HasSuffix(u.Host, ":")
	}
	n, err := strconv.Atoi(p)
	return err == nil && p[0] != '0' && n <= 65535
}

// isHost reports whether h is a lower-case DNS name of two or more labels of
// at most 63 bytes each, letters, digits and inner hyphens only, the last one
// starting with a letter: an IP literal, a numeric host (2130706433),
// a single label (localhost), a trailing dot, a non-ASCII name and the
// local-only suffixes .localhost, .local and .internal are refused. A DNS
// name can still resolve to an internal address; the egress proxy decides by
// the resolved address.
func isHost(h string) bool {
	labels := strings.Split(h, ".")
	if len(h) > 253 || len(labels) < 2 || slices.Contains([]string{"localhost", "local", "internal"}, labels[len(labels)-1]) {
		return false
	}
	if last := labels[len(labels)-1]; last == "" || last[0] < 'a' || last[0] > 'z' {
		return false
	}
	return !slices.ContainsFunc(labels, func(l string) bool {
		return l == "" || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' || strings.ContainsFunc(l, func(r rune) bool {
			return (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-'
		})
	})
}

// isDigest reports whether s is "sha256:" plus 64 lowercase hex characters,
// the form numinance.types.v1.Digest requires.
func isDigest(s string) bool {
	h, ok := strings.CutPrefix(s, "sha256:")
	return ok && len(h) == 64 && !strings.ContainsFunc(h, func(r rune) bool {
		return (r < '0' || r > '9') && (r < 'a' || r > 'f')
	})
}

// durationSign ORs the seconds and nanos of a google.protobuf.Duration: below zero when either is, zero when both are.
func durationSign(v protoreflect.Value) int64 {
	d := v.Message()
	s := d.Get(d.Descriptor().Fields().ByName("seconds")).Int()
	return s | d.Get(d.Descriptor().Fields().ByName("nanos")).Int()
}

// NewLoader compiles the embedded schema of every kind in numinance.kinds.v1.
func NewLoader(opts ...Option) (*Loader, error) {
	l := &Loader{kinds: map[string]entry{}, required: map[protoreflect.FullName]bool{}, checks: valueChecks(), unique: uniqueKeys()}
	for _, o := range opts {
		o(l)
	}
	pkg := kindsv1.File_numinance_kinds_v1_meta_proto.Package()
	var err error
	protoregistry.GlobalFiles.RangeFilesByPackage(pkg, func(fd protoreflect.FileDescriptor) bool {
		for i := range fd.Messages().Len() {
			md := fd.Messages().Get(i)
			if md.Fields().ByName("api_version") == nil {
				continue // a spec or a shared message, not a kind
			}
			if produced(md.Name()) {
				if err = l.addProduced(md); err != nil {
					return false
				}
				continue
			}
			if err = l.add(md); err != nil {
				return false
			}
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	if len(l.kinds) == 0 {
		return nil, fmt.Errorf("kind: no kinds registered in %s", pkg)
	}
	for _, f := range requiredFields() {
		l.required[f] = true
	}
	return l, nil
}

func (l *Loader) add(md protoreflect.MessageDescriptor) error {
	if err := envelope(md); err != nil {
		return fmt.Errorf("kind: %s: %w", md.Name(), err)
	}
	e, err := compileEntry(md)
	if err != nil {
		return err
	}
	l.kinds[string(md.Name())] = e
	return nil
}

// addProduced compiles the schema of a produced kind, kept apart from the
// loaded kinds so Decode still refuses it.
func (l *Loader) addProduced(md protoreflect.MessageDescriptor) error {
	e, err := compileEntry(md)
	if err != nil {
		return err
	}
	if l.produced == nil {
		l.produced = map[string]entry{}
	}
	l.produced[string(md.Name())] = e
	return nil
}

// compileEntry compiles the embedded strict schema of the kind message md.
func compileEntry(md protoreflect.MessageDescriptor) (entry, error) {
	name := "gen/" + string(md.FullName()) + ".jsonschema.strict.bundle.json"
	b, err := fs.ReadFile(schemas.FS, name)
	if err != nil {
		return entry{}, fmt.Errorf("kind: schema of %s: %w", md.Name(), err)
	}
	s, err := schema.Compile(b)
	if err != nil {
		return entry{}, fmt.Errorf("kind: schema of %s: %w", md.Name(), err)
	}
	typ, err := protoregistry.GlobalTypes.FindMessageByName(md.FullName())
	if err != nil {
		return entry{}, fmt.Errorf("kind: type of %s: %w", md.Name(), err)
	}
	return entry{schema: s, typ: typ}, nil
}

// envelope reports whether md has the fields and types check relies on, so
// a malformed kind fails NewLoader rather than panicking in Decode.
func envelope(md protoreflect.MessageDescriptor) error {
	singular := func(name protoreflect.Name, k protoreflect.Kind) protoreflect.FieldDescriptor {
		fd := md.Fields().ByName(name)
		if fd == nil || fd.IsList() || fd.IsMap() || fd.Kind() != k {
			return nil
		}
		return fd
	}
	meta := (*kindsv1.Metadata)(nil).ProtoReflect().Descriptor().FullName()
	if singular("api_version", protoreflect.StringKind) == nil || singular("kind", protoreflect.StringKind) == nil {
		return errors.New("api_version and kind are not singular strings")
	}
	if fd := singular("metadata", protoreflect.MessageKind); fd == nil || fd.Message().FullName() != meta {
		return fmt.Errorf("metadata is not a %s", meta)
	}
	if singular("spec", protoreflect.MessageKind) == nil {
		return errors.New("spec is not a message")
	}
	fd := md.Fields().ByName("extensions")
	if fd == nil || !fd.IsMap() || fd.MapKey().Kind() != protoreflect.StringKind ||
		fd.MapValue().Message() == nil || fd.MapValue().Message().FullName() != "google.protobuf.Value" {
		return errors.New("extensions is not a map<string, google.protobuf.Value>")
	}
	return nil
}

// Decode parses one kind document of at most MaxBytes, validates it against
// its kind schema, decodes it into the kind message and runs the loader
// checks. The result is a message of the type the document's kind names.
// Errors wrap the sentinels of this package and never quote a value of the
// document; a schema violation names its path, built from the document's
// keys (an unknown key among them), quoted.
func (l *Loader) Decode(b []byte) (proto.Message, error) {
	v, err := parse(b)
	if err != nil {
		return nil, err
	}
	name, _ := v["kind"].(string)
	k, ok := l.kinds[name]
	if ok && alphaKind(protoreflect.Name(name)) && !l.alpha {
		// name is the constant of an alpha kind here, not free document text.
		return nil, fmt.Errorf("%w: %s", ErrAlpha, name)
	}
	if !ok {
		if produced(protoreflect.Name(name)) {
			// name is the constant of a produced kind here, not free document text.
			return nil, fmt.Errorf("%w: %s is not configuration", ErrProduced, name)
		}
		return nil, ErrKind
	}
	if err := k.schema.Validate(v); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSchema, err)
	}
	js, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSchema, err)
	}
	m := k.typ.New().Interface()
	if err := protojson.Unmarshal(js, m); err != nil {
		// protojson errors quote the offending value; report the stage only.
		return nil, fmt.Errorf("%w: a value is not in protobuf JSON form", ErrSchema)
	}
	if err := l.check(m.ProtoReflect()); err != nil {
		return nil, err
	}
	return m, nil
}

// checkEnvelope checks apiVersion, kind, and that metadata and spec are present.
func checkEnvelope(m protoreflect.Message) error {
	fields := m.Descriptor().Fields()
	if want := apiVersionOf(m.Descriptor().Name()); m.Get(fields.ByName("api_version")).String() != want {
		return fmt.Errorf("%w: apiVersion is not %s", ErrInvalid, want)
	}
	// Decode chooses the message by kind, so this guards direct callers of check.
	if m.Get(fields.ByName("kind")).String() != string(m.Descriptor().Name()) {
		return fmt.Errorf("%w: kind does not name the message", ErrInvalid)
	}
	for _, f := range []protoreflect.Name{"metadata", "spec"} {
		// A produced kind may have no metadata; loaded kinds always do (envelope).
		if fd := fields.ByName(f); fd != nil && !m.Has(fd) {
			return fmt.Errorf("%w: %s is missing", ErrInvalid, f)
		}
	}
	return nil
}

// check enforces what the schema cannot.
func (l *Loader) check(m protoreflect.Message) error {
	if err := checkEnvelope(m); err != nil {
		return err
	}
	fields := m.Descriptor().Fields()
	md := m.Get(fields.ByName("metadata")).Message()
	name := md.Get(md.Descriptor().Fields().ByName("name")).String()
	if name == "" {
		return fmt.Errorf("%w: metadata.name is empty", ErrInvalid)
	}
	if len(name) > MaxNameBytes || strings.ContainsFunc(name, unsafeNameRune) {
		return fmt.Errorf("%w: metadata.name is over %d bytes or has a control or bidi character", ErrInvalid, MaxNameBytes)
	}
	if asciiName(m.Descriptor().Name()) && !isPath(name) {
		return fmt.Errorf("%w: metadata.name of %s is not ASCII segments", ErrInvalid, m.Descriptor().Name())
	}
	scope := md.Get(md.Descriptor().Fields().ByName("scope")).Enum()
	if orgOnly(m.Descriptor().Name()) && scope != protoreflect.EnumNumber(kindsv1.Scope_SCOPE_ORG) {
		return fmt.Errorf("%w: %s is org-scoped only", ErrInvalid, m.Descriptor().Name())
	}
	if err := l.walk(m); err != nil {
		return err
	}
	if scope != protoreflect.EnumNumber(kindsv1.Scope_SCOPE_ORG) {
		if err := belowOrg(m.Get(fields.ByName("spec")).Message()); err != nil {
			return err
		}
	}
	if m.Descriptor().FullName() == "numinance.kinds.v1.ControlPack" {
		// Encode may pass a dynamicpb message, so read it through the generated type.
		p := &kindsv1.ControlPack{}
		if b, err := proto.Marshal(m.Interface()); err != nil || proto.Unmarshal(b, p) != nil {
			return fmt.Errorf("%w: the ControlPack does not round-trip", ErrInvalid)
		}
		return checkControlPack(p.GetSpec())
	}
	if m.Descriptor().FullName() == "numinance.kinds.v1.EvalSet" {
		e := &kindsv1.EvalSet{}
		if b, err := proto.Marshal(m.Interface()); err != nil || proto.Unmarshal(b, e) != nil {
			return fmt.Errorf("%w: the EvalSet does not round-trip", ErrInvalid)
		}
		return checkEvalSet(e.GetSpec())
	}
	if m.Descriptor().FullName() == "numinance.kinds.v1.Playbook" {
		p := &kindsv1.Playbook{}
		if b, err := proto.Marshal(m.Interface()); err != nil || proto.Unmarshal(b, p) != nil {
			return fmt.Errorf("%w: the Playbook does not round-trip", ErrInvalid)
		}
		return checkPlaybook(p.GetSpec())
	}
	if m.Descriptor().FullName() == "numinance.kinds.v1.StageRegistry" {
		r := &kindsv1.StageRegistry{}
		if b, err := proto.Marshal(m.Interface()); err != nil || proto.Unmarshal(b, r) != nil {
			return fmt.Errorf("%w: the StageRegistry does not round-trip", ErrInvalid)
		}
		return checkStageRegistry(r.GetSpec())
	}
	return nil
}

// checkStageRegistry refuses required_approvals on a gate that is not a human
// gate: only a human gate counts approvals, and a count on a
// gate decided by policy would read as a promise nothing keeps.
func checkStageRegistry(spec *kindsv1.StageRegistrySpec) error {
	for _, st := range spec.GetStages() {
		if g := st.GetGate(); g != nil && g.RequiredApprovals != nil && g.GetKind() != kindsv1.GateKind_GATE_KIND_HUMAN {
			return fmt.Errorf("%w: required_approvals is set on a gate that is not GATE_KIND_HUMAN", ErrInvalid)
		}
	}
	return nil
}

// belowOrg refuses, in a document whose metadata.scope is not SCOPE_ORG, a set
// field that carries MERGE_DIRECTIVE_FORBID and a residency regions list, which
// the profile context refuses below the org. The scope is the document's own claim; the profile context decides it
// from where the document was loaded.
func belowOrg(m protoreflect.Message) error {
	var err error
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		d, _ := proto.GetExtension(fd.Options(), kindsv1.E_Merge).(kindsv1.MergeDirective)
		if d == kindsv1.MergeDirective_MERGE_DIRECTIVE_FORBID || fd.FullName() == "numinance.kinds.v1.ResidencyPolicySpec.regions" {
			err = fmt.Errorf("%w: %s is set only at the org scope", ErrInvalid, fd.FullName())
			return false
		}
		// Directives sit only on fields reached through singular messages.
		if fd.Message() != nil && !fd.IsList() && !fd.IsMap() && strings.HasPrefix(string(fd.Message().FullName()), "numinance.") {
			err = belowOrg(v.Message())
		}
		return err == nil
	})
	return err
}

// checkControlPack refuses an approval that names a group or dual control
// without a human, which would leave open whether agents may approve, and an
// export view that names a control the pack does not define, which would
// export less than it says.
func checkControlPack(spec *kindsv1.ControlPackSpec) error {
	ids := map[string]bool{}
	for _, c := range spec.GetControls() {
		ids[c.GetId()] = true
		if a := c.GetApproval(); (a.GetGroup() != "" || a.GetDualControl()) && !a.GetHuman() {
			return fmt.Errorf("%w: an approval with a group or dual control does not require a human", ErrInvalid)
		}
	}
	for _, v := range spec.GetExportViews() {
		if slices.ContainsFunc(v.GetControls(), func(id string) bool { return !ids[id] }) {
			return fmt.Errorf("%w: an export view names a control the pack does not define", ErrInvalid)
		}
	}
	return nil
}

// unsafeNameRune reports a control character, a line or paragraph separator,
// or any invisible format character (category Cf: bidi marks, embeddings,
// overrides and isolates, zero-width and joiner marks, BOM, soft hyphen, tags).
func unsafeNameRune(r rune) bool {
	return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == 0x2028 || r == 0x2029
}

// walk refuses a zero enum value, an empty required field and a
// non-reverse-DNS key of a map named extensions anywhere in m, descending
// into numinance messages only: google.protobuf.Value under extensions has a
// zero enum (NULL_VALUE) that is legitimate.
func (l *Loader) walk(m protoreflect.Message) error {
	fields := m.Descriptor().Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		v := m.Get(fd)
		if l.required[fd.FullName()] && (fd.IsList() && v.List().Len() == 0 || !fd.IsList() && !fd.IsMap() && !m.Has(fd)) {
			return fmt.Errorf("%w: %s is empty", ErrInvalid, fd.FullName())
		}
		if key := l.unique[fd.FullName()]; key != nil {
			seen := map[string]bool{}
			for j := range v.List().Len() {
				k := key(v.List().Get(j))
				if seen[k] {
					return fmt.Errorf("%w: %s has a duplicate entry", ErrInvalid, fd.FullName())
				}
				seen[k] = true
			}
		}
		var vals []protoreflect.Value
		switch {
		case fd.IsList():
			for j := range v.List().Len() {
				vals = append(vals, v.List().Get(j))
			}
		case fd.IsMap():
			ext := fd.Name() == "extensions" && fd.MapKey().Kind() == protoreflect.StringKind
			badKey := false
			v.Map().Range(func(k protoreflect.MapKey, e protoreflect.Value) bool {
				badKey = ext && !reverseDNS(k.String())
				vals = append(vals, e)
				return !badKey
			})
			if badKey {
				return fmt.Errorf("%w: a key of %s is not reverse-DNS", ErrInvalid, fd.FullName())
			}
			fd = fd.MapValue()
		case !fd.HasPresence() || m.Has(fd):
			vals = []protoreflect.Value{v}
		}
		for _, e := range vals {
			if err := l.value(fd, e); err != nil {
				return err
			}
		}
	}
	return nil
}

// value checks one value of field fd: a list element, a map value or a singular field.
func (l *Loader) value(fd protoreflect.FieldDescriptor, v protoreflect.Value) error {
	if ok := l.checks[fd.FullName()]; ok != nil && !ok(v) {
		return fmt.Errorf("%w: a value of %s fails its check", ErrInvalid, fd.FullName())
	}
	switch fd.Kind() {
	case protoreflect.EnumKind:
		if v.Enum() == 0 {
			return fmt.Errorf("%w: %s is %s", ErrInvalid, fd.FullName(), fd.Enum().Values().ByNumber(0).Name())
		}
	case protoreflect.MessageKind, protoreflect.GroupKind:
		if strings.HasPrefix(string(fd.Message().FullName()), "numinance.") {
			return l.walk(v.Message())
		}
	}
	return nil
}

// reverseDNS reports whether s is at least two dot-joined labels, each
// matching ^[a-z]([a-z0-9-]*[a-z0-9])?$, in at most 253 bytes: the plugin_id
// rule of numinance.plugin.v1.
func reverseDNS(s string) bool {
	labels := strings.Split(s, ".")
	if len(s) > 253 || len(labels) < 2 {
		return false
	}
	return !slices.ContainsFunc(labels, func(l string) bool {
		if l == "" || l[0] < 'a' || l[0] > 'z' || l[len(l)-1] == '-' {
			return true
		}
		return strings.ContainsFunc(l, func(r rune) bool {
			return (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-'
		})
	})
}

// CheckProduced validates a document of a produced kind (ContextBundle), which
// Decode never loads. It parses the document under the same limits as Decode
// and checks, in order: the kind is a produced kind (else ErrKind); the strict
// schema (which fixes the kind); protojson decoding; apiVersion and a present spec (and
// metadata only where the kind has that field); then the walk of a loaded
// kind: digests and other value checks, zero enums, required fields and unique
// keys, including the required fields of the bundle and the pin of
// every BundleSource.trust to TRUST_CLASS_UNTRUSTED_RETRIEVED. It returns the decoded message. Errors never quote a value of the
// document.
func (l *Loader) CheckProduced(b []byte) (proto.Message, error) {
	v, err := parse(b)
	if err != nil {
		return nil, err
	}
	name, _ := v["kind"].(string)
	e, ok := l.produced[name]
	if !ok {
		return nil, ErrKind
	}
	if err := e.schema.Validate(v); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSchema, err)
	}
	js, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSchema, err)
	}
	m := e.typ.New().Interface()
	if err := protojson.Unmarshal(js, m); err != nil {
		return nil, fmt.Errorf("%w: a value is not in protobuf JSON form", ErrSchema)
	}
	if err := checkEnvelope(m.ProtoReflect()); err != nil {
		return nil, err
	}
	// The same value checks and required fields as a loaded kind: a digest
	// that is not a digest, a zero enum or an empty required field.
	if err := l.walk(m.ProtoReflect()); err != nil {
		return nil, err
	}
	return m, nil
}
