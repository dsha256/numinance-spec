package ghprovider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// tokenPattern matches the forms a GitHub credential takes (classic and fine-grained personal access tokens, OAuth, user,
// server and refresh tokens). A recording never contains one: a match is replaced.
var tokenPattern = regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{16,}|github_pat_[A-Za-z0-9_]{20,})\b`)

// emailPattern matches an e-mail address, which a recording replaces: it is personal data.
var emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)

// Rename is a name the recording replaces everywhere (a repository, an owner, a login) so no real name is committed.
type Rename struct{ From, To string }

// Redactor makes a recorded exchange safe to commit.
type Redactor struct{ Renames []Rename }

// ParseRename parses "from=to"; both sides must be non-empty and the replacement must not contain the original.
func ParseRename(s string) (Rename, error) {
	from, to, ok := strings.Cut(s, "=")
	if !ok || from == "" || to == "" || strings.Contains(to, from) {
		return Rename{}, errors.New("a rename is from=to, both non-empty, and the replacement does not contain the original")
	}
	return Rename{From: from, To: to}, nil
}

// Scrub applies the renames (longest original first, so "acme/shop" is replaced before "acme"), then replaces credential
// patterns and e-mail addresses. Renames come first, so a replacement cannot reintroduce what the later passes remove.
func (r Redactor) Scrub(in string) string {
	renames := append([]Rename(nil), r.Renames...)
	sort.SliceStable(renames, func(i, j int) bool { return len(renames[i].From) > len(renames[j].From) })
	for _, rn := range renames {
		in = strings.ReplaceAll(in, rn.From, rn.To)
	}
	in = tokenPattern.ReplaceAllString(in, "REDACTED")
	return emailPattern.ReplaceAllString(in, "redacted@example.invalid")
}

// Describe is the cassette's "redaction" field for a recording made with r.
func (r Redactor) Describe() string {
	// The originals are never named: the description is committed with the recording.
	names := fmt.Sprintf("%d names renamed", len(r.Renames))
	return "Recorded from the live service by the scm-github recording build. No header is recorded (the handle and every credential travel in headers). " +
		"Credential patterns and e-mail addresses are replaced; " + names + "."
}

// Recorder is an http.RoundTripper that sends each request through Inner and appends the redacted exchange to the cassette
// file Out, which it rewrites whole (mode 0600, through a temporary file and a rename) after every exchange, so a provider
// that is killed leaves a complete file. It records what the provider's own client sent and received and never a header.
type Recorder struct {
	Inner    http.RoundTripper
	Out      string
	Redactor Redactor

	mu   sync.Mutex
	list []Interaction
}

// RoundTrip implements http.RoundTripper. The response is returned to the caller unchanged and unredacted; the recording is
// the redacted copy. An exchange that cannot be recorded (not JSON, an unwritable file)
// is an error: the provider fails the call rather than let a run go unrecorded.
func (rec *Recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	var reqBody []byte
	if req.Body != nil {
		var err error
		reqBody, err = io.ReadAll(io.LimitReader(req.Body, MaxRequestBytes+1))
		_ = req.Body.Close()
		if err != nil || len(reqBody) > MaxRequestBytes {
			return nil, errors.New("the request body is unreadable or over its bound")
		}
		req = req.Clone(req.Context())
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
		req.ContentLength = int64(len(reqBody))
	}
	resp, err := rec.Inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	if len(raw) > MaxResponseBytes {
		return resp, nil // the client refuses it; nothing is recorded
	}
	path := req.URL.EscapedPath()
	if req.URL.RawQuery != "" {
		path += "?" + req.URL.RawQuery
	}
	in, err := rec.interaction(req.Method, path, reqBody, resp.StatusCode, raw)
	if err != nil {
		return nil, err
	}
	if err := rec.append(in); err != nil {
		return nil, err
	}
	return resp, nil
}

func (rec *Recorder) interaction(method, path string, reqBody []byte, status int, body []byte) (Interaction, error) {
	var err error
	in := Interaction{Method: method, Path: rec.Redactor.Scrub(path), Status: status}
	if in.Request, err = rec.scrubJSON(reqBody); err != nil {
		return Interaction{}, err
	}
	if in.Body, err = rec.scrubJSON(body); err != nil {
		return Interaction{}, err
	}
	return in, nil
}

// scrubJSON redacts a JSON document; an empty one stays empty and a document that is not JSON is refused.
func (rec *Recorder) scrubJSON(b []byte) (json.RawMessage, error) {
	if len(bytes.TrimSpace(b)) == 0 {
		return nil, nil
	}
	if !json.Valid(b) {
		return nil, errors.New("the exchange is not JSON and is not recorded")
	}
	s := rec.Redactor.Scrub(string(b))
	if !json.Valid([]byte(s)) {
		return nil, errors.New("the redaction broke the JSON and the exchange is not recorded")
	}
	return json.RawMessage(s), nil
}

func (rec *Recorder) append(in Interaction) error {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.list = append(rec.list, in)
	b, err := json.MarshalIndent(Cassette{Redaction: rec.Redactor.Describe(), Interactions: rec.list}, "", " ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(rec.Out), ".cassette-*")
	if err != nil {
		return fmt.Errorf("the cassette cannot be written: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // a no-op after the rename
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("the cassette cannot be written: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("the cassette cannot be written: %w", err)
	}
	if err := os.Rename(tmp.Name(), rec.Out); err != nil {
		return fmt.Errorf("the cassette cannot be written: %w", err)
	}
	return nil
}
