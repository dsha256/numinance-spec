// Package replay serves recorded, redacted Jenkins exchanges to the ciexecutor-jenkins provider, so the unit tests, the
// conformance kit and any host-level acceptance need no network and no credential (ADR 0015, replay mode). A cassette is a list of
// HTTP exchanges (method, path with query, urlencoded form, status, headers, JSON answer); a request with no recorded exchange
// fails closed. The format differs from the GitHub providers' only in what Jenkins needs: the answer headers (Location,
// Content-Length) and a urlencoded request body.
package replay

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/numinance/numinance-spec/plugins/official/ciexecutor-jenkins/provider"
	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

//go:embed cassettes/*.json
var cassettes embed.FS

// Fixed values of the cassette: the repository and the commit its answers name.
const (
	Repository = "acme/shop"
	Revision   = "c0ffee00c0ffee00c0ffee00c0ffee00c0ffee01"
)

// Interaction is one recorded HTTP exchange. It holds no request header: the provider sends the credential handle and the crumb in
// headers and a recording never carries them.
type Interaction struct {
	Method string `json:"method"`
	// Path is the request path and query as the client sent it, after the server's base URL. The query is compared as a set of
	// values, so the order of its parameters does not matter.
	Path string `json:"path"`
	// Form is the urlencoded request body; empty for none. It is compared as a set of values too.
	Form   string `json:"form,omitempty"`
	Status int    `json:"status"`
	// Headers are the answer headers a recording keeps: Location and Content-Length only.
	Headers map[string]string `json:"headers,omitempty"`
	// Body is the JSON answer.
	Body json.RawMessage `json:"body,omitempty"`
}

// Cassette is a set of recorded exchanges.
type Cassette struct {
	// Redaction says how the recording was made safe to commit.
	Redaction    string        `json:"redaction"`
	Interactions []Interaction `json:"interactions"`
}

// ParseCassette reads a cassette, refusing one with an unknown field, an exchange with no method or path, a status outside 100 to 599 or
// a header other than the two a recording keeps.
func ParseCassette(b []byte) (Cassette, error) {
	var c Cassette
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Cassette{}, fmt.Errorf("cassette: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Cassette{}, errors.New("cassette: there is more than one JSON value in the file")
	}
	if c.Redaction == "" {
		return Cassette{}, errors.New("cassette: it does not say how it was redacted")
	}
	for i, in := range c.Interactions {
		if in.Method == "" || in.Path == "" || in.Status < 100 || in.Status > 599 {
			return Cassette{}, fmt.Errorf("cassette: interaction %d is not complete", i)
		}
		for k := range in.Headers {
			if k != "Location" && k != "Content-Length" {
				return Cassette{}, fmt.Errorf("cassette: interaction %d keeps a header other than Location and Content-Length", i)
			}
		}
	}
	return c, nil
}

// ErrNoRecording is the error of a request that no recorded exchange answers. A replay fails closed: it never invents an answer and
// never passes a request to the network.
var ErrNoRecording = errors.New("no recorded exchange answers this request")

// Replay is an http.RoundTripper over cassettes. Exchanges with the same method, path and form answer in the order they were
// recorded, and the last of them answers every later request, so a poll that sees the same state many times needs one recording.
type Replay struct {
	mu     sync.Mutex
	queues map[string][]Interaction
}

// NewReplay returns the transport over the exchanges of cassettes.
func NewReplay(cassettes ...Cassette) *Replay {
	r := &Replay{queues: map[string][]Interaction{}}
	for _, c := range cassettes {
		for _, in := range c.Interactions {
			k := key(in.Method, in.Path, in.Form)
			r.queues[k] = append(r.queues[k], in)
		}
	}
	return r
}

// key is the identity of a request: the query and the form are compared as values, so their order does not matter.
func key(method, path, form string) string {
	p, q, _ := cut(path)
	if vals, err := url.ParseQuery(q); err == nil {
		q = vals.Encode()
	}
	f := form
	if vals, err := url.ParseQuery(form); err == nil {
		f = vals.Encode()
	}
	return method + " " + p + "?" + q + " " + f
}

func cut(s string) (before, after string, found bool) {
	for i := range len(s) {
		if s[i] == '?' {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

// RoundTrip implements http.RoundTripper.
func (r *Replay) RoundTrip(req *http.Request) (*http.Response, error) {
	var form []byte
	if req.Body != nil {
		var err error
		form, err = io.ReadAll(io.LimitReader(req.Body, ghprovider.MaxRequestBytes+1))
		_ = req.Body.Close()
		if err != nil || len(form) > ghprovider.MaxRequestBytes {
			return nil, ErrNoRecording
		}
	}
	k := key(req.Method, req.URL.EscapedPath()+"?"+req.URL.RawQuery, string(form))
	r.mu.Lock()
	q := r.queues[k]
	var in Interaction
	switch {
	case len(q) == 0:
		r.mu.Unlock()
		return nil, ErrNoRecording
	case len(q) == 1:
		in = q[0]
	default:
		in, r.queues[k] = q[0], q[1:]
	}
	r.mu.Unlock()
	h := http.Header{"Content-Type": {"application/json"}}
	for k, v := range in.Headers {
		h.Set(k, v)
	}
	body := in.Body
	if req.Method == http.MethodHead {
		body = nil
	}
	resp := &http.Response{
		StatusCode: in.Status, Status: http.StatusText(in.Status), Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header: h, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)), Request: req,
	}
	if cl, err := strconv.ParseInt(in.Headers["Content-Length"], 10, 64); err == nil {
		resp.ContentLength = cl
	}
	return resp, nil
}

// Transport returns the cassette transport over every embedded cassette, as the provider's transport in replay mode.
func Transport() (provider.Transport, error) {
	entries, err := cassettes.ReadDir("cassettes")
	if err != nil {
		return provider.Transport{}, err
	}
	var all []Cassette
	for _, e := range entries {
		b, err := cassettes.ReadFile("cassettes/" + e.Name())
		if err != nil {
			return provider.Transport{}, err
		}
		c, err := ParseCassette(b)
		if err != nil {
			return provider.Transport{}, fmt.Errorf("%s: %w", e.Name(), err)
		}
		all = append(all, c)
	}
	return provider.Transport{RT: NewReplay(all...), Replay: true}, nil
}
