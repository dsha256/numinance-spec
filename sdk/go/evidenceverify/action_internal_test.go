package evidenceverify

import "testing"

// A tool gateway receipt names the tool capability tool.<plugin>.<name> (design section 11): the open verifier must read the
// chain the engine writes, so the three-segment form is an action and the malformed ones are not.
func TestIsAction_ToolCapabilities(t *testing.T) {
	tests := []struct {
		action string
		want   bool
	}{
		{"tool.jenkins.get_build_log", true},
		{"tool.mcp-datadog.create_datadog_notebook", true},
		{"scm.open_pull_request", true},
		{"ChangeClosed.v1", true},
		{"tool.jenkins", true}, // the two-segment capability port.verb
		{"tool.Jenkins.get", false},
		{"tool.jenkins-.get", false},
		{"tool..get", false},
		{"tool.jenkins.get.more", false},
		{"other.jenkins.get", false},
		{"tool.jenkins.Get", false},
	}
	for _, tt := range tests {
		if got := isAction(tt.action); got != tt.want {
			t.Errorf("isAction(%q) = %v, want %v", tt.action, got, tt.want)
		}
	}
	if !isCapability("tool.jenkins.get_build_log") {
		t.Error("isCapability(tool.jenkins.get_build_log) = false, want true: a tool receipt needs a decision and a grant like any capability")
	}
}
