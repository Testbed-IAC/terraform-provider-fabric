package fabricclient

import (
	"errors"
	"net/http"
	"testing"
)

func TestFabricClient_MapHTTPErr(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		code int
		want error
	}{
		{name: "not found", code: http.StatusNotFound, want: ErrNotFound},
		{name: "forbidden", code: http.StatusForbidden, want: ErrPermissionDenied},
		{name: "unauthorized", code: http.StatusUnauthorized, want: ErrPermissionDenied},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := mapHTTPErr(&http.Response{StatusCode: tc.code}, errors.New("api"))
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}
