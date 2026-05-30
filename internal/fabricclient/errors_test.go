package fabricclient

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func responseWithBody(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func genericErr(message string) error {
	return errors.New(message)
}

func TestFabric_MapHTTPErr_401_ReturnsUnauthorized(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
	}{
		{name: "with body", body: `{"errors":[{"details":"token expired"}]}`},
		{name: "empty body", body: ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := mapHTTPErr(responseWithBody(http.StatusUnauthorized, tc.body),
				genericErr("401 Unauthorized"))
			if !errors.Is(err, ErrUnauthorized) {
				t.Fatalf("err = %v, want ErrUnauthorized", err)
			}
			if tc.body != "" && !strings.Contains(err.Error(), "token expired") {
				t.Fatalf("err = %v, want it to contain body", err)
			}
		})
	}
}

func TestFabric_MapHTTPErr_403_ReturnsForbidden(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
	}{
		{name: "with body", body: `{"errors":[{"details":"not a project member"}]}`},
		{name: "empty body", body: ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := mapHTTPErr(responseWithBody(http.StatusForbidden, tc.body),
				genericErr("403 Forbidden"))
			if !errors.Is(err, ErrForbidden) {
				t.Fatalf("err = %v, want ErrForbidden", err)
			}
		})
	}
}

func TestFabric_MapHTTPErr_404_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
	}{
		{name: "with body", body: `{"errors":[{"details":"slice not found"}]}`},
		{name: "empty body", body: ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := mapHTTPErr(responseWithBody(http.StatusNotFound, tc.body),
				genericErr("404 Not Found"))
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("err = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestFabric_MapHTTPErr_400_ReturnsBadRequest(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
	}{
		{name: "with body", body: `{"errors":[{"details":"invalid graphml"}]}`},
		{name: "long body truncated", body: strings.Repeat("x", 500)},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := mapHTTPErr(responseWithBody(http.StatusBadRequest, tc.body),
				genericErr("400 Bad Request"))
			if !errors.Is(err, ErrBadRequest) {
				t.Fatalf("err = %v, want ErrBadRequest", err)
			}
			if len(tc.body) > 300 && !strings.Contains(err.Error(), "(truncated)") {
				t.Fatalf("err = %v, want truncation marker", err)
			}
		})
	}
}

func TestFabric_MapHTTPErr_500_ReturnsServerError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
	}{
		{name: "with body", body: `{"errors":[{"details":"internal error"}]}`},
		{name: "empty body", body: ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := mapHTTPErr(responseWithBody(http.StatusInternalServerError, tc.body),
				genericErr("500 Internal Server Error"))
			if !errors.Is(err, ErrServerError) {
				t.Fatalf("err = %v, want ErrServerError", err)
			}
		})
	}
}

func TestFabric_MapHTTPErr_OtherStatus_IncludesCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		code int
	}{
		{name: "418 teapot", code: http.StatusTeapot},
		{name: "503 unavailable", code: http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := mapHTTPErr(responseWithBody(tc.code, "oops"),
				genericErr("weird status"))
			if err == nil {
				t.Fatal("err = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), http.StatusText(tc.code)) &&
				!strings.Contains(err.Error(), "HTTP") {
				t.Fatalf("err = %v, want HTTP code in message", err)
			}
		})
	}
}

func TestFabric_MapHTTPErr_UndefinedResponseType_ReturnsHelpful(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
	}{
		{name: "plain", err: errors.New("undefined response type")},
		{name: "wrapped", err: errors.New("undefined response type")},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := mapHTTPErr(nil, tc.err)
			if err == nil {
				t.Fatal("err = nil, want non-nil")
			}
			msg := err.Error()
			if !strings.Contains(msg, "orchestrator_url") {
				t.Fatalf("err = %v, want it to mention orchestrator_url", err)
			}
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, should wrap original", err)
			}
		})
	}
}

func TestFabric_MapHTTPErr_UndefinedResponseType_WithHTTPResp_IncludesBody(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		status      int
		contentType string
		body        string
		wantSubstrs []string
	}{
		{
			name:        "html login page on 200",
			status:      200,
			contentType: "text/html; charset=utf-8",
			body:        "<html><body>Please sign in</body></html>",
			wantSubstrs: []string{"HTTP 200", `Content-Type "text/html`, "Please sign in"},
		},
		{
			name:        "plain text 200 with gateway message",
			status:      200,
			contentType: "text/plain",
			body:        "Bad Gateway",
			wantSubstrs: []string{"HTTP 200", "text/plain", "Bad Gateway"},
		},
		{
			name:        "empty body on 200",
			status:      200,
			contentType: "",
			body:        "",
			wantSubstrs: []string{"HTTP 200", "<empty body>"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp := &http.Response{
				StatusCode: tc.status,
				Header:     http.Header{"Content-Type": []string{tc.contentType}},
				Body:       io.NopCloser(strings.NewReader(tc.body)),
			}
			err := mapHTTPErr(resp, genericErr("undefined response type"))
			if err == nil {
				t.Fatal("err = nil, want non-nil")
			}
			msg := err.Error()
			for _, sub := range tc.wantSubstrs {
				if !strings.Contains(msg, sub) {
					t.Fatalf("msg missing %q; full msg:\n%s", sub, msg)
				}
			}
		})
	}
}

func TestFabric_MapHTTPErr_NilResponse_WrapsOriginal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   error
	}{
		{name: "network error", in: errors.New("dial tcp: connection refused")},
		{name: "tls error", in: errors.New("x509: certificate signed by unknown authority")},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := mapHTTPErr(nil, tc.in)
			if err == nil {
				t.Fatal("err = nil, want non-nil")
			}
			if !errors.Is(err, tc.in) {
				t.Fatalf("err = %v, should wrap original %v", err, tc.in)
			}
		})
	}
}

func TestFabric_MapHTTPErr_NilError_ReturnsNil(t *testing.T) {
	t.Parallel()
	if got := mapHTTPErr(&http.Response{StatusCode: 200}, nil); got != nil {
		t.Fatalf("err = %v, want nil", got)
	}
}

func TestFabric_Truncate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{name: "short", in: "abc", n: 5, want: "abc"},
		{name: "exact", in: "abcde", n: 5, want: "abcde"},
		{name: "long", in: "abcdefghij", n: 5, want: "abcde...(truncated)"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := truncate(tc.in, tc.n); got != tc.want {
				t.Fatalf("truncate = %q, want %q", got, tc.want)
			}
		})
	}
}
