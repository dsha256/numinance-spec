package kindyaml_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	kindsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/kinds/v1"
	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
)

// TestLoader_Decode_PlaybookRollout pins the optional PlaybookSpec.rollout: the example decodes with the plan, a playbook
// without one still decodes with none (the release then stops for a human), and each case breaks the example in one place.
func TestLoader_Decode_PlaybookRollout(t *testing.T) {
	l := newLoader(t)
	doc := example(t, "Playbook/service-release.yaml")
	m, err := l.Decode([]byte(doc))
	if err != nil {
		t.Fatalf("Decode(service-release) = %v", err)
	}
	r := m.(*kindsv1.Playbook).GetSpec().GetRollout()
	if r.GetStaging() != "staging" || r.GetProduction() != "production" || len(r.GetStages()) != 3 || r.GetWindow().GetSeconds() != 900 {
		t.Errorf("Decode(service-release) rollout = %v, want staging, production, 3 stages and a 900s window", r)
	}
	if got := r.GetStages()[2].GetWeight(); got != 100 {
		t.Errorf("last stage weight = %d, want 100", got)
	}
	plain, err := l.Decode([]byte(example(t, "Playbook/dependency-upgrade.yaml")))
	if err != nil {
		t.Fatalf("Decode(dependency-upgrade) = %v", err)
	}
	if plain.(*kindsv1.Playbook).GetSpec().GetRollout() != nil {
		t.Error("a playbook without rollout decodes with one, want none: silence must not become a plan")
	}

	const firstStage = "      - {name: ten-percent, weight: 10}\n"
	var many strings.Builder
	for i := 1; i <= 8; i++ {
		fmt.Fprintf(&many, "      - {name: s%d, weight: %d}\n", i, i)
	}
	tests := []struct {
		name string
		old  string
		new  string
		want error
	}{
		{name: "staging the same as production", old: "staging: staging", new: "staging: production", want: kindyaml.ErrInvalid},
		{name: "empty staging", old: "staging: staging", new: `staging: ""`, want: kindyaml.ErrInvalid},
		{name: "upper-case environment", old: "production: production", new: "production: Production", want: kindyaml.ErrInvalid},
		{name: "environment with a colon", old: "production: production", new: `production: "prod:eu"`, want: kindyaml.ErrInvalid},
		{name: "no stages", old: "    stages:\n" + firstStage + "      - {name: half, weight: 50}\n      - {name: all, weight: 100}\n", new: "    stages: []\n", want: kindyaml.ErrInvalid},
		{name: "stages end below 100", old: "{name: all, weight: 100}", new: "{name: all, weight: 90}", want: kindyaml.ErrInvalid},
		{name: "weight over 100", old: "{name: all, weight: 100}", new: "{name: all, weight: 101}", want: kindyaml.ErrInvalid},
		{name: "weight of zero", old: "{name: ten-percent, weight: 10}", new: "{name: ten-percent, weight: 0}", want: kindyaml.ErrInvalid},
		{name: "weights that do not rise", old: "{name: half, weight: 50}", new: "{name: half, weight: 10}", want: kindyaml.ErrInvalid},
		{name: "weights that fall", old: "{name: half, weight: 50}", new: "{name: half, weight: 5}", want: kindyaml.ErrInvalid},
		{name: "duplicate stage name", old: "{name: half, weight: 50}", new: "{name: ten-percent, weight: 50}", want: kindyaml.ErrInvalid},
		{name: "empty stage name", old: "{name: half, weight: 50}", new: `{name: "", weight: 50}`, want: kindyaml.ErrInvalid},
		{name: "stage without a weight", old: "{name: half, weight: 50}", new: "{name: half}", want: kindyaml.ErrSchema},
		{name: "fractional weight", old: "{name: half, weight: 50}", new: "{name: half, weight: 50.5}", want: kindyaml.ErrSchema},
		{name: "more than eight stages", old: firstStage, new: many.String() + firstStage, want: kindyaml.ErrInvalid},
		{name: "no window", old: "    window: 900s\n", new: "", want: kindyaml.ErrInvalid},
		{name: "window of zero", old: "window: 900s", new: "window: 0s", want: kindyaml.ErrInvalid},
		{name: "window below the minimum", old: "window: 900s", new: "window: 59s", want: kindyaml.ErrInvalid},
		{name: "window above the maximum", old: "window: 900s", new: "window: 86401s", want: kindyaml.ErrInvalid},
		{name: "negative window", old: "window: 900s", new: "window: -900s", want: kindyaml.ErrInvalid},
		{name: "window not in protobuf JSON form", old: "window: 900s", new: "window: 15m", want: kindyaml.ErrSchema},
		{name: "unknown rollout key", old: "    window: 900s\n", new: "    window: 900s\n    auto_promote: true\n", want: kindyaml.ErrSchema},
		{name: "the minimum window", old: "window: 900s", new: "window: 60s", want: nil},
		{name: "the maximum window", old: "window: 900s", new: "window: 86400s", want: nil},
		{name: "one stage at 100", old: firstStage + "      - {name: half, weight: 50}\n", new: "", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(doc, tt.old) {
				t.Fatalf("example has no %q", tt.old)
			}
			d := strings.Replace(doc, tt.old, tt.new, 1)
			if m, err := l.Decode([]byte(d)); !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
				t.Errorf("Decode(%q) = %v, %v; want error %v", d, m, err, tt.want)
			}
		})
	}
}
