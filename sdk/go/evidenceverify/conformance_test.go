package evidenceverify_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/numinance/numinance-spec/sdk/go/evidenceverify"
)

const vectorRoot = "../../../conformance/evidence/v1"

type vectorFile struct {
	Vector      string `json:"vector"`
	Status      string `json:"status"`
	Export      string `json:"export"`
	Materialize []struct {
		Op     string `json:"op"`
		Path   string `json:"path"`
		Target string `json:"target"`
		Size   int64  `json:"size"`
		Count  int    `json:"count"`
	} `json:"materialize"`
	Flags struct {
		RunnerKeys      []string `json:"runner_keys"`
		TrustBundleKeys bool     `json:"trust_bundle_keys"`
		SkipSignatures  bool     `json:"skip_signatures"`
		LogKey          string   `json:"log_key"`
		LogOrigin       string   `json:"log_origin"`
		AllowUnanchored bool     `json:"allow_unanchored"`
		AllowIncomplete bool     `json:"allow_incomplete"`
		AllowUnsigned   bool     `json:"allow_unsigned"`
		Complete        bool     `json:"complete"`
		ExpectChange    string   `json:"expect_change"`
		ExpectHead      string   `json:"expect_head"`
	} `json:"flags"`
	Expect struct {
		Level            *string  `json:"level"`
		ExitCode         int      `json:"exit_code"`
		FindingCodes     []string `json:"finding_codes"`
		CredentialScopes string   `json:"credential_scopes"`
		NetworkPolicy    string   `json:"network_policy"`
		Target           struct {
			LevelEnum       *string  `json:"level_enum"`
			WeakenedByFlags []string `json:"weakened_by_flags"`
		} `json:"target"`
	} `json:"expect"`
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		data, err := os.ReadFile(p) //nolint:gosec // the walk reads the vector files it lists
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600) //nolint:gosec // the target is under the test temp directory
	})
	if err != nil {
		t.Fatal(err)
	}
}

// materialize builds the directory the vector is run on and returns the path
// to hand to the verifier.
func materialize(t *testing.T, v *vectorFile) string {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, "export")
	if v.Export != "" {
		copyTree(t, filepath.Join(vectorRoot, "exports", v.Export), dir)
	}
	verifyPath := dir
	for _, s := range v.Materialize {
		switch s.Op {
		case "symlink":
			p := filepath.Join(dir, s.Path)
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(s.Target, p); err != nil {
				t.Skipf("no symbolic links: %v", err)
			}
		case "mkdir":
			if err := os.MkdirAll(filepath.Join(dir, s.Path), 0o750); err != nil {
				t.Fatal(err)
			}
		case "pad_file":
			if err := os.Truncate(filepath.Join(dir, s.Path), s.Size); err != nil {
				t.Fatal(err)
			}
		case "add_named_file":
			if err := os.WriteFile(filepath.Join(dir, s.Path), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		case "add_files":
			for i := 0; i < s.Count; i++ {
				name := filepath.Join(dir, "extra-"+strings.Repeat("0", 3-len(itoa(i)))+itoa(i)+".txt")
				if err := os.WriteFile(name, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
		case "export_path_symlink":
			link := filepath.Join(base, "link")
			if err := os.Symlink(dir, link); err != nil {
				t.Skipf("no symbolic links: %v", err)
			}
			verifyPath = link
		case "empty_path":
			verifyPath = ""
		case "no_directory":
			if err := os.RemoveAll(dir); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("unknown materialize op %q", s.Op)
		}
	}
	return verifyPath
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func options(t *testing.T, v *vectorFile) (evidenceverify.Options, error) {
	t.Helper()
	var o evidenceverify.Options
	f := v.Flags
	for _, k := range f.RunnerKeys {
		key, err := evidenceverify.ReadKeyFile(filepath.Join(vectorRoot, k))
		if err != nil {
			return o, err
		}
		o.RunnerKeys = append(o.RunnerKeys, key)
	}
	if f.LogKey != "" {
		key, err := evidenceverify.ReadKeyFile(filepath.Join(vectorRoot, f.LogKey))
		if err != nil {
			return o, err
		}
		o.LogKey = key
	}
	o.LogOrigin = f.LogOrigin
	o.TrustBundleKeys, o.SkipSignatures = f.TrustBundleKeys, f.SkipSignatures
	o.AllowUnanchored, o.AllowIncomplete, o.AllowUnsigned, o.Complete = f.AllowUnanchored, f.AllowIncomplete, f.AllowUnsigned, f.Complete
	o.ExpectChange, o.ExpectHead = f.ExpectChange, f.ExpectHead
	return o, nil
}

// TestConformance_Vectors runs every vector of conformance/evidence/v1 through
// the verifier and asserts the exit code, level, finding codes, ok flag, notes
// and the NEW report fields exactly as expected.json states.
func TestConformance_Vectors(t *testing.T) {
	names, err := os.ReadDir(filepath.Join(vectorRoot, "vectors"))
	if err != nil {
		t.Fatal(err)
	}
	ran := 0
	for _, e := range names {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(vectorRoot, "vectors", e.Name(), "expected.json"))
		if err != nil {
			t.Fatal(err)
		}
		var v vectorFile
		if err := json.Unmarshal(data, &v); err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		ran++
		t.Run(v.Status+"/"+v.Vector, func(t *testing.T) {
			path := materialize(t, &v)
			opts, err := options(t, &v)
			var rep *evidenceverify.Report
			if err == nil {
				rep, err = evidenceverify.Verify(context.Background(), path, opts)
			}
			if got := evidenceverify.ExitCode(rep, err); got != v.Expect.ExitCode {
				t.Fatalf("exit code = %d, want %d (err %v)", got, v.Expect.ExitCode, err)
			}
			if v.Expect.Level == nil {
				if rep != nil {
					t.Fatalf("a report was produced, want none: %s", rep.Level)
				}
				return
			}
			if rep == nil {
				t.Fatalf("no report (err %v), want level %q", err, *v.Expect.Level)
			}
			if rep.Level != *v.Expect.Level {
				t.Errorf("level = %q, want %q (findings %v)", rep.Level, *v.Expect.Level, rep.Findings)
			}
			if got, want := rep.FindingCodes(), nonNil(v.Expect.FindingCodes); !slices.Equal(got, want) {
				t.Errorf("finding codes = %v, want %v", got, want)
			}
			if rep.OK != (v.Expect.ExitCode == 0) {
				t.Errorf("ok = %v with exit code %d", rep.OK, v.Expect.ExitCode)
			}
			if n := v.Expect.CredentialScopes; n != "" && strings.HasPrefix(rep.CredentialScopes, "unsourced") != (n == "unsourced") {
				t.Errorf("credential_scopes note = %q, want %s", rep.CredentialScopes, n)
			}
			if n := v.Expect.NetworkPolicy; n != "" && strings.HasPrefix(rep.NetworkPolicy, "unsourced") != (n == "unsourced") {
				t.Errorf("network_policy note = %q, want %s", rep.NetworkPolicy, n)
			}
			if tg := v.Expect.Target; tg.LevelEnum != nil {
				if rep.LevelEnum != *tg.LevelEnum {
					t.Errorf("level_enum = %q, want %q", rep.LevelEnum, *tg.LevelEnum)
				}
				if !slices.Equal(rep.WeakenedByFlags, nonNil(tg.WeakenedByFlags)) {
					t.Errorf("weakened_by_flags = %v, want %v", rep.WeakenedByFlags, tg.WeakenedByFlags)
				}
			}
		})
	}
	if ran < 270 {
		t.Errorf("ran %d vectors, want at least 270", ran)
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
