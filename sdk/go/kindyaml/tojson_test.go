package kindyaml_test

import (
	"errors"
	"testing"

	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
)

func TestYAMLToJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
		err  error
	}{
		{name: "a mapping", in: "a: 1\nb: [x, y]\n", want: `{"a":1,"b":["x","y"]}`},
		{name: "an anchor", in: "a: &x 1\nb: *x\n", err: kindyaml.ErrUnsupported},
		{name: "a duplicate key", in: "a: 1\na: 2\n", err: kindyaml.ErrDuplicateKey},
		{name: "two documents", in: "a: 1\n---\nb: 2\n", err: kindyaml.ErrUnsupported},
		{name: "a sequence root", in: "- a\n", err: kindyaml.ErrUnsupported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := kindyaml.YAMLToJSON([]byte(tt.in))
			if !errors.Is(err, tt.err) {
				t.Fatalf("YAMLToJSON(%q) error = %v, want %v", tt.in, err, tt.err)
			}
			if tt.err == nil && string(got) != tt.want {
				t.Errorf("YAMLToJSON(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}
