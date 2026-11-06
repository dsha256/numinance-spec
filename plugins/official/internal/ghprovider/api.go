package ghprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
)

// Limits of what the provider reads from the code host: every answer is untrusted, so it is size-limited and
// time-bound before it is decoded.
const (
	// MaxResponseBytes bounds one HTTP answer; more is an error and nothing is decoded.
	MaxResponseBytes = 4 << 20
	// MaxRequestBytes bounds one HTTP request body the provider sends.
	MaxRequestBytes = 1 << 20
	// CallTimeout bounds one HTTP exchange.
	CallTimeout = 30 * time.Second
	// HandleHeader is the header that carries the credential handle to the egress proxy, which strips it and injects the
	// credential the broker pushed for it (internal/credentials/adapters/egress.HandleHeader).
	HandleHeader = "Numinance-Handle"
)

// BaseURL is the REST API of the code host.
const BaseURL = "https://api.github.com"

// APIError is an answer of the code host that is not a success.
type APIError struct {
	Status  int
	Message string
}

// Error implements error; it carries the status and nothing the host wrote.
func (e *APIError) Error() string { return fmt.Sprintf("the code host answered %d", e.Status) }

// Client is a REST client of the code host. It sends the credential handle in HandleHeader and nothing else that
// authenticates: there is no token in this process.
type Client struct {
	// Transport sends the request; the default client is used when it is nil. A replay provider sets the cassette transport.
	Transport http.RoundTripper
	// Handle is the credential handle of the invocation; empty sends none (a replay).
	Handle string
}

// Do sends one request with a JSON body (nil for none) and decodes a JSON answer into out (nil to discard it). A status of
// 400 or above is an *APIError; an answer over MaxResponseBytes is an error, never truncated.
func (c Client) Do(ctx context.Context, method, path string, body, out any) (int, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil || len(b) > MaxRequestBytes {
			return 0, errors.New("the request body is not encodable or over its bound")
		}
		rd = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, BaseURL+path, rd)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Handle != "" {
		req.Header.Set(HandleHeader, c.Handle)
	}
	rt := c.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}
	resp, err := (&http.Client{Transport: rt, Timeout: CallTimeout, CheckRedirect: noRedirect}).Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		return resp.StatusCode, err
	}
	if len(raw) > MaxResponseBytes {
		return resp.StatusCode, ErrOverLimit
	}
	if resp.StatusCode >= 400 {
		return resp.StatusCode, &APIError{Status: resp.StatusCode, Message: apiMessage(raw)}
	}
	if out != nil && len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, fmt.Errorf("the code host's answer is not the JSON expected: %w", err)
		}
	}
	return resp.StatusCode, nil
}

// ErrOverLimit is an answer over MaxResponseBytes.
var ErrOverLimit = errors.New("the code host's answer is over the limit")

// noRedirect refuses every redirect: a hop to another host would carry the handle header to a destination the manifest
// never named, and the egress proxy judges only the first.
func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// apiMessage reads the host's "message" for the one rule that needs it (a branch that is not protected, a reference that
// exists); it is never logged or returned.
func apiMessage(raw []byte) string {
	var m struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	return m.Message
}

// Query builds a query string from name and value pairs with url.Values.Encode, so a value that carries '&', '=' or '#' (a branch
// name is legal with all three) stays one value and never adds a parameter.
func Query(kv ...string) string {
	v := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		v.Set(kv[i], kv[i+1])
	}
	return "?" + v.Encode()
}

// Seg escapes one URL path segment of a name the request carried.
func Seg(s string) string { return url.PathEscape(s) }

// Failure maps an error of the client onto the reason the call ends FAILED with.
func Failure(ctx context.Context, err error) error {
	var ae *APIError
	switch {
	case errors.As(err, &ae):
		switch {
		case ae.Status == http.StatusNotFound:
			return Fail(pluginv1.Reason_REASON_NOT_FOUND)
		case ae.Status == http.StatusUnauthorized || ae.Status == http.StatusForbidden && !rateLimited(ae):
			return Fail(pluginv1.Reason_REASON_PERMISSION_DENIED)
		case ae.Status == http.StatusTooManyRequests || ae.Status == http.StatusForbidden:
			return Fail(pluginv1.Reason_REASON_RATE_LIMITED)
		case ae.Status == http.StatusConflict || ae.Status == http.StatusUnprocessableEntity || ae.Status == http.StatusMethodNotAllowed:
			return Fail(pluginv1.Reason_REASON_CONFLICT)
		case ae.Status >= 500:
			return Fail(pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE)
		}
		return Fail(pluginv1.Reason_REASON_UPSTREAM_ERROR)
	case errors.Is(err, ErrOverLimit):
		return Fail(pluginv1.Reason_REASON_LIMIT_EXCEEDED)
	case errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil:
		return Fail(pluginv1.Reason_REASON_TIMEOUT)
	}
	return Fail(pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE)
}

func rateLimited(e *APIError) bool { return strings.Contains(strings.ToLower(e.Message), "rate limit") }
