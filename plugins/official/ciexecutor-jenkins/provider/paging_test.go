package provider_test

import (
	"net/http"
	"strings"
	"testing"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
)

// startedRun starts a run under key and moves it to a build, then buries it under n newer builds of other runs.
func startedRun(t *testing.T, f *fake, key string, newer int) (id string) {
	t.Helper()
	s := f.server(&host{})
	resp, err := start(t, s, key)
	if err != nil {
		t.Fatalf("StartOperation() = %v", err)
	}
	f.run()
	f.newer(newer)
	return resp.GetOperationId()
}

func TestRuns_PagesThroughOlderBuilds(t *testing.T) {
	t.Parallel()
	t.Run("a run on the third page is adopted and no second build is requested", func(t *testing.T) {
		t.Parallel()
		f := newFake()
		id := startedRun(t, f, "k1", 250)
		posts, gets := f.posts(), f.buildGets()
		resp, err := start(t, f.server(&host{}), "k1")
		if err != nil {
			t.Fatalf("StartOperation(repeat) = %v, want the run adopted", err)
		}
		if !resp.GetExisting() || resp.GetOperationId() != id {
			t.Errorf("StartOperation(repeat) = existing %v id %q, want existing with id %q", resp.GetExisting(), resp.GetOperationId(), id)
		}
		if got := f.posts(); got != posts {
			t.Errorf("POSTs after the repeat = %d, want %d: no second build", got, posts)
		}
		if got := f.buildGets() - gets; got != 3 {
			t.Errorf("GETs of the build list by the repeat = %d, want 3 pages", got)
		}
	})
	t.Run("a run on the tenth page is still found", func(t *testing.T) {
		t.Parallel()
		f := newFake()
		id := startedRun(t, f, "k1", 999)
		got, err := get(t, f.server(&host{}), id)
		if err != nil {
			t.Fatalf("GetOperation() = %v, want the run at build 1000", err)
		}
		if got.GetState().String() != "OPERATION_STATE_RUNNING" {
			t.Errorf("GetOperation().State = %s, want RUNNING", got.GetState())
		}
	})
	t.Run("a run beyond 1000 builds fails closed and starts nothing", func(t *testing.T) {
		t.Parallel()
		f := newFake()
		startedRun(t, f, "k1", 1000)
		posts, gets := f.posts(), f.buildGets()
		_, err := start(t, f.server(&host{}), "k1")
		if got := reason(t, err); got != pluginv1.Reason_REASON_LIMIT_EXCEEDED {
			t.Errorf("StartOperation(repeat) = %s, want LIMIT_EXCEEDED", got)
		}
		if got := f.posts(); got != posts {
			t.Errorf("POSTs after the repeat = %d, want %d", got, posts)
		}
		if got := f.buildGets() - gets; got != 10 {
			t.Errorf("GETs of the build list by the repeat = %d, want 10 pages", got)
		}
	})
	t.Run("a short last page ends the paging", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name  string
			n     int
			pages int
		}{
			{"empty list", 0, 1},
			{"fifty builds", 50, 1},
			{"exactly one full page", 100, 2},
			{"one build over a page", 101, 2},
			{"two full pages", 200, 3},
		} {
			f := newFake()
			f.pad(tc.n)
			_, err := get(t, f.server(&host{}), strings.Repeat("a", 32)+".release")
			if got := reason(t, err); got != pluginv1.Reason_REASON_NOT_FOUND {
				t.Errorf("%s: GetOperation() = %s, want NOT_FOUND", tc.name, got)
			}
			if got := f.buildGets(); got != tc.pages {
				t.Errorf("%s: GETs of the build list = %d, want %d", tc.name, got, tc.pages)
			}
		}
	})
	t.Run("a malformed answer for a later page is an upstream error", func(t *testing.T) {
		t.Parallel()
		f := newFake()
		f.pad(100)
		f.override = func(r *http.Request) *http.Response {
			if strings.HasSuffix(r.URL.Query().Get("tree"), "{100,200}") {
				return answer(200, `{"builds":"not a list"}`)
			}
			return nil
		}
		_, err := get(t, f.server(&host{}), strings.Repeat("a", 32)+".release")
		if got := reason(t, err); got != pluginv1.Reason_REASON_UPSTREAM_ERROR {
			t.Errorf("GetOperation() = %s, want UPSTREAM_ERROR", got)
		}
	})
	t.Run("a page larger than the range asked for is an upstream error", func(t *testing.T) {
		t.Parallel()
		f := newFake()
		f.pad(150)
		f.override = func(r *http.Request) *http.Response {
			if strings.HasPrefix(r.URL.Query().Get("tree"), "builds") {
				var b strings.Builder
				b.WriteString(`{"builds":[`)
				for i := range 101 {
					if i > 0 {
						b.WriteString(",")
					}
					b.WriteString(`{"number":1,"building":false,"result":"SUCCESS","timestamp":1,"duration":1,"actions":[]}`)
				}
				b.WriteString("]}")
				return answer(200, b.String())
			}
			return nil
		}
		_, err := get(t, f.server(&host{}), strings.Repeat("a", 32)+".release")
		if got := reason(t, err); got != pluginv1.Reason_REASON_UPSTREAM_ERROR {
			t.Errorf("GetOperation() = %s, want UPSTREAM_ERROR", got)
		}
	})
}
