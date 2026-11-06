package authorexecutor

import (
	"errors"
	"fmt"
	"regexp"

	"google.golang.org/protobuf/reflect/protoreflect"

	authorexecutorv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/authorexecutor/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

// planResponseName is checked before a stream message is reinterpreted as a PlanResponse.
const planResponseName protoreflect.FullName = "numinance.ports.authorexecutor.v1.PlanResponse"

// NewPlanRule returns the plugin.PortRule of one Plan stream: the rule of NewExecuteRule over PlanResponse messages, and a
// FILE_CHANGED event breaks the stream, because a planning run is read only (conformance cases PLAN-001 and PLAN-002).
func NewPlanRule() plugin.PortRule { return &executeRule{plan: true} }

var (
	changeID = regexp.MustCompile(`^chg_[0-9a-f]{32}$`)
	runID    = regexp.MustCompile(`^run_[0-9a-f]{32}$`)
	grantID  = regexp.MustCompile(`^grant_[0-9a-f]{32}$`)
)

// ValidatePlanRequest checks the shape of a PlanRequest the host sends (conformance case PLAN-003): the request is set, it names
// no work unit, its identifiers are well formed, plan_ref names the task, allowed_paths is empty, no credential capability is
// named, and no allowed capability is unknown or writes (FILESYSTEM_WRITE, SHELL and NETWORK write). An adapter refuses a request
// that fails it with INVALID_ARGUMENT before it runs anything. The checks that need the host's state stay with the host.
func ValidatePlanRequest(req *authorexecutorv1.PlanRequest) error {
	r := req.GetRequest()
	switch {
	case r == nil:
		return errors.New("request unset")
	case r.GetWorkUnitId() != "":
		return errors.New("work_unit_id is set: a planning run has no work unit")
	case !changeID.MatchString(r.GetChangeId()):
		return errors.New("change_id is malformed")
	case !runID.MatchString(r.GetRunId()):
		return errors.New("run_id is malformed")
	case !grantID.MatchString(r.GetGrantId()):
		return errors.New("grant_id is malformed")
	case r.GetPlanRef().GetArtifactRef() == "" || r.GetPlanRef().GetSha256() == nil:
		return errors.New("plan_ref unset: it names the task of the planning run")
	case len(r.GetAllowedPaths()) != 0:
		return errors.New("allowed_paths is not empty: a planning run writes nothing")
	case len(r.GetCredentialCapabilities()) != 0:
		return errors.New("credential_capabilities is not empty: a planning run asks for no credential")
	}
	seen := make(map[typesv1.ExecutorCapability]bool, len(r.GetAllowedCapabilities()))
	for _, c := range r.GetAllowedCapabilities() {
		switch {
		case c == typesv1.ExecutorCapability_EXECUTOR_CAPABILITY_UNSPECIFIED || typesv1.ExecutorCapability_EXECUTOR_CAPABILITY_UNSPECIFIED.Descriptor().Values().ByNumber(c.Number()) == nil:
			return fmt.Errorf("allowed_capabilities holds an unknown capability %d", c)
		case seen[c]:
			return fmt.Errorf("allowed_capabilities repeats %s", c)
		case c == typesv1.ExecutorCapability_EXECUTOR_CAPABILITY_FILESYSTEM_WRITE || c == typesv1.ExecutorCapability_EXECUTOR_CAPABILITY_SHELL || c == typesv1.ExecutorCapability_EXECUTOR_CAPABILITY_NETWORK:
			return fmt.Errorf("allowed_capabilities holds %s, which writes", c)
		}
		seen[c] = true
	}
	return nil
}
