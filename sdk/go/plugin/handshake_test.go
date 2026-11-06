package plugin_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/numinance/numinance-spec/sdk/go/plugin"
)

func TestParseLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want plugin.Line
		bad  bool
	}{
		{name: "go-plugin shaped line", line: "1|1|unix|/tmp/nump1/p.sock|grpc", want: plugin.Line{Core: 1, App: 1, Network: "unix", Address: "/tmp/nump1/p.sock", Protocol: "grpc"}},
		{name: "later app protocol", line: "1|2|unix|/tmp/p|grpc", want: plugin.Line{Core: 1, App: 2, Network: "unix", Address: "/tmp/p", Protocol: "grpc"}},
		{name: "core protocol 2", line: "2|1|unix|/tmp/p|grpc", bad: true},
		{name: "app protocol 0", line: "1|0|unix|/tmp/p|grpc", bad: true},
		{name: "tcp network", line: "1|1|tcp|127.0.0.1:1|grpc", bad: true},
		{name: "netrpc protocol", line: "1|1|unix|/tmp/p|netrpc", bad: true},
		{name: "go-plugin sixth field (certificate)", line: "1|1|unix|/tmp/p|grpc|MIIB", bad: true},
		{name: "four fields", line: "1|1|unix|/tmp/p", bad: true},
		{name: "relative path", line: "1|1|unix|p.sock|grpc", bad: true},
		{name: "dot dot segment", line: "1|1|unix|/tmp/../etc/p|grpc", bad: true},
		{name: "control character in the address", line: "1|1|unix|/tmp/p\x00|grpc", bad: true},
		{name: "address over the sun_path limit", line: "1|1|unix|/" + strings.Repeat("a", 120) + "|grpc", bad: true},
		{name: "line over the bound", line: "1|1|unix|/" + strings.Repeat("a", 1100) + "|grpc", bad: true},
		{name: "empty", line: "", bad: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := plugin.ParseLine(tc.line)
			if tc.bad {
				if !errors.Is(err, plugin.ErrHandshakeLine) {
					t.Errorf("ParseLine(%q) error = %v, want ErrHandshakeLine", tc.line, err)
				}
				if err != nil && strings.Contains(err.Error(), "/tmp") {
					t.Errorf("ParseLine(%q) error %q echoes the value", tc.line, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Errorf("ParseLine(%q) = %+v, %v, want %+v", tc.line, got, err, tc.want)
			}
			if got.String() != tc.line {
				t.Errorf("Line.String() = %q, want %q", got.String(), tc.line)
			}
		})
	}
}

func TestParseProtocols(t *testing.T) {
	if got, err := plugin.ParseProtocols("1,2"); err != nil || len(got) != 2 || got[1] != 2 {
		t.Errorf("ParseProtocols(1,2) = %v, %v, want [1 2]", got, err)
	}
	for _, s := range []string{"", "0", "1,1", "a", "1,,2", strings.Repeat("1,", 20) + "2"} {
		if _, err := plugin.ParseProtocols(s); err == nil {
			t.Errorf("ParseProtocols(%q) error = nil, want an error", s)
		}
	}
	if got := plugin.FormatProtocols([]uint{1, 3}); got != "1,3" {
		t.Errorf("FormatProtocols([1 3]) = %q, want 1,3", got)
	}
}
