package provider_test

import (
	"errors"
	"testing"

	ticketingkit "github.com/numinance/numinance-spec/conformance/ticketing/v1"
)

// TestConformance_ReplayingJiraProviderPassesEveryCase runs the ticketing kit against the replaying build of the provider: all
// 24 cases, none skipped. The cassette is one tracker whose state moves from case to case (a transition, a link and a comment
// change what the next read answers), so the cases run in the kit's order against one provider.
func TestConformance_ReplayingJiraProviderPassesEveryCase(t *testing.T) {
	cases := ticketingkit.Cases()
	if len(cases) != 24 {
		t.Fatalf("the kit has %d cases, want 24", len(cases))
	}
	s := subject(newTarget(t))
	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			err := c.Check(t.Context(), s)
			if errors.Is(err, ticketingkit.ErrNotApplicable) {
				t.Fatalf("%s (%s) was skipped: %v; the Jira provider must run every case", c.ID, c.Name, err)
			}
			if err != nil {
				t.Errorf("%s (%s): %v", c.ID, c.Name, err)
			}
		})
	}
}
