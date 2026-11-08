package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
)

// Limits of what the provider reads from the tracker: every answer is untrusted, so it is size-limited and time-bound before it
// is decoded.
const (
	// MaxResponseBytes bounds one HTTP answer; more is LIMIT_EXCEEDED and nothing is decoded.
	MaxResponseBytes = 4 << 20
	maxRequestBytes  = 1 << 20
	callTimeout      = 30 * time.Second
	// apiRoot is the REST API version this provider speaks (Jira Cloud platform REST API, version 3).
	apiRoot = "/rest/api/3"
)

// apiError is an answer of the tracker with a status the call did not expect. It carries the status only: the tracker's
// message is never read, logged or returned.
type apiError struct{ status int }

func (e *apiError) Error() string { return fmt.Sprintf("the tracker answered %d", e.status) }

var (
	errOverLimit = errors.New("the tracker answer is over the limit")
	errBadAnswer = errors.New("the tracker answer is not what was expected")
)

// client is a REST client bound to one instance host. It builds every URL itself, from the host it was given and the path
// segments of the call; it never follows a redirect, never fetches a URL found in an answer and sets no credential header
// (the replay needs none; a live build asks the host for a handle, see the README).
type client struct {
	rt   http.RoundTripper
	host string
}

// do sends one request and decodes a JSON answer into out (nil discards it). A status outside 200 to 299 is an *apiError.
func (c client) do(ctx context.Context, method string, segments []string, query url.Values, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil || len(b) > maxRequestBytes {
			return errBadAnswer
		}
		rd = bytes.NewReader(b)
	}
	if !safeSegments(segments) {
		return errBadAnswer
	}
	u := url.URL{Scheme: "https", Host: c.host}
	u = *u.JoinPath(append([]string{apiRoot}, segments...)...)
	u.RawQuery = query.Encode()
	if u.Host != c.host || u.User != nil {
		return errBadAnswer
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Transport: c.rt, Timeout: callTimeout, CheckRedirect: noRedirect}).Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > MaxResponseBytes {
		return errOverLimit
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &apiError{status: resp.StatusCode}
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return errBadAnswer
		}
	}
	return nil
}

// noRedirect refuses every redirect: a hop to another host would carry whatever the egress proxy injects to a destination the
// manifest never named, and the proxy judges only the first. The 3xx answer itself is then an unexpected status.
func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// failure maps an error of the client onto the call's FAILED reason with its fixed message. readNotFound says that a 403 means
// "may not see it" (a read of an issue), which the port answers exactly like a 404.
func failure(ctx context.Context, err error, readNotFound bool) error {
	var ae *apiError
	switch {
	case errors.As(err, &ae):
		switch {
		case ae.status == http.StatusNotFound || (ae.status == http.StatusForbidden && readNotFound):
			return notFound()
		case ae.status == http.StatusTooManyRequests:
			return fail(pluginv1.Reason_REASON_RATE_LIMITED, msgRateLimited)
		case ae.status == http.StatusRequestTimeout || ae.status >= 500:
			return fail(pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE, msgUnavailable)
		}
		return fail(pluginv1.Reason_REASON_UPSTREAM_ERROR, msgUpstream)
	case errors.Is(err, errOverLimit):
		return fail(pluginv1.Reason_REASON_LIMIT_EXCEEDED, msgOverLimit)
	case errors.Is(err, errBadAnswer):
		return fail(pluginv1.Reason_REASON_UPSTREAM_ERROR, msgUpstream)
	case errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil:
		return fail(pluginv1.Reason_REASON_TIMEOUT, msgTimeout)
	}
	// A transport error, or a replay with no recorded exchange: the tracker could not be reached. Never an empty answer.
	return fail(pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE, msgUnavailable)
}

// refused maps the failure of a write the tracker rejected because of the issue's state (400, 409, 422) to CONFLICT; every
// other failure is mapped by failure.
func refused(ctx context.Context, err error) error {
	var ae *apiError
	if errors.As(err, &ae) && (ae.status == http.StatusBadRequest || ae.status == http.StatusConflict || ae.status == http.StatusUnprocessableEntity) {
		return fail(pluginv1.Reason_REASON_CONFLICT, msgRefused)
	}
	return failure(ctx, err, false)
}

// pathSegmentRe is what a path segment of a request looks like. The grammar already forbids anything else; this is the second
// check at the one place a URL is built, so a segment can never be "..", hold a slash or leave its place in the path.
var pathSegmentRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func safeSegments(segs []string) bool {
	for _, s := range segs {
		if !pathSegmentRe.MatchString(s) {
			return false
		}
	}
	return true
}
