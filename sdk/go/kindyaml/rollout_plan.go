package kindyaml

import (
	"fmt"
	"regexp"
	"time"

	"google.golang.org/protobuf/reflect/protoreflect"

	kindsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/kinds/v1"
)

// Bounds of a Playbook rollout plan: the observation window is long enough to see something and short enough not to hold a
// release open for ever, and a rollout has a few stages, not a schedule.
const (
	// MinRolloutWindow and MaxRolloutWindow bound PlaybookSpec.rollout.window.
	MinRolloutWindow = time.Minute
	MaxRolloutWindow = 24 * time.Hour
	// MaxRolloutStages bounds PlaybookSpec.rollout.stages.
	MaxRolloutStages = 8
)

var rolloutNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// isRolloutName reports whether s is an environment or stage name of a rollout plan: lower-case letters, digits and hyphens.
func isRolloutName(s string) bool { return rolloutNameRe.MatchString(s) }

// isRolloutWindow reports whether a google.protobuf.Duration is within [MinRolloutWindow, MaxRolloutWindow].
func isRolloutWindow(v protoreflect.Value) bool {
	d := v.Message()
	f := d.Descriptor().Fields()
	s, n := d.Get(f.ByName("seconds")).Int(), d.Get(f.ByName("nanos")).Int()
	if s < 0 || n < 0 {
		return false
	}
	// Seconds are capped first so the sum cannot overflow.
	if s > int64(MaxRolloutWindow/time.Second) {
		return false
	}
	w := time.Duration(s)*time.Second + time.Duration(n)
	return w >= MinRolloutWindow && w <= MaxRolloutWindow
}

// isRolloutWeight reports whether w is a traffic share, 1 to 100 percent.
func isRolloutWeight(v protoreflect.Value) bool { return v.Uint() >= 1 && v.Uint() <= 100 }

// checkPlaybook refuses what the field checks cannot see in a rollout plan: staging and production are two environments, the
// stages are few, uniquely named, and their weights rise strictly to 100, so a plan that passes every stage is a promotion.
func checkPlaybook(spec *kindsv1.PlaybookSpec) error {
	r := spec.GetRollout()
	if r == nil {
		return nil
	}
	if r.GetStaging() == r.GetProduction() {
		return fmt.Errorf("%w: rollout staging and production are the same environment", ErrInvalid)
	}
	if len(r.GetStages()) > MaxRolloutStages {
		return fmt.Errorf("%w: rollout has more than %d stages", ErrInvalid, MaxRolloutStages)
	}
	seen := map[string]bool{}
	var prev uint32
	for _, s := range r.GetStages() {
		if seen[s.GetName()] {
			return fmt.Errorf("%w: rollout stage names are not unique", ErrInvalid)
		}
		seen[s.GetName()] = true
		if s.GetWeight() <= prev {
			return fmt.Errorf("%w: rollout stage weights do not rise strictly", ErrInvalid)
		}
		prev = s.GetWeight()
	}
	if prev != 100 {
		return fmt.Errorf("%w: the last rollout stage does not end at weight 100", ErrInvalid)
	}
	return nil
}
