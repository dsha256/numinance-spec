package ghprovider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// Interaction is one recorded HTTP exchange. It holds no header: the provider sends the credential handle in a header and
// a recording never carries it, and the answer bodies hold no token, e-mail address or other personal data (the file that
// holds them says how each was redacted).
type Interaction struct {
	Method string `json:"method"`
	// Path is the request path and query as the client sent it, after BaseURL.
	Path string `json:"path"`
	// Request is the JSON body the client sent; null for none.
	Request json.RawMessage `json:"request,omitempty"`
	Status  int             `json:"status"`
	// Body is the JSON answer.
	Body json.RawMessage `json:"body,omitempty"`
}

// Cassette is a set of recorded exchanges.
type Cassette struct {
	// Redaction says how the recording was made safe to commit.
	Redaction    string        `json:"redaction"`
	Interactions []Interaction `json:"interactions"`
}

// ParseCassette reads a cassette, refusing one with an unknown field, an exchange with no method or path, or a status
// outside 100 to 599.
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
	}
	return c, nil
}

// ErrNoRecording is the error of a request that no recorded exchange answers. A replay fails closed: it never invents an
// answer and never passes a request to the network.
var ErrNoRecording = errors.New("no recorded exchange answers this request")

// Replay is an http.RoundTripper over cassettes. Exchanges with the same method, path and request body answer in the order
// they were recorded, and the last of them answers every later request, so a poll that sees the same state many times needs
// one recording.
type Replay struct {
	mu     sync.Mutex
	queues map[string][]Interaction
}

// NewReplay returns the transport over the exchanges of cassettes.
func NewReplay(cassettes ...Cassette) (*Replay, error) {
	r := &Replay{queues: map[string][]Interaction{}}
	for _, c := range cassettes {
		for _, in := range c.Interactions {
			k, err := key(in.Method, in.Path, in.Request)
			if err != nil {
				return nil, err
			}
			r.queues[k] = append(r.queues[k], in)
		}
	}
	return r, nil
}

// key is the identity of a request: the JSON body is compared as a value, so key order and white space do not matter.
func key(method, path string, body []byte) (string, error) {
	canon := ""
	if len(bytes.TrimSpace(body)) > 0 && string(bytes.TrimSpace(body)) != "null" {
		var v any
		if err := json.Unmarshal(body, &v); err != nil {
			return "", fmt.Errorf("cassette: a request body is not JSON: %w", err)
		}
		b, err := json.Marshal(v) // encoding/json sorts map keys
		if err != nil {
			return "", err
		}
		canon = string(b)
	}
	return method + " " + path + " " + canon, nil
}

// RoundTrip implements http.RoundTripper.
func (r *Replay) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(io.LimitReader(req.Body, MaxRequestBytes+1))
		_ = req.Body.Close()
		if err != nil || len(body) > MaxRequestBytes {
			return nil, ErrNoRecording
		}
	}
	path := req.URL.EscapedPath()
	if req.URL.RawQuery != "" {
		path += "?" + req.URL.RawQuery
	}
	k, err := key(req.Method, path, body)
	if err != nil {
		return nil, ErrNoRecording
	}
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
	resp := &http.Response{
		StatusCode: in.Status, Status: http.StatusText(in.Status), Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewReader(in.Body)),
		ContentLength: int64(len(in.Body)), Request: req,
	}
	return resp, nil
}
