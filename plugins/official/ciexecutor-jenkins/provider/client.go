package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// callTimeout bounds one HTTP exchange; the request context may be shorter.
const callTimeout = 30 * time.Second

// client is the HTTP client of one call. It sends the credential handle in ghprovider.HandleHeader and nothing else that
// authenticates: the Jenkins user name and API token live in the egress proxy, which injects them for the handle.
type client struct {
	rt     http.RoundTripper
	handle string
	cfg    Config
}

type reply struct {
	status int
	header http.Header
}

// request is one exchange. query goes in the URL, form in a urlencoded body, header carries the crumb.
type request struct {
	method, path string
	query, form  url.Values
	header       map[string]string
	// okRedirect accepts a 3xx answer as success (Jenkins answers a queue cancel and a build stop with a redirect).
	okRedirect bool
}

// errBadAnswer is a 2xx answer that is not the JSON expected: it breaks the grammar of the API, so it is an upstream error and
// not an outage.
var errBadAnswer = errors.New("the answer of Jenkins is not the JSON expected")

func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// do sends the request and decodes a JSON answer into out (nil to discard it). A status of 400 or above, or a 3xx the caller did
// not expect, is a *ghprovider.APIError; an answer over ghprovider.MaxResponseBytes is ghprovider.ErrOverLimit and nothing of
// it is decoded. The body of an error answer is never kept: it is Jenkins's HTML, untrusted and not needed.
func (c client) do(ctx context.Context, r request, out any) (reply, error) {
	var body io.Reader
	if r.form != nil {
		enc := r.form.Encode()
		if len(enc) > ghprovider.MaxRequestBytes {
			return reply{}, errors.New("the request body is over its bound")
		}
		body = strings.NewReader(enc)
	}
	target := "https://" + c.cfg.Authority() + c.cfg.Prefix + r.path
	if len(r.query) > 0 {
		target += "?" + r.query.Encode()
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, r.method, target, body)
	if err != nil {
		return reply{}, err
	}
	req.Header.Set("Accept", "application/json")
	if r.form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range r.header {
		req.Header.Set(k, v)
	}
	if c.handle != "" {
		req.Header.Set(ghprovider.HandleHeader, c.handle)
	}
	rt := c.rt
	if rt == nil {
		rt = http.DefaultTransport
	}
	resp, err := (&http.Client{Transport: rt, Timeout: callTimeout, CheckRedirect: noRedirect}).Do(req)
	if err != nil {
		return reply{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, ghprovider.MaxResponseBytes+1))
	if err != nil {
		return reply{status: resp.StatusCode}, err
	}
	rep := reply{status: resp.StatusCode, header: resp.Header}
	if len(raw) > ghprovider.MaxResponseBytes {
		return rep, ghprovider.ErrOverLimit
	}
	if resp.StatusCode >= 400 || (resp.StatusCode >= 300 && !r.okRedirect) {
		return rep, &ghprovider.APIError{Status: resp.StatusCode}
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return rep, fmt.Errorf("%w: %w", errBadAnswer, err)
		}
	}
	return rep, nil
}

// failure maps an error of the client onto the reason the call ends FAILED with. A 400 from Jenkins is a job that does not
// take what was sent (not parameterized, a parameter it does not know), which is the port's FAILED_PRECONDITION (CONFLICT).
func failure(ctx context.Context, err error) error {
	var ae *ghprovider.APIError
	if errors.Is(err, errBadAnswer) {
		return upstream()
	}
	if errors.As(err, &ae) && ae.Status == http.StatusBadRequest {
		return ghprovider.Failure(ctx, &ghprovider.APIError{Status: http.StatusConflict})
	}
	return ghprovider.Failure(ctx, err)
}

// isStatus reports whether err is an answer with the given status.
func isStatus(err error, status int) bool {
	var ae *ghprovider.APIError
	return errors.As(err, &ae) && ae.Status == status
}
