package evidenceverify

import (
	"testing"
)

// snapshotJSON is one canonical snapshot with the candidate text substituted.
func snapshotJSON(candidate string) string {
	return `{"export_version":1,"snapshots":[{"as_of":7,"candidates":[` + candidate + `],"language":"go","n":100,"task_class":"bugfix"}]}`
}

func TestRoutePerformance_Document_Shape(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		ok   bool
	}{
		{"a candidate with a figure", snapshotJSON(`{"executor":"a","figure":{"accepted":90,"n":100},"model":"m","rank":1}`), true},
		{"a candidate with no figure", snapshotJSON(`{"executor":"a","model":"m","rank":1}`), true},
		{"no candidates is a shape the completeness check judges", snapshotJSON(``), true},
		{"a rank of zero", snapshotJSON(`{"executor":"a","model":"m","rank":0}`), false},
		{"a figure of no outcomes", snapshotJSON(`{"executor":"a","figure":{"accepted":0,"n":0},"model":"m","rank":1}`), false},
		{"an empty executor", snapshotJSON(`{"executor":"","model":"m","rank":1}`), false},
		{"an unknown key", snapshotJSON(`{"executor":"a","model":"m","rank":1,"x":1}`), false},
		{"a figure that is null", snapshotJSON(`{"executor":"a","figure":null,"model":"m","rank":1}`), false},
		{"a snapshot with white space", "{\"export_version\": 1,\"snapshots\":[]}", false},
		{"an as_of over 2^53", `{"export_version":1,"snapshots":[{"as_of":9007199254740992,"candidates":[],"language":"go","n":100,"task_class":"bugfix"}]}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeDoc([]byte(tt.doc), maxDoc, routePerformanceFields())
			if (err == nil) != tt.ok {
				t.Errorf("decodeDoc(%s) error = %v, want ok %v", tt.name, err, tt.ok)
			}
		})
	}
}

func TestRoutePerformance_Figures_CannotAcceptMoreThanTheyCount(t *testing.T) {
	good, err := decodeDoc([]byte(snapshotJSON(`{"executor":"a","figure":{"accepted":100,"n":100},"model":"m","rank":1}`)), maxDoc, routePerformanceFields())
	if err != nil {
		t.Fatal(err)
	}
	bad, err := decodeDoc([]byte(snapshotJSON(`{"executor":"a","figure":{"accepted":101,"n":100},"model":"m","rank":1}`)), maxDoc, routePerformanceFields())
	if err != nil {
		t.Fatal(err)
	}
	if !snapshotFiguresOK(good.list("snapshots")[0]) {
		t.Error("snapshotFiguresOK(100 of 100) = false, want true")
	}
	if snapshotFiguresOK(bad.list("snapshots")[0]) {
		t.Error("snapshotFiguresOK(101 of 100) = true, want false")
	}
}
