package fabricclient

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// contentTypeFixTransport rewrites Content-Type: text/html to application/json
// on 2xx responses whose body is actually valid JSON. The FABRIC orchestrator
// returns this mis-labelled shape on several endpoints; the generated OpenAPI
// client checks Content-Type before parsing and otherwise falls back to its
// "undefined response type" error, losing the body entirely.
type contentTypeFixTransport struct {
	rt http.RoundTripper
}

func (t contentTypeFixTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.rt.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, nil
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/html") {
		return resp, nil
	}
	body, readErr := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); readErr == nil {
		readErr = closeErr
	}
	if readErr != nil {
		// Restore an empty body so the caller doesn't double-close.
		resp.Body = io.NopCloser(bytes.NewReader(nil))
		return resp, readErr
	}
	if json.Valid(body) {
		resp.Header.Set("Content-Type", "application/json")
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return resp, nil
}

// withContentTypeFix wraps c's transport so mislabelled text/html-but-JSON
// orchestrator responses get a Content-Type rewrite before the generated
// client tries to decode them.
func withContentTypeFix(c *http.Client) *http.Client {
	if c == nil {
		c = &http.Client{}
	}
	rt := c.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}
	c.Transport = contentTypeFixTransport{rt: rt}
	return c
}
