package provider

import "strings"

// ParseForTest parses a tool run the way Run does and returns the rule ids it found, joined.
func ParseForTest(check string, out Output) (string, error) {
	found, err := parse(check, out.Dir, out.Stdout)
	rules := make([]string, 0, len(found))
	for _, f := range found {
		rules = append(rules, f.rule)
	}
	return strings.Join(rules, " "), err
}
