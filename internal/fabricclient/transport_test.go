package fabricclient

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type stubRT struct {
	resp *http.Response
	err  error
}

func (s stubRT) RoundTrip(*http.Request) (*http.Response, error) { return s.resp, s.err }

func makeResp(status int, contentType, body string) *http.Response {
	r := &http.Response{
		StatusCode: status,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	return r
}

func TestFabric_ContentTypeFixTransport(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		resp        *http.Response
		rtErr       error
		wantCT      string
		wantBody    string
		wantPassErr bool
	}{
		{
			name:     "200 text/html with valid JSON gets rewritten",
			resp:     makeResp(200, "text/html", `{"data":[{"slice_id":"abc"}]}`),
			wantCT:   "application/json",
			wantBody: `{"data":[{"slice_id":"abc"}]}`,
		},
		{
			name:     "200 text/html with charset gets rewritten",
			resp:     makeResp(200, "text/html; charset=utf-8", `{"ok":true}`),
			wantCT:   "application/json",
			wantBody: `{"ok":true}`,
		},
		{
			name:     "200 text/html with non-JSON body left alone",
			resp:     makeResp(200, "text/html", `<html><body>Sign in</body></html>`),
			wantCT:   "text/html",
			wantBody: `<html><body>Sign in</body></html>`,
		},
		{
			name:     "200 application/json untouched",
			resp:     makeResp(200, "application/json", `{"ok":true}`),
			wantCT:   "application/json",
			wantBody: `{"ok":true}`,
		},
		{
			name:     "500 text/html JSON left alone (only 2xx rewritten)",
			resp:     makeResp(500, "text/html", `{"err":"x"}`),
			wantCT:   "text/html",
			wantBody: `{"err":"x"}`,
		},
		{
			name:        "transport error propagates",
			resp:        nil,
			rtErr:       errors.New("dial fail"),
			wantPassErr: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rt := contentTypeFixTransport{rt: stubRT{resp: tc.resp, err: tc.rtErr}}
			got, err := rt.RoundTrip(&http.Request{})
			if tc.wantPassErr {
				if err == nil {
					t.Fatalf("err = nil, want propagated error")
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if ct := got.Header.Get("Content-Type"); ct != tc.wantCT {
				t.Fatalf("content-type = %q, want %q", ct, tc.wantCT)
			}
			body, _ := io.ReadAll(got.Body)
			if string(body) != tc.wantBody {
				t.Fatalf("body = %q, want %q", string(body), tc.wantBody)
			}
		})
	}
}

func TestFabric_WithContentTypeFix_NilClient(t *testing.T) {
	t.Parallel()
	c := withContentTypeFix(nil)
	if c == nil {
		t.Fatal("client = nil")
	}
	if _, ok := c.Transport.(contentTypeFixTransport); !ok {
		t.Fatalf("transport = %T, want contentTypeFixTransport", c.Transport)
	}
}
