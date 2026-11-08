// Package replay serves recorded, redacted Jira exchanges to the ticketing-jira provider, so the unit tests, the conformance kit
// and the host-level acceptance need no network and no token (ADR 0015, replay mode).
//
// A cassette is a list of HTTP exchanges: method, path, query, request body (optional), status and JSON answer. A tracker is
// stateful (a transition changes what the next read answers), so an exchange may also carry a precondition on and a change of
// a small state: "given" says which values the named variables must hold for the exchange to answer, "then" sets them
// afterwards. A variable that was never set holds its value in the cassette's "initial" map, or the empty string. Of the
// exchanges that match a request, the one with the most "given" entries answers; a request with no matching exchange fails
// closed (ErrNoRecording), so a replay never invents an answer and never reaches the network.
package replay

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// Host is the reserved example host of the embedded cassette (RFC 2606 reserves .test); it is the replay's instance host.
const Host = "acme.example.test"

//go:embed testdata/jira.json
var embedded []byte

const maxRequestBytes = 1 << 20

// Interaction is one recorded HTTP exchange. It holds no header: the provider sets none that authenticates in a replay, and a
// recording never carries one.
type Interaction struct {
	Method string `json:"method"`
	// Path is the request path as sent, escaped.
	Path string `json:"path"`
	// Query is the raw query string as sent.
	Query string `json:"query,omitempty"`
	// Request is the JSON body the client sent; absent means any body matches.
	Request json.RawMessage   `json:"request,omitempty"`
	Given   map[string]string `json:"given,omitempty"`
	Then    map[string]string `json:"then,omitempty"`
	Status  int               `json:"status"`
	// Body is the JSON answer.
	Body json.RawMessage `json:"body,omitempty"`
}

// Cassette is a set of recorded exchanges.
type Cassette struct {
	// Redaction says how the recording was made safe to commit.
	Redaction string `json:"redaction"`
	// Host is the instance host every request must name.
	Host         string            `json:"host"`
	Initial      map[string]string `json:"initial,omitempty"`
	Interactions []Interaction     `json:"interactions"`
}

// Parse reads a cassette, refusing one with an unknown field, a second JSON value, no redaction note, no host or an exchange
// that is not complete.
func Parse(b []byte) (Cassette, error) {
	var c Cassette
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Cassette{}, fmt.Errorf("cassette: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Cassette{}, errors.New("cassette: there is more than one JSON value in the file")
	}
	if c.Redaction == "" || c.Host == "" {
		return Cassette{}, errors.New("cassette: it does not say how it was redacted or for which host")
	}
	for i, in := range c.Interactions {
		if in.Method == "" || in.Path == "" || in.Status < 100 || in.Status > 599 {
			return Cassette{}, fmt.Errorf("cassette: interaction %d is not complete", i)
		}
		if _, err := canonical(in.Request); err != nil {
			return Cassette{}, fmt.Errorf("cassette: interaction %d: %w", i, err)
		}
	}
	return c, nil
}

// ErrNoRecording is the error of a request that no recorded exchange answers.
var ErrNoRecording = errors.New("no recorded exchange answers this request")

// Replay is an http.RoundTripper over a cassette.
type Replay struct {
	mu    sync.Mutex
	c     Cassette
	canon []string
	state map[string]string
}

// New returns the transport over the exchanges of c.
func New(c Cassette) (*Replay, error) {
	r := &Replay{c: c, state: map[string]string{}}
	for _, in := range c.Interactions {
		k, err := canonical(in.Request)
		if err != nil {
			return nil, err
		}
		r.canon = append(r.canon, k)
	}
	return r, nil
}

// canonical returns a JSON body as a value, so key order and white space do not matter; "" for none.
func canonical(body []byte) (string, error) {
	if t := bytes.TrimSpace(body); len(t) == 0 || string(t) == "null" {
		return "", nil
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return "", fmt.Errorf("a request body is not JSON: %w", err)
	}
	b, err := json.Marshal(v) // encoding/json sorts map keys
	return string(b), err
}

// Transport returns the cassette transport over the embedded cassette, as the provider's transport in replay mode.
func Transport() (ghprovider.Transport, error) {
	c, err := Parse(embedded)
	if err != nil {
		return ghprovider.Transport{}, err
	}
	rt, err := New(c)
	if err != nil {
		return ghprovider.Transport{}, err
	}
	return ghprovider.Transport{RT: rt, Replay: true}, nil
}

// Embedded returns the embedded cassette, for a test that checks it.
func Embedded() []byte { return bytes.Clone(embedded) }

func (r *Replay) value(name string) string {
	if v, ok := r.state[name]; ok {
		return v
	}
	return r.c.Initial[name]
}

// RoundTrip implements http.RoundTripper.
func (r *Replay) RoundTrip(req *http.Request) (*http.Response, error) {
	// A replay serves one instance over https and carries no credential: a request with a header that authenticates, or for
	// another host, is not one the cassette recorded.
	if req.URL.Scheme != "https" || req.URL.Host != r.c.Host || req.URL.User != nil ||
		req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" || req.Header.Get(ghprovider.HandleHeader) != "" {
		return nil, ErrNoRecording
	}
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(io.LimitReader(req.Body, maxRequestBytes+1))
		_ = req.Body.Close()
		if err != nil || len(body) > maxRequestBytes {
			return nil, ErrNoRecording
		}
	}
	canon, err := canonical(body)
	if err != nil {
		return nil, ErrNoRecording
	}
	r.mu.Lock()
	best := -1
	for i, in := range r.c.Interactions {
		if in.Method != req.Method || in.Path != req.URL.EscapedPath() || in.Query != req.URL.RawQuery || (r.canon[i] != "" && r.canon[i] != canon) {
			continue
		}
		ok := true
		for name, want := range in.Given {
			ok = ok && r.value(name) == want
		}
		if ok && (best < 0 || len(in.Given) > len(r.c.Interactions[best].Given)) {
			best = i
		}
	}
	if best < 0 {
		r.mu.Unlock()
		return nil, ErrNoRecording
	}
	in := r.c.Interactions[best]
	for name, v := range in.Then {
		r.state[name] = v
	}
	r.mu.Unlock()
	return &http.Response{
		StatusCode: in.Status, Status: http.StatusText(in.Status), Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewReader(in.Body)),
		ContentLength: int64(len(in.Body)), Request: req,
	}, nil
}
